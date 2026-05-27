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

package user_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
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

const userAddr = "redpanda_user.test"

// TestIntegration_User exercises redpanda_user end-to-end against the bufconn-backed
// fake dataplane. Covers Create with password_wo (write-only), refresh,
// no-op re-plan, mechanism update via UpdateUser RPC, and password_wo_version
// bump triggering a password rewrite.
func TestIntegration_User(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockUserConfig("scram-sha-256", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userAddr, "name", "tfrp-mock-user"),
					resource.TestCheckResourceAttr(userAddr, "mechanism", "scram-sha-256"),
					resource.TestCheckResourceAttr(userAddr, "allow_deletion", "true"),
					resource.TestCheckResourceAttrSet(userAddr, "id"),
				),
			},
			{
				Config: mockUserConfig("scram-sha-256", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userAddr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userAddr, "name", "tfrp-mock-user"),
					resource.TestCheckResourceAttr(userAddr, "mechanism", "scram-sha-256"),
				),
			},
			{
				Config: mockUserConfig("scram-sha-512", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userAddr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(userAddr, "mechanism", "scram-sha-512"),
			},
			{
				Config: mockUserConfig("scram-sha-512", 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userAddr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(userAddr, "password_wo_version", "2"),
			},
		},
	})
}

func mockUserConfig(mechanism string, passwordWOVersion int) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_user" "test" {
  name                = "tfrp-mock-user"
  password_wo         = "mock-password-12345"
  password_wo_version = %d
  mechanism           = %q
  cluster_api_url     = "bufnet"
  allow_deletion      = true
}
`, passwordWOVersion, mechanism)
}

// mockUserConfigFull renders the standard integration HCL for redpanda_user. All
// non-import scenarios use this shape; mechanism, name, password_wo_version,
// cluster_api_url, and allow_deletion are parameterized.
func mockUserConfigFull(name, mechanism, clusterAPIURL string, passwordWOVersion int, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_user" "test" {
  name                = %q
  password_wo         = "mock-password-12345"
  password_wo_version = %d
  mechanism           = %q
  cluster_api_url     = %q
  allow_deletion      = %t
}
`, name, passwordWOVersion, mechanism, clusterAPIURL, allowDeletion)
}

