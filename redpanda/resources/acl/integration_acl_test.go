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

package acl_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/acl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const aclAddr = "redpanda_acl.test"

// TestIntegration_ACL exercises redpanda_acl end-to-end against the bufconn-backed
// fake dataplane. Covers Create, refresh, no-op re-plan, and TF-local update
// (allow_deletion flip).
func TestIntegration_ACL(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_acl.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockACLConfig(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "resource_type", "TOPIC"),
					resource.TestCheckResourceAttr(addr, "resource_name", "test-topic"),
					resource.TestCheckResourceAttr(addr, "resource_pattern_type", "LITERAL"),
					resource.TestCheckResourceAttr(addr, "principal", "User:test-user"),
					resource.TestCheckResourceAttr(addr, "host", "*"),
					resource.TestCheckResourceAttr(addr, "operation", "READ"),
					resource.TestCheckResourceAttr(addr, "permission_type", "ALLOW"),
					resource.TestCheckResourceAttr(addr, "allow_deletion", "true"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				Config: mockACLConfig(true),
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
					resource.TestCheckResourceAttr(addr, "principal", "User:test-user"),
					resource.TestCheckResourceAttr(addr, "operation", "READ"),
				),
			},
			{
				Config: mockACLConfig(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "allow_deletion", "false"),
			},
			{
				Config: mockACLConfig(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "allow_deletion", "true"),
			},
		},
	})
}

func mockACLConfig(allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = "test-topic"
  resource_pattern_type = "LITERAL"
  principal             = "User:test-user"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = "bufnet"
  allow_deletion        = %t
}
`, allowDeletion)
}

// mockACLFullConfig parameterizes every leaf so each scenario can mutate one
// field while pinning the rest. The full surface lets each test assert all
// leaves end-to-end without leaving any to schema defaults.
func mockACLFullConfig(resourceType, resourceName, resourcePatternType, principal, host, operation, permissionType, clusterAPIURL string, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_acl" "test" {
  resource_type         = %q
  resource_name         = %q
  resource_pattern_type = %q
  principal             = %q
  host                  = %q
  operation             = %q
  permission_type       = %q
  cluster_api_url       = %q
  allow_deletion        = %t
}
`, resourceType, resourceName, resourcePatternType, principal, host, operation, permissionType, clusterAPIURL, allowDeletion)
}

// aclID returns the canonical id string for an ACL with the given identity
// fields. Mirrors GenerateID in redpanda/models/acl/resource_model.go and is
// used to pin the id leaf at exact values in state-check assertions.
func aclID(resourceType, resourceName, resourcePatternType, principal, host, operation, permissionType string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		resourceType, resourceName, resourcePatternType, principal, host, operation, permissionType)
}

// TestIntegration_ACL_CreateAndRefresh covers the canonical Create + NoopReapply
// cycle and pins every leaf to an exact value. The load-bearing proof for the
// id leaf's UseStateForUnknown plan modifier is that id (a composite of the 7
// identity fields per GenerateID) is IDENTICAL across the two steps — a
// single CompareValue(ValuesSame()) shared between both steps'
// ConfigStateChecks accumulates the values and the comparer asserts equality.
func TestIntegration_ACL_CreateAndRefresh(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
		url   = "bufnet"
	)
	cfg := mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, true)
	wantID := aclID(rt, rn, rpt, princ, host, op, perm)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact(rn)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_pattern_type"), knownvalue.StringExact(rpt)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("principal"), knownvalue.StringExact(princ)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("host"), knownvalue.StringExact(host)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("operation"), knownvalue.StringExact(op)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("permission_type"), knownvalue.StringExact(perm)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(aclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact(rn)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_pattern_type"), knownvalue.StringExact(rpt)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("principal"), knownvalue.StringExact(princ)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("host"), knownvalue.StringExact(host)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("operation"), knownvalue.StringExact(op)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("permission_type"), knownvalue.StringExact(perm)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ACL_UpdateLeaf_AllowDeletion exercises the in-place Update path
