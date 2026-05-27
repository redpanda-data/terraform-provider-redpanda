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

package pipeline_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const pipelineAddr = "redpanda_pipeline.test"

const minimalPipelineConfigYaml = `
input:
  generate:
    mapping: 'root = "hello"'
    interval: 1s
output:
  drop: {}
`

const updatedPipelineConfigYaml = `
input:
  generate:
    mapping: 'root = "hello"'
    interval: 1s
output:
  drop: {}
# updated
`

// TestIntegration_Pipeline
func TestIntegration_Pipeline(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_pipeline.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockPipelineConfig("mock-pipeline", "stopped", minimalPipelineConfigYaml),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "display_name", "mock-pipeline"),
					resource.TestCheckResourceAttr(addr, "state", "stopped"),
					resource.TestCheckResourceAttr(addr, "allow_deletion", "true"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "display_name", "mock-pipeline"),
					resource.TestCheckResourceAttr(addr, "state", "stopped"),
				),
			},
			{
				Config: mockPipelineConfig("mock-pipeline", "stopped", minimalPipelineConfigYaml),
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
				Config: mockPipelineConfig("mock-pipeline", "stopped", minimalPipelineConfigYaml+"\n# updated\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: mockPipelineConfig("mock-pipeline", "running", minimalPipelineConfigYaml+"\n# updated\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "state", "running"),
			},
		},
	})
}

func mockPipelineConfig(displayName, state, configYaml string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  display_name    = %q
  description     = "mock pipeline"
  state           = %q
  config_yaml     = %q
  cluster_api_url = "bufnet"
  allow_deletion  = true
}
`, displayName, state, configYaml)
}

// mockPipelineBaseConfig renders the canonical pipeline HCL with the common
// scalar + tags surface.
func mockPipelineBaseConfig(displayName, description, state, clusterAPIURL string, allowDeletion bool, tagsLiteral, yamlCfg string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  display_name    = %q
  description     = %q
  state           = %q
  config_yaml     = %q
  cluster_api_url = %q
  allow_deletion  = %t
  tags = %s
}
`, displayName, description, state, yamlCfg, clusterAPIURL, allowDeletion, tagsLiteral)
}

// mockPipelineResourcesConfig renders pipeline HCL with a resources block.
// cpuShares and memoryShares are emitted only when non-empty
func mockPipelineResourcesConfig(displayName, cpuShares, memoryShares string) string {
	var resourcesBlock string
	switch {
	case cpuShares != "" && memoryShares != "":
		resourcesBlock = fmt.Sprintf(`
  resources = {
    cpu_shares    = %q
    memory_shares = %q
  }
`, cpuShares, memoryShares)
	case cpuShares != "":
		resourcesBlock = fmt.Sprintf(`
  resources = {
    cpu_shares = %q
  }
`, cpuShares)
	case memoryShares != "":
		resourcesBlock = fmt.Sprintf(`
  resources = {
    memory_shares = %q
  }
`, memoryShares)
	default:
		// both empty: omit the resources block entirely (Null density).
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  display_name    = %q
  description     = "test description"
  state           = "stopped"
  config_yaml     = %q
  cluster_api_url = "bufnet"
  allow_deletion  = true
%s
}
`, displayName, minimalPipelineConfigYaml, resourcesBlock)
}

// mockPipelineServiceAccountConfig renders pipeline HCL with a service_account
// block.
func mockPipelineServiceAccountConfig(displayName, clientID, clientSecret string, secretVersion int) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  display_name    = %q
  description     = "test description"
  state           = "stopped"
  config_yaml     = %q
  cluster_api_url = "bufnet"
  allow_deletion  = true
  service_account = {
    client_id      = %q
    client_secret  = %q
    secret_version = %d
  }
}
`, displayName, minimalPipelineConfigYaml, clientID, clientSecret, secretVersion)
}

