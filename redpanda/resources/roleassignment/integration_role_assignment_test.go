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

package roleassignment_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
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

const raAddr = "redpanda_role_assignment.test"

// mockRAConfig produces an HCL config with a co-declared redpanda_role.owner
// (the role being assigned to) and the role_assignment under test. Both
// resources share cluster_api_url. The role uses allow_deletion = true so the
// terminal cleanup destroy succeeds.
func mockRAConfig(roleName, principal, clusterAPIURL string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_role" "owner" {
  name            = %q
  cluster_api_url = %q
  allow_deletion  = true
}

resource "redpanda_role_assignment" "test" {
  role_name       = redpanda_role.owner.name
  principal       = %q
  cluster_api_url = %q
}
`, roleName, clusterAPIURL, principal, clusterAPIURL)
}

// mockRATwoRolesConfig co-declares TWO roles (owner_a and owner_b) so a
// RequiresReplace-on-role_name step can flip between them — both roles must
// exist simultaneously in the bufconn fake to satisfy the
// destroy-old-assignment-then-create-new-assignment cycle that
// DestroyBeforeCreate triggers. activeRoleRef must be a Terraform expression
// referencing one of the two role resources (e.g.
// "redpanda_role.owner_a.name") so the framework establishes a graph edge
// from the assignment to the active role — without that edge, Terraform may
// create the assignment before the role exists and the assignment Create
// hits NotFound on UpdateRoleMembership.
func mockRATwoRolesConfig(roleNameA, roleNameB, activeRoleRef, principal, clusterAPIURL string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_role" "owner_a" {
  name            = %q
  cluster_api_url = %q
  allow_deletion  = true
}

resource "redpanda_role" "owner_b" {
  name            = %q
  cluster_api_url = %q
  allow_deletion  = true
}

resource "redpanda_role_assignment" "test" {
  role_name       = %s
  principal       = %q
  cluster_api_url = %q
}
`, roleNameA, clusterAPIURL, roleNameB, clusterAPIURL, activeRoleRef, principal, clusterAPIURL)
}

// TestIntegration_RoleAssignment exercises redpanda_role_assignment end-to-end
// against the bufconn-backed fake console SecurityService (shared with
// redpanda_role). The role is co-declared as a dependency since the real
// backend requires the role to exist before assignment.
func TestIntegration_RoleAssignment(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_role_assignment.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockRoleAssignmentConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "role_name", "tfrp-mock-role-for-assign"),
					resource.TestCheckResourceAttr(addr, "principal", "User:alice"),
					resource.TestCheckResourceAttr(addr, "id", "tfrp-mock-role-for-assign:User:alice"),
				),
			},
			{
				Config: mockRoleAssignmentConfig,
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
					resource.TestCheckResourceAttr(addr, "role_name", "tfrp-mock-role-for-assign"),
					resource.TestCheckResourceAttr(addr, "principal", "User:alice"),
				),
			},
		},
	})
}

const mockRoleAssignmentConfig = `
provider "redpanda" {}

resource "redpanda_role" "owner" {
  name            = "tfrp-mock-role-for-assign"
  cluster_api_url = "bufnet"
  allow_deletion  = true
}

resource "redpanda_role_assignment" "test" {
  role_name       = redpanda_role.owner.name
  principal       = "User:alice"
  cluster_api_url = "bufnet"
}
`

// TestIntegration_RoleAssignment_CreateAndRefresh covers the canonical Create +
// NoopReapply cycle and pins every leaf to an exact value. id is derived as
// "<role_name>:<principal>" by Create; the UseStateForUnknown plan modifier
// is proved by a single CompareValue(ValuesSame()) shared between the two
// steps that asserts id is IDENTICAL across Create and Noop.
func TestIntegration_RoleAssignment_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-role"
		principal = "User:alice"
		url       = "bufnet"
		wantID    = "tfrp-mock-ra-role:User:alice"
	)
	cfg := mockRAConfig(roleName, principal, url)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_RoleAssignment_PrincipalPreservation_User pins the contract that
