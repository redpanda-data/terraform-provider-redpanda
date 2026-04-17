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

package serverlesscluster_test

// Acceptance tests for redpanda_serverless_cluster against the
// in-process fake at redpanda/cloud/cloudtest. See
// redpanda/resources/cluster/resource_cluster_acc_test.go for the
// equivalent cluster coverage.

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud/cloudtest"
)

func newAccProtoV6Factories(t *testing.T) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()
	t.Setenv("TF_ACC", "1")
	_, conn := cloudtest.Start(t)
	return map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": providerserver.NewProtocol6WithError(
			redpanda.NewWithTestConn(context.Background(), "test", "acc-test", conn)(),
		),
	}
}

// hclMinimalServerlessCluster builds a minimal
// redpanda_serverless_cluster HCL config with a mutable
// networking_config attribute so Step 2 can force ProposedNewState !=
// PriorState. networking_config.public is updated between steps
// because it's one of the few updateable non-replace fields handled by
// the provider's GenerateServerlessClusterUpdateRequest.
func hclMinimalServerlessCluster(publicState string) string {
	return `
provider "redpanda" {}

resource "redpanda_serverless_cluster" "test" {
  name              = "fake-serverless"
  serverless_region = "pro-us-east-1"
  resource_group_id = "rg-fake"
  networking_config = {
    public  = "` + publicState + `"
    private = "STATE_DISABLED"
  }
}
`
}

// serverlessClusterNonNullStateForUnknownPaths enumerates every
// UseNonNullStateForUnknown path on the redpanda_serverless_cluster
// schema. Update when the schema changes.
var serverlessClusterNonNullStateForUnknownPaths = []tfjsonpath.Path{
	tfjsonpath.New("cluster_api_url"),
	tfjsonpath.New("console_private_url"),
	tfjsonpath.New("console_url"),
	tfjsonpath.New("dataplane_api").AtMapKey("private_url"),
	tfjsonpath.New("dataplane_api").AtMapKey("url"),
	tfjsonpath.New("kafka_api").AtMapKey("private_seed_brokers"),
	tfjsonpath.New("kafka_api").AtMapKey("seed_brokers"),
	tfjsonpath.New("prometheus").AtMapKey("private_url"),
	tfjsonpath.New("prometheus").AtMapKey("url"),
	tfjsonpath.New("schema_registry").AtMapKey("private_url"),
	tfjsonpath.New("schema_registry").AtMapKey("url"),
}

// TestAcc_ServerlessCluster_OptionalComputedParents_NonEmptyDiff
// mirrors TestAcc_Cluster_OptionalComputedParents_NonEmptyDiff:
// Step 2 mutates `tags` to force the framework's Computed-null-config
// marking branch to fire; the PreApply plancheck asserts every
// UseNonNullStateForUnknown path is known (modifier copied non-null
// state forward).
func TestAcc_ServerlessCluster_OptionalComputedParents_NonEmptyDiff(t *testing.T) {
	factories := newAccProtoV6Factories(t)

	checks := make([]plancheck.PlanCheck, 0, len(serverlessClusterNonNullStateForUnknownPaths))
	for _, p := range serverlessClusterNonNullStateForUnknownPaths {
		checks = append(checks, cloudtest.ExpectKnownNotUnknown("redpanda_serverless_cluster.test", p))
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: hclMinimalServerlessCluster("STATE_ENABLED")},
			{
				Config: hclMinimalServerlessCluster("STATE_DISABLED"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: checks,
				},
			},
		},
	})
}
