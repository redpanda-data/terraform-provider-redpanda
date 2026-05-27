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

package serverlesscluster_test

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

const scAddr = "redpanda_serverless_cluster.test"

// TestIntegration_ServerlessCluster exercises redpanda_serverless_cluster against the
// bufconn fake. The fake pre-populates every Computed-only sub-message
// (kafka_api, schema_registry, dataplane_api, console_url, prometheus) so
// Flatten lands a stable state on first apply. Update has no FieldMask;
// changing tags is the load-bearing update path.
func TestIntegration_ServerlessCluster(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_serverless_cluster.test"
	initialTags := `{ environment = "dev" }`
	updatedTags := `{ environment = "staging", owner = "qa" }`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockServerlessClusterConfig(initialTags),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-sc"),
					resource.TestCheckResourceAttr(addr, "serverless_region", "pro-us-east-1"),
					resource.TestCheckResourceAttr(addr, "tags.environment", "dev"),
					resource.TestCheckResourceAttrSet(addr, "id"),
					resource.TestCheckResourceAttrSet(addr, "dataplane_api.url"),
				),
			},
			{
				Config: mockServerlessClusterConfig(initialTags),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttr(addr, "tags.environment", "dev"),
			},
			{
				Config: mockServerlessClusterConfig(updatedTags),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "tags.environment", "staging"),
					resource.TestCheckResourceAttr(addr, "tags.owner", "qa"),
				),
			},
		},
	})
}

func mockServerlessClusterConfig(tags string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-sc-rg"
}

resource "redpanda_serverless_cluster" "test" {
  name              = "tfrp-mock-sc"
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = "pro-us-east-1"
  tags              = %s
}
`, tags)
}

// scBaseConfig builds the minimal serverless_cluster HCL parameterized by name,
// region, and an optional raw HCL tags fragment. Pass tagsHCL == "" to omit
// tags entirely; networking_config and private_link_id are always omitted so
// the fake's defaults apply.
func scBaseConfig(name, region, tagsHCL string) string {
	tagsLine := ""
	if tagsHCL != "" {
		tagsLine = fmt.Sprintf("  tags              = %s\n", tagsHCL)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-sc-rg"
}

resource "redpanda_serverless_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = %q
%s}
`, name, region, tagsLine)
}

// scWithNetworkingConfig builds a serverless_cluster HCL with an explicit
// networking_config block. plinkID is included when non-empty — required by
// the `private_link_id_required` cross-field constraint when private is
// STATE_ENABLED. Used by the UpdateLeaf_NetworkingConfig scenario.
func scWithNetworkingConfig(name, region, public, private, plinkID string) string {
	plinkLine := ""
	if plinkID != "" {
		plinkLine = fmt.Sprintf("  private_link_id   = %q\n", plinkID)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-sc-rg"
}

resource "redpanda_serverless_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = %q
  networking_config = {
    public  = %q
    private = %q
  }
%s}
`, name, region, public, private, plinkLine)
}

// scWithPrivateLinkID builds a serverless_cluster HCL that sets
// private_link_id to plinkID. Pass plinkID == "" to omit the field (Null state).
func scWithPrivateLinkID(name, region, plinkID string) string {
	plinkLine := ""
	if plinkID != "" {
		plinkLine = fmt.Sprintf("  private_link_id   = %q\n", plinkID)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-sc-rg"
}

resource "redpanda_serverless_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = %q
%s}
`, name, region, plinkLine)
}

