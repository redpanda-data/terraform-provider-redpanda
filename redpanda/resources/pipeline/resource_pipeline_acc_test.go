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

package pipeline_test

// Acceptance tests for redpanda_pipeline against the in-process fake
// at redpanda/cloud/cloudtest. Pipeline dials the dataplane, so this
// test uses cloudtest.StartWithDataplane and
// redpanda.NewWithTestConnAndDataplane to route the dataplane dial
// through an in-process bufconn.

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
	_, cpConn, dpConn := cloudtest.StartWithDataplane(t)
	return map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": providerserver.NewProtocol6WithError(
			redpanda.NewWithTestConnAndDataplane(context.Background(), "test", "acc-test", cpConn, dpConn)(),
		),
	}
}

// hclMinimalPipeline builds a redpanda_pipeline HCL config with a
// mutable description so Step 2 can force ProposedNewState !=
// PriorState. cluster_api_url is a literal that the fake's
// ConnPoolWithSpawnFunc routes to the dataplane bufconn.
func hclMinimalPipeline(description string) string {
	return `
provider "redpanda" {}

resource "redpanda_pipeline" "test" {
  cluster_api_url = "https://fake-dataplane.redpanda.test"
  display_name    = "fake-pipeline"
  description     = "` + description + `"
  config_yaml     = "input: { generate: { mapping: \"root = {}\" } }"
  allow_deletion  = true
}
`
}

// pipelineNonNullStateForUnknownPaths enumerates every
// UseNonNullStateForUnknown path on the redpanda_pipeline schema.
var pipelineNonNullStateForUnknownPaths = []tfjsonpath.Path{
	tfjsonpath.New("status"),
	tfjsonpath.New("status").AtMapKey("error"),
	tfjsonpath.New("url"),
}

// TestAcc_Pipeline_OptionalComputedParents_NonEmptyDiff asserts every
// UseNonNullStateForUnknown path on the pipeline resource stays known
// after an Update that changes `description`.
func TestAcc_Pipeline_OptionalComputedParents_NonEmptyDiff(t *testing.T) {
	factories := newAccProtoV6Factories(t)

	checks := make([]plancheck.PlanCheck, 0, len(pipelineNonNullStateForUnknownPaths))
	for _, p := range pipelineNonNullStateForUnknownPaths {
		checks = append(checks, cloudtest.ExpectKnownNotUnknown("redpanda_pipeline.test", p))
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: hclMinimalPipeline("initial")},
			{
				Config: hclMinimalPipeline("updated"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: checks,
				},
			},
		},
	})
}
