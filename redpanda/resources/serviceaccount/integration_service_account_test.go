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

package serviceaccount_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/iam/v1/iamv1grpc"
	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
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

const saAddr = "redpanda_service_account.test"

// TestIntegration_ServiceAccount exercises redpanda_service_account end-to-end
// against the bufconn-backed IAM fake. Covers Create with synthetic
// client_secret returned once, refresh (verifies the provider's
// preserveClientSecretFromPrev carries the secret forward), no-op,
// description update (Update RPC with FieldMask=[description]), and name
// update (FieldMask=[name]).
func TestIntegration_ServiceAccount(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_service_account.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockServiceAccountConfig("tfrp-mock-sa", "initial description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-sa"),
					resource.TestCheckResourceAttr(addr, "description", "initial description"),
					resource.TestCheckResourceAttrSet(addr, "id"),
					resource.TestCheckResourceAttrSet(addr, "auth0_client_credentials.client_id"),
					resource.TestCheckResourceAttrSet(addr, "auth0_client_credentials.client_secret"),
				),
			},
			{
				Config: mockServiceAccountConfig("tfrp-mock-sa", "initial description"),
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
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-sa"),
					resource.TestCheckResourceAttr(addr, "description", "initial description"),
					resource.TestCheckResourceAttrSet(addr, "auth0_client_credentials.client_secret"),
				),
			},
			{
				Config: mockServiceAccountConfig("tfrp-mock-sa", "updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "description", "updated description"),
					resource.TestCheckResourceAttrSet(addr, "auth0_client_credentials.client_secret"),
				),
			},
			{
				Config: mockServiceAccountConfig("tfrp-mock-sa-renamed", "updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-sa-renamed"),
					resource.TestCheckResourceAttrSet(addr, "auth0_client_credentials.client_secret"),
				),
			},
		},
	})
}

func mockServiceAccountConfig(name, description string) string {
	return mockServiceAccountConfigFull(name, description)
}

// mockServiceAccountConfigFull renders the standard integration HCL for
// redpanda_service_account. All hand-rolled scenarios use this shape;
// name and description are parameterized. The backend requires at least
// one role binding at create.
func mockServiceAccountConfigFull(name, description string) string {
	return mockServiceAccountConfigWithBinding(name, description, "Reader", "fake-rg-id")
}

func mockServiceAccountConfigWithBinding(name, description, roleName, resourceID string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_service_account" "test" {
  name        = %q
  description = %q
  role_bindings = [
    {
      role_name = %q
      scope = {
        resource_type = "RESOURCE_GROUP"
        resource_id   = %q
      }
    },
  ]
}
`, name, description, roleName, resourceID)
}

// mockServiceAccountConfigNoBindings renders an SA without role_bindings —
// the shape the backend now rejects at create.
func mockServiceAccountConfigNoBindings(name, description string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_service_account" "test" {
  name        = %q
  description = %q
}
`, name, description)
}

// clientSecretPath is the tfjsonpath for the nested write-only-on-create
// client_secret leaf. Captured once so the long expression stays out of
// every state check.
func clientSecretPath() tfjsonpath.Path {
	return tfjsonpath.New("auth0_client_credentials").AtMapKey("client_secret")
}

// clientIDPath is the tfjsonpath for the nested client_id leaf.
func clientIDPath() tfjsonpath.Path {
	return tfjsonpath.New("auth0_client_credentials").AtMapKey("client_id")
}

// bindingPath is the tfjsonpath for a leaf of the first role_bindings element.
func bindingPath(leaf string) tfjsonpath.Path {
	return tfjsonpath.New("role_bindings").AtSliceIndex(0).AtMapKey(leaf)
}

// bindingScopePath is the tfjsonpath for a leaf of the first binding's scope.
func bindingScopePath(leaf string) tfjsonpath.Path {
	return tfjsonpath.New("role_bindings").AtSliceIndex(0).AtMapKey("scope").AtMapKey(leaf)
}

// TestIntegration_ServiceAccount_CreateAndRefresh validates the Create + no-op
// re-plan cycle. Every leaf is asserted at exact value post-create.
// CompareValue(ValuesSame()) instances pin id and client_secret across
// the noop step: id proves UseStateForUnknown; client_secret proves the
// provider's preserveClientSecretFromPrev fires.
func TestIntegration_ServiceAccount_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-create"
		description = "initial description for create"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())
	secretPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact(description)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientIDPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, bindingPath("role_name"), knownvalue.StringExact("Reader")),
				statecheck.ExpectKnownValue(saAddr, bindingScopePath("resource_type"), knownvalue.StringExact("RESOURCE_GROUP")),
				statecheck.ExpectKnownValue(saAddr, bindingScopePath("resource_id"), knownvalue.StringExact("fake-rg-id")),
				statecheck.ExpectKnownValue(saAddr, bindingScopePath("dataplane_id"), knownvalue.Null()),
				idPreserved.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretPreserved.AddStateValue(saAddr, clientSecretPath()),
			}),
			integration.NoopReapplyStep(saAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact(description)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientIDPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, bindingPath("role_name"), knownvalue.StringExact("Reader")),
				idPreserved.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretPreserved.AddStateValue(saAddr, clientSecretPath()),
			}),
		},
	})
}

