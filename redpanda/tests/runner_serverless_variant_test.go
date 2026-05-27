//go:build live_test && (all || serverless_aws_public || serverless_aws_private || serverless_aws_both || serverless_gcp)

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

package tests

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

func runServerlessClusterVariantTest(t *testing.T, testSuffix, region string, publicNetworking, privateNetworking bool) {
	ctx := context.Background()

	name := acc.RandomName(acc.NamePrefix + testSuffix)
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["region"] = config.StringVariable(region)

	publicState := "STATE_DISABLED"
	if publicNetworking {
		publicState = "STATE_ENABLED"
	}
	privateState := "STATE_DISABLED"
	if privateNetworking {
		privateState = "STATE_ENABLED"
	}

	if !publicNetworking || privateNetworking {
		origTestCaseVars["public_networking"] = config.StringVariable(publicState)
		origTestCaseVars["private_networking"] = config.StringVariable(privateState)
	}

	if privateNetworking {
		origTestCaseVars["allowed_principals"] = config.ListVariable(
			config.StringVariable("arn:aws:iam::123456789012:root"),
		)
		origTestCaseVars["allow_private_link_deletion"] = config.BoolVariable(true)
	}

	rename := acc.RandomName(acc.NamePrefix + testSuffix + "-rename")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	checkFuncs, err := acc.BuildTestCheckFuncs(acc.ServerlessClusterDir, name, privateNetworking)
	if err != nil {
		t.Fatal(err)
	}

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: name, Client: c}.SweepServerlessCluster("")
	}))
	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: rename, Client: c}.SweepServerlessCluster("")
	}))
	if privateNetworking {
		acc.Register(acc.KindServerlessPrivateLink, acc.CleanupFunc(func(_ context.Context) error {
			return sweep.ServerlessPrivateLink{PrivateLinkName: name + "-private-link", Client: c}.SweepServerlessPrivateLink("")
		}))
	}
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
	}))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessClusterDir),
				ConfigVariables:          origTestCaseVars,
				Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessClusterDir),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
		},
	})
}
