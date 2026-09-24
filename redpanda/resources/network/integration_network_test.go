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

package network_test

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const networkAddr = "redpanda_network.test"

// TestIntegration_Network exercises redpanda_network end-to-end against the
// bufconn-backed fake controlplane. Network has no Update RPC (every
// configurable field is RequiresReplace), so the scenario is: minimal
// create → no-op re-plan → refresh → name-rename triggers
// destroy-before-create.
func TestIntegration_Network(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	initialName := "tfrp-mock-net-initial"
	renamedName := "tfrp-mock-net-renamed"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockNetworkConfig(initialName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(networkAddr, "name", initialName),
					resource.TestCheckResourceAttr(networkAddr, "cloud_provider", "aws"),
					resource.TestCheckResourceAttr(networkAddr, "region", "us-east-1"),
					resource.TestCheckResourceAttr(networkAddr, "cluster_type", "dedicated"),
					resource.TestCheckResourceAttrSet(networkAddr, "id"),
				),
			},
			{
				Config: mockNetworkConfig(initialName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(networkAddr, "name", initialName),
					resource.TestCheckResourceAttrSet(networkAddr, "id"),
				),
			},
			{
				Config: mockNetworkConfig(renamedName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(networkAddr, "name", renamedName),
			},
		},
	})
}

func mockNetworkConfig(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}
`, name)
}

// inPlaceConfig builds an in-place (cidr_block) variant network HCL. All
// parameters are RequiresReplace fields; tests mutate one across steps to
// drive the corresponding RR scenario.
func inPlaceConfig(name, cloudProvider, clusterType, region, cidr string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = %q
  region            = %q
  cluster_type      = %q
  cidr_block        = %q
}
`, name, cloudProvider, region, clusterType, cidr)
}

// rrRGConfig declares two resource_groups and binds the network to one of
// them by label ("rg1" or "rg2"). Used by the RequiresReplace_ResourceGroupID
// scenario to mutate resource_group_id without re-using a single rg.
func rrRGConfig(netName, rgLabel string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "rg1" {
  name = "tfrp-mock-net-rg-1"
}

resource "redpanda_resource_group" "rg2" {
  name = "tfrp-mock-net-rg-2"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.%s.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}
`, netName, rgLabel)
}

// byovpcAWSConfig builds a BYOVPC (AWS customer_managed_resources) variant.
func byovpcAWSConfig(name, mgmtBucketARN string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    aws = {
      management_bucket = {
        arn = %q
      }
      dynamodb_table = {
        arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb"
      }
      vpc = {
        arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a"
      }
      private_subnets = {
        arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"]
      }
    }
  }
}
`, name, mgmtBucketARN)
}