// TestIntegration_ServiceAccount_NoOpUpdate_PreservesClientSecret is a
// standalone preservation test for client_secret across a Create + Noop
// reapply. Named so a future reader immediately recognizes which contract
// is pinned. The CompareValue(ValuesSame()) instance captures the secret
// post-Create and post-Noop; the comparer
// fires on regression if preserveClientSecretFromPrev is removed or
// broken (Read would emit a Null secret since FlattenAuth0ClientCredentials
// only sets ClientID; absent the preservation hook the secret would shift
// from "fake-client-secret-N" to null between the two AddStateValue calls).
func TestIntegration_ServiceAccount_NoOpUpdate_PreservesClientSecret(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-noop"
		description = "noop description"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	secretSame := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				secretSame.AddStateValue(saAddr, clientSecretPath()),
			}),
			integration.NoopReapplyStep(saAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				secretSame.AddStateValue(saAddr, clientSecretPath()),
			}),
		},
	})
}

// TestIntegration_ServiceAccount_UpdateLeaf_Description mutates description in
// place. The fake's UpdateServiceAccount honors the FieldMask; the
// PostApplyPostRefresh ExpectEmptyPlan plus the secretPreserved
// CompareValue(ValuesSame()) prove unmasked fields (client_secret,
// client_id, name) survive the partial update. id is identical across
// both steps via idUnchanged — proves the change was in-place, not a
// replace.
func TestIntegration_ServiceAccount_UpdateLeaf_Description(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-sa-desc"
	cfg1 := mockServiceAccountConfigFull(name, "desc-v1")
	cfg2 := mockServiceAccountConfigFull(name, "desc-v2")

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())
	secretPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact("desc-v1")),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				idUnchanged.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretPreserved.AddStateValue(saAddr, clientSecretPath()),
			}),
			integration.UpdateLeafStep(saAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact("desc-v2")),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientIDPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				idUnchanged.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretPreserved.AddStateValue(saAddr, clientSecretPath()),
			}),
		},
	})
}

// TestIntegration_ServiceAccount_UpdateLeaf_Name mutates name in place. id is the
// server-assigned 20-char UUID (NOT name-derived), so an in-place name
// update must leave id identical — idUnchanged (ValuesSame) pins this.
// secretPreserved (ValuesSame) proves the FieldMask=[name] update doesn't
// disturb the unmasked client_secret.
func TestIntegration_ServiceAccount_UpdateLeaf_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		nameA       = "tfrp-mock-sa-name-a"
		nameB       = "tfrp-mock-sa-name-b"
		description = "name update description"
	)
	cfg1 := mockServiceAccountConfigFull(nameA, description)
	cfg2 := mockServiceAccountConfigFull(nameB, description)

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())
	secretPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				idUnchanged.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretPreserved.AddStateValue(saAddr, clientSecretPath()),
			}),
			integration.UpdateLeafStep(saAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameB)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact(description)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientIDPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
				idUnchanged.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretPreserved.AddStateValue(saAddr, clientSecretPath()),
			}),
		},
	})
}

// TestIntegration_ServiceAccount_ImportRoundTrip exercises the composite-ID
// import format "<id>:<client_secret>". client_secret is server-issued exactly
// once on Create and never echoed by subsequent reads, so the operator must
// supply the secret they captured at creation time as the second segment of
// the import ID. With the composite form, post-import state contains the
// secret verbatim and ImportStateVerify confirms the round-trip — every leaf,
// including client_secret, matches the pre-import state. role_bindings is
// ignored: it is Create-only, never echoed by the server, and therefore null
// after import by design.
func TestIntegration_ServiceAccount_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-import"
		description = "import description"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact(description)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(saAddr, func(s *terraform.State) (string, error) {
				rs, ok := s.RootModule().Resources[saAddr]
				if !ok {
					return "", fmt.Errorf("resource %q not found in state", saAddr)
				}
				secret := rs.Primary.Attributes["auth0_client_credentials.client_secret"]
				if secret == "" {
					return "", errors.New("auth0_client_credentials.client_secret missing from state at import time")
				}
				return rs.Primary.ID + ":" + secret, nil
			}, []string{"role_bindings"}),
		},
	})
}

