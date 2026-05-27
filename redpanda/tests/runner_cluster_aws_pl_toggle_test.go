//go:build live_test && (all || byovpc_aws)

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
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

// testRunnerClusterWithAwsPrivateLinkToggle runs the standard lifecycle then
// toggles PL from true to false — regression guard for
// terraform-plugin-framework#1211.
func testRunnerClusterWithAwsPrivateLinkToggle(ctx context.Context, name, rename, version, testFile string, customVars map[string]config.Variable, t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	if acc.ThroughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(acc.ThroughputTier)
	}

	if len(customVars) > 0 {
		for k, v := range customVars {
			origTestCaseVars[k] = v
		}
	}
	if version != "" {
		origTestCaseVars["version"] = config.StringVariable(version)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)
	updateTestCaseVars["cluster_allow_deletion"] = config.BoolVariable(true)

	toggleTestCaseVars := make(map[string]config.Variable)
	maps.Copy(toggleTestCaseVars, updateTestCaseVars)
	toggleTestCaseVars["aws_private_link_enabled"] = config.BoolVariable(false)

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: name, Client: c}.SweepCluster("")
	}))
	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: rename, Client: c}.SweepCluster("")
	}))
	acc.Register(acc.KindNetwork, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Network{NetworkName: name, Client: c}.SweepNetworks("")
	}))
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
	}))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory(testFile),
				ConfigVariables: origTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ResourceGroupName, "name", name),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", name),
				),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			{
				ResourceName:             acc.ClusterResourceName,
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          origTestCaseVars,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"tags", "allow_deletion"},
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			{
				ConfigDirectory: config.StaticDirectory(testFile),
				ConfigVariables: updateTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ResourceGroupName, "name", name),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
				),
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          toggleTestCaseVars,
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
		},
	},
	)
}