// byovpcGCPConfig builds a BYOVPC (GCP customer_managed_resources) variant.
func byovpcGCPConfig(name, networkName string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    gcp = {
      network_name       = %q
      network_project_id = "tfrp-bv-proj"
      management_bucket = {
        name = "tfrp-bv-bkt"
      }
    }
  }
}
`, name, networkName)
}

// TestIntegration_Network_CreateAndRefresh_InPlace validates the Create + no-op
// cycle for the in-place (cidr_block) variant. Asserts every in-place leaf
// and explicitly asserts customer_managed_resources is Null, the inverse
// variant proof that pins the variant partitioning at the state level, not
// only in the HCL. id is captured across both steps via a shared
// CompareValue(ValuesSame()), the load-bearing proof that UseStateForUnknown
// preserves id across the noop.
func TestIntegration_Network_CreateAndRefresh_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-ip-create"
	cfg := inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20")

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("dedicated")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.StringExact("10.0.0.0/20")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("customer_managed_resources"), knownvalue.Null()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider_access_id"), knownvalue.Null()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("zones"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("use1-az1"),
				})),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("dedicated")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.StringExact("10.0.0.0/20")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("customer_managed_resources"), knownvalue.Null()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_Name_InPlace mutates `name` and asserts
// the framework plans DestroyBeforeCreate. The load-bearing proof of an
// actual destroy-and-recreate (rather than an unobservable in-place tweak)
// is that the server-assigned id DIFFERS across steps: a shared
// CompareValue(ValuesDiffer()) captures pre- and post-replace ids.
func TestIntegration_Network_RequiresReplace_Name_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		nameA = "tfrp-mock-net-rr-name-a"
		nameB = "tfrp-mock-net-rr-name-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				inPlaceConfig(nameA, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(networkAddr,
				inPlaceConfig(nameB, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameB)),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_CloudProvider_InPlace mutates
// `cloud_provider` from "aws" to "gcp" and asserts DestroyBeforeCreate.
func TestIntegration_Network_RequiresReplace_CloudProvider_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rr-cp"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(networkAddr,
				inPlaceConfig(name, "gcp", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("gcp")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_ClusterType_InPlace mutates `cluster_type`
// from "dedicated" to "byoc" (without switching to BYOVPC: cidr_block stays
// set; only the cluster_type enum changes) and asserts DestroyBeforeCreate.
func TestIntegration_Network_RequiresReplace_ClusterType_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rr-ct"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("dedicated")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(networkAddr,
				inPlaceConfig(name, "aws", "byoc", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_Region_InPlace mutates `region` from
// "us-east-1" to "us-west-2" and asserts DestroyBeforeCreate.
func TestIntegration_Network_RequiresReplace_Region_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rr-reg"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-west-2", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-west-2")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_ResourceGroupID_InPlace switches the
// network's resource_group_id between two declared resource_groups, asserting
// the framework plans DestroyBeforeCreate when the rg target changes.
func TestIntegration_Network_RequiresReplace_ResourceGroupID_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rr-rg"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, rrRGConfig(name, "rg1"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, rrRGConfig(name, "rg2"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_CidrBlock_InPlace mutates `cidr_block`
// from "10.0.0.0/20" to "10.1.0.0/20" and asserts DestroyBeforeCreate.
func TestIntegration_Network_RequiresReplace_CidrBlock_InPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rr-cidr"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.StringExact("10.0.0.0/20")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.1.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.StringExact("10.1.0.0/20")),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Network_CreateAndRefresh_BYOVPC validates the Create + no-op cycle
// for the BYOVPC (customer_managed_resources.aws) variant. The symmetric
// inverse-variant proof asserts cidr_block is Null in state, pinning the
// variant partitioning at the state level, not only in the HCL.
func TestIntegration_Network_CreateAndRefresh_BYOVPC(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name      = "tfrp-mock-net-bv-create"
		bucketARN = "arn:aws:s3:::tfrp-bv-mgmt"
	)
	cfg := byovpcAWSConfig(name, bucketARN)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.Null()),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("management_bucket").AtMapKey("arn"),
					knownvalue.StringExact(bucketARN)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("dynamodb_table").AtMapKey("arn"),
					knownvalue.StringExact("arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb")),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("vpc").AtMapKey("arn"),
					knownvalue.StringExact("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a")),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("private_subnets").AtMapKey("arns"),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"),
					})),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.Null()),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("management_bucket").AtMapKey("arn"),
					knownvalue.StringExact(bucketARN)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_CMR_AWS mutates the management_bucket.arn
// sub-field of the AWS customer_managed_resources and asserts
// DestroyBeforeCreate. Both customer_managed_resources.aws (the object) and
// its parent block carry RequiresReplace, so any sub-field change destroys
// the network.
func TestIntegration_Network_RequiresReplace_CMR_AWS(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name       = "tfrp-mock-net-rr-cmr-aws"
		bucketARNA = "arn:aws:s3:::tfrp-bv-mgmt-a"
		bucketARNB = "arn:aws:s3:::tfrp-bv-mgmt-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSConfig(name, bucketARNA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("management_bucket").AtMapKey("arn"),
					knownvalue.StringExact(bucketARNA)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, byovpcAWSConfig(name, bucketARNB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("management_bucket").AtMapKey("arn"),
					knownvalue.StringExact(bucketARNB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_CMR_GCP mutates the network_name sub-field
// of the GCP customer_managed_resources and asserts DestroyBeforeCreate.
func TestIntegration_Network_RequiresReplace_CMR_GCP(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name         = "tfrp-mock-net-rr-cmr-gcp"
		networkNameA = "tfrp-bv-net"
		networkNameB = "tfrp-bv-net-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcGCPConfig(name, networkNameA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("network_name"),
					knownvalue.StringExact(networkNameA)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("network_project_id"),
					knownvalue.StringExact("tfrp-bv-proj")),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("management_bucket").AtMapKey("name"),
					knownvalue.StringExact("tfrp-bv-bkt")),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.Null()),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, byovpcGCPConfig(name, networkNameB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("network_name"),
					knownvalue.StringExact(networkNameB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// tgwAWSConfig builds an AWS network variant with a Transit Gateway egress_spec.
func tgwAWSConfig(name, tgwID string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
  egress_spec = {
    aws = {
      transit_gateway_id = %q
    }
  }
}
`, name, tgwID)
}

