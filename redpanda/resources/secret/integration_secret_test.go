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

package secret_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
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

const (
	secretAddr      = "redpanda_secret.test"
	resourceName    = "TFRP_MOCK_SECRET"
	resourceNameAlt = "TFRP_MOCK_SECRET2"
	dataValueV1     = "bW9jay1zZWNyZXQtZGF0YQ=="
	dataValueV2     = "bmV3LXNlY3JldC1kYXRh"
	scopeConnect    = `["SCOPE_REDPANDA_CONNECT"]`
	scopeConnectMCP = `["SCOPE_REDPANDA_CONNECT", "SCOPE_MCP_SERVER"]`
)

// TestIntegration_Secret exercises redpanda_secret end-to-end against the
// bufconn-backed fake dataplane. Covers Create with write-only secret_data,
// refresh, no-op re-plan, scopes update (UpdateSecret RPC), and
// secret_data_version bump triggering a data rewrite.
func TestIntegration_Secret(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_secret.test"
	const baseScope = `["SCOPE_REDPANDA_CONNECT"]`
	const extendedScope = `["SCOPE_REDPANDA_CONNECT", "SCOPE_MCP_SERVER"]`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockSecretConfig(baseScope, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "TFRP_MOCK_SECRET"),
					resource.TestCheckResourceAttr(addr, "secret_data_version", "1"),
					resource.TestCheckResourceAttr(addr, "allow_deletion", "true"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				Config: mockSecretConfig(baseScope, 1),
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
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "TFRP_MOCK_SECRET"),
					resource.TestCheckResourceAttr(addr, "secret_data_version", "1"),
				),
			},
			{
				Config: mockSecretConfig(extendedScope, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "scopes.#", "2"),
			},
			{
				Config: mockSecretConfig(extendedScope, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "secret_data_version", "2"),
			},
		},
	})
}

func mockSecretConfig(scopes string, version int) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_secret" "test" {
  name                = "TFRP_MOCK_SECRET"
  secret_data         = "bW9jay1zZWNyZXQtZGF0YQ=="
  secret_data_version = %d
  scopes              = %s
  cluster_api_url     = "bufnet"
  allow_deletion      = true
}
`, version, scopes)
}

// mockSecretFullConfig parameterizes every leaf so each scenario can mutate
// one field while pinning the rest. labels is intentionally omitted here;
// scenarios that need labels use mockSecretWithLabelsConfig.
func mockSecretFullConfig(name, secretData, clusterAPIURL, scopes string, version int64, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_secret" "test" {
  name                = %q
  secret_data         = %q
  secret_data_version = %d
  scopes              = %s
  cluster_api_url     = %q
  allow_deletion      = %t
}
`, name, secretData, version, scopes, clusterAPIURL, allowDeletion)
}

