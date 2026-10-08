//go:build live_test && (all || cloud_provider_access)

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

package cloudprovideraccess_test

import (
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

// TestAcc_CloudProviderAccess runs the resource and both datasources against
// the control plane. Create records the role without assuming it, so a
// synthetic ARN needs no AWS resources. The organization must have the
// enable-cross-account-byoc flag.
func TestAcc_CloudProviderAccess(t *testing.T) {
	name := acc.RandomName(acc.NamePrefix + "cpa")
	const (
		prereqs  = "data.redpanda_cloud_provider_access_prerequisites.test"
		lookup   = "data.redpanda_cloud_provider_access.test"
		roleARN  = "arn:aws:iam::123456789012:role/tfrp-acc-cpa"
		otherARN = "arn:aws:iam::123456789012:role/tfrp-acc-cpa-2"
	)

	createVars := make(map[string]config.Variable)
	maps.Copy(createVars, acc.ProviderCfgIDSecretVars)
	createVars["cloud_provider_access_name"] = config.StringVariable(name)
	createVars["role_arn"] = config.StringVariable(roleARN)

	replaceVars := make(map[string]config.Variable)
	maps.Copy(replaceVars, createVars)
	replaceVars["cloud_provider_access_name"] = config.StringVariable(name + "-rotated")
	replaceVars["role_arn"] = config.StringVariable(otherARN)

	externalIDMatchesPrereqs := statecheck.CompareValuePairs(
		acc.CloudProviderAccessResourceName, tfjsonpath.New("aws").AtMapKey("external_id"),
		prereqs, tfjsonpath.New("aws").AtMapKey("external_id"), compare.ValuesSame())
	idReplaced := statecheck.CompareValue(compare.ValuesDiffer())

	steps := acc.UpgradeEntrySteps(t, acc.CloudProviderAccessDir, createVars)
	steps = append(steps, []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(acc.CloudProviderAccessDir),
			ConfigVariables:          createVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("aws").AtMapKey("role_arn"), knownvalue.StringExact(roleARN)),
				statecheck.ExpectKnownValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("state"), knownvalue.StringExact("ACTIVE")),
				statecheck.ExpectKnownValue(prereqs, tfjsonpath.New("aws").AtMapKey("principal_arn"), knownvalue.NotNull()),
				externalIDMatchesPrereqs,
				statecheck.CompareValuePairs(acc.CloudProviderAccessResourceName, tfjsonpath.New("id"), lookup, tfjsonpath.New("id"), compare.ValuesSame()),
				statecheck.ExpectKnownValue(lookup, tfjsonpath.New("aws").AtMapKey("role_arn"), knownvalue.StringExact(roleARN)),
				idReplaced.AddStateValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("id")),
			},
		},
		{
			ConfigDirectory:          config.StaticDirectory(acc.CloudProviderAccessDir),
			ConfigVariables:          replaceVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(acc.CloudProviderAccessResourceName, plancheck.ResourceActionCreateBeforeDestroy),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("aws").AtMapKey("role_arn"), knownvalue.StringExact(otherARN)),
				externalIDMatchesPrereqs,
				idReplaced.AddStateValue(acc.CloudProviderAccessResourceName, tfjsonpath.New("id")),
			},
		},
		{
			ConfigDirectory:          config.StaticDirectory(acc.CloudProviderAccessDir),
			ConfigVariables:          replaceVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ResourceName:             acc.CloudProviderAccessResourceName,
			ImportState:              true,
			ImportStateVerify:        true,
		},
	}...)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps:    steps,
	})
}
