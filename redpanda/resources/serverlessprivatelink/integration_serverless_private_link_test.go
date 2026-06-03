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

package serverlessprivatelink_test

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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const splAddr = "redpanda_serverless_private_link.test"

// NOTE on cloud_provider variants: the upstream proto
// controlplanev1.ServerlessPrivateLink.CloudProviderConfig oneof ships only
// one implementation (ServerlessPrivateLink_AwsConfig); no GcpConfig or
// AzureConfig proto type exists. The CloudProviders() validator accepts
// "gcp"/"azure" but those values have no destination config block. As a
// result there is no _CreateAndRefresh_GCP variant scenario and no
// _RequiresReplace_CloudProvider scenario — both would have no executable
// HCL form. This reflects product reality (AWS-only backend support).

// TestIntegration_ServerlessPrivateLink exercises redpanda_serverless_private_link
// against the bufconn fake. Update has no FieldMask (full replacement of the
// aws_config oneof), so the update step rewrites allowed_principals.
func TestIntegration_ServerlessPrivateLink(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	initialPrincipals := `["arn:aws:iam::123456789012:root"]`
	updatedPrincipals := `["arn:aws:iam::123456789012:root", "arn:aws:iam::987654321098:root"]`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockSPLConfig(initialPrincipals),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(splAddr, "name", "tfrp-mock-spl"),
					resource.TestCheckResourceAttr(splAddr, "cloud_provider", "aws"),
					resource.TestCheckResourceAttr(splAddr, "serverless_region", "pro-us-east-1"),
					resource.TestCheckResourceAttr(splAddr, "aws_config.allowed_principals.#", "1"),
					resource.TestCheckResourceAttrSet(splAddr, "id"),
					resource.TestCheckResourceAttrSet(splAddr, "status.aws.vpc_endpoint_service_name"),
				),
			},
			{
				Config: mockSPLConfig(initialPrincipals),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(splAddr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttr(splAddr, "aws_config.allowed_principals.#", "1"),
			},
			{
				Config: mockSPLConfig(updatedPrincipals),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(splAddr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(splAddr, "aws_config.allowed_principals.#", "2"),
			},
		},
	})
}

func mockSPLConfig(principals string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-spl-rg"
}

resource "redpanda_serverless_private_link" "test" {
  name              = "tfrp-mock-spl"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  serverless_region = "pro-us-east-1"
  aws_config = {
    allowed_principals = %s
  }
  allow_deletion = true
}
`, principals)
}

// splConfig renders the canonical single-resource-group HCL used by every
// integration scenario in this file. Principal list is inlined to keep call
// sites short; allow_deletion is parameterized so the cleanup gate can be
// toggled without rewriting the helper.
func splConfig(name, serverlessRegion, principals string, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-spl-rg"
}

resource "redpanda_serverless_private_link" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  serverless_region = %q
  aws_config = {
    allowed_principals = %s
  }
  allow_deletion = %t
}
`, name, serverlessRegion, principals, allowDeletion)
}

