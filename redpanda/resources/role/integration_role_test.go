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

package role_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
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

const roleAddr = "redpanda_role.test"

// mockRoleFullConfig produces an HCL config for redpanda_role with every
// configurable leaf explicit. The full surface lets each scenario assert all
// leaves end-to-end without leaving any to the schema default.
func mockRoleFullConfig(name, clusterAPIURL string, allowDeletion, deleteACLs bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_role" "test" {
  name            = %q
  cluster_api_url = %q
  allow_deletion  = %t
  delete_acls     = %t
}
`, name, clusterAPIURL, allowDeletion, deleteACLs)
}

// TestIntegration_Role exercises redpanda_role end-to-end against the bufconn-backed
// fake console SecurityService. Covers Create, refresh, no-op re-plan, and
// TF-local allow_deletion flip (Update has no RPC for role).
func TestIntegration_Role(t *testing.T) {
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
				Config: mockRoleConfig(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(roleAddr, "name", "tfrp-mock-role"),
					resource.TestCheckResourceAttr(roleAddr, "allow_deletion", "true"),
					resource.TestCheckResourceAttr(roleAddr, "id", "tfrp-mock-role"),
				),
			},
			{
				Config: mockRoleConfig(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleAddr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttr(roleAddr, "name", "tfrp-mock-role"),
			},
			{
				Config: mockRoleConfig(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleAddr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(roleAddr, "allow_deletion", "false"),
			},
			{
				Config: mockRoleConfig(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleAddr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(roleAddr, "allow_deletion", "true"),
			},
		},
	})
}

func mockRoleConfig(allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_role" "test" {
  name            = "tfrp-mock-role"
  cluster_api_url = "bufnet"
  allow_deletion  = %t
}
`, allowDeletion)
}

