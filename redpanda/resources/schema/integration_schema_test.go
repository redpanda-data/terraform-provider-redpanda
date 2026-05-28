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

package schema_test

import (
	"context"
	"fmt"
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

const (
	mockSchemaClusterID  = "mockschemaclusterid0"
	mockSchemaClusterID2 = "mockschemaclusterid2"
	mockSchemaClusterID3 = "mockschemaclusterid3"
	mockSchemaClusterID4 = "mockschemaclusterid4"
	mockSchemaClusterID5 = "mockschemaclusterid5"
	mockSchemaClusterID6 = "mockschemaclusterid6"
	mockSchemaClusterID7 = "mockschemaclusterid7"
	mockSchemaClusterID8 = "mockschemaclusterid8"
	mockSchemaClusterID9 = "mockschemaclusterid9"
	mockSchemaClusterIDa = "mockschemaclusterida"
	mockSchemaClusterIDb = "mockschemaclusteridb"
	mockSchemaClusterIDc = "mockschemaclusteridc"
	mockSchemaClusterIDd = "mockschemaclusteridd"

	avroSchemaV1 = `{"type":"record","name":"TestRecord","fields":[{"name":"id","type":"int"}]}`
	avroSchemaV2 = `{"type":"record","name":"TestRecord","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}`

	avroUserV1  = `{"type":"record","name":"User","fields":[{"name":"id","type":"int"}]}`
	avroEventV1 = `{"type":"record","name":"Event","fields":[{"name":"ts","type":"long"}]}`
	avroEventV2 = `{"type":"record","name":"Event","fields":[{"name":"ts","type":"long"},{"name":"source","type":"string"}]}`

	schemaAddr = "redpanda_schema.test"
)

// TestIntegration_Schema exercises redpanda_schema end-to-end against the
// httptest-backed Schema Registry fake. The cluster fake is pre-seeded with
// a cluster whose SchemaRegistry.Url points at the in-process httptest server.
// Covers create, refresh, no-op update, schema-body update (new version), and
// compatibility change.
func TestIntegration_Schema(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	srv.Cluster.Seed(&controlplanev1.Cluster{
		Id:    mockSchemaClusterID,
		Name:  "mock-schema-cluster",
		State: controlplanev1.Cluster_STATE_READY,
	})

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_schema.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockSchemaConfig(avroSchemaV1, "BACKWARD"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "subject", "mock-subject"),
					resource.TestCheckResourceAttr(addr, "schema_type", "AVRO"),
					resource.TestCheckResourceAttr(addr, "version", "1"),
					resource.TestCheckResourceAttr(addr, "compatibility", "BACKWARD"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "subject", "mock-subject"),
					resource.TestCheckResourceAttr(addr, "version", "1"),
				),
			},
			{
				Config: mockSchemaConfig(avroSchemaV1, "BACKWARD"),
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
				Config: mockSchemaConfig(avroSchemaV2, "BACKWARD"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "version", "2"),
			},
			{
				Config: mockSchemaConfig(avroSchemaV2, "FULL"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "compatibility", "FULL"),
			},
		},
	})
}

func mockSchemaConfig(schemaBody, compatibility string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema" "test" {
  cluster_id     = %q
  subject        = "mock-subject"
  schema         = %q
  schema_type    = "AVRO"
  compatibility  = %q
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = true
}
`, mockSchemaClusterID, schemaBody, compatibility)
}

// schemaSetup returns a mock.Server pre-seeded with one cluster (id=clusterID)
// plus the standard provider factory map. Mirrors integration.Setup but seeds the
// cluster fake so the SR resource has a cluster to look up. The SR fake URL is
// stamped on the cluster automatically via Cluster.SetSchemaRegistryURL in
// mock.New.
func schemaSetup(t *testing.T, clusterIDs ...string) (srv *mock.Server, factories map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")
	srv = mock.New(t)
	for _, id := range clusterIDs {
		srv.Cluster.Seed(&controlplanev1.Cluster{
			Id:    id,
			Name:  "mock-schema-cluster-" + id,
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

// schemaCfg returns minimal HCL for redpanda_schema with explicit schema_type
// and compatibility. No references; uses Basic auth (username + password).
func schemaCfg(clusterID, subject, schemaBody, schemaType, compatibility string, allowDeletion bool) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema" "test" {
  cluster_id     = %q
  subject        = %q
  schema         = %q
  schema_type    = %q
  compatibility  = %q
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = %t
}
`, clusterID, subject, schemaBody, schemaType, compatibility, allowDeletion)
}

