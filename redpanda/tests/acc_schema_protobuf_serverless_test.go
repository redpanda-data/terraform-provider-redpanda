//go:build live_test && (all || serverless_aws_public)

// Copyright 2025 Redpanda Data, Inc.
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

package tests

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

// Each body is authored message-first with a package-relative type reference,
// so Schema Registry canonicalizes it on write (enum ahead of the message,
// fully-qualified ref). The provider must preserve the submitted form to plan
// clean. V2/V3 add fields (compatibility = NONE accepts any change).
const (
	protobufBodyV1 = `syntax = "proto3";
package tfrp.acc.v1;

message Event {
  Status status = 1;
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
}
`
	protobufBodyV2 = `syntax = "proto3";
package tfrp.acc.v1;

message Event {
  Status status = 1;
  string id = 2;
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
}
`
	protobufBodyV3 = `syntax = "proto3";
package tfrp.acc.v1;

message Event {
  Status status = 1;
  string id = 2;
  int64 ts = 3;
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
  STATUS_CLOSED = 2;
}
`
	// Each avro body differs from its predecessor in exactly one metadata key
	// (doc, then default, then aliases, then order); every one must register a
	// new version.
	avroDocBodyV1     = `{"type":"record","name":"AccUser","doc":"v1 doc","fields":[{"name":"id","type":"string"}]}`
	avroDocBodyV2     = `{"type":"record","name":"AccUser","doc":"v2 doc","fields":[{"name":"id","type":"string"}]}`
	avroDefaultBodyV3 = `{"type":"record","name":"AccUser","doc":"v2 doc","fields":[{"name":"id","type":"string","default":"anon"}]}`
	avroAliasesBodyV4 = `{"type":"record","name":"AccUser","doc":"v2 doc","fields":[{"name":"id","type":"string","default":"anon","aliases":["ident"]}]}`
	avroOrderBodyV5   = `{"type":"record","name":"AccUser","doc":"v2 doc","fields":[{"name":"id","type":"string","default":"anon","aliases":["ident"],"order":"descending"}]}`

	// JSON bodies vary one annotation keyword at a time (description, then
	// default); each must register a new version.
	jsonBodyV1        = `{"type":"object","properties":{"id":{"type":"integer"}}}`
	jsonDescBodyV2    = `{"type":"object","description":"v2 desc","properties":{"id":{"type":"integer"}}}`
	jsonDefaultBodyV3 = `{"type":"object","description":"v2 desc","properties":{"id":{"type":"integer","default":0}}}`
)

// protobufBodyV3Comment is protobufBodyV3 plus a comment — derived so the
// comment-only premise holds by construction. Schema Registry's protobuf
// canonical form drops comments, so a comment-only change must NOT mint a
// new version — and the provider's equivalence check must plan clean after
// applying it.
var protobufBodyV3Comment = strings.Replace(protobufBodyV3, "message Event", "// comment-only change: must not version\nmessage Event", 1)

