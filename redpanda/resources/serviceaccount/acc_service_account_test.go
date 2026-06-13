//go:build live_test && (all || service_account)

// Copyright 2023 Redpanda Data, Inc.
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

package serviceaccount_test

import (
	"fmt"
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_ServiceAccount(t *testing.T) {
	name := acc.RandomName(acc.NamePrefix + "sa")

	createVars := make(map[string]config.Variable)
	maps.Copy(createVars, acc.ProviderCfgIDSecretVars)
	createVars["service_account_name"] = config.StringVariable(name)
	createVars["service_account_description"] = config.StringVariable("acc-test service account, initial description")

	updateVars := make(map[string]config.Variable)
	maps.Copy(updateVars, createVars)
	updateVars["service_account_description"] = config.StringVariable("acc-test service account, updated description")

	clientSecretPath := tfjsonpath.New("auth0_client_credentials").AtMapKey("client_secret")
	secretPin := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServiceAccountDir),
				ConfigVariables:          createVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(acc.ServiceAccountResourceName, "id"),
					resource.TestCheckResourceAttr(acc.ServiceAccountResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ServiceAccountResourceName, "description", "acc-test service account, initial description"),
					resource.TestCheckResourceAttrSet(acc.ServiceAccountResourceName, "auth0_client_credentials.client_id"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(acc.ServiceAccountResourceName, clientSecretPath, knownvalue.NotNull()),
					secretPin.AddStateValue(acc.ServiceAccountResourceName, clientSecretPath),
				},
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServiceAccountDir),
				ConfigVariables:          updateVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ServiceAccountResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ServiceAccountResourceName, "description", "acc-test service account, updated description"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(acc.ServiceAccountResourceName, clientSecretPath, knownvalue.NotNull()),
					secretPin.AddStateValue(acc.ServiceAccountResourceName, clientSecretPath),
				},
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServiceAccountDir),
				ConfigVariables:          updateVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ResourceName:             acc.ServiceAccountResourceName,
				ImportState:              true,
				ImportStateVerify:        true,
				// role_bindings is Create-only and never echoed by the
				// server, so it is null after import by design.
				ImportStateVerifyIgnore: []string{"role_bindings"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[acc.ServiceAccountResourceName]
					if !ok {
						return "", fmt.Errorf("resource %q not found in state", acc.ServiceAccountResourceName)
					}
					secret := rs.Primary.Attributes["auth0_client_credentials.client_secret"]
					if secret == "" {
						return "", fmt.Errorf("auth0_client_credentials.client_secret missing from state at import time")
					}
					return rs.Primary.ID + ":" + secret, nil
				},
			},
		},
	})
}