// mockPipelineServiceAccountWithStateConfig renders pipeline HCL with a
// service_account block and a configurable state field.
func mockPipelineServiceAccountWithStateConfig(displayName, state, clientID, clientSecret string, secretVersion int) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  display_name    = %q
  description     = "test description"
  state           = %q
  config_yaml     = %q
  cluster_api_url = "bufnet"
  allow_deletion  = true
  service_account = {
    client_id      = %q
    client_secret  = %q
    secret_version = %d
  }
}
`, displayName, state, minimalPipelineConfigYaml, clientID, clientSecret, secretVersion)
}

// TestIntegration_Pipeline_CreateAndRefresh exercises the canonical Create +
// NoopReapply cycle.
func TestIntegration_Pipeline_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-create"
	cfg := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())
	urlPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("description"), knownvalue.StringExact("test description")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("stopped")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("test")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("url"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("status"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources"), knownvalue.Null()),
				idPreserved.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
				urlPreserved.AddStateValue(pipelineAddr, tfjsonpath.New("url")),
			}),
			integration.NoopReapplyStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("description"), knownvalue.StringExact("test description")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("stopped")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("test")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("url"), knownvalue.NotNull()),
				idPreserved.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
				urlPreserved.AddStateValue(pipelineAddr, tfjsonpath.New("url")),
			}),
		},
	})
}

// TestIntegration_Pipeline_UpdateLeaf_DisplayName mutates display_name in-place.
// display_name is Required but does NOT carry RequiresReplace, so this proves
// the Update path runs (ResourceActionUpdate, not DestroyBeforeCreate) and id
// is stable across the in-place mutation.
func TestIntegration_Pipeline_UpdateLeaf_DisplayName(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		nameBefore = "tfrp-mock-pipe-dn-before"
		nameAfter  = "tfrp-mock-pipe-dn-after"
	)
	cfg1 := mockPipelineBaseConfig(nameBefore, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfg2 := mockPipelineBaseConfig(nameAfter, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(nameBefore)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(nameAfter)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_UpdateLeaf_ConfigYaml mutates the Sensitive Required
// config_yaml leaf in-place. id stable across the update.
func TestIntegration_Pipeline_UpdateLeaf_ConfigYaml(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-cfg"
	cfg1 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfg2 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, updatedPipelineConfigYaml)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_UpdateLeaf_State drives the state field stopped → running
// → stopped. Exercises the provider's StartPipeline + StopPipeline call paths
// and the waitForPipelineState polling loop. Backoff is collapsed to
// microseconds by SetTestModeWaits (REDPANDA_TF_ACCEPTANCE_TEST_MODE=1, set by
// integration.Setup), so the fake's instantaneous state transitions converge on
// the first poll without dead time.
//
// id is stable across all 3 steps (in-place update, no replace).
func TestIntegration_Pipeline_UpdateLeaf_State(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-state"
	cfgStopped := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfgRunning := mockPipelineBaseConfig(name, "test description", "running", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfgStopped, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("stopped")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfgRunning, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("running")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfgStopped, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("stopped")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_UpdateLeaf_Tags mutates the tags map in-place. id stable
// across the update.
func TestIntegration_Pipeline_UpdateLeaf_Tags(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-tags"
	cfg1 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfg2 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "prod" }`, minimalPipelineConfigYaml)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("test")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("prod")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_UpdateLeaf_AllowDeletion flips allow_deletion
// true→false→true. Unlike role (no Update RPC), pipeline's Update DOES call
// UpdatePipeline on every Update path — but allow_deletion is a TF-local
// sentinel that doesn't appear in the proto, so the gRPC payload is
// effectively unchanged on these flips. The assertion is that the framework
// plans ResourceActionUpdate (not Replace) and id is stable across all 3
// steps. End with allow_deletion=true so the TestCase's terminal cleanup
// destroy succeeds (Delete blocks when allow_deletion=false).
func TestIntegration_Pipeline_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-allowdel"
	cfgTrue := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfgFalse := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", false, `{ env = "test" }`, minimalPipelineConfigYaml)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfgTrue, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfgFalse, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfgTrue, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_NestedMatrix_Resources_Null omits the resources block from
// config entirely. The Flatten guard only restores prev when it's non-null;
// prev is null here, so Flatten falls through to the proto-derived value
// (the fake returns nil Resources → ObjectNull). State settles at null. Noop
// step verifies plan-null matches state-null (no drift).
func TestIntegration_Pipeline_NestedMatrix_Resources_Null(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-res-null"
	cfg := mockPipelineResourcesConfig(name, "", "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.NoopReapplyStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources"), knownvalue.Null()),
			}),
		},
	})
}