// TestIntegration_Network_CreateAndRefresh_TGW validates the Create + no-op cycle
// for the AWS Transit Gateway egress_spec variant.
func TestIntegration_Network_CreateAndRefresh_TGW(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name  = "tfrp-mock-net-tgw-create"
		tgwID = "tgw-0123456789abcdef0"
	)
	cfg := tgwAWSConfig(name, tgwID)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("aws").AtMapKey("transit_gateway_id"),
					knownvalue.StringExact(tgwID)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("aws").AtMapKey("transit_gateway_id"),
					knownvalue.StringExact(tgwID)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_TGW mutates egress_spec.aws.transit_gateway_id
// and asserts DestroyBeforeCreate. egress_spec.aws carries RequiresReplace since
// Network has no Update RPC.
func TestIntegration_Network_RequiresReplace_TGW(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name   = "tfrp-mock-net-rr-tgw"
		tgwIDA = "tgw-0123456789abcdef0"
		tgwIDB = "tgw-fedcba9876543210f"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, tgwAWSConfig(name, tgwIDA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("aws").AtMapKey("transit_gateway_id"),
					knownvalue.StringExact(tgwIDA)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, tgwAWSConfig(name, tgwIDB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("aws").AtMapKey("transit_gateway_id"),
					knownvalue.StringExact(tgwIDB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// hubEgressAzureConfig builds an Azure network variant with a hub-VNet egress_spec.
func hubEgressAzureConfig(name, hubVnetID, firewallIP string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "azure"
  region            = "westus2"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
  egress_spec = {
    azure = {
      hub_vnet_id         = %q
      firewall_private_ip = %q
    }
  }
}
`, name, hubVnetID, firewallIP)
}

// TestIntegration_Network_CreateAndRefresh_AzureHubEgress validates the Create +
// no-op cycle for the Azure hub-VNet egress_spec variant.
func TestIntegration_Network_CreateAndRefresh_AzureHubEgress(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name       = "tfrp-mock-net-hub-create"
		hubVnetID  = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/hub-vnet"
		firewallIP = "10.1.0.4"
	)
	cfg := hubEgressAzureConfig(name, hubVnetID, firewallIP)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("azure").AtMapKey("hub_vnet_id"),
					knownvalue.StringExact(hubVnetID)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("azure").AtMapKey("firewall_private_ip"),
					knownvalue.StringExact(firewallIP)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("azure").AtMapKey("hub_vnet_id"),
					knownvalue.StringExact(hubVnetID)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_AzureHubEgress mutates
// egress_spec.azure.firewall_private_ip and asserts DestroyBeforeCreate.
// egress_spec.azure carries RequiresReplace: egress is on the update shape,
// but in-place egress changes are unverified against the data plane.
func TestIntegration_Network_RequiresReplace_AzureHubEgress(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-net-rr-hub"
		hubVnetID   = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/hub-vnet"
		firewallIPA = "10.1.0.4"
		firewallIPB = "10.1.0.5"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, hubEgressAzureConfig(name, hubVnetID, firewallIPA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("azure").AtMapKey("firewall_private_ip"),
					knownvalue.StringExact(firewallIPA)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, hubEgressAzureConfig(name, hubVnetID, firewallIPB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("azure").AtMapKey("firewall_private_ip"),
					knownvalue.StringExact(firewallIPB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// hubEgressGCPConfig builds a GCP network variant with a VPC Hub egress_spec.
func hubEgressGCPConfig(name, hubProject, hubName string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
  egress_spec = {
    gcp = {
      hub_vpc_project = %q
      hub_vpc_name    = %q
    }
  }
}
`, name, hubProject, hubName)
}

// TestIntegration_Network_CreateAndRefresh_GCPHubEgress validates the Create +
// no-op cycle for the GCP VPC Hub egress_spec variant.
func TestIntegration_Network_CreateAndRefresh_GCPHubEgress(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name       = "tfrp-mock-net-hub-create"
		hubProject = "tfrp-hub-proj"
		hubName    = "tfrp-hub-vpc"
	)
	cfg := hubEgressGCPConfig(name, hubProject, hubName)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("gcp").AtMapKey("hub_vpc_project"),
					knownvalue.StringExact(hubProject)),
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("gcp").AtMapKey("hub_vpc_name"),
					knownvalue.StringExact(hubName)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("gcp").AtMapKey("hub_vpc_project"),
					knownvalue.StringExact(hubProject)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_RequiresReplace_GCPHubEgress mutates
// egress_spec.gcp.hub_vpc_name and asserts DestroyBeforeCreate.
// egress_spec.gcp carries RequiresReplace since Network has no Update RPC.
func TestIntegration_Network_RequiresReplace_GCPHubEgress(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name       = "tfrp-mock-net-rr-hub"
		hubProject = "tfrp-hub-proj"
		hubNameA   = "tfrp-hub-vpc-a"
		hubNameB   = "tfrp-hub-vpc-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, hubEgressGCPConfig(name, hubProject, hubNameA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("gcp").AtMapKey("hub_vpc_name"),
					knownvalue.StringExact(hubNameA)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, hubEgressGCPConfig(name, hubProject, hubNameB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr,
					tfjsonpath.New("egress_spec").AtMapKey("gcp").AtMapKey("hub_vpc_name"),
					knownvalue.StringExact(hubNameB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_ImportRoundTrip exercises the bearer-id import path.
// Network's ImportState uses ImportStatePassthroughID on the "id" attribute,
// so the import id is the xid-like string assigned at Create. Network's
// ImportState does NOT call ClusterForID: no live-controlplane-lookup risk.
// nil idFunc tells the helper to use the bearer "id" from prior state; nil
// verifyIgnore means every attribute must roundtrip identically.
func TestIntegration_Network_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name  = "tfrp-mock-net-import"
		tgwID = "tgw-0123456789abcdef0"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				tgwAWSConfig(name, tgwID),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(networkAddr,
						tfjsonpath.New("egress_spec").AtMapKey("aws").AtMapKey("transit_gateway_id"),
						knownvalue.StringExact(tgwID)),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				}),
			integration.ImportRoundTripStep(networkAddr, nil, nil),
		},
	})
}