// a "User:<name>" prefix supplied in config is preserved verbatim through
// Create and Read. The API accepts the prefixed form natively; the provider
// MUST NOT mutate the principal between plan and state, otherwise Terraform's
// plan/state consistency check rejects with "Provider produced inconsistent
// result after apply" on a Required field.
func TestIntegration_RoleAssignment_PrincipalPreservation_User(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-userprefix"
		principal = "User:alice"
		url       = "bufnet"
		wantID    = "tfrp-mock-ra-userprefix:User:alice"
	)
	cfg := mockRAConfig(roleName, principal, url)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_RoleAssignment_PrincipalPreservation_Group covers the GBAC path:
// a "Group:<name>" prefix is preserved verbatim through Create and Read. The
// backend accepts Group: principals; the provider must not strip the prefix.
func TestIntegration_RoleAssignment_PrincipalPreservation_Group(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-groupprefix"
		principal = "Group:engineers"
		url       = "bufnet"
		wantID    = "tfrp-mock-ra-groupprefix:Group:engineers"
	)
	cfg := mockRAConfig(roleName, principal, url)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_RoleAssignment_BarePrincipal_RejectedAtPlan pins the
// contract that a principal without a recognized type prefix
// (User:/Group:) is rejected at plan time. The dataplane SecurityService
// canonicalizes bare principals to "User:<name>" server-side, which causes
// Read drift (state has bare, ListRoleMembers returns prefixed). Rather
// than tolerate the drift, the schema validator rejects the bare form so
// the failure surfaces at plan, before any RPC is dispatched.
func TestIntegration_RoleAssignment_BarePrincipal_RejectedAtPlan(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-bare"
		principal = "alice"
		url       = "bufnet"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      mockRAConfig(roleName, principal, url),
				ExpectError: regexp.MustCompile(`(?i)principal.*prefix`),
			},
		},
	})
}

// TestIntegration_RoleAssignment_RequiresReplace_RoleName mutates the
// RequiresReplace role_name leaf. Both roles are co-declared so the destroy
// of the old assignment and the create of the new assignment both find their
// role in the fake. The load-bearing proof of DestroyBeforeCreate is that id
// — derived as "<role_name>:<principal>" — DIFFERS between the two steps.
func TestIntegration_RoleAssignment_RequiresReplace_RoleName(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleA     = "tfrp-mock-ra-rr-role-a"
		roleB     = "tfrp-mock-ra-rr-role-b"
		principal = "User:alice"
		url       = "bufnet"
		wantIDA   = "tfrp-mock-ra-rr-role-a:User:alice"
		wantIDB   = "tfrp-mock-ra-rr-role-b:User:alice"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, mockRATwoRolesConfig(roleA, roleB, "redpanda_role.owner_a.name", principal, url), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleA)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantIDA)),
				idChanged.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(raAddr, mockRATwoRolesConfig(roleA, roleB, "redpanda_role.owner_b.name", principal, url), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleB)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantIDB)),
				idChanged.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_RoleAssignment_RequiresReplace_Principal mutates the
// RequiresReplace principal leaf "User:alice" → "User:bob". id is principal-derived so
// it changes; ValuesDiffer asserts the destroy-then-create cycle ran.
func TestIntegration_RoleAssignment_RequiresReplace_Principal(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleName    = "tfrp-mock-ra-rr-princ"
		principalA  = "User:alice"
		principalB  = "User:bob"
		url         = "bufnet"
		wantIDAlice = "tfrp-mock-ra-rr-princ:User:alice"
		wantIDBob   = "tfrp-mock-ra-rr-princ:User:bob"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, mockRAConfig(roleName, principalA, url), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principalA)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantIDAlice)),
				idChanged.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(raAddr, mockRAConfig(roleName, principalB, url), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principalB)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantIDBob)),
				idChanged.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_RoleAssignment_RequiresReplace_ClusterAPIURL mutates the