// TestAcc_Schema_Protobuf_Serverless_Lifecycle provisions a serverless cluster
// and drives protobuf, Avro, and JSON schemas through create + six updates,
// asserting an empty plan after each apply (the canonicalization round-trip
// holds across the lifecycle). Metadata-only Avro (doc/default/aliases/order)
// and JSON (description/default) changes must each register a new version; a
// protobuf comment-only change must not. Flipping the Avro body back to an
// already-registered form and forward again pins SR's identical-body dedup:
// the existing version is returned, none minted. Final imports read back the
// registry-stored protobuf and Avro bodies so the run reports whether
// Redpanda actually canonicalized them. Destroy is implicit.
func TestAcc_Schema_Protobuf_Serverless_Lifecycle(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(acc.RepoRoot(), "redpanda", "tests", "testdata", "schema_protobuf_serverless")
	const schemaAddr = "redpanda_schema.protobuf_test"
	const avroAddr = "redpanda_schema.avro_doc_test"
	const jsonAddr = "redpanda_schema.json_doc_test"

	name := acc.RandomName(acc.NamePrefix + "spb")

	baseVars := make(map[string]config.Variable)
	maps.Copy(baseVars, acc.ProviderCfgIDSecretVars)
	baseVars["resource_group_name"] = config.StringVariable(name)
	baseVars["cluster_name"] = config.StringVariable(name)
	baseVars["region"] = config.StringVariable("eu-west-1")

	withBody := func(protoBody, avroBody, jsonBody string) map[string]config.Variable {
		v := make(map[string]config.Variable)
		maps.Copy(v, baseVars)
		v["protobuf_schema_definition"] = config.StringVariable(protoBody)
		v["avro_schema_definition"] = config.StringVariable(avroBody)
		v["json_schema_definition"] = config.StringVariable(jsonBody)
		return v
	}

	versions := func(protoVer, avroVer, jsonVer string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(schemaAddr, "version", protoVer),
			resource.TestCheckResourceAttr(avroAddr, "version", avroVer),
			resource.TestCheckResourceAttr(jsonAddr, "version", jsonVer),
		)
	}

	// Shared by the last apply step and both import steps: the import
	// read-backs are only meaningful against the last-applied variables.
	finalVars := withBody(protobufBodyV3Comment, avroOrderBodyV5, jsonDefaultBodyV3)

	importID := func(addr string) resource.ImportStateIdFunc {
		return func(s *terraform.State) (string, error) {
			rs, ok := s.RootModule().Resources[addr]
			if !ok {
				return "", fmt.Errorf("%s not found in state", addr)
			}
			a := rs.Primary.Attributes
			return fmt.Sprintf("%s,%s,%s", a["cluster_id"], a["subject"], a["version"]), nil
		}
	}

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}
	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: name, Client: c}.SweepServerlessCluster("")
	}))
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
	}))

	emptyPlan := resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{ // create
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV1, avroDocBodyV1, jsonBodyV1),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("1", "1", "1"),
			},
			{ // proto adds a field; avro changes only doc; json only description
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV2, avroDocBodyV2, jsonDescBodyV2),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("2", "2", "2"),
			},
			{ // proto adds field+enum value; avro and json change only default
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV3, avroDefaultBodyV3, jsonDefaultBodyV3),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("3", "3", "3"),
			},
			{ // proto changes only a comment (no new version); avro only aliases
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV3Comment, avroAliasesBodyV4, jsonDefaultBodyV3),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("3", "4", "3"),
			},
			{ // avro changes only order
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          finalVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("3", "5", "3"),
			},
			{ // avro flips back to the already-registered V4 body: SR dedups, version returns to 4
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV3Comment, avroAliasesBodyV4, jsonDefaultBodyV3),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("3", "4", "3"),
			},
			{ // avro flips forward to the V5 body again: dedup returns existing version 5, no 6 minted
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          finalVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
				Check:                    versions("3", "5", "3"),
			},
			{ // read back the registry-stored protobuf body to confirm canonicalization
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          finalVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ResourceName:             schemaAddr,
				ImportState:              true,
				ImportStateVerify:        false,
				ImportStateIdFunc:        importID(schemaAddr),
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					stored := states[0].Attributes["schema"]
					if stored == protobufBodyV3 {
						t.Logf("CANONICALIZATION: registry stored the body VERBATIM (== submitted V3) — protobuf fix is dormant on Redpanda")
					} else {
						t.Logf("CANONICALIZATION: registry REWROTE the body (!= submitted V3) — protobuf fix is load-bearing.\n--- stored ---\n%s", stored)
					}
					return nil
				},
			},
			{ // read back the registry-stored avro body: the metadata-heavy final form
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          finalVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ResourceName:             avroAddr,
				ImportState:              true,
				ImportStateVerify:        false,
				ImportStateIdFunc:        importID(avroAddr),
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					stored := states[0].Attributes["schema"]
					if stored == avroOrderBodyV5 {
						t.Logf("CANONICALIZATION: registry stored the avro body VERBATIM (== submitted V5) — metadata survives as-authored")
					} else {
						t.Logf("CANONICALIZATION: registry REWROTE the avro body (!= submitted V5) — layered equivalence is load-bearing.\n--- stored ---\n%s", stored)
					}
					return nil
				},
			},
		},
	})
}