// TestIntegration_Role_CreateAndRefresh covers the canonical Create + NoopReapply
// cycle and pins every leaf to an exact value. The load-bearing proof for the
// id leaf's UseStateForUnknown plan modifier is that the server-derived id
// (which equals name per Create) is IDENTICAL across the two steps — a single
// CompareValue(ValuesSame()) shared between both steps' ConfigStateChecks
// accumulates the values and the comparer asserts equality.
func TestIntegration_Role_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name = "tfrp-mock-role-create"
		url  = "bufnet"
	)
	cfg := mockRoleFullConfig(name, url, true, false)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idPreserved.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(roleAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idPreserved.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Role_RequiresReplace_Name mutates the RequiresReplace `name` leaf
// and asserts the framework plans DestroyBeforeCreate. The load-bearing proof
// that the resource was actually destroyed and recreated (not updated
// in-place) is that the id DIFFERS between the two steps — id == name per
// Create, so the comparer captures "tfrp-mock-role-a" then "tfrp-mock-role-b"
// and ValuesDiffer asserts they are not equal.
func TestIntegration_Role_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		initialName = "tfrp-mock-role-a"
		renamedName = "tfrp-mock-role-b"
		url         = "bufnet"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, mockRoleFullConfig(initialName, url, true, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(initialName)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(initialName)),
				idChanged.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(roleAddr, mockRoleFullConfig(renamedName, url, true, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(renamedName)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(renamedName)),
				idChanged.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Role_RequiresReplace_ClusterAPIURL mutates the RequiresReplace
// `cluster_api_url` leaf. The bufconn dialer is address-agnostic — it ignores
// the URL string and routes through the in-memory listener — so changing
// "bufnet" → "bufnet2" triggers the plan-level DestroyBeforeCreate. id is
// name-derived and name is unchanged, so ValuesSame holds on id across the
// destroy+create; the RR proof is the PreApply ResourceActionDestroyBeforeCreate
// plancheck baked into RequiresReplaceStep.
func TestIntegration_Role_RequiresReplace_ClusterAPIURL(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name = "tfrp-mock-role-rr-url"
		url1 = "bufnet"
		url2 = "bufnet2"
	)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, mockRoleFullConfig(name, url1, true, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url1)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idStable.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(roleAddr, mockRoleFullConfig(name, url2, true, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url2)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idStable.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Role_UpdateLeaf_AllowDeletion exercises the in-place Update path
// for the TF-only `allow_deletion` and `delete_acls` extras. Role has no
// UpdateRole RPC (roles are immutable); the Update handler in
// resource_role.go just writes plan to state. Flipping allow_deletion
// true→false→true and then delete_acls false→true must each produce
// ResourceActionUpdate (not DestroyBeforeCreate) and the id must remain
// stable across all four steps — proof that the in-place path actually
// runs. We end with allow_deletion=true so the terminal cleanup destroy
// succeeds (Delete blocks when allow_deletion=false).
func TestIntegration_Role_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name = "tfrp-mock-role-update"
		url  = "bufnet"
	)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, mockRoleFullConfig(name, url, true, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idStable.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(roleAddr, mockRoleFullConfig(name, url, false, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idStable.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(roleAddr, mockRoleFullConfig(name, url, true, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idStable.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(roleAddr, mockRoleFullConfig(name, url, true, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("delete_acls"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				idStable.AddStateValue(roleAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Role_ImportRoundTrip verifies that role's ImportState correctly
// reconstructs resource state when the cluster_id resolves through the
// bufconn-backed ClusterFake.
func TestIntegration_Role_ImportRoundTrip(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Cluster.SetClusterByID("cgg5hmkar1m4l0pjg6tg", "bufnet")

	const roleName = "tfrp-mock-role-import"
	cfg := mockRoleFullConfig(roleName, "bufnet", true, false)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, cfg, nil),
			// ImportState sets allow_deletion from schema default; delete_acls is not set.
			integration.ImportRoundTripStep(roleAddr, func(_ *terraform.State) (string, error) {
				return roleName + ",cgg5hmkar1m4l0pjg6tg", nil
			}, []string{"allow_deletion", "delete_acls"}),
		},
	})
}

// TestIntegration_Role_ErrorPath_GetNotFound covers the Read→NotFound path. After
// a successful Create, the role is removed from the fake's store
// out-of-band via a direct DeleteRole call on the fake. The next plan
// triggers Read, which sees NotFound (via roleExists's substring match on
// the error message), calls RemoveResource, and TF plans a re-create.
// PreApply asserts ResourceActionCreate; PostApplyPostRefresh asserts an
// empty plan after the re-create lands.
func TestIntegration_Role_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name = "tfrp-mock-role-notfound"
		url  = "bufnet"
	)
	cfg := mockRoleFullConfig(name, url, true, false)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
			}),
			{
				PreConfig: func() {
					_, err := srv.Security.DeleteRole(context.Background(), &consolev1alpha1.DeleteRoleRequest{
						Request: &dataplanev1.DeleteRoleRequest{RoleName: name},
					})
					if err != nil {
						t.Fatalf("PreConfig: delete role %q: %v", name, err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
				},
			},
		},
	})
}

// TestIntegration_Role_ErrorPath_CreateAlreadyExists injects AlreadyExists on
// CreateRole. The provider's Create surfaces the gRPC error as a
// diagnostic; ExpectError matches the regexp against the diagnostic text.
func TestIntegration_Role_ErrorPath_CreateAlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name = "tfrp-mock-role-exists"
		url  = "bufnet"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				consolev1alpha1grpc.SecurityService_CreateRole_FullMethodName,
				codes.AlreadyExists,
				mockRoleFullConfig(name, url, true, false),
				"already exists",
			),
		},
	})
}

// TestIntegration_Role_ErrorPath_DeleteFailed covers the destroy-failed path.
// After a successful Create, an Internal-coded error is injected on the next
// DeleteRole RPC. Internal is NOT one of the graceful-removal codes
// (NotFound / ClusterUnreachable / PermissionDenied) that
// utils.HandleGracefulRemoval treats as a clean drop-from-state — it falls
// through to the ErrorNotHandled branch and surfaces as a TF diagnostic.
// The Destroy:true step triggers the destroy plan; ExpectError matches the
// regexp. After the failing step the override is consumed; the TestCase's
// terminal cleanup destroy runs against the untainted fake and removes the
// resource cleanly.
func TestIntegration_Role_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name = "tfrp-mock-role-delfail"
		url  = "bufnet"
	)
	cfg := mockRoleFullConfig(name, url, true, false)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(roleAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(roleAddr, tfjsonpath.New("id"), knownvalue.StringExact(name)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						consolev1alpha1grpc.SecurityService_DeleteRole_FullMethodName,
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