// RequiresReplace cluster_api_url leaf "bufnet" → "bufnet2". The bufconn
// dialer is address-agnostic — it routes through the in-memory listener
// regardless of the URL string — so Create on the new resource still
// succeeds. id is "<role_name>:<principal>"-derived and unchanged across
// this RR, so ValuesSame holds on id; the load-bearing proof of RR is the
// PreApply ResourceActionDestroyBeforeCreate plancheck baked into
// RequiresReplaceStep.
func TestIntegration_RoleAssignment_RequiresReplace_ClusterAPIURL(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-rr-url"
		principal = "User:alice"
		url1      = "bufnet"
		url2      = "bufnet2"
		wantID    = "tfrp-mock-ra-rr-url:User:alice"
	)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, mockRAConfig(roleName, principal, url1), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url1)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(raAddr, mockRAConfig(roleName, principal, url2), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url2)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(raAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_RoleAssignment_Import covers the composite-ID import parser
// under two scenarios:
//
//   - "RoundTrip" — canonical "<role>:User:<name>" round-trips verbatim.
//     cluster_api_url is in ImportStateVerifyIgnore because the composite ID
//     does not carry the URL and ListRoleMembers cannot recover it; Read
//     skips the API check when the URL is empty so refresh succeeds.
//   - "LegacyBarePrincipal_SelfHeals" — a state file produced by a
//     pre-validator provider may carry a bare principal (e.g. "alice"). The
//     SecurityService canonicalizes to "User:alice"; Read canonicalizes
//     state before lookup and writes the canonical form back, so legacy
//     bare state self-heals to an empty plan.
func TestIntegration_RoleAssignment_Import(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		_, factories := integration.Setup(t)

		const (
			roleName  = "tfrp-mock-ra-import"
			principal = "User:alice"
			url       = "bufnet"
			wantID    = "tfrp-mock-ra-import:User:alice"
		)
		cfg := mockRAConfig(roleName, principal, url)

		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				integration.CreateStep(raAddr, cfg, []statecheck.StateCheck{
					statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
					statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
					statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
					statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				}),
				integration.ImportRoundTripStep(raAddr, nil, []string{"cluster_api_url"}),
			},
		})
	})

	t.Run("LegacyBarePrincipal_SelfHeals", func(t *testing.T) {
		srv, factories := integration.Setup(t)

		const (
			roleName    = "tfrp-mock-ra-import-legacy"
			principal   = "User:alice"
			url         = "bufnet"
			legacyID    = "tfrp-mock-ra-import-legacy:alice|bufnet"
			canonicalID = "tfrp-mock-ra-import-legacy:User:alice"
		)
		// Step 1 applies a role-only config so the role exists in both TF
		// state and the fake. Step 2's PreConfig drops a bare "alice"
		// directly into the fake (canonicalized to "User:alice" by the
		// fake) to simulate the membership a pre-validator provider would
		// have written. The import in step 2 uses the new pipe-suffix
		// format so the imported state is fully populated (no
		// cluster_api_url gap), then step 3's refresh asserts
		// ExpectEmptyPlan — proving Read's canonicalization is what closes
		// the drift, not any RequiresReplace fallout.
		roleOnly := fmt.Sprintf(`
provider "redpanda" {}
resource "redpanda_role" "owner" {
  name            = %q
  cluster_api_url = %q
  allow_deletion  = true
}
`, roleName, url)
		fullCfg := mockRAConfig(roleName, principal, url)
		legacyIDFunc := func(_ *terraform.State) (string, error) { return legacyID, nil }

		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				{
					Config: roleOnly,
				},
				{
					PreConfig: func() {
						srv.Security.SeedRoleWithMembers(roleName, "alice")
					},
					ResourceName:       raAddr,
					ImportState:        true,
					ImportStateIdFunc:  legacyIDFunc,
					ImportStatePersist: true,
					Config:             fullCfg,
				},
				{
					Config: fullCfg,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction(raAddr, plancheck.ResourceActionNoop),
						},
						PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
						statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
						statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(canonicalID)),
					},
				},
			},
		})
	})
}