// for the TF-only `allow_deletion` extra. ACL has no UpdateACL RPC (ACLs are
// immutable); the Update handler in resource_acl.go just writes plan to
// state. Flipping allow_deletion true→false→true must produce
// ResourceActionUpdate (not DestroyBeforeCreate) and id must remain stable
// across all three steps — proof that the in-place path actually runs. We
// end with allow_deletion=true so the terminal cleanup destroy succeeds
// (Delete blocks when allow_deletion=false before any RPC call).
func TestIntegration_ACL_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
		url   = "bufnet"
	)
	wantID := aclID(rt, rn, rpt, princ, host, op, perm)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(aclAddr, mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, false), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(aclAddr, mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// requiresReplaceIdentityTest runs a RequiresReplace scenario for a single
// identity field. Every identity field is part of GenerateID's composite, so
// mutating any one of them causes id to DIFFER — that ValuesDiffer assertion
// is the load-bearing proof that the destroy-then-create cycle ran (alongside
// the ResourceActionDestroyBeforeCreate plancheck baked into
// RequiresReplaceStep).
func requiresReplaceIdentityTest(t *testing.T, rt1, rt2, rn1, rn2, rpt1, rpt2, princ1, princ2, host1, host2, op1, op2, perm1, perm2 string) {
	t.Helper()
	_, factories := integration.Setup(t)

	const url = "bufnet"
	wantID1 := aclID(rt1, rn1, rpt1, princ1, host1, op1, perm1)
	wantID2 := aclID(rt2, rn2, rpt2, princ2, host2, op2, perm2)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, mockACLFullConfig(rt1, rn1, rpt1, princ1, host1, op1, perm1, url, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact(rn1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_pattern_type"), knownvalue.StringExact(rpt1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("principal"), knownvalue.StringExact(princ1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("host"), knownvalue.StringExact(host1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("operation"), knownvalue.StringExact(op1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("permission_type"), knownvalue.StringExact(perm1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID1)),
				idChanged.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(aclAddr, mockACLFullConfig(rt2, rn2, rpt2, princ2, host2, op2, perm2, url, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact(rn2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_pattern_type"), knownvalue.StringExact(rpt2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("principal"), knownvalue.StringExact(princ2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("host"), knownvalue.StringExact(host2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("operation"), knownvalue.StringExact(op2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("permission_type"), knownvalue.StringExact(perm2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID2)),
				idChanged.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ACL_RequiresReplace_ResourceType mutates the RequiresReplace
// `resource_type` leaf (TOPIC→GROUP). id changes because resource_type is
// field #1 in GenerateID's composite.
func TestIntegration_ACL_RequiresReplace_ResourceType(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "GROUP",
		"test-topic", "test-topic",
		"LITERAL", "LITERAL",
		"User:test-user", "User:test-user",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

// TestIntegration_ACL_RequiresReplace_ResourceName mutates the RequiresReplace
// `resource_name` leaf. id changes because resource_name is field #2 in
// GenerateID's composite.
func TestIntegration_ACL_RequiresReplace_ResourceName(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "TOPIC",
		"test-topic", "other-topic",
		"LITERAL", "LITERAL",
		"User:test-user", "User:test-user",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

// TestIntegration_ACL_RequiresReplace_ResourcePatternType mutates the
// RequiresReplace `resource_pattern_type` leaf (LITERAL→PREFIXED). id changes
// because resource_pattern_type is field #3 in GenerateID's composite.
func TestIntegration_ACL_RequiresReplace_ResourcePatternType(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "TOPIC",
		"test-topic", "test-topic",
		"LITERAL", "PREFIXED",
		"User:test-user", "User:test-user",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

// TestIntegration_ACL_RequiresReplace_Principal mutates the RequiresReplace
// `principal` leaf. id changes because principal is field #4 in GenerateID's
// composite.
func TestIntegration_ACL_RequiresReplace_Principal(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "TOPIC",
		"test-topic", "test-topic",
		"LITERAL", "LITERAL",
		"User:test-user", "User:other-user",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

// TestIntegration_ACL_RequiresReplace_Host mutates the RequiresReplace `host` leaf
// (wildcard "*" → IPv4 literal "10.0.0.1"). id changes because host is field
// #5 in GenerateID's composite. The proto validator on host accepts only "*"
// or a valid IP address, so a hostname like "localhost" is rejected — an IPv4
// literal is the unambiguous alternative.
func TestIntegration_ACL_RequiresReplace_Host(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "TOPIC",
		"test-topic", "test-topic",
		"LITERAL", "LITERAL",
		"User:test-user", "User:test-user",
		"*", "10.0.0.1",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

// TestIntegration_ACL_RequiresReplace_Operation mutates the RequiresReplace
// `operation` leaf (READ→WRITE). id changes because operation is field #6 in
// GenerateID's composite.
func TestIntegration_ACL_RequiresReplace_Operation(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "TOPIC",
		"test-topic", "test-topic",
		"LITERAL", "LITERAL",
		"User:test-user", "User:test-user",
		"*", "*",
		"READ", "WRITE",
		"ALLOW", "ALLOW",
	)
}

// TestIntegration_ACL_RequiresReplace_PermissionType mutates the RequiresReplace
// `permission_type` leaf (ALLOW→DENY). id changes because permission_type is
// field #7 in GenerateID's composite.
func TestIntegration_ACL_RequiresReplace_PermissionType(t *testing.T) {
	requiresReplaceIdentityTest(t,
		"TOPIC", "TOPIC",
		"test-topic", "test-topic",
		"LITERAL", "LITERAL",
		"User:test-user", "User:test-user",
		"*", "*",
		"READ", "READ",
		"ALLOW", "DENY",
	)
}

// TestIntegration_ACL_RequiresReplace_ClusterApiUrl mutates the RequiresReplace
// `cluster_api_url` leaf (bufnet→bufnet2). The bufconn dialer is
// address-agnostic — it ignores the URL string and routes through the
// in-memory listener — so changing the value triggers the plan-level
// DestroyBeforeCreate and the Create on the new resource still succeeds.
//
// Unlike the 7 identity-field RR scenarios, cluster_api_url is NOT in the
// GenerateID composite, so id stays identical across the replace. The
// load-bearing proof that RequiresReplace fired is the
// ResourceActionDestroyBeforeCreate plancheck baked into
// RequiresReplaceStep; idStable's ValuesSame check is the inverse witness
// confirming the id formula isn't affected by the url change.
func TestIntegration_ACL_RequiresReplace_ClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
		url1  = "bufnet"
		url2  = "bufnet2"
	)
	wantID := aclID(rt, rn, rpt, princ, host, op, perm)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url1, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url1)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(aclAddr, mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url2, true), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact(url2)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idStable.AddStateValue(aclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_ACL_ImportRoundTrip verifies that acl's ImportState correctly
// reconstructs resource state when the cluster_id resolves through the
// bufconn-backed ClusterFake.
func TestIntegration_ACL_ImportRoundTrip(t *testing.T) {
	srv, factories := integration.Setup(t)

	// Seed the ClusterFake so ClusterForID("cgg5hmkar1m4l0pjg6tg") resolves.
	srv.Cluster.SetClusterByID("cgg5hmkar1m4l0pjg6tg", "bufnet")

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
	)
	cfg := mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, "bufnet", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, cfg, nil),
			{
				ResourceName:      aclAddr,
				ImportState:       true,
				ImportStateVerify: true,
				// ACL schema has no `id` attribute and no `cluster_id` attribute; the
				// composite-import ID is "<cluster_id>,<rt>,<rn>,<rpt>,<princ>,<host>,<op>,<perm>"
				// and the ImportState handler sets `cluster_api_url` (the resolved
				// dataplane URL) — that's the only stable identifier in post-import state.
				ImportStateVerifyIdentifierAttribute: "cluster_api_url",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[aclAddr]
					if !ok {
						return "", fmt.Errorf("resource %q not found", aclAddr)
					}
					a := rs.Primary.Attributes
					return fmt.Sprintf("cgg5hmkar1m4l0pjg6tg,%s,%s,%s,%s,%s,%s,%s",
						a["resource_type"], a["resource_name"], a["resource_pattern_type"],
						a["principal"], a["host"], a["operation"], a["permission_type"]), nil
				},
				ImportStateVerifyIgnore: []string{"allow_deletion"},
			},
		},
	})
}

// TestIntegration_ACL_ErrorPath_GetNotFound covers the Read→NotFound path. After a
// successful Create, the ACL is removed from the fake's store out-of-band via
// a direct DeleteACLs call on the fake. The next plan triggers Read, which
// runs ListACLs, sees no matching record, and calls
// utils.HandleGracefulRemoval with utils.NotFoundError{...}. NotFound is one
// of the "clean removal" codes (graceful_removal.go:71) so it returns
// RemoveFromState regardless of allow_deletion; TF then plans a re-create.
// PreApply asserts ResourceActionCreate; PostApplyPostRefresh asserts an
// empty plan after the re-create lands.
func TestIntegration_ACL_ErrorPath_GetNotFound(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
		url   = "bufnet"
	)
	cfg := mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, true)
	wantID := aclID(rt, rn, rpt, princ, host, op, perm)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
			}),
			{
				PreConfig: func() {
					flt := &dataplanev1.DeleteACLsRequest_Filter{
						ResourceType:        dataplanev1.ACL_RESOURCE_TYPE_TOPIC,
						ResourceName:        utils.StringToStringPointer(rn),
						ResourcePatternType: dataplanev1.ACL_RESOURCE_PATTERN_TYPE_LITERAL,
						Principal:           utils.StringToStringPointer(princ),
						Host:                utils.StringToStringPointer(host),
						Operation:           dataplanev1.ACL_OPERATION_READ,
						PermissionType:      dataplanev1.ACL_PERMISSION_TYPE_ALLOW,
					}
					if _, err := srv.ACL.DeleteACLs(context.Background(), &dataplanev1.DeleteACLsRequest{Filter: flt}); err != nil {
						t.Fatalf("PreConfig: delete ACL: %v", err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(aclAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt)),
					statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				},
			},
		},
	})
}