// TestIntegration_NetworkDataSource_Read covers the network datasource against
// the fake, including the egress_spec mirror: the datasource must surface what
// the resource created.
func TestIntegration_NetworkDataSource_Read(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name    = "tfrp-mock-net-ds"
		tgwID   = "tgw-0123456789abcdef0"
		dsAddr  = "data.redpanda_network.test"
		dsBlock = `
data "redpanda_network" "test" {
  id = redpanda_network.test.id
}
`
	)
	cfg := tgwAWSConfig(name, tgwID) + dsBlock

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(dsAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(dsAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
					statecheck.ExpectKnownValue(dsAddr, tfjsonpath.New("cloud_provider_access_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue(dsAddr,
						tfjsonpath.New("egress_spec").AtMapKey("aws").AtMapKey("transit_gateway_id"),
						knownvalue.StringExact(tgwID)),
				},
			},
		},
	})
}

// TestIntegration_Network_ErrorPath_GetNotFound covers the Read→NotFound path.
// After a successful Create, an OverrideOnce on GetNetwork is injected so
// the next Read (which fires during the second step's plan) gets NotFound.
// The provider's Read sees NotFound via utils.IsNotFound (string match catches
// the wrapped error) and calls RemoveResource. The next plan sees the
// resource missing from state → re-Create.
func TestIntegration_Network_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-net-notfound"
	cfg := inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.NetworkService_GetNetwork_FullMethodName,
						status.Error(codes.NotFound, "network not found"),
					)
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_Network_ErrorPath_CreateAlreadyExists injects AlreadyExists on
// CreateNetwork. The provider's Create surfaces the gRPC error as a
// diagnostic; ExpectError matches the regexp against the diagnostic text.
// Network's Create has no AlreadyExists adoption path (unlike user), so the
// error surfaces directly.
func TestIntegration_Network_ErrorPath_CreateAlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-net-exists"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.NetworkService_CreateNetwork_FullMethodName,
				codes.AlreadyExists,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				"already exists",
			),
		},
	})
}

