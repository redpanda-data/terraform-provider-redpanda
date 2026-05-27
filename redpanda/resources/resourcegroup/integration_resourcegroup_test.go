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

package resourcegroup_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
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

const resourceGroupAddr = "redpanda_resource_group.test"

func mockResourceGroupConfig(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = %q
}
`, name)
}

// TestIntegration_ResourceGroup exercises redpanda_resource_group end-to-end against
// the bufconn-backed fake controlplane. Validates the full provider →
// SpawnConn → bufconn → fake → response round-trip.
func TestIntegration_ResourceGroup(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	initialName := "tfrp-mock-rg-initial"
	renamedName := "tfrp-mock-rg-renamed"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockResourceGroupConfig(initialName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupAddr, "name", initialName),
					resource.TestCheckResourceAttrSet(resourceGroupAddr, "id"),
				),
			},
			{
				Config: mockResourceGroupConfig(initialName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceGroupAddr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupAddr, "name", initialName),
					resource.TestCheckResourceAttrSet(resourceGroupAddr, "id"),
				),
			},
			{
				Config: mockResourceGroupConfig(renamedName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceGroupAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(resourceGroupAddr, "name", renamedName),
			},
		},
	})
}

// TestIntegration_ResourceGroup_CreateAndRefresh validates the Create + no-op re-plan
// cycle. The id leaf is Computed + UseStateForUnknown; the load-bearing
// assertion is that id is IDENTICAL across the Create step and the noop
// re-apply step. A single CompareValue(ValuesSame()) instance is shared
// between the two steps' ConfigStateChecks — the framework calls CheckState
// once per step, the checker accumulates values, and the comparer asserts
// equality once two values are present.
func TestIntegration_ResourceGroup_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-rg-create"
	cfg := mockResourceGroupConfig(name)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(resourceGroupAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(resourceGroupAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(resourceGroupAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(resourceGroupAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ResourceGroup_RequiresReplace_Name mutates the RequiresReplace
// `name` leaf and asserts the framework plans DestroyBeforeCreate. The
// load-bearing proof that the resource was actually destroyed and recreated
// (rather than updated in-place) is that the server-assigned id DIFFERS
// between the two steps — a single CompareValue(ValuesDiffer()) instance
// shared across both steps captures the pre- and post-replace ids and the
// comparer asserts they are not equal.
func TestIntegration_ResourceGroup_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		initialName = "tfrp-mock-rg-a"
		renamedName = "tfrp-mock-rg-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(resourceGroupAddr, mockResourceGroupConfig(initialName), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(initialName)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(resourceGroupAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(resourceGroupAddr, mockResourceGroupConfig(renamedName), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(renamedName)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(resourceGroupAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ResourceGroup_ImportRoundTrip exercises the bearer-id import path.
// The resource's ImportState uses ImportStatePassthroughID on the "id"
// attribute, so the import id is the UUID assigned at Create. nil idFunc
// tells the helper to use the bearer "id" from prior state; nil verifyIgnore
// means every attribute must roundtrip identically (no write-only fields).
func TestIntegration_ResourceGroup_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-rg-import"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(resourceGroupAddr, mockResourceGroupConfig(name), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(resourceGroupAddr, nil, nil),
		},
	})
}

// TestIntegration_ResourceGroup_ErrorPath_GetNotFound covers the Read→NotFound path:
// after Create, the fake's stored group is removed out-of-band via the fake's
// own DeleteResourceGroup RPC so the next GetResourceGroup naturally returns
// NotFound. This simulates real cloud-side drift (the resource was deleted
// outside Terraform). The provider's Read sees NotFound, calls
// RemoveResource, and the next plan sees the resource missing from state →
// re-Create. PreApply asserts ResourceActionCreate; PostApplyPostRefresh
// asserts an empty plan after the re-create lands.
func TestIntegration_ResourceGroup_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-rg-notfound"
	cfg := mockResourceGroupConfig(name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(resourceGroupAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					listResp, err := srv.ResourceGroup.ListResourceGroups(context.Background(),
						&controlplanev1.ListResourceGroupsRequest{Filter: &controlplanev1.ListResourceGroupsRequest_Filter{NameContains: name}})
					if err != nil {
						t.Fatalf("PreConfig: list groups: %v", err)
					}
					if got := len(listResp.GetResourceGroups()); got != 1 {
						t.Fatalf("PreConfig: want 1 group named %q, got %d", name, got)
					}
					id := listResp.GetResourceGroups()[0].GetId()
					if _, err := srv.ResourceGroup.DeleteResourceGroup(context.Background(),
						&controlplanev1.DeleteResourceGroupRequest{Id: id}); err != nil {
						t.Fatalf("PreConfig: delete group %q: %v", id, err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceGroupAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_ResourceGroup_ErrorPath_CreateAlreadyExists injects AlreadyExists
// on CreateResourceGroup. The provider's Create surfaces the gRPC error as a
// Terraform diagnostic; ExpectError matches the regexp against the diagnostic
// text.
func TestIntegration_ResourceGroup_ErrorPath_CreateAlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-rg-exists"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.ResourceGroupService_CreateResourceGroup_FullMethodName,
				codes.AlreadyExists,
				mockResourceGroupConfig(name),
				"already exists",
			),
		},
	})
}

// TestIntegration_ResourceGroup_ErrorPath_DeleteFailed covers the destroy-failed
// path. After a successful Create, an Internal-coded error is injected on
// the next DeleteResourceGroup. The Destroy:true step triggers the destroy
// plan; ExpectError matches the error regexp. After this step the override
// is consumed; the TestCase's terminal cleanup destroy runs against the
// untainted fake and removes the resource cleanly.
func TestIntegration_ResourceGroup_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-rg-delfail"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(resourceGroupAddr, mockResourceGroupConfig(name), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(resourceGroupAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ResourceGroupService_DeleteResourceGroup_FullMethodName,
						status.Error(codes.Internal, "synthetic delete failure"),
					)
				},
				Config:      mockResourceGroupConfig(name),
				Destroy:     true,
				ExpectError: regexp.MustCompile("synthetic delete failure"),
			},
		},
	})
}