// TestIntegration_ACL_ErrorPath_CreateAlreadyExists injects AlreadyExists on
// CreateACL. The provider's Create handler probes ListACLs after
// AlreadyExists to support adopt-on-conflict (resource_acl.go:111-115); since
// the ACL doesn't exist in the fake's store (only CreateACL is overridden),
// the probe returns false → NonRetryableError → diagnostic surfaces.
// ExpectError matches the regexp against the diagnostic text.
func TestIntegration_ACL_ErrorPath_CreateAlreadyExists(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
		url   = "bufnet"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				dataplanev1grpc.ACLService_CreateACL_FullMethodName,
				codes.AlreadyExists,
				mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, true),
				"already exists",
			),
		},
	})
}

// TestIntegration_ACL_ErrorPath_DeleteFailed covers the destroy-failed path. After a
// successful Create, an Internal-coded error is injected on the next
// DeleteACLs RPC. Internal is NOT one of the graceful-removal codes
// (NotFound / ClusterUnreachable / PermissionDenied) that
// utils.HandleGracefulRemoval treats as a clean drop-from-state — it falls
// through to the ErrorNotHandled branch and surfaces as a TF diagnostic. The
// config uses allow_deletion=true so the in-resource guard at
// resource_acl.go:228 passes and the DeleteACLs RPC is actually called. After
// the failing step the override is consumed; the TestCase's terminal cleanup
// destroy runs against the untainted fake and removes the resource cleanly.
func TestIntegration_ACL_ErrorPath_DeleteFailed(t *testing.T) {
	srv, factories := integration.Setup(t)

	const (
		rt    = "TOPIC"
		rn    = "test-topic"
		rpt   = "LITERAL"
		princ = "User:test-user"
		host  = "*"
		op    = "READ"
		perm  = "ALLOW"
		url   = "bufnet"
	)
	cfg := mockACLFullConfig(rt, rn, rpt, princ, host, op, perm, url, true)
	wantID := aclID(rt, rn, rpt, princ, host, op, perm)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(aclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				statecheck.ExpectKnownValue(aclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						dataplanev1grpc.ACLService_DeleteACLs_FullMethodName,
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

// TestIntegration_ACL_UpgradeState_NormalizesClusterApiUrl drives the v0->v1
// state upgrade through the provider server's UpgradeResourceState RPC and
// asserts the legacy host:443 cluster_api_url is rewritten to https://host so
// the format change alone no longer forces replacement. Notably acl has
// no ImportState, so the upgrader is its only in-place migration path.
func TestIntegration_ACL_UpgradeState_NormalizesClusterApiUrl(t *testing.T) {
	_, factories := integration.Setup(t)
	ctx := context.Background()
	schemaType := acl.ResourceACLSchema(ctx).Type().TerraformType(ctx)

	const priorState = `{` +
		`"allow_deletion":true,` +
		`"cluster_api_url":"bufnet:443",` +
		`"host":"*",` +
		`"id":"TOPIC:*:LITERAL:User:app:*:ALL:ALLOW",` +
		`"operation":"ALL",` +
		`"permission_type":"ALLOW",` +
		`"principal":"User:app",` +
		`"resource_name":"*",` +
		`"resource_pattern_type":"LITERAL",` +
		`"resource_type":"TOPIC"` +
		`}`

	upgraded := integration.UpgradeState(t, factories, "redpanda_acl", 0, priorState, schemaType)

	var obj map[string]tftypes.Value
	require.NoError(t, upgraded.As(&obj))
	var got string
	require.NoError(t, obj["cluster_api_url"].As(&got))
	assert.Equal(t, "https://bufnet", got)
}
