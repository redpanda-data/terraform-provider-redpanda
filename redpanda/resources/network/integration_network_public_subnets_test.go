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
	"fmt"
	"regexp"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	pubSubnetA = "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0aaa1111bbbb2222c"
	pubSubnetB = "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0ddd3333eeee4444f"
	pubSubnetC = "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0999888877776666a"
)

var publicSubnetsPath = tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("public_subnets")

func arnList(arns ...string) knownvalue.Check {
	checks := make([]knownvalue.Check, len(arns))
	for i, a := range arns {
		checks[i] = knownvalue.StringExact(a)
	}
	return knownvalue.ListExact(checks)
}

// byovpcAWSPublicSubnetsConfig is the AWS BYOVPC network with an optional
// public_subnets block; no ARNs leaves the block out entirely, which is the
// shape of every network created before the field existed.
func byovpcAWSPublicSubnetsConfig(name string, publicARNs ...string) string {
	block := ""
	if len(publicARNs) > 0 {
		quoted := make([]string, len(publicARNs))
		for i, a := range publicARNs {
			quoted[i] = fmt.Sprintf("%q", a)
		}
		block = fmt.Sprintf(`
      public_subnets = {
        arns = [%s]
      }`, strings.Join(quoted, ", "))
	}
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
        arn = "arn:aws:s3:::tfrp-bv-mgmt"
      }
      dynamodb_table = {
        arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb"
      }
      vpc = {
        arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a"
      }
      private_subnets = {
        arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"]
      }%s
    }
  }
}
`, name, block)
}

func TestIntegration_Network_PublicSubnets_CreateWith(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-create"
	cfg := byovpcAWSPublicSubnetsConfig(name, pubSubnetA, pubSubnetB)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.NoopReapplyStep(networkAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetB)),
			}),
		},
	})
}

// TestIntegration_Network_PublicSubnets_AddInPlace is the adopt flow: a
// network created without public subnets gains them without replacement, so a
// cluster already on the network survives.
func TestIntegration_Network_PublicSubnets_AddInPlace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-add"
	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath, knownvalue.Null()),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA)),
				idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// The control plane compares public subnets as a set, so a reordered list is
// not a change and must not plan one.
func TestIntegration_Network_PublicSubnets_ReorderIsNoop(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-reorder"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA, pubSubnetB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetB)),
			}),
			integration.NoopReapplyStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetB, pubSubnetA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetB)),
			}),
		},
	})
}

// Write-once: once set, a different set of subnets cannot be applied in
// place, so the plan replaces the network like every other AWS customer-managed
// leaf.
func TestIntegration_Network_PublicSubnets_ChangeRequiresReplace(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-change"
	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA), []statecheck.StateCheck{
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA, pubSubnetC), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetC)),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// Removing the block from config after it is set is a no-op: the control
// plane cannot clear public subnets, and state keeps what the server holds.
func TestIntegration_Network_PublicSubnets_RemoveBlockIsNoop(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-remove"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA)),
			}),
			integration.NoopReapplyStep(networkAddr, byovpcAWSPublicSubnetsConfig(name), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA)),
			}),
		},
	})
}

func TestIntegration_Network_PublicSubnets_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-import"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA, pubSubnetB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetB)),
			}),
			integration.ImportRoundTripStep(networkAddr, nil, nil),
		},
	})
}

func TestIntegration_NetworkDataSource_PublicSubnets(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name    = "tfrp-mock-net-pub-ds"
		dsAddr  = "data.redpanda_network.test"
		dsBlock = `