// TestIntegration_Pipeline_NestedMatrix_Resources_Full sets both cpu_shares and
// memory_shares. The PipelineFake's `pipelineRecord` struct does NOT store
// resources, so the API echo returns nil Resources. The Flatten guard
// restores prev.Resources from prior state when prev is non-null — that's
// what makes this scenario tractable. State carries the user-supplied
// values forward across Create + Noop.
//
// cpu_shares="500m" (CPUSharesValidator requires multiple of 100m).
// memory_shares="512Mi" (MemorySharesValidator accepts Ki/Mi/Gi units).
//
// Partial density (one of {cpu_shares, memory_shares} set, the other null)
// is intentionally NOT exercised. Both fields carry the proto-field REQUIRED
// annotation (`(google.api.field_behavior) = REQUIRED` →
// `\xe0A\x02` in the descriptor; `(buf.validate.field).required = true` →
// `\xc8\x01\x01`), so the protovalidate interceptor rejects a Resources
// message with either sub-field empty. The TF schema marks both Optional,
// but the cross-field "if resources present, both sub-fields must be set"
// constraint lives at the proto layer. The _Full + _Null pair covers both
// leaves (cpu_shares + memory_shares) end-to-end; no Partial scenario can
// satisfy proto validation without already being structurally equivalent
// to _Full.
func TestIntegration_Pipeline_NestedMatrix_Resources_Full(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-res-full"
	cfg := mockPipelineResourcesConfig(name, "500m", "512Mi")

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources").AtMapKey("cpu_shares"), knownvalue.StringExact("500m")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources").AtMapKey("memory_shares"), knownvalue.StringExact("512Mi")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources").AtMapKey("cpu_shares"), knownvalue.StringExact("500m")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("resources").AtMapKey("memory_shares"), knownvalue.StringExact("512Mi")),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_NestedMatrix_ServiceAccount_Full sets client_id +
// client_secret + secret_version. After Create:
//   - client_id is echoed back by the fake; visible in state.
//   - client_secret is WriteOnly: stripped from state by the framework, ends
//     up Null.
//   - secret_version is preserved via UseStateForUnknown + the Flatten guard
//     that restores prev.SecretVersion.
//
// state="stopped" avoids the Read-path running-time SA-omit complexity.
//
// Note: NestedMatrix_ServiceAccount_Empty / _Null are NOT exercised at this
// path. client_id + client_secret are Required-within-block, so the Empty
// density is unsatisfiable. The block-level Null case is covered implicitly
// by every other scenario in this file that omits service_account from
// config (asserted as service_account=Null in CreateAndRefresh).
func TestIntegration_Pipeline_NestedMatrix_ServiceAccount_Full(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name         = "tfrp-mock-pipe-sa-full"
		clientID     = "test-client-id"
		clientSecret = "$${secrets.test_secret}"
	)
	cfg := mockPipelineServiceAccountConfig(name, clientID, clientSecret, 1)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_id"), knownvalue.StringExact(clientID)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_id"), knownvalue.StringExact(clientID)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(1)),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_NestedMatrix_ServiceAccount_SecretVersionBump exercises
// the Update path's "rewrite the write-only client_secret" trigger. The
// provider's Update only sends the service account in the UpdatePipeline
// payload when:
//   - state.ServiceAccount was previously null (first-add), or
//   - client_id changed, or
//   - secret_version changed.
//
// Bumping secret_version 1 → 2 triggers the third branch. Asserts:
//   - secret_version=2 in state,
//   - client_secret still Null (WriteOnly contract),
//   - id stable (in-place update, not replace).
func TestIntegration_Pipeline_NestedMatrix_ServiceAccount_SecretVersionBump(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name         = "tfrp-mock-pipe-sa-bump"
		clientID     = "test-client-id"
		clientSecret = "$${secrets.test_secret}"
	)
	cfg1 := mockPipelineServiceAccountConfig(name, clientID, clientSecret, 1)
	cfg2 := mockPipelineServiceAccountConfig(name, clientID, clientSecret, 2)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(pipelineAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(2)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_id"), knownvalue.StringExact(clientID)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_RequiresReplace_ClusterApiUrl mutates the only
// RequiresReplace leaf. The bufconn dialer is address-agnostic — it ignores
// the URL string and routes through the in-memory listener — so changing
// "bufnet" → "bufnet2" triggers the plan-level DestroyBeforeCreate and the
// Create on the new resource still succeeds.
//
// Pipeline id is server-generated (the fake emits sequential
// "tfrp-mock-pipeline-<N>"), so destroy+recreate allocates a NEW id different
// from the original. ValuesDiffer on id is the load-bearing proof — distinct
// from role/user where id was name-derived and unchanged across this scenario.
func TestIntegration_Pipeline_RequiresReplace_ClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-rr-url"
	cfg1 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfg2 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet2", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(pipelineAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet2")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Pipeline_ImportRoundTrip verifies that pipeline's ImportState
