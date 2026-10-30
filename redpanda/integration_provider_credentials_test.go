//go:build integration

// Copyright 2026 Redpanda Data, Inc.
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

package redpanda_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
)

func credentialFactories(t *testing.T) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()
	_, factories := integration.SetupWithProviderAuth(t)
	return factories
}

const unusedResources = `
resource "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}

data "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}
`

func planOnly(t *testing.T, factories map[string]func() (tfprotov6.ProviderServer, error), config string, expectErr *regexp.Regexp) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps:                    []resource.TestStep{{PlanOnly: true, Config: config, ExpectError: expectErr}},
	})
}

func TestIntegration_ProviderAccessToken_FromVariableApplies(t *testing.T) {
	srv, factories := integration.SetupWithProviderAuth(t)
	srv.RequireBearerToken("mock-token")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `
variable "tok" {
  default = "mock-token"
}

provider "redpanda" {
  access_token = var.tok
}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-token-var"
}
`,
			Check: resource.TestCheckResourceAttr("redpanda_resource_group.test", "name", "tfrp-mock-token-var"),
		}},
	})
}

func TestIntegration_ProviderAccessToken_UnknownFailsAtPlan(t *testing.T) {
	planOnly(t, credentialFactories(t), `
resource "terraform_data" "tok" {
  input = "mock-token"
}

provider "redpanda" {
  access_token = terraform_data.tok.output
}
`+unusedResources, regexp.MustCompile(`access_token.*must be known at plan`))
}

func TestIntegration_ProviderKnownTokenBesideUnknownPairApplies(t *testing.T) {
	srv, factories := integration.SetupWithProviderAuth(t)
	srv.RequireBearerToken("mock-token")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `
resource "terraform_data" "pair" {
  input = "pending"
}

provider "redpanda" {
  access_token  = "mock-token"
  client_id     = terraform_data.pair.output
  client_secret = terraform_data.pair.output
}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-token-beside-pair"
}
`,
			Check: resource.TestCheckResourceAttr("redpanda_resource_group.test", "name", "tfrp-mock-token-beside-pair"),
		}},
	})
}