data "redpanda_network" "test" {
  id = redpanda_network.test.id
}
`
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: byovpcAWSPublicSubnetsConfig(name, pubSubnetA, pubSubnetB) + dsBlock,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(dsAddr, publicSubnetsPath.AtMapKey("arns"), arnList(pubSubnetA, pubSubnetB)),
				},
			},
		},
	})
}

// The control plane gates public_subnets behind a per-resource-group preview
// flag on UpdateNetwork as well as CreateNetwork; the denial must surface as
// the apply error and leave the network in state untouched.
func TestIntegration_Network_ErrorPath_UpdateNetworkDenied(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-net-pub-denied"
	// Terraform re-wraps diagnostic text, so the pattern tolerates line breaks.
	const denied = "(?s)PermissionDenied.*public_subnets is not\\s+enabled for this organization"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath, knownvalue.Null()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.NetworkService_UpdateNetwork_FullMethodName,
						status.Error(codes.PermissionDenied, "customer_managed_resources.aws.public_subnets is not enabled for this organization. Please contact support to enable this preview feature."),
					)
				},
				Config:      byovpcAWSPublicSubnetsConfig(name, pubSubnetA),
				ExpectError: regexp.MustCompile(denied),
			},
			integration.NoopReapplyStep(networkAddr, byovpcAWSPublicSubnetsConfig(name), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, publicSubnetsPath, knownvalue.Null()),
			}),
		},
	})
}

// Customer-managed resources are immutable, and nested plan modifiers do not
// run under a null parent, so the leaf RequiresReplace markers never see the
// block or an arm being removed. Both shapes are refused before planning:
// dropping the block needs a cidr_block, whose own RequiresReplace then
// replaces the network, and an empty block fails the resources oneof rule.
// The final step restores the config so the post-test destroy runs.
func TestIntegration_Network_CMR_AWS_ClearedIsRejected(t *testing.T) {
	_, factories := integration.Setup(t)
	const name = "tfrp-mock-net-cmr-aws-cleared"
	cfg := byovpcAWSPublicSubnetsConfig(name, pubSubnetA)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, nil),
			{
				Config:      cmrClearedConfig(name, "aws", "us-east-1", ""),
				ExpectError: regexp.MustCompile("cidr_block must be set when customer_managed_resources is not set"),
			},
			{
				Config:      cmrClearedConfig(name, "aws", "us-east-1", "customer_managed_resources = {}"),
				ExpectError: regexp.MustCompile("exactly one field is required in oneof"),
			},
			integration.NoopReapplyStep(networkAddr, cfg, nil),
		},
	})
}

func TestIntegration_Network_CMR_GCP_ClearedIsRejected(t *testing.T) {
	_, factories := integration.Setup(t)
	const name = "tfrp-mock-net-cmr-gcp-cleared"
	cfg := byovpcGCPConfig(name, "tfrp-bv-gcp-vpc")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, nil),
			{
				Config:      cmrClearedConfig(name, "gcp", "us-central1", ""),
				ExpectError: regexp.MustCompile("cidr_block must be set when customer_managed_resources is not set"),
			},
			{
				Config:      cmrClearedConfig(name, "gcp", "us-central1", "customer_managed_resources = {}"),
				ExpectError: regexp.MustCompile("exactly one field is required in oneof"),
			},
			integration.NoopReapplyStep(networkAddr, cfg, nil),
		},
	})
}

// Dropping the block together with adding a cidr_block is the one valid way
// to clear customer-managed resources, and cidr_block forces replacement.
func TestIntegration_Network_RequiresReplace_CMR_AWS_BlockRemoved(t *testing.T) {
	_, factories := integration.Setup(t)
	const name = "tfrp-mock-net-cmr-aws-removed"
	idChanged := statecheck.CompareValue(compare.ValuesDiffer())
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA), []statecheck.StateCheck{
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, cmrClearedConfig(name, "aws", "us-east-1", `cidr_block = "10.0.0.0/20"`), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("customer_managed_resources"), knownvalue.Null()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// Switching the customer-managed arm without changing cloud_provider: the
// incoming arm's leaves go from null to set, which is a change their
// RequiresReplace markers see, so the plan is a replacement whose create the
// control plane then rejects for the provider mismatch.
func TestIntegration_Network_RequiresReplace_CMR_ArmSwitched(t *testing.T) {
	_, factories := integration.Setup(t)
	const name = "tfrp-mock-net-cmr-arm-switch"
	gcpArm := `customer_managed_resources = {
    gcp = {
      network_name       = "tfrp-bv-gcp-vpc"
      network_project_id = "tfrp-bv-gcp-project"
      management_bucket = {
        name = "tfrp-bv-gcp-mgmt"
      }
    }
  }`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAWSPublicSubnetsConfig(name, pubSubnetA), nil),
			{
				Config: cmrClearedConfig(name, "aws", "us-east-1", gcpArm),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(networkAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ExpectError: regexp.MustCompile("must be CLOUD_PROVIDER_GCP"),
			},
		},
	})
}

// cmrClearedConfig is the BYOVPC network with its customer-managed block
// replaced by extra (empty for none), everything else unchanged.
func cmrClearedConfig(name, cloudProvider, region, extra string) string {
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
  cluster_type      = "byoc"
  %s
}
`, name, cloudProvider, region, extra)
}