// TestIntegration_ServiceAccount_Import_BareID_Errors asserts that the
// composite-ID requirement fails loudly when the user attempts to import with
// only the resource ID and no client_secret. A silent null secret would
// propagate through every downstream output that consumes it; the explicit
// error tells the operator they need the captured-at-creation value.
func TestIntegration_ServiceAccount_Import_BareID_Errors(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-import-bare"
		description = "import bare description"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg, nil),
			{
				ResourceName: saAddr,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[saAddr]
					if !ok {
						return "", fmt.Errorf("resource %q not found in state", saAddr)
					}
					return rs.Primary.ID, nil
				},
				ExpectError: regexp.MustCompile(`(?s)invalid import ID.*<id>:<client_secret>`),
			},
		},
	})
}

// TestIntegration_ServiceAccount_ClientSecret_FullLifecycle_ExactValue is the
// prophylactic pin for the H9 refactor that collapses injectClientSecret +
// preserveClientSecretFromPrev into a single HasClientSecret()-aware Flatten
// path. The IAM contract gives client_secret exactly once — in
// CreateServiceAccountResponse — so the secret in state must thread through
// every subsequent operation (Refresh, Update with FieldMask, no-op apply)
// purely via the provider's preserve / inject machinery. This test walks the
// full lifecycle in one TestCase and pins the secret two ways at every step:
//
//   - knownvalue.StringExact("fake-client-secret-1") — the exact deterministic
//     value the fake produces on the single Create in this test. Catches any
//     silent rewrite (e.g., null + later re-injection of a fresh value).
//   - secretPin (CompareValue(ValuesSame())) — one accumulator across all five
//     steps. Catches drift even if the value happened to land on something
//     else NotNull.
//
// Existing tests already pin shorter intervals (Create+Noop, Create+Update);
// the gap this fills is one accumulator across every transition the refactor
// would touch.
func TestIntegration_ServiceAccount_ClientSecret_FullLifecycle_ExactValue(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name    = "tfrp-mock-sa-lifecycle"
		renamed = "tfrp-mock-sa-lifecycle-renamed"
		descV1  = "lifecycle desc v1"
		descV2  = "lifecycle desc v2"
	)
	cfgInitial := mockServiceAccountConfigFull(name, descV1)
	cfgDescUpdate := mockServiceAccountConfigFull(name, descV2)
	cfgRename := mockServiceAccountConfigFull(renamed, descV2)

	secretPin := statecheck.CompareValue(compare.ValuesSame())

	// The fake produces "fake-client-secret-N" on each Create where N is its
	// monotonic counter; first Create per Setup yields seq=1.
	secretChecks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.StringExact("fake-client-secret-1")),
			secretPin.AddStateValue(saAddr, clientSecretPath()),
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfgInitial, secretChecks()),
			integration.NoopReapplyStep(saAddr, cfgInitial, secretChecks()),
			integration.UpdateLeafStep(saAddr, cfgDescUpdate, secretChecks()),
			integration.UpdateLeafStep(saAddr, cfgRename, secretChecks()),
			integration.NoopReapplyStep(saAddr, cfgRename, secretChecks()),
		},
	})
}

