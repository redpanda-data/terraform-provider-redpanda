//go:build integration

// Copyright 2026 Redpanda Data, Inc.
//
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
	"regexp"
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
// bufconn-backed fake controlplane. Network has no Update RPC — every
// configurable field is RequiresReplace — so the scenario is: minimal
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
// and explicitly asserts customer_managed_resources is Null — the inverse
// variant proof that pins the variant partitioning at the state level, not
// only in the HCL. id is captured across both steps via a shared
// CompareValue(ValuesSame()) — the load-bearing proof that UseStateForUnknown
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
// is that the server-assigned id DIFFERS across steps — a shared
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
// from "dedicated" to "byoc" (without switching to BYOVPC — cidr_block stays
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
// inverse-variant proof asserts cidr_block is Null in state — pinning the
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

// TestIntegration_Network_ImportRoundTrip exercises the bearer-id import path.
// Network's ImportState uses ImportStatePassthroughID on the "id" attribute,
// so the import id is the xid-like string assigned at Create. Network's
// ImportState does NOT call ClusterForID — no live-controlplane-lookup risk.
// nil idFunc tells the helper to use the bearer "id" from prior state; nil
// verifyIgnore means every attribute must roundtrip identically.
func TestIntegration_Network_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-import"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr,
				inPlaceConfig(name, "aws", "dedicated", "us-east-1", "10.0.0.0/20"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				}),
			integration.ImportRoundTripStep(networkAddr, nil, nil),
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