// TestIntegration_Network_ErrorPath_EgressArmMismatch pins the control plane's
// CEL rule that the egress_spec arm must match the network's cloud_provider
// (network.proto: egress_spec.matches_cloud_provider). The generated
// protovalidate pass rejects it client-side with the proto's own message; the
// fake mirrors the control-plane rejection as backstop.
func TestIntegration_Network_ErrorPath_EgressArmMismatch(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-net-egress-mismatch"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
  egress_spec = {
    aws = {
      transit_gateway_id = "tgw-0123456789abcdef0"
    }
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile("egress_spec.cloud_provider must match cloud_provider"),
			},
		},
	})
}

// TestIntegration_Network_ErrorPath_EgressEmpty pins rejection of an
// egress_spec block with no arm set. The CEL matches_cloud_provider rule
// covers it (no arm ever matches cloud_provider), surfacing through the same
// protovalidate path; the oneof-required rule backs it at the control plane.
func TestIntegration_Network_ErrorPath_EgressEmpty(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := `
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-net-egress-empty"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
  egress_spec       = {}
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile("egress_spec.cloud_provider must match cloud_provider"),
			},
		},
	})
}

// TestIntegration_Network_ErrorPath_DeleteFailed covers the destroy-failed path.
// After a successful Create, an Internal-coded error is injected on the next
// DeleteNetwork RPC. The Destroy:true step triggers the destroy plan;
// ExpectError matches the error regexp. codes.Internal is non-retryable so
// the override fires once and the diagnostic surfaces immediately.
func TestIntegration_Network_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-net-delfail"
	cfg := inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.NetworkService_DeleteNetwork_FullMethodName,
						status.Error(codes.Internal, "synthetic delete failure"),
					)
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("synthetic delete failure"),
			},
		},
	})
}

// cpaNetworkConfig builds a BYOC AWS network and the cloud provider accesses
// in accesses, keyed by resource label with the role ARN as value. The network
// uses the access labelled network, or none when network is "". cbd sets
// create_before_destroy on every access.
func cpaNetworkConfig(name, network string, accesses map[string]string, cbd bool) string {
	labels := slices.Sorted(maps.Keys(accesses))
	lifecycle := ""
	if cbd {
		lifecycle = "lifecycle {\n    create_before_destroy = true\n  }"
	}
	var b strings.Builder
	b.WriteString(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}
`)
	for _, label := range labels {
		fmt.Fprintf(&b, `
resource "redpanda_cloud_provider_access" %q {
  name           = %q
  cloud_provider = "aws"
  aws = {
    role_arn = %q
  }
  %s
}
`, label, cpaName(label, accesses[label]), accesses[label], lifecycle)
	}
	access := ""
	if network != "" {
		access = "cloud_provider_access_id = redpanda_cloud_provider_access." + network + ".id"
	}
	fmt.Fprintf(&b, `
resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "byoc"
  cidr_block        = "10.0.0.0/20"
  %s
}

data "redpanda_network" "test" {
  id = redpanda_network.test.id
}
`, name, access)
	return b.String()
}