// mockUserConfigImport renders an import-friendly HCL: no password_wo_version
// (import doesn't restore it). allow_deletion=true so the TestCase's terminal
// cleanup destroy succeeds (the resource's Delete path rejects with "user
// deletion not allowed" when allow_deletion=false). Mismatch between this
// config's `true` and the import-time schema default of `false` is bridged by
// ImportStateVerifyIgnore.
func mockUserConfigImport(name, mechanism, clusterAPIURL string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_user" "test" {
  name            = %q
  password_wo     = "mock-password-12345"
  mechanism       = %q
  cluster_api_url = %q
  allow_deletion  = true
}
`, name, mechanism, clusterAPIURL)
}

// TestIntegration_User_CreateAndRefresh validates the Create + no-op re-plan cycle.
// Every leaf is asserted at exact value post-create. The id leaf is Computed +
// UseStateForUnknown; mechanism is Optional+Computed+UseStateForUnknown. Both
// are pinned across the noop step via CompareValue(ValuesSame()) instances —
// the framework calls CheckState once per step, the checker accumulates
// values, and the comparer asserts equality once two values are present.
// password is Null (not set in config); password_wo is Null (WriteOnly, never
// in state).
func TestIntegration_User_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-user-create"
	cfg := mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())
	mechanismPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password"), knownvalue.Null()),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo"), knownvalue.Null()),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(userAddr, tfjsonpath.New("id")),
				mechanismPreserved.AddStateValue(userAddr, tfjsonpath.New("mechanism")),
			}),
			integration.NoopReapplyStep(userAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password"), knownvalue.Null()),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo"), knownvalue.Null()),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(userAddr, tfjsonpath.New("id")),
				mechanismPreserved.AddStateValue(userAddr, tfjsonpath.New("mechanism")),
			}),
		},
	})
}

// TestIntegration_User_UpdateLeaf_Mechanism mutates mechanism in-place (scram-sha-256
// → scram-sha-512) and asserts the framework plans Update. The load-bearing
// proof that the resource was updated in-place (not replaced) is that id is
// IDENTICAL across both steps — a single CompareValue(ValuesSame()) instance
// captures the pre- and post-update ids and the comparer asserts equality.
func TestIntegration_User_UpdateLeaf_Mechanism(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-user-mech"
	cfg1 := mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true)
	cfg2 := mockUserConfigFull(name, "scram-sha-512", "bufnet", 1, true)

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(userAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-512")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_User_UpdateLeaf_Password bumps password_wo_version (1→2) to trigger
// a password rewrite via the UpdateUser RPC. The WriteOnly contract is
// asserted post-update: password_wo remains Null in state (never persisted).
// mechanism is unchanged (UseStateForUnknown) and id is identical across both
// steps (in-place update, not replace).
func TestIntegration_User_UpdateLeaf_Password(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-user-pwd"
	cfg1 := mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true)
	cfg2 := mockUserConfigFull(name, "scram-sha-256", "bufnet", 2, true)

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo"), knownvalue.Null()),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(userAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo_version"), knownvalue.Int64Exact(2)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("password_wo"), knownvalue.Null()),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_User_RequiresReplace_Name mutates the RequiresReplace `name` leaf
// and asserts the framework plans DestroyBeforeCreate. The load-bearing proof
// that the resource was actually destroyed and recreated (rather than updated
// in-place) is that the server-assigned id DIFFERS between the two steps — a
// single CompareValue(ValuesDiffer()) instance shared across both steps
// captures the pre- and post-replace ids and the comparer asserts they are not
// equal. id is the user's name (Flatten copies name → id), so a name change
// implies an id change.
func TestIntegration_User_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		nameA = "tfrp-mock-user-rr-a"
		nameB = "tfrp-mock-user-rr-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, mockUserConfigFull(nameA, "scram-sha-256", "bufnet", 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(userAddr, mockUserConfigFull(nameB, "scram-sha-256", "bufnet", 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameB)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_User_RequiresReplace_ClusterApiUrl mutates the RequiresReplace
// `cluster_api_url` leaf. The bufconn dialer is address-agnostic — it ignores
// the URL string and routes through the in-memory listener — so changing
// "bufnet" → "bufnet2" triggers the plan-level DestroyBeforeCreate and the
// Create on the new resource still succeeds. id is name-derived (Flatten
// copies name → id) and name doesn't change in this test, so ValuesSame
// holds across the replace; idStable mirrors the sibling role test
// (TestIntegration_Role_RequiresReplace_ClusterAPIURL).
func TestIntegration_User_RequiresReplace_ClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-user-url"

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(userAddr, mockUserConfigFull(name, "scram-sha-256", "bufnet2", 1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet2")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idStable.AddStateValue(userAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_User_ImportRoundTrip exercises the composite import ID format
// "<user_name>,<cluster_id>". The user resource's ImportState calls
// ClusterForID (a controlplane GetCluster RPC) to resolve cluster_api_url from
// the cluster's DataplaneApi.Url. The cluster fake is seeded with a known id
// and DataplaneApi.Url="bufnet" so the import path resolves to the same URL
// as the create config.
//
// ImportStateVerifyIgnore covers:
//   - password_wo: write-only, never in state (verify expects every config
//     attr to roundtrip; write-only requires an explicit ignore).
//   - password_wo_version: not restored by ImportState (the import ID format
//     only carries user_name + cluster_id + optional password + mechanism).
//   - allow_deletion: ImportState writes the schema default (false) via
//     ImportStateBoolFromSchemaDefault, but the test config sets it to true
//     so the framework's terminal cleanup destroy succeeds.
func TestIntegration_User_ImportRoundTrip(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name      = "tfrp-mock-user-import"
		clusterID = "mockuserclusterid001" // must be exactly 20 chars per GetClusterRequest validator
	)

	srv.Cluster.Seed(&controlplanev1.Cluster{
		Id:    clusterID,
		Name:  "mock-user-import-cluster",
		State: controlplanev1.Cluster_STATE_READY,
		DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
			Url: "bufnet",
		},
	})

	cfg := mockUserConfigImport(name, "scram-sha-256", "bufnet")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(userAddr, func(s *terraform.State) (string, error) {
				rs, ok := s.RootModule().Resources[userAddr]
				if !ok {
					return "", fmt.Errorf("resource %q not found in state", userAddr)
				}
				return rs.Primary.Attributes["name"] + "," + clusterID, nil
			}, []string{"password_wo", "password_wo_version", "allow_deletion"}),
		},
	})
}

// TestIntegration_User_ErrorPath_GetUser_NotFound covers the Read→NotFound path. The
// user is deleted from the fake's store out-of-band via the fake's own
// DeleteUser RPC so the next ListUsers (driven by FindUserByName) returns an
// empty list — which the provider's FindUserByName converts to a NotFound
// error. HandleGracefulRemoval recognizes NotFound and returns RemoveFromState
// regardless of allow_deletion, so the provider's Read drops the resource
// from state and the next plan sees the resource missing → re-Create.
// PreApply asserts ResourceActionCreate; PostApplyPostRefresh asserts an empty
// plan after the re-create lands.
func TestIntegration_User_ErrorPath_GetUser_NotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-user-notfound"
	cfg := mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					if _, err := srv.User.DeleteUser(context.Background(),
						&dataplanev1.DeleteUserRequest{Name: name}); err != nil {
						t.Fatalf("PreConfig: delete user %q: %v", name, err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_User_ErrorPath_CreateUser_AlreadyExists injects AlreadyExists on
// CreateUser. The provider's Create has a special branch that probes
// ListUsers for an existing user on AlreadyExists (to adopt a lost-response
// retry); since no user is pre-seeded in the fake, the probe returns NotFound
// and Create surfaces the original AlreadyExists as a Terraform diagnostic.
// ExpectError matches the regexp against the diagnostic text.
func TestIntegration_User_ErrorPath_CreateUser_AlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-user-exists"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				dataplanev1grpc.UserService_CreateUser_FullMethodName,
				codes.AlreadyExists,
				mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true),
				"AlreadyExists",
			),
		},
	})
}

// TestIntegration_User_ErrorPath_UpdateUser_Failed injects an Internal-coded error on
// the next UpdateUser RPC. After a successful Create, the second step flips
// mechanism (in-place update) — the override is consumed by the UpdateUser
// call, which surfaces as a "failed to update user" diagnostic. ExpectError
// matches the regexp.
//
// Why Internal (not Unavailable): the provider retries on Unavailable via
// utils.Retry with a 2-minute budget. A single OverrideOnce(Unavailable)
// would cause one failed attempt then the second attempt would fall through
// to the real fake and succeed — masking the error. Internal is non-retryable
// and correctly exercises the resp.Diagnostics.AddError path. Do not flip
// this to Unavailable.
func TestIntegration_User_ErrorPath_UpdateUser_Failed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-user-updfail"
	cfg1 := mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true)
	cfg2 := mockUserConfigFull(name, "scram-sha-512", "bufnet", 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("mechanism"), knownvalue.StringExact("scram-sha-256")),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.UserService_UpdateUser_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      cfg2,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_User_ErrorPath_DeleteUser_Failed covers the destroy-failed path.
// After a successful Create, an Internal-coded error is injected on the next
// DeleteUser. The Destroy:true step triggers the destroy plan; ExpectError
// matches the error regexp. After this step the override is consumed; the
// TestCase's terminal cleanup destroy runs against the untainted fake and
// removes the resource cleanly.
//
// Why Internal (not PermissionDenied): the provider's Delete path runs the
// error through HandleGracefulRemoval, which treats NotFound,
// ClusterUnreachable, and PermissionDenied as graceful-removal cases (with
// allow_deletion=true they become RemoveFromState warnings, no error
// diagnostic). Internal is NOT in the graceful list so it surfaces as an
// "Failed to delete user" error diagnostic — which is the test-visible path
// we want to exercise. Do not flip this to PermissionDenied.
func TestIntegration_User_ErrorPath_DeleteUser_Failed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-user-delfail"
	cfg := mockUserConfigFull(name, "scram-sha-256", "bufnet", 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(userAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(userAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.UserService_DeleteUser_FullMethodName,
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
