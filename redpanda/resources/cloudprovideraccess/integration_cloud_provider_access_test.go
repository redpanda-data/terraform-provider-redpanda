//go:build integration

// Copyright 2026 Redpanda Data, Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package cloudprovideraccess_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock/fakes"
	"google.golang.org/grpc/codes"
)

const (
	cpaAddr     = "redpanda_cloud_provider_access.test"
	roleARN     = "arn:aws:iam::123456789012:role/tfrp-mock-provisioner"
	otherARN    = "arn:aws:iam::123456789012:role/tfrp-mock-provisioner-2"
	cpaDataAddr = "data.redpanda_cloud_provider_access.test"
	prereqAddr  = "data.redpanda_cloud_provider_access_prerequisites.test"
)

func cpaConfig(name, arn string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_cloud_provider_access" "test" {
  name           = %q
  cloud_provider = "aws"
  aws = {
    role_arn = %q
  }
}
`, name, arn)
}

func cpaStateChecks(name, arn string, extra ...statecheck.StateCheck) []statecheck.StateCheck {
	return append([]statecheck.StateCheck{
		statecheck.ExpectKnownValue(cpaAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
		statecheck.ExpectKnownValue(cpaAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
		statecheck.ExpectKnownValue(cpaAddr, tfjsonpath.New("aws").AtMapKey("role_arn"), knownvalue.StringExact(arn)),
		statecheck.ExpectKnownValue(cpaAddr, tfjsonpath.New("aws").AtMapKey("external_id"), knownvalue.StringExact(fakes.FakeOrgID)),
		statecheck.ExpectKnownValue(cpaAddr, tfjsonpath.New("state"), knownvalue.StringExact("ACTIVE")),
		statecheck.ExpectKnownValue(cpaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
	}, extra...)
}

// TestIntegration_CloudProviderAccess walks the lifecycle of a resource with
// no update RPC: create, a no-op re-plan that keeps the id, a replacement for
// each configurable leaf that yields a new id, and an import round trip. The
// server-set external_id must be accepted again on every replacement create.
func TestIntegration_CloudProviderAccess(t *testing.T) {
	_, factories := integration.Setup(t)

	idStable := statecheck.CompareValue(compare.ValuesSame())
	idReplaced := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(cpaAddr, cpaConfig("tfrp-mock-cpa", roleARN), cpaStateChecks("tfrp-mock-cpa", roleARN,
				idStable.AddStateValue(cpaAddr, tfjsonpath.New("id")),
				idReplaced.AddStateValue(cpaAddr, tfjsonpath.New("id")),
			)),
			integration.NoopReapplyStep(cpaAddr, cpaConfig("tfrp-mock-cpa", roleARN), cpaStateChecks("tfrp-mock-cpa", roleARN,
				idStable.AddStateValue(cpaAddr, tfjsonpath.New("id")),
			)),
			integration.RequiresReplaceStep(cpaAddr, cpaConfig("tfrp-mock-cpa", otherARN), cpaStateChecks("tfrp-mock-cpa", otherARN,
				idReplaced.AddStateValue(cpaAddr, tfjsonpath.New("id")),
			)),
			integration.RequiresReplaceStep(cpaAddr, cpaConfig("tfrp-mock-cpa-renamed", otherARN), cpaStateChecks("tfrp-mock-cpa-renamed", otherARN,
				idReplaced.AddStateValue(cpaAddr, tfjsonpath.New("id")),
			)),
			integration.ImportRoundTripStep(cpaAddr, nil, nil),
		},
	})
}

// TestIntegration_CloudProviderAccess_DeletedOutOfBand covers drift: an access
// deleted outside Terraform reads as NotFound, leaves state, and is planned
// for creation again.
func TestIntegration_CloudProviderAccess_DeletedOutOfBand(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := cpaConfig("tfrp-mock-cpa-drift", roleARN)
	var id string

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			func() resource.TestStep {
				s := integration.CreateStep(cpaAddr, cfg, cpaStateChecks("tfrp-mock-cpa-drift", roleARN))
				s.Check = captureID(&id)
				return s
			}(),
			{
				PreConfig: func() {
					if !srv.CloudProviderAccess.DeleteOutOfBand(id) {
						t.Fatalf("PreConfig: no cloud provider access %q to delete", id)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(cpaAddr, plancheck.ResourceActionCreate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: cpaStateChecks("tfrp-mock-cpa-drift", roleARN),
			},
		},
	})
}

// TestIntegration_CloudProviderAccess_DeleteWhileReferenced pins that the
// control plane's refusal to delete an access a network still uses surfaces
// as a destroy error naming the create_before_destroy fix, and that the access
// stays in state until the network is gone.
func TestIntegration_CloudProviderAccess_DeleteWhileReferenced(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := cpaConfig("tfrp-mock-cpa-inuse", roleARN)
	var id, networkID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			func() resource.TestStep {
				s := integration.CreateStep(cpaAddr, cfg, cpaStateChecks("tfrp-mock-cpa-inuse", roleARN))
				s.Check = captureID(&id)
				return s
			}(),
			{
				PreConfig: func() {
					op, err := srv.Network.CreateNetwork(context.Background(), &controlplanev1.CreateNetworkRequest{
						Network: &controlplanev1.NetworkCreate{
							Name:                  "tfrp-mock-net-oob",
							ResourceGroupId:       "00000000-0000-0000-0000-000000000000",
							CloudProvider:         controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
							Region:                "us-east-1",
							CidrBlock:             "10.0.0.0/20",
							ClusterType:           controlplanev1.Cluster_TYPE_BYOC,
							CloudProviderAccessId: id,
						},
					})
					if err != nil {
						t.Fatalf("PreConfig: create referencing network: %v", err)
					}
					networkID = op.GetOperation().GetResourceId()
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`(?s)FailedPrecondition.*create_before_destroy`),
			},
			{
				PreConfig: func() {
					if _, err := srv.Network.DeleteNetwork(context.Background(), &controlplanev1.DeleteNetworkRequest{Id: networkID}); err != nil {
						t.Fatalf("PreConfig: delete referencing network: %v", err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(cpaAddr, plancheck.ResourceActionNoop)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestIntegration_CloudProviderAccess_CreateAlreadyExists surfaces the
// control plane's per-organization name uniqueness as a create error.
func TestIntegration_CloudProviderAccess_CreateAlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.CloudProviderAccessService_CreateCloudProviderAccess_FullMethodName,
				codes.AlreadyExists,
				cpaConfig("tfrp-mock-cpa-dup", roleARN),
				"already exists",
			),
		},
	})
}

// TestIntegration_CloudProviderAccess_PlanTimeValidation pins the rules the
// provider rejects before apply: only aws, a present aws block, the name
// pattern, and the IAM role ARN shape.
func TestIntegration_CloudProviderAccess_PlanTimeValidation(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"gcp provider": {
			body: `name = "tfrp-mock-cpa"
  cloud_provider = "gcp"
  aws = { role_arn = "` + roleARN + `" }`,
			want: `value must be one of: \["aws"\]`,
		},
		"missing aws block": {
			body: `name = "tfrp-mock-cpa"
  cloud_provider = "aws"`,
			want: `The argument "aws" is required`,
		},
		"invalid name": {
			body: `name = "tfrp/mock"
  cloud_provider = "aws"
  aws = { role_arn = "` + roleARN + `" }`,
			want: `does not match regex pattern`,
		},
		"not a role arn": {
			body: `name = "tfrp-mock-cpa"
  cloud_provider = "aws"
  aws = { role_arn = "arn:aws:iam::123456789012:user/tfrp-mock" }`,
			want: `does not match regex pattern`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, factories := integration.Setup(t)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{{
					Config: fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_cloud_provider_access" "test" {
  %s
}
`, tc.body),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}