// mockSecretWithLabelsConfig renders a full config plus a labels block. Used
// only by RequiresReplace_Labels.
func mockSecretWithLabelsConfig(name, secretData, clusterAPIURL, scopes string, version int64, allowDeletion bool, labels map[string]string) string {
	var labelsBlock string
	if len(labels) > 0 {
		labelsBlock = "  labels = {\n"
		for k, v := range labels {
			labelsBlock += fmt.Sprintf("    %q = %q\n", k, v)
		}
		labelsBlock += "  }\n"
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_secret" "test" {
  name                = %q
  secret_data         = %q
  secret_data_version = %d
  scopes              = %s
  cluster_api_url     = %q
  allow_deletion      = %t
%s}
`, name, secretData, version, scopes, clusterAPIURL, allowDeletion, labelsBlock)
}

// TestIntegration_Secret_CreateAndRefresh covers the canonical Create + NoopReapply
// cycle. Every leaf is asserted at an exact value. secret_data is asserted
// Null in state across both steps (WriteOnly contract). The load-bearing
// proof for the id leaf's UseStateForUnknown plan modifier is that id is
// IDENTICAL across the two steps — a single CompareValue(ValuesSame())
// shared between both steps' ConfigStateChecks accumulates the values and
// the comparer asserts equality. id is derived from name (id == name) so
// it must be "TFRP_MOCK_SECRET" exactly.
func TestIntegration_Secret_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("name"), knownvalue.StringExact(resourceName)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
					knownvalue.StringExact("SCOPE_REDPANDA_CONNECT"),
				})),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("labels"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idPreserved.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(secretAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("name"), knownvalue.StringExact(resourceName)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
					knownvalue.StringExact("SCOPE_REDPANDA_CONNECT"),
				})),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("labels"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idPreserved.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_UpdateLeaf_AllowDeletion flips allow_deletion
// true→false→true. The Update handler short-circuits when neither
// secret_data_version nor scopes changed: it copies plan.AllowDeletion into
// state without making any gRPC call. ResourceActionUpdate (not
// DestroyBeforeCreate) is the plan-level proof; id stable across all 3
// steps is the state-level proof that no replacement happened. End with
// allow_deletion=true so terminal cleanup destroy passes the in-Delete
// guard.
func TestIntegration_Secret_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := integration.Setup(t)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_UpdateLeaf_Scopes mutates scopes from a 1-element set to a
// 2-element set in place. The UpdateSecretRequest proto has no FieldMask;
// the fake's UpdateSecret performs a full replacement of the stored scope
// slice (fakes/secret.go:89-105). ResourceActionUpdate proves no replace;
// id stable confirms (id == name unchanged). secret_data_version is held at
// 1 so no secret_data is sent — only the scopes flip drives the RPC.
func TestIntegration_Secret_UpdateLeaf_Scopes(t *testing.T) {
	_, factories := integration.Setup(t)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
					knownvalue.StringExact("SCOPE_REDPANDA_CONNECT"),
				})),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnectMCP, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
					knownvalue.StringExact("SCOPE_REDPANDA_CONNECT"),
					knownvalue.StringExact("SCOPE_MCP_SERVER"),
				})),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_UpdateLeaf_SecretData bumps secret_data_version 1→2, which
// the Update handler (resource_secret.go:200) treats as the trigger to send
// the new secret_data in the UpdateSecretRequest. The write-only secret_data
// remains Null in state across both steps; the version int is the only
// user-visible witness of the rewrite. ResourceActionUpdate plus id-stable
// confirm in-place mutation.
func TestIntegration_Secret_UpdateLeaf_SecretData(t *testing.T) {
	_, factories := integration.Setup(t)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV2, "bufnet", scopeConnect, 2, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(2)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_RequiresReplace_Name mutates name from TFRP_MOCK_SECRET to
// TFRP_MOCK_SECRET2. name is RequiresReplace; id == name (GetUpdatedModel in
// models/secret/resource_model.go sets both from the proto's Id field), so
// id DIFFERS across the replace. The ResourceActionDestroyBeforeCreate
// plancheck baked into RequiresReplaceStep is the plan-level proof; the
// idChanged ValuesDiffer comparer is the state-level proof.
func TestIntegration_Secret_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("name"), knownvalue.StringExact(resourceName)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idChanged.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(secretAddr, mockSecretFullConfig(resourceNameAlt, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("name"), knownvalue.StringExact(resourceNameAlt)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceNameAlt)),
				idChanged.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_UpdateLabels mutates labels from null to a non-empty
// map, then mutates the map in place. labels is backend-mutable
// (UpdateSecretRequest carries a labels field), so each change is an in-place
// ResourceActionUpdate, not a replace. id == name and name is unchanged, so id
// is SAME across both updates — the idStable ValuesSame comparer is the inverse
// witness that the resource is not being recreated.
func TestIntegration_Secret_UpdateLabels(t *testing.T) {
	_, factories := integration.Setup(t)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("labels"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(secretAddr, mockSecretWithLabelsConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true, map[string]string{"env": "test"}), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("labels"), knownvalue.MapExact(map[string]knownvalue.Check{
					"env": knownvalue.StringExact("test"),
				})),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(secretAddr, mockSecretWithLabelsConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true, map[string]string{"env": "prod"}), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("labels"), knownvalue.MapExact(map[string]knownvalue.Check{
					"env": knownvalue.StringExact("prod"),
				})),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_RequiresReplace_ClusterApiUrl mutates cluster_api_url from
// bufnet to bufnet2. The bufconn dialer is address-agnostic — it ignores
// the URL string and routes through the in-memory listener — so the value
// flip triggers RequiresReplace and the Create on the new resource still
// succeeds. cluster_api_url is not part of the id formula, so id is SAME
// across the replace. The load-bearing proof is the
// ResourceActionDestroyBeforeCreate plancheck baked into
// RequiresReplaceStep; idStable's ValuesSame check is the inverse witness.
func TestIntegration_Secret_RequiresReplace_ClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(secretAddr, mockSecretFullConfig(resourceName, dataValueV1, "bufnet2", scopeConnect, 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet2")),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				idStable.AddStateValue(secretAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Secret_WriteOnly_SecretData_Preservation explicitly pins the
// WriteOnly contract for secret_data: it must be Null in state both after
// Create (when the user supplied a value in config) and after an explicit
// RefreshState round-trip (which calls GetSecret and re-populates state
// from the fake's response — the fake doesn't return secret_data because
// the proto Secret message has no such field; the WriteOnly attribute is
// always Null in state).
func TestIntegration_Secret_WriteOnly_SecretData_Preservation(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data"), knownvalue.Null()),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
			}),
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(secretAddr, "secret_data"),
					resource.TestCheckResourceAttr(secretAddr, "id", resourceName),
				),
			},
		},
	})
}

// TestIntegration_Secret_ImportRoundTrip verifies that secret's ImportState correctly
// reconstructs resource state when the cluster_id resolves through the
// bufconn-backed ClusterFake.
func TestIntegration_Secret_ImportRoundTrip(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Cluster.SetClusterByID("cgg5hmkar1m4l0pjg6tg", "bufnet")

	const rsrcName = "TFRP_MOCK_SECRET_IMPORT"
	cfg := mockSecretFullConfig(rsrcName, dataValueV1, "bufnet", scopeConnect, 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, cfg, nil),
			integration.ImportRoundTripStep(secretAddr, func(_ *terraform.State) (string, error) {
				return rsrcName + ",cgg5hmkar1m4l0pjg6tg", nil
			}, []string{"allow_deletion", "secret_data"}),
		},
	})
}

// TestIntegration_Secret_ErrorPath_GetFailed injects a one-shot Internal-coded
// error on the next GetSecret RPC. After Create, Terraform calls Read which
// invokes GetSecret via utils.Retry. codes.Internal is not in the retryable
// set (utils.IsUnavailable check fails) so it surfaces via
// utils.NonRetryableError. The Read handler then calls
// utils.HandleGracefulRemoval, but Internal is NOT one of the
// graceful-removal codes — it falls through to ErrorNotHandled and emits
// a TF diagnostic.
func TestIntegration_Secret_ErrorPath_GetFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.SecretService_GetSecret_FullMethodName,
						status.Error(codes.Internal, "synthetic get failure"),
					)
				},
				Config:      cfg,
				ExpectError: regexp.MustCompile("synthetic get failure"),
			},
		},
	})
}

// TestIntegration_Secret_ErrorPath_CreateFailed injects Internal on CreateSecret.
// Internal is non-retryable (utils.IsUnavailable fails) and not
// AlreadyExists, so the Create handler skips the GetSecret probe and surfs
// the error directly via NonRetryableError + DeserializeGrpcError.
func TestIntegration_Secret_ErrorPath_CreateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				dataplanev1grpc.SecretService_CreateSecret_FullMethodName,
				codes.Internal,
				cfg,
				"synthetic create failure",
			),
		},
	})
}

// TestIntegration_Secret_ErrorPath_UpdateFailed injects Internal on UpdateSecret
// during a secret_data_version bump (1→2). The Update handler's
// versionChanged path enters the UpdateSecret RPC and the override returns
// Internal. Same non-retryable surface as Create — the diagnostic message
// reaches the apply.
func TestIntegration_Secret_ErrorPath_UpdateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfgV1 := mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true)
	cfgV2 := mockSecretFullConfig(resourceName, dataValueV2, "bufnet", scopeConnect, 2, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, cfgV1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("secret_data_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.SecretService_UpdateSecret_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      cfgV2,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_Secret_ErrorPath_DeleteFailed covers the destroy-failed path.
// After a successful Create, an Internal-coded error is injected on the
// next DeleteSecret RPC. Internal is NOT one of the graceful-removal codes
// (NotFound / ClusterUnreachable / PermissionDenied) — it falls through to
// utils.HandleGracefulRemoval's ErrorNotHandled branch and surfaces as a
// TF diagnostic. The config uses allow_deletion=true so the in-resource
// guard at resource_secret.go:264 passes and DeleteSecret is actually
// called. After the failing step the override is consumed; the TestCase's
// terminal cleanup destroy runs against the untainted fake and removes the
// resource cleanly.
func TestIntegration_Secret_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	cfg := mockSecretFullConfig(resourceName, dataValueV1, "bufnet", scopeConnect, 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(secretAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("id"), knownvalue.StringExact(resourceName)),
				statecheck.ExpectKnownValue(secretAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.SecretService_DeleteSecret_FullMethodName,
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