// cpaName derives an access name from its role, so a block whose role
// changes also gets a new name, as the per-organization name uniqueness
// requires of a create_before_destroy replacement.
func cpaName(_, roleARN string) string {
	return "cpa-" + roleARN[strings.LastIndex(roleARN, "/")+1:]
}

const (
	roleA      = "arn:aws:iam::123456789012:role/tfrp-mock-a"
	roleB      = "arn:aws:iam::123456789012:role/tfrp-mock-b"
	roleOtherA = "arn:aws:iam::123456789012:role/tfrp-mock-a-rotated"
	roleOther  = "arn:aws:iam::210987654321:role/tfrp-mock-other-account"
)

func cpaNetworkChecks(name, access string, extra ...statecheck.StateCheck) []statecheck.StateCheck {
	cpa := "redpanda_cloud_provider_access." + access
	return append([]statecheck.StateCheck{
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.StringExact("10.0.0.0/20")),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("customer_managed_resources"), knownvalue.Null()),
		statecheck.CompareValuePairs(networkAddr, tfjsonpath.New("cloud_provider_access_id"), cpa, tfjsonpath.New("id"), compare.ValuesSame()),
		statecheck.CompareValuePairs("data.redpanda_network.test", tfjsonpath.New("cloud_provider_access_id"), cpa, tfjsonpath.New("id"), compare.ValuesSame()),
	}, extra...)
}

// TestIntegration_Network_CloudProviderAccess covers a network provisioned
// through a cloud provider access: the id echoes into the resource and the
// datasource, survives a no-op re-plan and an import, and re-pointing the
// network at another access updates it in place, keeping the network id.
func TestIntegration_Network_CloudProviderAccess(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-cpa"
	both := map[string]string{"a": roleA, "b": roleB}
	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cpaNetworkConfig(name, "a", both, false), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
			integration.NoopReapplyStep(networkAddr, cpaNetworkConfig(name, "a", both, false), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
			integration.ImportRoundTripStep(networkAddr, nil, []string{"timeouts"}),
			integration.UpdateLeafStep(networkAddr, cpaNetworkConfig(name, "b", both, false), cpaNetworkChecks(name, "b",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
			integration.UpdateLeafStep(networkAddr, cpaNetworkConfig(name, "a", both, false), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
		},
	})
}

// TestIntegration_Network_CloudProviderAccess_RotateInOneApply pins role
// rotation with a second access block: one config change adds the new access,
// re-points the network and removes the old access. With create_before_destroy
// recorded on the old access, Terraform updates the network before deleting
// it, which the control plane requires; the network keeps its id.
func TestIntegration_Network_CloudProviderAccess_RotateInOneApply(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rotate"
	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cpaNetworkConfig(name, "a", map[string]string{"a": roleA}, true), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
			{
				Config: cpaNetworkConfig(name, "b", map[string]string{"b": roleB}, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("redpanda_cloud_provider_access.a", plancheck.ResourceActionDestroy),
						plancheck.ExpectResourceAction("redpanda_cloud_provider_access.b", plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: cpaNetworkChecks(name, "b",
					idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
				),
			},
		},
	})
}

// TestIntegration_Network_CloudProviderAccess_RotateInPlace pins role rotation
// by editing the access block itself: with create_before_destroy, the new
// access is created under its new name, the network is re-pointed in place and
// the old access is deleted last, all in one apply.
func TestIntegration_Network_CloudProviderAccess_RotateInPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-rotate-inplace"
	idStable := statecheck.CompareValue(compare.ValuesSame())
	accessReplaced := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cpaNetworkConfig(name, "a", map[string]string{"a": roleA}, true), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
				accessReplaced.AddStateValue("redpanda_cloud_provider_access.a", tfjsonpath.New("id")),
			)),
			{
				Config: cpaNetworkConfig(name, "a", map[string]string{"a": roleOtherA}, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("redpanda_cloud_provider_access.a", plancheck.ResourceActionCreateBeforeDestroy),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: cpaNetworkChecks(name, "a",
					idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
					accessReplaced.AddStateValue("redpanda_cloud_provider_access.a", tfjsonpath.New("id")),
					statecheck.ExpectKnownValue("redpanda_cloud_provider_access.a", tfjsonpath.New("aws").AtMapKey("role_arn"), knownvalue.StringExact(roleOtherA)),
				),
			},
		},
	})
}