// TestIntegration_CloudProviderAccess_DataSources reads an access back through
// redpanda_cloud_provider_access and the trust-policy values through
// redpanda_cloud_provider_access_prerequisites.
func TestIntegration_CloudProviderAccess_DataSources(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := cpaConfig("tfrp-mock-cpa-ds", roleARN) + `
data "redpanda_cloud_provider_access" "test" {
  id = redpanda_cloud_provider_access.test.id
}

data "redpanda_cloud_provider_access_prerequisites" "test" {
  cloud_provider = "aws"
}
`
	idMatches := statecheck.CompareValuePairs(cpaAddr, tfjsonpath.New("id"), cpaDataAddr, tfjsonpath.New("id"), compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(cpaAddr, cfg, []statecheck.StateCheck{
				idMatches,
				statecheck.ExpectKnownValue(cpaDataAddr, tfjsonpath.New("name"), knownvalue.StringExact("tfrp-mock-cpa-ds")),
				statecheck.ExpectKnownValue(cpaDataAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(cpaDataAddr, tfjsonpath.New("aws").AtMapKey("role_arn"), knownvalue.StringExact(roleARN)),
				statecheck.ExpectKnownValue(cpaDataAddr, tfjsonpath.New("aws").AtMapKey("external_id"), knownvalue.StringExact(fakes.FakeOrgID)),
				statecheck.ExpectKnownValue(cpaDataAddr, tfjsonpath.New("state"), knownvalue.StringExact("ACTIVE")),
				statecheck.ExpectKnownValue(prereqAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(prereqAddr, tfjsonpath.New("aws").AtMapKey("principal_arn"), knownvalue.StringExact(fakes.FakeCloudProviderAccessPrincipalARN)),
				statecheck.ExpectKnownValue(prereqAddr, tfjsonpath.New("aws").AtMapKey("external_id"), knownvalue.StringExact(fakes.FakeOrgID)),
			}),
		},
	})
}

// TestIntegration_CloudProviderAccess_DataSourceNotFound surfaces a missing
// access as a read error rather than an empty result.
func TestIntegration_CloudProviderAccess_DataSourceNotFound(t *testing.T) {
	_, factories := integration.Setup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `
provider "redpanda" {}

data "redpanda_cloud_provider_access" "test" {
  id = "aaaaaaaaaaaaaaaaaaaa"
}
`,
			ExpectError: regexp.MustCompile(`not found`),
		}},
	})
}

func captureID(dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[cpaAddr]
		if !ok {
			return fmt.Errorf("%s not in state", cpaAddr)
		}
		*dst = rs.Primary.ID
		return nil
	}
}
