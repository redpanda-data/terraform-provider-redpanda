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

// Acceptance tests for redpanda_serverless_private_link against the
// in-process fake at redpanda/cloud/cloudtest.

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

// hclServerlessPrivateLink builds a redpanda_serverless_private_link
// HCL config with a mutable allowed_principals list so Step 2 can force
// ProposedNewState != PriorState via Update (provider's
// GenerateServerlessPrivateLinkUpdateRequest handles this field).
func hclServerlessPrivateLink(allowedPrincipal string) string {
	return `
provider "redpanda" {}

resource "redpanda_serverless_private_link" "test" {
  name              = "fake-spl"
  resource_group_id = "rg-fake"
  cloud_provider    = "aws"
  serverless_region = "pro-us-east-1"
  allow_deletion    = true
  cloud_provider_config = {
    aws = {
      allowed_principals = ["` + allowedPrincipal + `"]
    }
  }
}
`
}

// serverlessPrivateLinkNonNullStateForUnknownPaths enumerates every
// UseNonNullStateForUnknown path on the schema. Update when the schema
// changes.
var serverlessPrivateLinkNonNullStateForUnknownPaths = []tfjsonpath.Path{
	tfjsonpath.New("status").AtMapKey("aws").AtMapKey("availability_zones"),
	tfjsonpath.New("status").AtMapKey("aws").AtMapKey("vpc_endpoint_service_name"),
}

// TestAcc_ServerlessPrivateLink_OptionalComputedParents_NonEmptyDiff
// asserts that after a real Update (allowed_principals change), every
// UseNonNullStateForUnknown path remains known.
func TestAcc_ServerlessPrivateLink_OptionalComputedParents_NonEmptyDiff(t *testing.T) {
	factories := newAccProtoV6Factories(t)

	checks := make([]plancheck.PlanCheck, 0, len(serverlessPrivateLinkNonNullStateForUnknownPaths))
	for _, p := range serverlessPrivateLinkNonNullStateForUnknownPaths {
		checks = append(checks, cloudtest.ExpectKnownNotUnknown("redpanda_serverless_private_link.test", p))
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: hclServerlessPrivateLink("arn:aws:iam::111111111111:root")},
			{
				Config: hclServerlessPrivateLink("arn:aws:iam::222222222222:root"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: checks,
				},
			},
		},
	})
}