// TestIntegration_Network_CloudProviderAccess_ReplaceInUseAccess pins that
// replacing an access a network uses, without create_before_destroy, never
// replaces the network: Terraform deletes the access first, the control plane
// refuses before anything changes, and the error names the fix.
func TestIntegration_Network_CloudProviderAccess_ReplaceInUseAccess(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-inuse"
	idStable := statecheck.CompareValue(compare.ValuesSame())
	original := cpaNetworkConfig(name, "a", map[string]string{"a": roleA}, false)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, original, cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
			{
				Config: cpaNetworkConfig(name, "a", map[string]string{"a": roleOtherA}, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("redpanda_cloud_provider_access.a", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ExpectError: regexp.MustCompile(`(?s)FailedPrecondition.*create_before_destroy`),
			},
			integration.NoopReapplyStep(networkAddr, original, cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
		},
	})
}

// TestIntegration_Network_CloudProviderAccess_AddOrRemoveRefused pins that
// adding an access to a network created without one, or removing it, plans an
// in-place update rather than a replacement, and that the control plane's
// refusal fails the apply and leaves the network as it was.
func TestIntegration_Network_CloudProviderAccess_AddOrRemoveRefused(t *testing.T) {
	cases := map[string]struct {
		created, changed string
		want             string
	}{
		"add":    {"", "a", `can\s+only\s+be\s+changed\s+on\s+a\s+network\s+created\s+with\s+one`},
		"remove": {"a", "", `cannot\s+be\s+cleared`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, factories := integration.Setup(t)
			accesses := map[string]string{"a": roleA}
			networkName := "tfrp-mock-net-" + name
			original := cpaNetworkConfig(networkName, tc.created, accesses, false)
			idStable := statecheck.CompareValue(compare.ValuesSame())
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{
					integration.CreateStep(networkAddr, original, []statecheck.StateCheck{
						idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
					}),
					{
						Config: cpaNetworkConfig(networkName, tc.changed, accesses, false),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionUpdate)},
						},
						ExpectError: regexp.MustCompile(tc.want),
					},
					integration.NoopReapplyStep(networkAddr, original, []statecheck.StateCheck{
						idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
					}),
				},
			})
		})
	}
}

// TestIntegration_Network_CloudProviderAccess_AddOnReplacement pins the paths
// onto cross-account provisioning for an existing network: the access can be
// added in the same apply that replaces the network, whether another attribute
// forces the replacement or the network is tainted, as terraform apply
// -replace does.
func TestIntegration_Network_CloudProviderAccess_AddOnReplacement(t *testing.T) {
	accesses := map[string]string{"a": roleA}
	cases := map[string]struct {
		changedName string
		taint       []string
	}{
		"sibling replace": {changedName: "tfrp-mock-net-add-renamed"},
		"tainted":         {changedName: "tfrp-mock-net-add", taint: []string{networkAddr}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, factories := integration.Setup(t)
			idReplaced := statecheck.CompareValue(compare.ValuesDiffer())
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{
					integration.CreateStep(networkAddr, cpaNetworkConfig("tfrp-mock-net-add", "", accesses, false), []statecheck.StateCheck{
						statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider_access_id"), knownvalue.Null()),
						idReplaced.AddStateValue(networkAddr, tfjsonpath.New("id")),
					}),
					{
						Taint:  tc.taint,
						Config: cpaNetworkConfig(tc.changedName, "a", accesses, false),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionDestroyBeforeCreate)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
						ConfigStateChecks: cpaNetworkChecks(tc.changedName, "a",
							idReplaced.AddStateValue(networkAddr, tfjsonpath.New("id")),
						),
					},
				},
			})
		})
	}
}

