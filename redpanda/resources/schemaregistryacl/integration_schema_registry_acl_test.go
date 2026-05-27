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

package schemaregistryacl_test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
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
)

const mockSRACLClusterID = "mocksraclcluster0000"

const (
	mockSRACLClusterID2         = "mocksraclcluster0002"
	mockSRACLClusterID3         = "mocksraclcluster0003"
	mockSRACLClusterID4         = "mocksraclcluster0004"
	mockSRACLClusterID5         = "mocksraclcluster0005"
	mockSRACLClusterID6         = "mocksraclcluster0006"
	mockSRACLClusterID7         = "mocksraclcluster0007"
	mockSRACLClusterID8         = "mocksraclcluster0008"
	mockSRACLClusterID9         = "mocksraclcluster0009"
	mockSRACLClusterID10        = "mocksraclcluster0010"
	mockSRACLClusterID11        = "mocksraclcluster0011"
	mockSRACLClusterID12        = "mocksraclcluster0012"
	mockSRACLClusterIDImport    = "mocksraclclusterimp0"
	mockSRACLClusterIDErrCreate = "mocksraclclustererrc"
	mockSRACLClusterIDErrRead   = "mocksraclclustererrr"
	mockSRACLClusterIDErrDelete = "mocksraclclustererrd"

	sraclAddr = "redpanda_schema_registry_acl.test"
)

// TestIntegration_SchemaRegistryACL exercises redpanda_schema_registry_acl end-to-end
// against the httptest-backed Schema Registry fake. The cluster fake is
// pre-seeded with a cluster whose SchemaRegistry.Url points at the in-process
// httptest server. All identifying fields are RequiresReplace, so the test
// covers create, refresh, and no-op only; the framework handles delete at
// teardown.
func TestIntegration_SchemaRegistryACL(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	srv.Cluster.Seed(&controlplanev1.Cluster{
		Id:    mockSRACLClusterID,
		Name:  "mock-sracl-cluster",
		State: controlplanev1.Cluster_STATE_READY,
	})

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_schema_registry_acl.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockSRACLConfig("User:alice", "SUBJECT", "mock-subject", "LITERAL", "*", "READ", "ALLOW"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "principal", "User:alice"),
					resource.TestCheckResourceAttr(addr, "resource_type", "SUBJECT"),
					resource.TestCheckResourceAttr(addr, "resource_name", "mock-subject"),
					resource.TestCheckResourceAttr(addr, "pattern_type", "LITERAL"),
					resource.TestCheckResourceAttr(addr, "host", "*"),
					resource.TestCheckResourceAttr(addr, "operation", "READ"),
					resource.TestCheckResourceAttr(addr, "permission", "ALLOW"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "principal", "User:alice"),
					resource.TestCheckResourceAttr(addr, "operation", "READ"),
				),
			},
			{
				Config: mockSRACLConfig("User:alice", "SUBJECT", "mock-subject", "LITERAL", "*", "READ", "ALLOW"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func mockSRACLConfig(principal, resourceType, resourceName, patternType, host, operation, permission string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema_registry_acl" "test" {
  cluster_id     = %q
  principal      = %q
  resource_type  = %q
  resource_name  = %q
  pattern_type   = %q
  host           = %q
  operation      = %q
  permission     = %q
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = true
}
`, mockSRACLClusterID, principal, resourceType, resourceName, patternType, host, operation, permission)
}

func sraclSetup(t *testing.T, clusterIDs ...string) (srv *mock.Server, factories map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")
	srv = mock.New(t)
	for _, id := range clusterIDs {
		srv.Cluster.Seed(&controlplanev1.Cluster{
			Id:    id,
			Name:  "mock-sracl-cluster-" + id,
			State: controlplanev1.Cluster_STATE_READY,
		})
	}
	factories = map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}
	return srv, factories
}

func sraclCfg(clusterID, principal, resourceType, resourceName, patternType, host, operation, permission string, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema_registry_acl" "test" {
  cluster_id     = %q
  principal      = %q
  resource_type  = %q
  resource_name  = %q
  pattern_type   = %q
  host           = %q
  operation      = %q
  permission     = %q
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = %t
}
`, clusterID, principal, resourceType, resourceName, patternType, host, operation, permission, allowDeletion)
}

func sraclID(clusterID, principal, resourceType, resourceName, patternType, host, operation, permission string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s",
		clusterID, principal, resourceType, resourceName, patternType, host, operation, permission)
}

