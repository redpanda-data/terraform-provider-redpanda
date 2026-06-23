//go:build live_test && (all || serverless_aws_public)

// Copyright 2025 Redpanda Data, Inc.
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

package tests

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
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
)

// TestAcc_Schema_Protobuf_Serverless_Lifecycle provisions a serverless cluster
// and drives a protobuf schema through create + two updates, asserting an empty
// plan after each apply (the canonicalization round-trip holds across the
// lifecycle). A final import reads back the registry-stored body so the run
// reports whether Redpanda actually canonicalized it. Destroy is implicit.
func TestAcc_Schema_Protobuf_Serverless_Lifecycle(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(acc.RepoRoot(), "redpanda", "tests", "testdata", "schema_protobuf_serverless")
	const schemaAddr = "redpanda_schema.protobuf_test"

	name := acc.RandomName(acc.NamePrefix + "spb")

	baseVars := make(map[string]config.Variable)
	maps.Copy(baseVars, acc.ProviderCfgIDSecretVars)
	baseVars["resource_group_name"] = config.StringVariable(name)
	baseVars["cluster_name"] = config.StringVariable(name)
	baseVars["region"] = config.StringVariable("eu-west-1")

	withBody := func(body string) map[string]config.Variable {
		v := make(map[string]config.Variable)
		maps.Copy(v, baseVars)
		v["protobuf_schema_definition"] = config.StringVariable(body)
		return v
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
				ConfigVariables:          withBody(protobufBodyV1),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
			},
			{ // update 1
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV2),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
			},
			{ // update 2
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV3),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks:         emptyPlan,
			},
			{ // read back the registry-stored body to confirm canonicalization
				ConfigDirectory:          config.StaticDirectory(dir),
				ConfigVariables:          withBody(protobufBodyV3),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ResourceName:             schemaAddr,
				ImportState:              true,
				ImportStateVerify:        false,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[schemaAddr]
					if !ok {
						return "", fmt.Errorf("%s not found in state", schemaAddr)
					}
					a := rs.Primary.Attributes
					return fmt.Sprintf("%s,%s,%s", a["cluster_id"], a["subject"], a["version"]), nil
				},
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
		},
	})
}