// scRRRGConfig declares two resource_groups and binds the serverless_cluster
// to one of them by label ("rg1" or "rg2"). Used by RequiresReplace_ResourceGroupID.
func scRRRGConfig(name, rgLabel string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "rg1" {
  name = "tfrp-mock-sc-rg-1"
}

resource "redpanda_resource_group" "rg2" {
  name = "tfrp-mock-sc-rg-2"
}

resource "redpanda_serverless_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.%s.id
  serverless_region = "pro-us-east-1"
}
`, name, rgLabel)
}

// TestIntegration_ServerlessCluster_CreateAndRefresh validates the Create + no-op
// cycle. Every leaf is asserted — Required inputs, Optional defaults populated
// by the fake (networking_config), and Computed-only outputs. id stability
// across the noop is the load-bearing UseStateForUnknown proof via
// CompareValue(ValuesSame()).
func TestIntegration_ServerlessCluster_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-car"
	cfg := scBaseConfig(name, "pro-us-east-1", "")

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-us-east-1")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("resource_group_id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("tags"), knownvalue.Null()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("private_link_id"), knownvalue.Null()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("public"), knownvalue.StringExact("STATE_ENABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("private"), knownvalue.StringExact("STATE_DISABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("dataplane_api").AtMapKey("url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("dataplane_api").AtMapKey("private_url"), knownvalue.StringExact("bufnet-private")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("kafka_api").AtMapKey("seed_brokers"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("mock-broker-0.mock.redpanda.cloud:9092"),
				})),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("kafka_api").AtMapKey("private_seed_brokers"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("mock-broker-0.private.mock.redpanda.cloud:9092"),
				})),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("schema_registry").AtMapKey("url"), knownvalue.StringExact("https://mock.schema-registry.redpanda.cloud")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("schema_registry").AtMapKey("private_url"), knownvalue.StringExact("https://mock.schema-registry.private.redpanda.cloud")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("console_url"), knownvalue.StringExact("https://mock.console.redpanda.cloud")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("console_private_url"), knownvalue.StringExact("https://mock.console.private.redpanda.cloud")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("prometheus").AtMapKey("url"), knownvalue.StringExact("https://mock.prometheus.redpanda.cloud")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("prometheus").AtMapKey("private_url"), knownvalue.StringExact("https://mock.prometheus.private.redpanda.cloud")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("planned_deletion"), knownvalue.Null()),
				idPreserved.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(scAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-us-east-1")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("public"), knownvalue.StringExact("STATE_ENABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("private"), knownvalue.StringExact("STATE_DISABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("planned_deletion"), knownvalue.Null()),
				idPreserved.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_RequiresReplace_Name mutates `name` and asserts
// DestroyBeforeCreate. The shared CompareValue(ValuesDiffer()) on id is the
// load-bearing proof that a fresh server-assigned id replaces the prior one —
// proving the framework destroyed and recreated rather than renaming in place.
func TestIntegration_ServerlessCluster_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		nameA = "tfrp-mock-sc-rr-name-a"
		nameB = "tfrp-mock-sc-rr-name-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scBaseConfig(nameA, "pro-us-east-1", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(scAddr, scBaseConfig(nameB, "pro-us-east-1", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameB)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_RequiresReplace_ResourceGroupID switches the
// cluster's resource_group_id between two declared resource_groups, asserting
// DestroyBeforeCreate when the rg target changes.
func TestIntegration_ServerlessCluster_RequiresReplace_ResourceGroupID(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-rr-rg"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scRRRGConfig(name, "rg1"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(scAddr, scRRRGConfig(name, "rg2"), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_RequiresReplace_ServerlessRegion mutates
// `serverless_region` and asserts DestroyBeforeCreate.
func TestIntegration_ServerlessCluster_RequiresReplace_ServerlessRegion(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-rr-region"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scBaseConfig(name, "pro-us-east-1", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-us-east-1")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(scAddr, scBaseConfig(name, "pro-eu-west-1", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-eu-west-1")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_UpdateLeaf_Tags mutates `tags` in-place. id stays
// SAME across the update — the load-bearing proof that this is an in-place
// update, not a destroy-recreate. Update has no FieldMask so the request
// carries the full new tag map; the fake replaces wholesale.
func TestIntegration_ServerlessCluster_UpdateLeaf_Tags(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-ul-tags"

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scBaseConfig(name, "pro-us-east-1", `{ environment = "dev" }`), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("tags").AtMapKey("environment"), knownvalue.StringExact("dev")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(scAddr, scBaseConfig(name, "pro-us-east-1", `{ environment = "staging", owner = "qa" }`), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("tags").AtMapKey("environment"), knownvalue.StringExact("staging")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("tags").AtMapKey("owner"), knownvalue.StringExact("qa")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("dataplane_api").AtMapKey("url"), knownvalue.StringExact("bufnet")),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_UpdateLeaf_NetworkingConfig mutates the
// networking_config block in-place. Enum strings round-trip through
// Expand→fake→Flatten. The proto cross-field constraints
// `network_must_not_have_both_private_public_disabled` and
// `private_link_id_required` force two valid configurations:
//   - Step 1: public=ENABLED, private=DISABLED (default; no private_link_id)
//   - Step 2: public=ENABLED, private=ENABLED + private_link_id set
//
// id is SAME across both — confirms in-place update.
func TestIntegration_ServerlessCluster_UpdateLeaf_NetworkingConfig(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name    = "tfrp-mock-sc-ul-netcfg"
		plinkID = "00000000000000000001"
	)

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scWithNetworkingConfig(name, "pro-us-east-1", "STATE_ENABLED", "STATE_DISABLED", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("public"), knownvalue.StringExact("STATE_ENABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("private"), knownvalue.StringExact("STATE_DISABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("private_link_id"), knownvalue.Null()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(scAddr, scWithNetworkingConfig(name, "pro-us-east-1", "STATE_ENABLED", "STATE_ENABLED", plinkID), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("public"), knownvalue.StringExact("STATE_ENABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("networking_config").AtMapKey("private"), knownvalue.StringExact("STATE_ENABLED")),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("private_link_id"), knownvalue.StringExact(plinkID)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_UpdateLeaf_PrivateLinkID exercises the
// proto3-optional `private_link_id` across all three user-perspective states:
// Step 1: omitted in HCL → state Null
// Step 2: set to value A → state A
// Step 3: changed to value B → state B
// id is SAME across all three transitions — proves truly in-place. The hand-
// picked values (...0001, ...0002) satisfy the `^[a-v0-9]{20}$` description.
func TestIntegration_ServerlessCluster_UpdateLeaf_PrivateLinkID(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name     = "tfrp-mock-sc-ul-plink"
		plinkIDA = "00000000000000000001"
		plinkIDB = "00000000000000000002"
	)

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scWithPrivateLinkID(name, "pro-us-east-1", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("private_link_id"), knownvalue.Null()),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(scAddr, scWithPrivateLinkID(name, "pro-us-east-1", plinkIDA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("private_link_id"), knownvalue.StringExact(plinkIDA)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(scAddr, scWithPrivateLinkID(name, "pro-us-east-1", plinkIDB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("private_link_id"), knownvalue.StringExact(plinkIDB)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(scAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessCluster_ImportRoundTrip exercises the bearer-id import
// path. ImportState uses ImportStatePassthroughID — no live controlplane
// lookup. nil idFunc uses the bearer "id" from prior state; nil verifyIgnore
// means every attribute must round-trip identically.
func TestIntegration_ServerlessCluster_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-import"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, scBaseConfig(name, "pro-us-east-1", ""), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(scAddr, nil, nil),
		},
	})
}

// TestIntegration_ServerlessCluster_ErrorPath_GetNotFound covers the Read→NotFound
// path. After a successful Create, an OverrideOnce on GetServerlessCluster
// makes the next Read return NotFound. The provider's Read sees NotFound,
// calls RemoveResource; the next plan sees the resource missing → re-Create.
func TestIntegration_ServerlessCluster_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-notfound"
	cfg := scBaseConfig(name, "pro-us-east-1", "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ServerlessClusterService_GetServerlessCluster_FullMethodName,
						status.Error(codes.NotFound, "serverless cluster not found"),
					)
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(scAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_ServerlessCluster_ErrorPath_CreateFailed injects Internal on
// CreateServerlessCluster. The provider's Create surfaces the gRPC error as a
// diagnostic; ExpectError matches the regexp against the diagnostic text.
func TestIntegration_ServerlessCluster_ErrorPath_CreateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-createfail"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.ServerlessClusterService_CreateServerlessCluster_FullMethodName,
				codes.Internal,
				scBaseConfig(name, "pro-us-east-1", ""),
				"synthetic create failure",
			),
		},
	})
}

// TestIntegration_ServerlessCluster_ErrorPath_UpdateFailed injects Internal on
// UpdateServerlessCluster. codes.Internal is non-retryable so the override
// fires once and the diagnostic surfaces immediately.
func TestIntegration_ServerlessCluster_ErrorPath_UpdateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-updfail"
	cfgV1 := scBaseConfig(name, "pro-us-east-1", `{ environment = "dev" }`)
	cfgV2 := scBaseConfig(name, "pro-us-east-1", `{ environment = "staging" }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, cfgV1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ServerlessClusterService_UpdateServerlessCluster_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      cfgV2,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_ServerlessCluster_ErrorPath_DeleteFailed injects Internal on
// DeleteServerlessCluster. Delete does NOT use HandleGracefulRemoval — the
// error surfaces via resp.Diagnostics.AddError. codes.Internal is non-retryable.
func TestIntegration_ServerlessCluster_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-sc-delfail"
	cfg := scBaseConfig(name, "pro-us-east-1", "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(scAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(scAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ServerlessClusterService_DeleteServerlessCluster_FullMethodName,
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