// TestIntegration_Network_CloudProviderAccess_SwapToOtherAccount surfaces the
// control plane's refusal to re-point a network at an access for another AWS
// account, and pins that the network is left in place on the old access.
func TestIntegration_Network_CloudProviderAccess_SwapToOtherAccount(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-xacct"
	accesses := map[string]string{"a": roleA, "other": roleOther}
	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cpaNetworkConfig(name, "a", accesses, false), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
			{
				Config: cpaNetworkConfig(name, "other", accesses, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionUpdate)},
				},
				ExpectError: regexp.MustCompile(`is\s+for\s+AWS\s+account\s+210987654321`),
			},
			integration.NoopReapplyStep(networkAddr, cpaNetworkConfig(name, "a", accesses, false), cpaNetworkChecks(name, "a",
				idStable.AddStateValue(networkAddr, tfjsonpath.New("id")),
			)),
		},
	})
}

// TestIntegration_Network_CloudProviderAccess_EmptyIDRejected pins that an
// empty cloud_provider_access_id is refused at plan: the API would treat it as
// no access and the network would read back null.
func TestIntegration_Network_CloudProviderAccess_EmptyIDRejected(t *testing.T) {
	_, factories := integration.Setup(t)
	cfg := strings.Replace(cpaNetworkConfig("tfrp-mock-net-empty", "", nil, false), `  cidr_block        = "10.0.0.0/20"`, `  cidr_block        = "10.0.0.0/20"
  cloud_provider_access_id = ""`, 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config:      cfg,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`must be a cloud provider access ID`),
		}},
	})
}

// TestIntegration_Network_CloudProviderAccess_Rejected pins the combinations
// the control plane refuses for a cloud_provider_access_id: the provider
// rejects the shape at plan time and surfaces an unknown access from apply.
func TestIntegration_Network_CloudProviderAccess_Rejected(t *testing.T) {
	const cmr = `
  customer_managed_resources = {
    aws = {
      management_bucket = { arn = "arn:aws:s3:::tfrp-bv-bucket" }
      dynamodb_table    = { arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb" }
      vpc               = { arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a" }
      private_subnets   = { arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"] }
    }
  }`
	cases := map[string]struct {
		cloudProvider, clusterType, cidr, extra, accessID string
		planOnly                                          bool
		want                                              string
	}{
		"dedicated":                       {"aws", "dedicated", `cidr_block = "10.0.0.0/20"`, "", "", true, `Cloud Provider Access Requires BYOC`},
		"gcp":                             {"gcp", "byoc", `cidr_block = "10.0.0.0/20"`, "", "", true, `Cloud Provider Access Requires AWS`},
		"with customer managed resources": {"aws", "byoc", "", cmr, "", true, `Conflicting Network Provisioning Modes`},
		"unknown access":                  {"aws", "byoc", `cidr_block = "10.0.0.0/20"`, "", "aaaaaaaaaaaaaaaaaaaa", false, `failed\s+to\s+create\s+network(?s:.*)NotFound`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, factories := integration.Setup(t)
			accessID := `redpanda_cloud_provider_access.test.id`
			if tc.accessID != "" {
				accessID = fmt.Sprintf("%q", tc.accessID)
			}
			cfg := fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_cloud_provider_access" "test" {
  name           = "tfrp-mock-cpa"
  cloud_provider = "aws"
  aws = {
    role_arn = "arn:aws:iam::123456789012:role/tfrp-mock"
  }
}

resource "redpanda_network" "test" {
  name                     = "tfrp-mock-net-cpa-bad"
  resource_group_id        = redpanda_resource_group.test.id
  cloud_provider           = %q
  region                   = "us-east-1"
  cluster_type             = %q
  %s
  cloud_provider_access_id = %s
  %s
}
`, tc.cloudProvider, tc.clusterType, tc.cidr, accessID, tc.extra)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{{
					Config:      cfg,
					PlanOnly:    tc.planOnly,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}