// correctly reconstructs resource state when the cluster_id resolves through
// the bufconn-backed ClusterFake.
func TestIntegration_Pipeline_ImportRoundTrip(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Cluster.SetClusterByID("cgg5hmkar1m4l0pjg6tg", "bufnet")

	cfg := mockPipelineConfig("tfrp-mock-pipe-import", "stopped", minimalPipelineConfigYaml)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(pipelineAddr, func(s *terraform.State) (string, error) {
				rs, ok := s.RootModule().Resources[pipelineAddr]
				if !ok {
					return "", fmt.Errorf("resource %q not found", pipelineAddr)
				}
				return rs.Primary.Attributes["id"] + ",cgg5hmkar1m4l0pjg6tg", nil
			}, []string{"allow_deletion"}),
		},
	})
}

// TestIntegration_Pipeline_ErrorPath_GetPipeline_NotFound covers the Read→NotFound
// path. After a successful Create, the pipeline is removed from the fake's
// store out-of-band via the fake's own DeletePipeline RPC so the next
// GetPipeline naturally returns NotFound. The provider's Read sees NotFound,
// calls RemoveResource, and the next plan sees the resource missing → re-
// Create. PreApply asserts ResourceActionCreate; PostApplyPostRefresh asserts
// an empty plan after the re-create lands.
//
// Pipeline id is server-generated, so we capture it from step-1 state via a
// TestCheckFunc closure that records the post-Create id into a local string.
// The step-2 PreConfig uses the captured id to call DeletePipeline directly.
// Using the Check hook here is the simplest closure-write path; the modern
// statecheck-based alternative would require a custom StateCheck implementation.
func TestIntegration_Pipeline_ErrorPath_GetPipeline_NotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-notfound"
	cfg := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	var capturedID string
	captureIDStep := integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
		statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
		statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
	})
	captureIDStep.Check = func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[pipelineAddr]
		if !ok {
			return fmt.Errorf("resource %q not found in state", pipelineAddr)
		}
		capturedID = rs.Primary.ID
		if capturedID == "" {
			return fmt.Errorf("resource %q has empty id", pipelineAddr)
		}
		return nil
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			captureIDStep,
			{
				PreConfig: func() {
					if _, err := srv.Pipeline.DeletePipeline(context.Background(),
						&dataplanev1.DeletePipelineRequest{Id: capturedID}); err != nil {
						t.Fatalf("PreConfig: delete pipeline %q: %v", capturedID, err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(pipelineAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_Pipeline_ErrorPath_CreatePipeline_AlreadyExists injects
// AlreadyExists on CreatePipeline. The provider's Create surfaces the gRPC
// error as a Terraform diagnostic; ExpectError matches the regexp against
// the diagnostic text.
func TestIntegration_Pipeline_ErrorPath_CreatePipeline_AlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-exists"
	cfg := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				dataplanev1grpc.PipelineService_CreatePipeline_FullMethodName,
				codes.AlreadyExists,
				cfg,
				"AlreadyExists",
			),
		},
	})
}

// TestIntegration_Pipeline_ErrorPath_UpdatePipeline_Failed injects an Internal-coded
// error on the next UpdatePipeline RPC. After a successful Create, the second
// step bumps config_yaml — the Update path issues GetPipeline (succeeds, the
// override doesn't match), then UpdatePipeline (the override matches and
// returns Internal), which surfaces as a "failed to update pipeline"
// diagnostic. ExpectError matches the regexp.
//
// Why Internal (not Unavailable): the provider retries on Unavailable via
// utils.Retry; OverrideOnce(Unavailable) would consume on the first retry
// and the next attempt would succeed against the fake, masking the error.
// Internal is non-retryable and exercises the visible AddError path.
func TestIntegration_Pipeline_ErrorPath_UpdatePipeline_Failed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-updfail"
	cfg1 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)
	cfg2 := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, updatedPipelineConfigYaml)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.PipelineService_UpdatePipeline_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      cfg2,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_Pipeline_ErrorPath_DeletePipeline_Failed covers the destroy-failed