// schemaCfgWithRef returns HCL with a single populated references entry.
// The SR fake stores references as-is without resolution, so the referenced
// subject does not need to exist for the test to succeed.
func schemaCfgWithRef(clusterID, subject, schemaBody, refName, refSubject string, refVersion int) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema" "test" {
  cluster_id     = %q
  subject        = %q
  schema         = %q
  schema_type    = "AVRO"
  compatibility  = "BACKWARD"
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = true

  references = [
    {
      name    = %q
      subject = %q
      version = %d
    },
  ]
}
`, clusterID, subject, schemaBody, refName, refSubject, refVersion)
}

// TestIntegration_Schema_CreateAndRefresh proves the full create + noop cycle. Every
// non-write-only leaf is asserted at exact value. The two CompareValue wires
// (id, version) are the UseStateForUnknown / stable-on-noop proof — the SR
// fake dedupes on body+type, so a clean re-apply does not allocate a new
// schema id.
func TestIntegration_Schema_CreateAndRefresh(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterID2)

	subject := "tfrp-mock-schema-car"
	cfg := schemaCfg(mockSchemaClusterID2, subject, avroUserV1, "AVRO", "BACKWARD", true)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())
	versionPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSchemaClusterID2)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("subject"), knownvalue.StringExact(subject)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("schema_type"), knownvalue.StringExact("AVRO")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("compatibility"), knownvalue.StringExact("BACKWARD")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references"), knownvalue.Null()),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("username"), knownvalue.StringExact("mock-user")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("password"), knownvalue.StringExact("mock-pass")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("password_wo"), knownvalue.Null()),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("allow_deletion"), knownvalue.Bool(true)),
				idPreserved.AddStateValue(schemaAddr, tfjsonpath.New("id")),
				versionPreserved.AddStateValue(schemaAddr, tfjsonpath.New("version")),
			}),
			integration.NoopReapplyStep(schemaAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("subject"), knownvalue.StringExact(subject)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("schema_type"), knownvalue.StringExact("AVRO")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("compatibility"), knownvalue.StringExact("BACKWARD")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references"), knownvalue.Null()),
				idPreserved.AddStateValue(schemaAddr, tfjsonpath.New("id")),
				versionPreserved.AddStateValue(schemaAddr, tfjsonpath.New("version")),
			}),
		},
	})
}

// TestIntegration_Schema_UpdateLeaf_Schema mutates the schema body in place. The SR
// fake's handleRegisterSchema allocates a new id on body change AND bumps the
// per-subject version. Both id and version use ValuesDiffer to assert the
// change actually happened — the id check is the load-bearing proof that the
// fake registered a new schema entry rather than returning the dedup cache.
func TestIntegration_Schema_UpdateLeaf_Schema(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterID3)

	subject := "tfrp-mock-schema-upd-body"
	cfgV1 := schemaCfg(mockSchemaClusterID3, subject, avroEventV1, "AVRO", "BACKWARD", true)
	cfgV2 := schemaCfg(mockSchemaClusterID3, subject, avroEventV2, "AVRO", "BACKWARD", true)

	versionBumped := statecheck.CompareValue(compare.ValuesDiffer())
	idDiffers := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfgV1, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				versionBumped.AddStateValue(schemaAddr, tfjsonpath.New("version")),
				idDiffers.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(schemaAddr, cfgV2, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(2)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				versionBumped.AddStateValue(schemaAddr, tfjsonpath.New("version")),
				idDiffers.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Schema_UpdateLeaf_Compatibility mutates only the compatibility
// level. Body is unchanged → no new schema registered → version and id are
// stable. The ValuesSame wires on both id and version pin this contract: a
// compat-only update must not bump the schema-id or version.
func TestIntegration_Schema_UpdateLeaf_Compatibility(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterID4)

	subject := "tfrp-mock-schema-upd-compat"
	cfgBackward := schemaCfg(mockSchemaClusterID4, subject, avroUserV1, "AVRO", "BACKWARD", true)
	cfgFull := schemaCfg(mockSchemaClusterID4, subject, avroUserV1, "AVRO", "FULL", true)

	versionStable := statecheck.CompareValue(compare.ValuesSame())
	idStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfgBackward, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("compatibility"), knownvalue.StringExact("BACKWARD")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				versionStable.AddStateValue(schemaAddr, tfjsonpath.New("version")),
				idStable.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(schemaAddr, cfgFull, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("compatibility"), knownvalue.StringExact("FULL")),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				versionStable.AddStateValue(schemaAddr, tfjsonpath.New("version")),
				idStable.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Schema_RequiresReplace_Subject mutates subject → DestroyBeforeCreate.
// Both subjects use the same body, so the SR fake's dedup logic would normally
// return the same id. But the fake's dedup is per-subject (lookup in
// f.subjects[subject]), so a new subject always allocates a fresh entry → fresh
// id. The idDiffers wire is the load-bearing RR proof.
func TestIntegration_Schema_RequiresReplace_Subject(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterID5)

	subjectA := "tfrp-mock-schema-rr-subj-a"
	subjectB := "tfrp-mock-schema-rr-subj-b"
	cfgA := schemaCfg(mockSchemaClusterID5, subjectA, avroUserV1, "AVRO", "BACKWARD", true)
	cfgB := schemaCfg(mockSchemaClusterID5, subjectB, avroUserV1, "AVRO", "BACKWARD", true)

	idDiffers := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfgA, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("subject"), knownvalue.StringExact(subjectA)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idDiffers.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(schemaAddr, cfgB, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("subject"), knownvalue.StringExact(subjectB)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idDiffers.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Schema_RequiresReplace_ClusterID mutates cluster_id between two
// seeded clusters. Both clusters share the SR fake (one httptest server backs
// every seeded cluster's SchemaRegistry.Url), so the actual registry traffic
// goes to the same in-memory store. The id-differs proof still holds because
// the destroy-side Delete clears the SR entry before the create-side fires.
func TestIntegration_Schema_RequiresReplace_ClusterID(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterID6, mockSchemaClusterID7)

	subject := "tfrp-mock-schema-rr-cluster"
	cfgC6 := schemaCfg(mockSchemaClusterID6, subject, avroUserV1, "AVRO", "BACKWARD", true)
	cfgC7 := schemaCfg(mockSchemaClusterID7, subject, avroUserV1, "AVRO", "BACKWARD", true)

	idDiffers := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfgC6, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSchemaClusterID6)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idDiffers.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(schemaAddr, cfgC7, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSchemaClusterID7)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idDiffers.AddStateValue(schemaAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Schema_NestedMatrix_References_Full covers the Full density of the
// references list. Null density is covered by CreateAndRefresh (references is
// Null there). Empty and Partial densities are unsatisfiable: references'
// sub-fields (name, subject, version) are all Required at the schema level,
// so an empty references entry or one with null sub-fields would fail
// framework validation. Only Null and Full are applicable.
//
// The SR fake stores references as-is without resolution, so the referenced
// subject does not need to exist in the fake. The noop step proves the
// references list survives via UseStateForUnknown.
func TestIntegration_Schema_NestedMatrix_References_Full(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterID8)

	subject := "tfrp-mock-schema-refs-full"
	refName := "BaseSchema"
	refSubject := "tfrp-mock-base-subject"
	refVersion := 1
	cfg := schemaCfgWithRef(mockSchemaClusterID8, subject, avroUserV1, refName, refSubject, refVersion)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact(refName)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references").AtSliceIndex(0).AtMapKey("subject"), knownvalue.StringExact(refSubject)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references").AtSliceIndex(0).AtMapKey("version"), knownvalue.Int64Exact(int64(refVersion))),
			}),
			integration.NoopReapplyStep(schemaAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact(refName)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references").AtSliceIndex(0).AtMapKey("subject"), knownvalue.StringExact(refSubject)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("references").AtSliceIndex(0).AtMapKey("version"), knownvalue.Int64Exact(int64(refVersion))),
			}),
		},
	})
}

// TestIntegration_Schema_ImportRoundTrip verifies the Bearer-form composite import
// path (cluster_id:subject:version). ImportState is hand-written and does not
// call a live ControlPlane — it's pure string parsing + attribute setting.
//
// verifyIgnore covers:
//   - allow_deletion: ImportState hardcodes false; pre-import state has true
//     (resource_schema.go:133).
//   - username, password: not populated by ImportState in the 3-part Bearer
//     form (resource_schema.go:113-129).
//   - password_wo_version: Computed with UseStateForUnknown; diverges between
//     import-Read and prior state.
func TestIntegration_Schema_ImportRoundTrip(t *testing.T) {
	clusterID := "mockschemaimport0001"
	_, factories := schemaSetup(t, clusterID)

	subject := "tfrp-mock-schema-import"
	cfg := schemaCfg(clusterID, subject, avroUserV1, "AVRO", "BACKWARD", true)

	idFunc := func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[schemaAddr]
		if !ok {
			return "", fmt.Errorf("resource %q not found in state", schemaAddr)
		}
		return fmt.Sprintf("%s,%s,%s",
			rs.Primary.Attributes["cluster_id"],
			rs.Primary.Attributes["subject"],
			rs.Primary.Attributes["version"],
		), nil
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("subject"), knownvalue.StringExact(subject)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(schemaAddr, idFunc, []string{
				"allow_deletion",
				"username",
				"password",
				"password_wo_version",
			}),
		},
	})
}

func TestIntegration_Schema_ErrorPath_CreateFails(t *testing.T) {
	srv, factories := schemaSetup(t, mockSchemaClusterID9)

	subject := "tfrp-mock-schema-create-fail"
	cfg := schemaCfg(mockSchemaClusterID9, subject, avroUserV1, "AVRO", "BACKWARD", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.RESTErrorPathStep(
				srv.SR,
				"POST", "/subjects/"+subject+"/versions",
				500, `{"error_code":50001,"message":"synth-create-err"}`,
				cfg, "synth-create-err",
			),
		},
	})
}

func TestIntegration_Schema_ErrorPath_ReadFails(t *testing.T) {
	srv, factories := schemaSetup(t, mockSchemaClusterIDa)

	subject := "tfrp-mock-schema-read-fail"
	cfg := schemaCfg(mockSchemaClusterIDa, subject, avroUserV1, "AVRO", "BACKWARD", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfg, nil),
			{
				PreConfig: func() {
					srv.SR.OverrideOnceHTTP(
						"GET", "/subjects/"+subject+"/versions/1",
						500, `{"error_code":50001,"message":"synth-read-err"}`,
					)
				},
				Config:      cfg,
				ExpectError: regexp.MustCompile("synth-read-err"),
			},
		},
	})
}

func TestIntegration_Schema_ErrorPath_DeleteFails(t *testing.T) {
	srv, factories := schemaSetup(t, mockSchemaClusterIDb)

	subject := "tfrp-mock-schema-delete-fail"
	cfg := schemaCfg(mockSchemaClusterIDb, subject, avroUserV1, "AVRO", "BACKWARD", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(schemaAddr, cfg, nil),
			{
				PreConfig: func() {
					srv.SR.OverrideOnceHTTP(
						"DELETE", "/subjects/"+subject,
						500, `{"error_code":50001,"message":"synth-delete-err"}`,
					)
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("synth-delete-err"),
			},
		},
	})
}

// schemaCfgAndDatasource returns HCL with the resource + datasource in the same config,
// so the datasource depends on the resource having been registered first.
func schemaCfgAndDatasource(clusterID, subject, schemaBody string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema" "test" {
  cluster_id     = %q
  subject        = %q
  schema         = %q
  schema_type    = "AVRO"
  compatibility  = "BACKWARD"
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = true
}

data "redpanda_schema" "test" {
  cluster_id = %q
  subject    = %q
  depends_on = [redpanda_schema.test]
}
`, clusterID, subject, schemaBody, clusterID, subject)
}