// TestIntegration_RoleAssignment_ErrorPath_CreateRoleNotFound injects NotFound on
// UpdateRoleMembership at Create time. The provider's Create surfaces the
// gRPC error as a "Failed to assign role" diagnostic; ExpectError matches
// the regexp against the diagnostic text.
//
// Use mockRAConfig (which co-declares the role) so HCL parses cleanly and
// the role resource itself creates successfully; the OverrideOnce fires
// when the role_assignment then calls UpdateRoleMembership, simulating the
// upstream "role disappeared between role-create and assignment-create"
// race or any other NotFound the backend might return.
func TestIntegration_RoleAssignment_ErrorPath_CreateRoleNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-notfound"
		principal = "User:alice"
		url       = "bufnet"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				consolev1alpha1grpc.SecurityService_UpdateRoleMembership_FullMethodName,
				codes.NotFound,
				mockRAConfig(roleName, principal, url),
				"not found",
			),
		},
	})
}

// TestIntegration_RoleAssignment_ErrorPath_CreateFails injects Internal on
// UpdateRoleMembership at Create time. Provider surfaces the gRPC error as
// a "Failed to assign role" diagnostic.
func TestIntegration_RoleAssignment_ErrorPath_CreateFails(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-createfail"
		principal = "User:alice"
		url       = "bufnet"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				consolev1alpha1grpc.SecurityService_UpdateRoleMembership_FullMethodName,
				codes.Internal,
				mockRAConfig(roleName, principal, url),
				"synthetic create failure",
			),
		},
	})
}

// TestIntegration_RoleAssignment_ErrorPath_ReadHardError covers the
// Read-fails-without-graceful-removal path. After a successful Create, an
// Internal-coded error is injected on the next ListRoleMembers RPC.
//
// codes.Internal does NOT match the "not found" / "notfound" / "does not
// exist" / "unknown role" substring checks in resource_role_assignment.go's
// roleAssignmentExists — those substrings would cause exists=false and a
// clean state-removal. Internal falls through to the hard-error branch and
// surfaces as "Could not check if role assignment ... exists" via
// resp.Diagnostics.AddError. The Config re-apply triggers the Read; the
// override is consumed on that first call.
func TestIntegration_RoleAssignment_ErrorPath_ReadHardError(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-readfail"
		principal = "User:alice"
		url       = "bufnet"
		wantID    = "tfrp-mock-ra-readfail:User:alice"
	)
	cfg := mockRAConfig(roleName, principal, url)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						consolev1alpha1grpc.SecurityService_ListRoleMembers_FullMethodName,
						status.Error(codes.Internal, "synthetic read failure"),
					)
				},
				Config:      cfg,
				ExpectError: regexp.MustCompile("synthetic read failure"),
			},
		},
	})
}

// TestIntegration_RoleAssignment_ErrorPath_DeleteFails injects Internal on
// UpdateRoleMembership during Destroy. Provider surfaces it as
// "Failed to unassign role" diagnostic. The override is consumed on the
// failing Destroy; the TestCase's terminal cleanup destroy then runs
// against the untainted fake and removes the resource cleanly.
func TestIntegration_RoleAssignment_ErrorPath_DeleteFails(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		roleName  = "tfrp-mock-ra-delfail"
		principal = "User:alice"
		url       = "bufnet"
		wantID    = "tfrp-mock-ra-delfail:User:alice"
	)
	cfg := mockRAConfig(roleName, principal, url)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(raAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("role_name"), knownvalue.StringExact(roleName)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("principal"), knownvalue.StringExact(principal)),
				statecheck.ExpectKnownValue(raAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						consolev1alpha1grpc.SecurityService_UpdateRoleMembership_FullMethodName,
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
