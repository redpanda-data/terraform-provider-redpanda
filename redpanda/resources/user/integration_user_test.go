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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestIntegration_User_ImportRoundTrip pins the "<user_name>,<cluster_id>"
// import ID. ImportState resolves cluster_api_url through GetCluster, so the
// cluster fake is seeded with DataplaneApi.Url="bufnet" to match the config.
// Verify ignores the password and allow_deletion attributes import cannot recover.
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

// TestIntegration_User_ErrorPath_UpdateUser_Failed pins that an UpdateUser
// error surfaces as a diagnostic. The injected code must stay Internal:
// Unavailable is retried, and the second attempt would reach the fake and
// succeed.
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

// TestIntegration_User_ErrorPath_DeleteUser_Failed pins that a DeleteUser
// error surfaces as a diagnostic. The injected code must stay Internal:
// HandleGracefulRemoval turns NotFound, ClusterUnreachable, and
// PermissionDenied into a state removal with no error.
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

// TestIntegration_User_ErrorPath_AllowDeletionBlocked pins the guard itself.
// UpdateLeaf_AllowDeletion proves the field round-trips; this proves what the
// field is for — with allow_deletion=false a destroy must be refused. The final
// step re-enables deletion so the framework's terminal destroy can proceed.
func TestIntegration_User_ErrorPath_AllowDeletionBlocked(t *testing.T) {
	_, factories := integration.Setup(t)

	noDelete := mockUserConfigFull("tfrp-mock-user-nodel", "scram-sha-256", "bufnet", 1, false)
	allowDelete := mockUserConfigFull("tfrp-mock-user-nodel", "scram-sha-256", "bufnet", 1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: noDelete,
				Check:  resource.TestCheckResourceAttr(userAddr, "allow_deletion", "false"),
			},
			{
				Config:      noDelete,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`user deletion not allowed`),
			},
			{
				Config: allowDelete,
				Check:  resource.TestCheckResourceAttr(userAddr, "allow_deletion", "true"),
			},
		},
	})
}

// TestIntegration_User_UpgradeState_NormalizesClusterApiUrl drives the v0->v1
// state upgrade through the provider server's UpgradeResourceState RPC and
// asserts the legacy host:443 cluster_api_url is rewritten to https://host so
// the format change alone does not force replacement.
func TestIntegration_User_UpgradeState_NormalizesClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)
	ctx := context.Background()
	schemaType := user.ResourceUserSchema(ctx).Type().TerraformType(ctx)

	const priorState = `{` +
		`"allow_deletion":true,` +
		`"cluster_api_url":"bufnet:443",` +
		`"id":"app",` +
		`"mechanism":"scram-sha-256",` +
		`"name":"app",` +
		`"password":null,` +
		`"password_wo":null,` +
		`"password_wo_version":null` +
		`}`

	upgraded := integration.UpgradeState(t, factories, "redpanda_user", 0, priorState, schemaType)

	var obj map[string]tftypes.Value
	require.NoError(t, upgraded.As(&obj))
	var got string
	require.NoError(t, obj["cluster_api_url"].As(&got))
	assert.Equal(t, "https://bufnet", got)
}
