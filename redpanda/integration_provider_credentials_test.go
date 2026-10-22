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
//    limitations under the License.

package redpanda_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

var noCredentialsErr = regexp.MustCompile(`(?s)Redpanda provider has no credentials.*skip_credentials_validation`)

// credentialFactories wires the provider to the bufconn fake without
// WithSkipAuth, so provider Configure resolves credentials exactly as the
// released binary does. The fake does not check bearer tokens. Every provider
// configuration in a test config shares this one reattached server, so the
// last Configure wins; a default-plus-alias config cannot be tested here,
// because Terraform runs one plugin process per configuration.
func credentialFactories(t *testing.T) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")
	t.Setenv(redpanda.ClientIDEnv, "")
	t.Setenv(redpanda.ClientSecretEnv, "")
	t.Setenv(redpanda.AccessTokenEnv, "")
	srv := mock.New(t)
	return map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
		),
	}
}

func TestIntegration_ProviderSkipCredentials_UnusedResourcesPlanClean(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: `
provider "redpanda" {
  skip_credentials_validation = true
}

resource "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}

data "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}
`,
		}},
	})
}

func TestIntegration_ProviderSkipCredentials_EmptyStringCredentialsPlanClean(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: `
provider "redpanda" {
  skip_credentials_validation = true
  client_id                   = ""
  client_secret               = ""
}

resource "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}
`,
		}},
	})
}

func TestIntegration_ProviderSkipCredentials_ResourceWithoutCredentialsFailsAtPlan(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: `
provider "redpanda" {
  skip_credentials_validation = true
}

resource "redpanda_resource_group" "forgot_alias" {
  name = "tfrp-mock-forgot"
}
`,
			ExpectError: noCredentialsErr,
		}},
	})
}

func TestIntegration_ProviderSkipCredentials_DataSourceWithoutCredentialsFailsAtPlan(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: `
provider "redpanda" {
  skip_credentials_validation = true
}

data "redpanda_resource_group" "forgot_alias" {
  name = "tfrp-mock-forgot"
}
`,
			ExpectError: noCredentialsErr,
		}},
	})
}

func TestIntegration_ProviderSkipCredentials_CredentialsStillUsedWhenSet(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{{
			Config: `
provider "redpanda" {
  skip_credentials_validation = true
  access_token                = "mock-token"
}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-with-token"
}
`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
			Check: resource.TestCheckResourceAttr("redpanda_resource_group.test", "name", "tfrp-mock-with-token"),
		}},
	})
}

func TestIntegration_ProviderWithoutSkip_MissingCredentialsFailsConfigure(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: `
provider "redpanda" {}

resource "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}
`,
			ExpectError: regexp.MustCompile(`Client configuration missing`),
		}},
	})
}
