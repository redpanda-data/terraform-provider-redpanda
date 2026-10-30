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
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

var noCredentialsErr = regexp.MustCompile(`(?s)Redpanda provider has no credentials.*skip_credentials_validation`)

func credentialFactories(t *testing.T) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()
	_, factories := integration.SetupWithProviderAuth(t)
	return factories
}

const (
	skipFlagged = `
provider "redpanda" {
  skip_credentials_validation = true
}
`
	ignoreEnvFlagged = `
provider "redpanda" {
  ignore_environment_credentials = true
}
`
	unusedResources = `
resource "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}

data "redpanda_resource_group" "unused" {
  count = 0
  name  = "tfrp-mock-unused"
}
`
	forgotAlias = `
resource "redpanda_resource_group" "forgot_alias" {
  name = "tfrp-mock-forgot"
}
`
)

func planOnly(t *testing.T, factories map[string]func() (tfprotov6.ProviderServer, error), config string, expectErr *regexp.Regexp) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps:                    []resource.TestStep{{PlanOnly: true, Config: config, ExpectError: expectErr}},
	})
}

func TestIntegration_ProviderFlags_UnusedResourcesPlanClean(t *testing.T) {
	for name, flagged := range map[string]string{"skip": skipFlagged, "ignore_env": ignoreEnvFlagged} {
		t.Run(name, func(t *testing.T) { planOnly(t, credentialFactories(t), flagged+unusedResources, nil) })
	}
}

func TestIntegration_ProviderFlags_EmptyStringCredentialsPlanClean(t *testing.T) {
	for name, flag := range map[string]string{"skip": "skip_credentials_validation", "ignore_env": "ignore_environment_credentials"} {
		t.Run(name, func(t *testing.T) {
			planOnly(t, credentialFactories(t), fmt.Sprintf(`
provider "redpanda" {
  %s = true
  client_id     = ""
  client_secret = ""
}
`, flag)+unusedResources, nil)
		})
	}
}

func TestIntegration_ProviderFlags_ResourceWithoutCredentialsFailsAtPlan(t *testing.T) {
	for name, flagged := range map[string]string{"skip": skipFlagged, "ignore_env": ignoreEnvFlagged} {
		t.Run(name, func(t *testing.T) { planOnly(t, credentialFactories(t), flagged+forgotAlias, noCredentialsErr) })
	}
}

func TestIntegration_ProviderFlags_DataSourceWithoutCredentialsFailsAtPlan(t *testing.T) {
	planOnly(t, credentialFactories(t), skipFlagged+`
data "redpanda_resource_group" "forgot_alias" {
  name = "tfrp-mock-forgot"
}
`, noCredentialsErr)
}

func TestIntegration_ProviderFlags_BlockCredentialsStillUsed(t *testing.T) {
	srv, factories := integration.SetupWithProviderAuth(t)
	srv.RequireBearerToken("mock-token")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `
provider "redpanda" {
  skip_credentials_validation    = true
  ignore_environment_credentials = true
  access_token                   = "mock-token"
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

func TestIntegration_ProviderSkipCredentials_UsesEnvironmentCredentials(t *testing.T) {
	srv, factories := integration.SetupWithProviderAuth(t)
	t.Setenv(redpanda.AccessTokenEnv, "env-token")
	srv.RequireBearerToken("env-token")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: skipFlagged + `
resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-from-env"
}
`,
			Check: resource.TestCheckResourceAttr("redpanda_resource_group.test", "name", "tfrp-mock-from-env"),
		}},
	})
}

func TestIntegration_ProviderIgnoreEnvironment_EnvironmentCredentialsIgnored(t *testing.T) {
	factories := credentialFactories(t)
	t.Setenv(redpanda.AccessTokenEnv, "env-token")
	t.Run("resource", func(t *testing.T) { planOnly(t, factories, ignoreEnvFlagged+forgotAlias, noCredentialsErr) })
	t.Run("data source", func(t *testing.T) {
		planOnly(t, factories, ignoreEnvFlagged+`