func TestIntegration_SchemaRegistryACL_CreateAndRefresh(t *testing.T) {
	_, factories := sraclSetup(t, mockSRACLClusterID2)

	cfg := sraclCfg(mockSRACLClusterID2, "User:alice", "SUBJECT", "mock-subject-car", "LITERAL", "*", "READ", "ALLOW", true)
	wantID := sraclID(mockSRACLClusterID2, "User:alice", "SUBJECT", "mock-subject-car", "LITERAL", "*", "READ", "ALLOW")

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(sraclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSRACLClusterID2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("principal"), knownvalue.StringExact("User:alice")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact("SUBJECT")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact("mock-subject-car")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("host"), knownvalue.StringExact("*")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("operation"), knownvalue.StringExact("READ")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("permission"), knownvalue.StringExact("ALLOW")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("username"), knownvalue.StringExact("mock-user")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("password"), knownvalue.StringExact("mock-pass")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("password_wo"), knownvalue.Null()),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(sraclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSRACLClusterID2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("principal"), knownvalue.StringExact("User:alice")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact("SUBJECT")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact("mock-subject-car")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("pattern_type"), knownvalue.StringExact("LITERAL")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("host"), knownvalue.StringExact("*")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("operation"), knownvalue.StringExact("READ")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("permission"), knownvalue.StringExact("ALLOW")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("username"), knownvalue.StringExact("mock-user")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("password"), knownvalue.StringExact("mock-pass")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID)),
				idPreserved.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