// path. After a successful Create, an Internal-coded error is injected on
// the next DeletePipeline RPC. Pipeline's Delete handler does NOT route the
// DeletePipeline RPC error through utils.HandleGracefulRemoval — that helper
// is only called when createPipelineClient fails. The DeletePipeline error
// surfaces directly via resp.Diagnostics.AddError, so Internal correctly
// produces a visible diagnostic.
//
// The Delete handler also issues a preceding GetPipeline (to check current
// state for stop-before-delete) and a possible StopPipeline call; the
// pipeline created in step 1 is in STATE_STOPPED so StopPipeline isn't
// issued. The override targets DeletePipeline only.
func TestIntegration_Pipeline_ErrorPath_DeletePipeline_Failed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-delfail"
	cfg := mockPipelineBaseConfig(name, "test description", "stopped", "bufnet", true, `{ env = "test" }`, minimalPipelineConfigYaml)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.PipelineService_DeletePipeline_FullMethodName,
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

// TestIntegration_Pipeline_NullDescription_Repro pins the null-vs-empty fix
// for description. Flatten emits types.StringNull() when the proto description
// is empty and prev is null, so omitting description from HCL no longer causes
// a plan/state consistency error.
func TestIntegration_Pipeline_NullDescription_Repro(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-pipe-nulldesc"
	// description NOT set in HCL — null in plan; Flatten preserves null via the
	// null-vs-empty-vs-prev guard so state stays null and no drift surfaces.
	cfg := fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  display_name    = %q
  state           = "stopped"
  config_yaml     = %q
  cluster_api_url = "bufnet"
  allow_deletion  = true
}
`, name, minimalPipelineConfigYaml)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(pipelineAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("description"), knownvalue.Null()),
			}),
		},
	})
}

// TestIntegration_Pipeline_ServiceAccount_PreservedAcrossRunningState pins the
// Read-path restore hook. The fake now omits ServiceAccount from GetPipeline
// responses when state=STATE_RUNNING (matching production). Without the hook at
// resource_pipeline.go Read, Flatten drops the SA wrapper from state and the
// next plan surfaces a spurious "+ service_account" diff. This test proves the
// hook keeps state stable: Create (stopped, SA set) → Start (running, fake
// returns nil SA) → NoopReapply (plan must be empty — hook restored SA).
func TestIntegration_Pipeline_ServiceAccount_PreservedAcrossRunningState(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name         = "tfrp-mock-pipe-sa-running"
		clientID     = "test-client-id"
		clientSecret = "$${secrets.test_secret}"
	)
	cfgStopped := mockPipelineServiceAccountWithStateConfig(name, "stopped", clientID, clientSecret, 1)
	cfgRunning := mockPipelineServiceAccountWithStateConfig(name, "running", clientID, clientSecret, 1)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			// Step 1: Create stopped — fake returns SA; state carries client_id.
			integration.CreateStep(pipelineAddr, cfgStopped, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("stopped")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_id"), knownvalue.StringExact(clientID)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			// Step 2: Transition to running — fake omits SA in GetPipeline;
			// hook restores SA from prev state. Plan must be empty afterward.
			integration.UpdateLeafStep(pipelineAddr, cfgRunning, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("running")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_id"), knownvalue.StringExact(clientID)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
			// Step 3: NoopReapply — config unchanged; fake still omits SA (still running).
			// Hook must restore SA so no diff surfaces.
			integration.NoopReapplyStep(pipelineAddr, cfgRunning, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("state"), knownvalue.StringExact("running")),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_id"), knownvalue.StringExact(clientID)),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("client_secret"), knownvalue.Null()),
				statecheck.ExpectKnownValue(pipelineAddr, tfjsonpath.New("service_account").AtMapKey("secret_version"), knownvalue.Int64Exact(1)),
				idStable.AddStateValue(pipelineAddr, tfjsonpath.New("id")),
			}),
		},
	})
}