data "redpanda_resource_group" "forgot_alias" {
  name = "tfrp-mock-forgot"
}
`, noCredentialsErr)
	})
}

func TestIntegration_ProviderBlockCredentialsWinOverEnvironment(t *testing.T) {
	srv, factories := integration.SetupWithProviderAuth(t)
	t.Setenv(redpanda.AccessTokenEnv, "env-token")
	srv.RequireBearerToken("mock-token")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `
provider "redpanda" {
  access_token = "mock-token"
}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-block-wins"
}
`,
			Check: resource.TestCheckResourceAttr("redpanda_resource_group.test", "name", "tfrp-mock-block-wins"),
		}},
	})
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

func TestIntegration_ProviderWithoutFlags_MissingCredentialsFailsConfigure(t *testing.T) {
	planOnly(t, credentialFactories(t), `
provider "redpanda" {}
`+unusedResources, regexp.MustCompile(`Client configuration missing`))
}

func TestIntegration_ProviderFlags_FromVariablePlansClean(t *testing.T) {
	planOnly(t, credentialFactories(t), `
variable "redpanda_enabled" {
  default = false
}

provider "redpanda" {
  skip_credentials_validation = !var.redpanda_enabled
}
`+unusedResources, nil)
}

func TestIntegration_ProviderFlags_UnknownFlagFailsAtPlan(t *testing.T) {
	for _, flag := range []string{"skip_credentials_validation", "ignore_environment_credentials"} {
		t.Run(flag, func(t *testing.T) {
			planOnly(t, credentialFactories(t), fmt.Sprintf(`
resource "terraform_data" "flag" {
  input = true
}

provider "redpanda" {
  %s = terraform_data.flag.output
}
`, flag)+unusedResources, regexp.MustCompile(flag+`.*must be known at plan`))
		})
	}
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

func TestIntegration_ProviderFlags_IgnoresCloudProviderEnv(t *testing.T) {
	t.Setenv("GOOGLE_CREDENTIALS_BASE64", "!!!notbase64")
	planOnly(t, credentialFactories(t), skipFlagged+unusedResources, nil)
}

func TestIntegration_ProviderFlags_SkipAuthHarnessHonoursIgnoreEnvironment(t *testing.T) {
	_, factories := integration.Setup(t)
	planOnly(t, factories, ignoreEnvFlagged+forgotAlias, noCredentialsErr)
}

func TestIntegration_ProviderFlags_SkipAuthHarnessKeepsBlockCredentials(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.RequireBearerToken("mock-token")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `
provider "redpanda" {
  ignore_environment_credentials = true
  access_token                   = "mock-token"
}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-skipauth-block"
}
`,
			Check: resource.TestCheckResourceAttr("redpanda_resource_group.test", "name", "tfrp-mock-skipauth-block"),
		}},
	})
}

func TestIntegration_ProviderFlags_SkipAuthHarnessRejectsUnknownValues(t *testing.T) {
	_, factories := integration.Setup(t)
	planOnly(t, factories, `
resource "terraform_data" "flag" {
  input = true
}

provider "redpanda" {
  ignore_environment_credentials = terraform_data.flag.output
}
`+unusedResources, regexp.MustCompile(`ignore_environment_credentials.*must be known at plan`))
}

func TestIntegration_ProviderFlags_CountZeroOrphanNeedsCredentials(t *testing.T) {
	const orphan = `
resource "redpanda_resource_group" "rg" {
  count = %d
  name  = "tfrp-mock-orphan"
}
`
	withToken := `
provider "redpanda" {
  access_token = "mock-token"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{
			{Config: withToken + fmt.Sprintf(orphan, 1)},
			{Config: skipFlagged + fmt.Sprintf(orphan, 0), ExpectError: noCredentialsErr},
			{Config: withToken + fmt.Sprintf(orphan, 0)},
		},
	})
}

func TestIntegration_ProviderFlags_DeferredDataSourceFailsAtApply(t *testing.T) {
	config := skipFlagged + `
resource "terraform_data" "dep" {}

data "redpanda_resource_group" "deferred" {
  name       = "tfrp-mock-deferred"
  depends_on = [terraform_data.dep]
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: credentialFactories(t),
		Steps: []resource.TestStep{
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config, ExpectError: noCredentialsErr},
		},
	})
}