const datasourceAddr = "data.redpanda_schema.test"

// TestIntegration_SchemaDataSource_ReadLatest verifies the datasource fetches the
// latest version of a schema registered via the resource.
func TestIntegration_SchemaDataSource_ReadLatest(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterIDc)

	subject := "tfrp-mock-ds-schema-latest"
	combinedCfg := schemaCfgAndDatasource(mockSchemaClusterIDc, subject, avroUserV1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: combinedCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(datasourceAddr, "subject", subject),
					resource.TestCheckResourceAttr(datasourceAddr, "schema_type", "AVRO"),
					resource.TestCheckResourceAttr(datasourceAddr, "version", "1"),
					resource.TestCheckResourceAttrSet(datasourceAddr, "id"),
					resource.TestCheckResourceAttrSet(datasourceAddr, "schema"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(datasourceAddr, tfjsonpath.New("cluster_id"), knownvalue.StringExact(mockSchemaClusterIDc)),
					statecheck.ExpectKnownValue(datasourceAddr, tfjsonpath.New("subject"), knownvalue.StringExact(subject)),
					statecheck.ExpectKnownValue(datasourceAddr, tfjsonpath.New("schema_type"), knownvalue.StringExact("AVRO")),
					statecheck.ExpectKnownValue(datasourceAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue(datasourceAddr, tfjsonpath.New("references"), knownvalue.Null()),
				},
			},
		},
	})
}

