//go:build live_test && (all || serverless_privatelink)

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

package serverlessprivatelink_test

import (
	"context"
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

func TestAcc_ServerlessPrivateLink(t *testing.T) {
	ctx := context.Background()

	name := acc.RandomName(acc.NamePrefix + "spl-test")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["private_link_name"] = config.StringVariable(name)
	origTestCaseVars["serverless_region"] = config.StringVariable("eu-west-1")
	origTestCaseVars["allowed_principals"] = config.ListVariable(
		config.StringVariable("arn:aws:iam::123456789012:root"),
	)

	allowDeleteVars := make(map[string]config.Variable)
	maps.Copy(allowDeleteVars, origTestCaseVars)
	allowDeleteVars["allow_deletion"] = config.BoolVariable(true)

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, allowDeleteVars)
	updateTestCaseVars["allowed_principals"] = config.ListVariable(
		config.StringVariable("arn:aws:iam::123456789012:root"),
		config.StringVariable("arn:aws:iam::987654321098:root"),
	)

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	acc.Register(acc.KindServerlessPrivateLink, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ServerlessPrivateLink{PrivateLinkName: name, Client: c}.SweepServerlessPrivateLink("")
	}))
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
	}))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessPrivateLinkDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ServerlessPrivateLinkResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ServerlessPrivateLinkResourceName, "cloud_provider", "aws"),
					resource.TestCheckResourceAttr(acc.ServerlessPrivateLinkResourceName, "serverless_region", "eu-west-1"),
					resource.TestCheckResourceAttrSet(acc.ServerlessPrivateLinkResourceName, "id"),
					resource.TestCheckResourceAttrSet(acc.ServerlessPrivateLinkResourceName, "resource_group_id"),
					resource.TestCheckResourceAttrSet(acc.ServerlessPrivateLinkResourceName, "state"),
					resource.TestCheckResourceAttrSet(acc.ServerlessPrivateLinkResourceName, "status.aws.vpc_endpoint_service_name"),
					resource.TestCheckResourceAttrSet(acc.ServerlessPrivateLinkResourceName, "status.aws.availability_zones.#"),
					resource.TestCheckResourceAttrSet(acc.ServerlessPrivateLinkResourceName, "status.aws.availability_zones.0"),
					resource.TestCheckResourceAttr(acc.ServerlessPrivateLinkResourceName, "aws_config.allowed_principals.#", "1"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessPrivateLinkDir),
				ConfigVariables:          allowDeleteVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessPrivateLinkDir),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ServerlessPrivateLinkResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ServerlessPrivateLinkResourceName, "aws_config.allowed_principals.#", "2"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:             acc.ServerlessPrivateLinkResourceName,
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessPrivateLinkDir),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"updated_at", "allow_deletion"},
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
		},
	})
}