func TestIntegration_SchemaRegistryACL_UpdateLeaf_AllowDeletion(t *testing.T) {
	_, factories := sraclSetup(t, mockSRACLClusterID3)

	cfgTrue := sraclCfg(mockSRACLClusterID3, "User:alice", "SUBJECT", "mock-subject-upd", "LITERAL", "*", "READ", "ALLOW", true)
	cfgFalse := sraclCfg(mockSRACLClusterID3, "User:alice", "SUBJECT", "mock-subject-upd", "LITERAL", "*", "READ", "ALLOW", false)

	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(sraclAddr, cfgTrue, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				idStable.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(sraclAddr, cfgFalse, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(false)),
				idStable.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(sraclAddr, cfgTrue, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				idStable.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

func requiresReplaceIdentityTest(
	t *testing.T,
	clusterID1, clusterID2,
	princ1, princ2,
	rt1, rt2,
	rn1, rn2,
	pt1, pt2,
	host1, host2,
	op1, op2,
	perm1, perm2 string,
) {
	t.Helper()
	_, factories := sraclSetup(t, clusterID1, clusterID2)

	cfg1 := sraclCfg(clusterID1, princ1, rt1, rn1, pt1, host1, op1, perm1, true)
	cfg2 := sraclCfg(clusterID2, princ2, rt2, rn2, pt2, host2, op2, perm2, true)

	wantID1 := sraclID(clusterID1, princ1, rt1, rn1, pt1, host1, op1, perm1)
	wantID2 := sraclID(clusterID2, princ2, rt2, rn2, pt2, host2, op2, perm2)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(sraclAddr, cfg1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(clusterID1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("principal"), knownvalue.StringExact(princ1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact(rn1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("pattern_type"), knownvalue.StringExact(pt1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("host"), knownvalue.StringExact(host1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("operation"), knownvalue.StringExact(op1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("permission"), knownvalue.StringExact(perm1)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID1)),
				idChanged.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(sraclAddr, cfg2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(clusterID2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("principal"), knownvalue.StringExact(princ2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_type"), knownvalue.StringExact(rt2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("resource_name"), knownvalue.StringExact(rn2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("pattern_type"), knownvalue.StringExact(pt2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("host"), knownvalue.StringExact(host2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("operation"), knownvalue.StringExact(op2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("permission"), knownvalue.StringExact(perm2)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("id"), knownvalue.StringExact(wantID2)),
				idChanged.AddStateValue(sraclAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_ClusterID(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID4, mockSRACLClusterID5,
		"User:alice", "User:alice",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "LITERAL",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_Principal(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID6, mockSRACLClusterID6,
		"User:alice", "User:bob",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "LITERAL",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_ResourceType(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID7, mockSRACLClusterID7,
		"User:alice", "User:alice",
		"SUBJECT", "REGISTRY",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "LITERAL",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_ResourceName(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID8, mockSRACLClusterID8,
		"User:alice", "User:alice",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "other-subject-rr",
		"LITERAL", "LITERAL",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_PatternType(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID9, mockSRACLClusterID9,
		"User:alice", "User:alice",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "PREFIXED",
		"*", "*",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_Host(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID10, mockSRACLClusterID10,
		"User:alice", "User:alice",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "LITERAL",
		"*", "10.0.0.1",
		"READ", "READ",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_Operation(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID11, mockSRACLClusterID11,
		"User:alice", "User:alice",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "LITERAL",
		"*", "*",
		"READ", "WRITE",
		"ALLOW", "ALLOW",
	)
}

func TestIntegration_SchemaRegistryACL_RequiresReplace_Permission(t *testing.T) {
	requiresReplaceIdentityTest(t,
		mockSRACLClusterID12, mockSRACLClusterID12,
		"User:alice", "User:alice",
		"SUBJECT", "SUBJECT",
		"mock-subject-rr", "mock-subject-rr",
		"LITERAL", "LITERAL",
		"*", "*",
		"READ", "READ",
		"ALLOW", "DENY",
	)
}

func TestIntegration_SchemaRegistryACL_ImportRoundTrip(t *testing.T) {
	_, factories := sraclSetup(t, mockSRACLClusterIDImport)

	cfg := sraclCfg(mockSRACLClusterIDImport, "User:alice", "SUBJECT", "mock-subject-import", "LITERAL", "*", "READ", "ALLOW", true)

	idFunc := func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[sraclAddr]
		if !ok {
			return "", fmt.Errorf("resource %q not found in state", sraclAddr)
		}
		attrs := rs.Primary.Attributes
		return fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s",
			attrs["cluster_id"],
			attrs["principal"],
			attrs["resource_type"],
			attrs["resource_name"],
			attrs["pattern_type"],
			attrs["host"],
			attrs["operation"],
			attrs["permission"],
			attrs["username"],
			attrs["password"],
		), nil
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(sraclAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSRACLClusterIDImport)),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("principal"), knownvalue.StringExact("User:alice")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("username"), knownvalue.StringExact("mock-user")),
				statecheck.ExpectKnownValue(sraclAddr, tfjsonpath.New("password"), knownvalue.StringExact("mock-pass")),
			}),
			// ImportRoundTrip pins the fix — do NOT add username or password to verifyIgnore without re-tracking the regression.
			{
				ResourceName:                         sraclAddr,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    idFunc,
				ImportStateVerifyIdentifierAttribute: "cluster_id",
				ImportStateVerifyIgnore: []string{
					"allow_deletion", // ImportState hardcodes false; pre-import state has true
					"password_wo_version",
				},
			},
		},
	})
}

func TestIntegration_SchemaRegistryACL_ErrorPath_CreateFails(t *testing.T) {
	srv, factories := sraclSetup(t, mockSRACLClusterIDErrCreate)

	cfg := sraclCfg(mockSRACLClusterIDErrCreate, "User:alice", "SUBJECT", "mock-subject-errc", "LITERAL", "*", "READ", "ALLOW", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.RESTErrorPathStep(srv.SR,
				"POST", "/security/acls",
				http.StatusInternalServerError,
				`{"error_code":50001,"message":"synthetic create failure"}`,
				cfg,
				"synthetic create failure",
			),
		},
	})
}

func TestIntegration_SchemaRegistryACL_ErrorPath_ReadFails(t *testing.T) {
	srv, factories := sraclSetup(t, mockSRACLClusterIDErrRead)

	cfg := sraclCfg(mockSRACLClusterIDErrRead, "User:alice", "SUBJECT", "mock-subject-errr", "LITERAL", "*", "READ", "ALLOW", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(sraclAddr, cfg, nil),
			{
				PreConfig: func() {
					srv.SR.OverrideOnceHTTP("GET", "/security/acls",
						http.StatusInternalServerError,
						`{"error_code":50001,"message":"synthetic read failure"}`)
				},
				RefreshState: true,
				ExpectError:  regexp.MustCompile("synthetic read failure"),
			},
		},
	})
}

func TestIntegration_SchemaRegistryACL_ErrorPath_DeleteFails(t *testing.T) {
	srv, factories := sraclSetup(t, mockSRACLClusterIDErrDelete)

	cfg := sraclCfg(mockSRACLClusterIDErrDelete, "User:alice", "SUBJECT", "mock-subject-errd", "LITERAL", "*", "READ", "ALLOW", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(sraclAddr, cfg, nil),
			{
				PreConfig: func() {
					srv.SR.OverrideOnceHTTP("DELETE", "/security/acls",
						http.StatusInternalServerError,
						`{"error_code":50001,"message":"synthetic delete failure"}`)
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("synthetic delete failure"),
			},
		},
	})
}