// rrRGConfig declares two resource groups and points the SPL at whichever
// label is requested. Used by RequiresReplace_ResourceGroupID to switch the
// rg dependency between steps and force a destroy-and-recreate.
func rrRGConfig(splName, rgLabel string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "rg1" {
  name = "tfrp-mock-spl-rg-1"
}

resource "redpanda_resource_group" "rg2" {
  name = "tfrp-mock-spl-rg-2"
}

resource "redpanda_serverless_private_link" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.%s.id
  cloud_provider    = "aws"
  serverless_region = "pro-us-east-1"
  aws_config = {
    allowed_principals = ["arn:aws:iam::123456789012:root"]
  }
  allow_deletion = true
}
`, splName, rgLabel)
}

const (
	splPrincipals1 = `["arn:aws:iam::123456789012:root"]`
	splPrincipals2 = `["arn:aws:iam::123456789012:root", "arn:aws:iam::987654321098:root"]`
)

// TestIntegration_ServerlessPrivateLink_AllowDeletionFlip_NoBackendCall flips
// allow_deletion — a provider-only attribute absent from the proto update
// request — with aws_config unchanged. The plan is a Terraform-level Update,
// but the provider must short-circuit: no UpdateServerlessPrivateLink RPC
// should fire. CallCount == 0 is the load-bearing assertion.
func TestIntegration_ServerlessPrivateLink_AllowDeletionFlip_NoBackendCall(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-allowdel"

	idUnchanged := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr, splConfig(name, "pro-us-east-1", splPrincipals1, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idUnchanged.AddStateValue(splAddr, tfjsonpath.New("id")),
			}),
			{
				Config: splConfig(name, "pro-us-east-1", splPrincipals1, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(splAddr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
					idUnchanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				},
				Check: func(*terraform.State) error {
					if n := srv.CallCount(controlplanev1grpc.ServerlessPrivateLinkService_UpdateServerlessPrivateLink_FullMethodName); n != 0 {
						return fmt.Errorf("allow_deletion-only flip called UpdateServerlessPrivateLink %d time(s); want 0 (no-op short-circuit)", n)
					}
					return nil
				},
			},
		},
	})
}

// TestIntegration_ServerlessPrivateLink_CreateAndRefresh validates the Create +
// no-op cycle for the only shipped variant (AWS). Asserts every leaf at
// exact value post-create. id, state, and status.aws.* are captured to
// prove UseStateForUnknown stability across the noop via a shared
// CompareValue(ValuesSame()) on id.
func TestIntegration_ServerlessPrivateLink_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-create"
	cfg := splConfig(name, "pro-us-east-1", splPrincipals1, true)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-us-east-1")),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("aws_config").AtMapKey("allowed_principals"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("arn:aws:iam::123456789012:root"),
				})),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("status").AtMapKey("aws").AtMapKey("vpc_endpoint_service_name"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("status").AtMapKey("aws").AtMapKey("availability_zones"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("use1-az1"),
				})),
				idPreserved.AddStateValue(splAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(splAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-us-east-1")),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				idPreserved.AddStateValue(splAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_UpdateLeaf_AllowedPrincipals mutates the
// principals list in place. The Update RPC has no FieldMask — aws_config
// is full-replacement — so the fake overwrites the list wholesale. id is
// stable via the shared CompareValue(ValuesSame()).
func TestIntegration_ServerlessPrivateLink_UpdateLeaf_AllowedPrincipals(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-upd-ap"

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("aws_config").AtMapKey("allowed_principals"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::123456789012:root"),
					})),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idStable.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals2, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("aws_config").AtMapKey("allowed_principals"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::123456789012:root"),
						knownvalue.StringExact("arn:aws:iam::987654321098:root"),
					})),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idStable.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_UpdateLeaf_AllowDeletion flips the
// allow_deletion sentinel true→false→true. The third step restores true
// so the framework's terminal destroy passes the Delete gate. id is stable
// across all three steps.
func TestIntegration_ServerlessPrivateLink_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-upd-ad"

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idStable.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals1, false),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idStable.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idStable.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_RequiresReplace_Name mutates name and
// asserts DestroyBeforeCreate. The load-bearing proof of actual replacement
// (not silent in-place mutation) is the xid-like id DIFFERING across steps,
// captured by a shared CompareValue(ValuesDiffer()).
func TestIntegration_ServerlessPrivateLink_RequiresReplace_Name(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		nameA = "tfrp-mock-spl-rr-name-a"
		nameB = "tfrp-mock-spl-rr-name-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr,
				splConfig(nameA, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(splAddr,
				splConfig(nameB, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameB)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_RequiresReplace_ResourceGroupID mutates
// the parent resource_group_id by swapping which of two rg dependencies
// the SPL targets. id DIFFERS across the destroy-and-recreate.
func TestIntegration_ServerlessPrivateLink_RequiresReplace_ResourceGroupID(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-rr-rg"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr,
				rrRGConfig(name, "rg1"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(splAddr,
				rrRGConfig(name, "rg2"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_RequiresReplace_ServerlessRegion mutates
// serverless_region. id DIFFERS across the destroy-and-recreate.
func TestIntegration_ServerlessPrivateLink_RequiresReplace_ServerlessRegion(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-rr-region"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-us-east-1")),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(splAddr,
				splConfig(name, "pro-eu-west-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("serverless_region"), knownvalue.StringExact("pro-eu-west-1")),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(splAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_ImportRoundTrip verifies the id-passthrough
// import path. ImportState calls resource.ImportStatePassthroughID — no live
// controlplane lookup, no composite ID.
//
// verifyIgnore includes "allow_deletion" because ImportState hardcodes
// allow_deletion=false; pre-import state has true. verifyIgnore includes
// "timeouts" because the config carries no timeouts block and import does
// not populate one.
func TestIntegration_ServerlessPrivateLink_ImportRoundTrip(t *testing.T) {
	_, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-import"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr,
				splConfig(name, "pro-us-east-1", splPrincipals1, true),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				}),
			integration.ImportRoundTripStep(splAddr, nil, []string{"allow_deletion", "timeouts"}),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_ErrorPath_GetNotFound covers the
// Read→NotFound path. After a successful Create, an OverrideOnce on
// GetServerlessPrivateLink is injected; the next Read fires during the
// second step's plan, gets NotFound, and the provider's Read calls
// resp.State.RemoveResource. TF then plans a fresh Create.
func TestIntegration_ServerlessPrivateLink_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-notfound"
	cfg := splConfig(name, "pro-us-east-1", splPrincipals1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ServerlessPrivateLinkService_GetServerlessPrivateLink_FullMethodName,
						status.Error(codes.NotFound, "serverless_private_link not found"),
					)
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(splAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_ServerlessPrivateLink_ErrorPath_CreateFailed injects AlreadyExists
// on CreateServerlessPrivateLink. The provider's Create has no adoption
// branch — the error surfaces directly as a diagnostic. ExpectError matches
// the regexp against the diagnostic text.
func TestIntegration_ServerlessPrivateLink_ErrorPath_CreateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-exists"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.ServerlessPrivateLinkService_CreateServerlessPrivateLink_FullMethodName,
				codes.AlreadyExists,
				splConfig(name, "pro-us-east-1", splPrincipals1, true),
				"AlreadyExists",
			),
		},
	})
}

// TestIntegration_ServerlessPrivateLink_ErrorPath_UpdateFailed injects Internal on
// UpdateServerlessPrivateLink for the next call. codes.Internal is
// non-retryable — the override fires once and the diagnostic surfaces
// immediately as "failed to update serverless private link" wrapping the
// synthetic error message.
func TestIntegration_ServerlessPrivateLink_ErrorPath_UpdateFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-upd-fail"
	cfg1 := splConfig(name, "pro-us-east-1", splPrincipals1, true)
	cfg2 := splConfig(name, "pro-us-east-1", splPrincipals2, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ServerlessPrivateLinkService_UpdateServerlessPrivateLink_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      cfg2,
				ExpectError: regexp.MustCompile("synthetic update failure"),
			},
		},
	})
}

// TestIntegration_ServerlessPrivateLink_ErrorPath_DeleteFailed covers the
// destroy-failed path. The SPL Delete handler does NOT use
// HandleGracefulRemoval — the error surfaces directly. codes.Internal is
// non-retryable; the override fires once and the diagnostic surfaces.
func TestIntegration_ServerlessPrivateLink_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-spl-del-fail"
	cfg := splConfig(name, "pro-us-east-1", splPrincipals1, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(splAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(splAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ServerlessPrivateLinkService_DeleteServerlessPrivateLink_FullMethodName,
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