// schemaCfgAndDatasourceWithVersion returns HCL with resource + datasource pinned to a specific version.
func schemaCfgAndDatasourceWithVersion(clusterID, subject, schemaBody string, version int) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema" "test" {
  cluster_id     = %q
  subject        = %q
  schema         = %q
  schema_type    = "AVRO"
  compatibility  = "BACKWARD"
  username       = "mock-user"
  password       = "mock-pass"
  allow_deletion = true
}

data "redpanda_schema" "test" {
  cluster_id = %q
  subject    = %q
  version    = %d
  depends_on = [redpanda_schema.test]
}
`, clusterID, subject, schemaBody, clusterID, subject, version)
}

// TestIntegration_SchemaDataSource_ReadSpecificVersion verifies the datasource fetches
// a specific version when the version attribute is set.
func TestIntegration_SchemaDataSource_ReadSpecificVersion(t *testing.T) {
	_, factories := schemaSetup(t, mockSchemaClusterIDd)

	subject := "tfrp-mock-ds-schema-version"
	// Register V2; then the datasource reads V1 — both co-exist in the fake.
	cfgV2WithDsV1 := schemaCfgAndDatasourceWithVersion(mockSchemaClusterIDd, subject, avroEventV2, 1)
	cfgV1Only := schemaCfg(mockSchemaClusterIDd, subject, avroEventV1, "AVRO", "BACKWARD", true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			// Seed V1 so the fake has something at version 1.
			integration.CreateStep(schemaAddr, cfgV1Only, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
			}),
			// Update to V2 so two versions exist, then read V1 via the datasource.
			{
				Config: cfgV2WithDsV1,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(schemaAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue(datasourceAddr, tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
				},
			},
		},
	})
}