// TestIntegration_ServiceAccount_ErrorPath_GetSA_NotFound covers the Read→NotFound
// path. The SA is deleted from the fake's store out-of-band via its
// DeleteServiceAccount RPC; the next plan's Read returns NotFound, the
// provider calls RemoveResource, and the next plan sees the resource
// missing → re-Create. PreApply asserts ResourceActionCreate;
// PostApplyPostRefresh asserts an empty plan after the re-create lands.
func TestIntegration_ServiceAccount_ErrorPath_GetSA_NotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-notfound"
		description = "notfound description"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	var capturedID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(saAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
				Check: resource.TestCheckResourceAttrWith(saAddr, "id", func(v string) error {
					capturedID = v
					return nil
				}),
			},
			{
				PreConfig: func() {
					if capturedID == "" {
						t.Fatal("PreConfig: capturedID is empty; expected id captured from step 1")
					}
					if _, err := srv.ServiceAccount.DeleteServiceAccount(context.Background(),
						&iamv1.DeleteServiceAccountRequest{Id: capturedID}); err != nil {
						t.Fatalf("PreConfig: delete service account %q: %v", capturedID, err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(saAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_ServiceAccount_ErrorPath_CreateSA_Internal injects an
// Internal-coded error on CreateServiceAccount. Internal is non-retryable;
// the provider surfaces it as a "failed to create service account"
// diagnostic which the ExpectError regexp matches.
//
// Why Internal (not AlreadyExists): unlike the user resource, the SA
// provider's Create does NOT have a "probe-and-adopt on AlreadyExists"
// branch. Either code surfaces as a diagnostic; Internal is the standard
// non-retryable choice across this package.
func TestIntegration_ServiceAccount_ErrorPath_CreateSA_Internal(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-createfail"
		description = "create fail description"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				iamv1grpc.ServiceAccountService_CreateServiceAccount_FullMethodName,
				codes.Internal,
				cfg,
				"synthetic create failure",
			),
		},
	})
}

// TestIntegration_ServiceAccount_ErrorPath_UpdateSA_Internal injects an
// Internal-coded error on the next UpdateServiceAccount RPC. After a
// successful Create, the second step flips description (in-place update)
// — the override is consumed by the UpdateServiceAccount call, which
// surfaces as a "failed to update service account" diagnostic.
//
// Why Internal (not Unavailable): the provider retries on Unavailable
// via utils.Retry with a 2-minute budget. A single OverrideOnce(Unavailable)
// would cause one failed attempt then the second attempt would fall
// through to the real fake and succeed — masking the error. Internal is
// non-retryable and correctly exercises the diagnostic path.
func TestIntegration_ServiceAccount_ErrorPath_UpdateSA_Internal(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-sa-updfail"
	cfg1 := mockServiceAccountConfigFull(name, "updfail-v1")
	cfg2 := mockServiceAccountConfigFull(name, "updfail-v2")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("description"), knownvalue.StringExact("updfail-v1")),
				statecheck.ExpectKnownValue(saAddr, clientIDPath(), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						iamv1grpc.ServiceAccountService_UpdateServiceAccount_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      cfg2,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_ServiceAccount_ErrorPath_DeleteSA_Internal covers the
// destroy-failed path. After a successful Create, an Internal-coded error
// is injected on the next DeleteServiceAccount. The Destroy:true step
// triggers the destroy plan; ExpectError matches the regexp. After this
// step the override is consumed; the TestCase's terminal cleanup destroy
// runs against the untainted fake and removes the resource cleanly.
//
// Why Internal (not NotFound): NotFound on Delete makes the provider call
// resp.State.RemoveResource gracefully (no error diagnostic). Internal is
// NOT in the graceful set so it surfaces as a "failed to delete service
// account" error diagnostic — which is the test-visible path we want to
// exercise.
func TestIntegration_ServiceAccount_ErrorPath_DeleteSA_Internal(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-delfail"
		description = "delfail description"
	)
	cfg := mockServiceAccountConfigFull(name, description)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(saAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(saAddr, clientSecretPath(), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						iamv1grpc.ServiceAccountService_DeleteServiceAccount_FullMethodName,
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

// TestIntegration_ServiceAccount_Create_NoBindings_Errors pins the backend's
// new create contract (reproduced from Buildkite build 1435): a service
// account created without role_bindings is rejected with InvalidArgument
// before any resource is written.
func TestIntegration_ServiceAccount_Create_NoBindings_Errors(t *testing.T) {
	_, factories := integration.Setup(t)

	cfg := mockServiceAccountConfigNoBindings("tfrp-mock-sa-nobind", "no bindings description")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?s)service account must be created with at least one role\s+binding`),
			},
		},
	})
}

// TestIntegration_ServiceAccount_RequiresReplace_RoleBindings flips a binding's
// role_name and asserts a destroy-before-create replace: role_bindings is
// Create-only on the proto, so the only way to change it is to recreate the
// service account. id and client_secret both change across the replace.
func TestIntegration_ServiceAccount_RequiresReplace_RoleBindings(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name        = "tfrp-mock-sa-bindrepl"
		description = "binding replace description"
	)
	cfg1 := mockServiceAccountConfigWithBinding(name, description, "Reader", "fake-rg-id")
	cfg2 := mockServiceAccountConfigWithBinding(name, description, "Writer", "fake-rg-id")

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())
	secretChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(saAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, bindingPath("role_name"), knownvalue.StringExact("Reader")),
				idChanged.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretChanged.AddStateValue(saAddr, clientSecretPath()),
			}),
			integration.RequiresReplaceStep(saAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(saAddr, bindingPath("role_name"), knownvalue.StringExact("Writer")),
				idChanged.AddStateValue(saAddr, tfjsonpath.New("id")),
				secretChanged.AddStateValue(saAddr, clientSecretPath()),
			}),
		},
	})
}
