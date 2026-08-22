//go:build live_test && (all || cluster_aws || cluster_aws_dual || cluster_gcp || byoc_aws || byoc_gcp)

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
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

func testRunner(ctx context.Context, name, rename, version, testFile string, customVars map[string]config.Variable, t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
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
	updateTestCaseVars["user_allow_deletion"] = config.BoolVariable(true)
	updateTestCaseVars["acl_allow_deletion"] = config.BoolVariable(true)

	compatibilityUpdateVars := make(map[string]config.Variable)
	maps.Copy(compatibilityUpdateVars, updateTestCaseVars)
	compatibilityUpdateVars["compatibility_level"] = config.StringVariable("FORWARD")

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

	checkFuncs, err := acc.BuildTestCheckFuncs(testFile, name)
	if err != nil {
		t.Fatal(err)
	}

	testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read test file: %w", err))
	}
	hasMaintenanceWindow := strings.Contains(string(testFileContent), "maintenance_window_config")

	// Dataplane steps are shared with the serverless runner and gated on what
	// this fixture actually declares.
	dp, err := newDataplaneFixture(testFile, name)
	if err != nil {
		t.Fatal(err)
	}
	// The cluster ID is stable across the rename; only the lookup name changes.
	clusterIDByName := func(lookup string) clusterIDFunc {
		return func() (string, error) {
			cluster, err := c.ClusterForName(ctx, lookup)
			if err != nil {
				return "", errors.New("test error: unable to get cluster by name")
			}
			return cluster.GetId(), nil
		}
	}
	idBeforeRename, idAfterRename := clusterIDByName(name), clusterIDByName(rename)

	if hasMaintenanceWindow {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(acc.ClusterResourceName, "maintenance_window_config.day_hour.hour_of_day", "0"))
	}

	steps := []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          origTestCaseVars,
			Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}
	// Pre-rename: the user import resolves the cluster by its original name.
	steps = append(steps, dp.UserImportSteps(origTestCaseVars, idBeforeRename)...)
	steps = append(steps, []resource.TestStep{
		{
			ResourceName:             acc.ClusterResourceName,
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          origTestCaseVars,
			ImportState:              true,
			ImportStateVerify:        true,
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          updateTestCaseVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.ResourceGroupName, "name", name),
				resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
				resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
			),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}...)
	steps = append(steps, []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          compatibilityUpdateVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.ResourceGroupName, "name", name),
				resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
				resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema" "product_schema"`) {
						return resource.TestCheckResourceAttr(acc.SchemaProductResourceName, "compatibility", "FORWARD")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
	}...)

	fieldMutationVars := make(map[string]config.Variable)
	maps.Copy(fieldMutationVars, updateTestCaseVars)
	fieldMutationVars["cluster_tags"] = config.MapVariable(map[string]config.Variable{
		"env":  config.StringVariable("acc"),
		"team": config.StringVariable("platform"),
	})
	fieldMutationChecks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(acc.ClusterResourceName, "tags.env", "acc"),
		resource.TestCheckResourceAttr(acc.ClusterResourceName, "tags.team", "platform"),
	}
	if hasMaintenanceWindow {
		fieldMutationVars["maintenance_hour_of_day"] = config.IntegerVariable(3)
		fieldMutationChecks = append(fieldMutationChecks,
			resource.TestCheckResourceAttr(acc.ClusterResourceName, "maintenance_window_config.day_hour.hour_of_day", "3"))
	}
	if dp.Topic {
		fieldMutationVars["partition_count"] = config.IntegerVariable(6)
		fieldMutationVars["topic_retention_ms"] = config.StringVariable("3600000")
		fieldMutationChecks = append(fieldMutationChecks,
			resource.TestCheckResourceAttr(acc.TopicResourceName, "partition_count", "6"),
			resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.retention.ms", "3600000"),
		)
	}
	steps = append(steps, resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(testFile),
		ConfigVariables:          fieldMutationVars,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
		Check:                    resource.ComposeAggregateTestCheckFunc(fieldMutationChecks...),
	})

	// Topic-configuration regressions: the redpanda.* strip branch and the
	// max.compaction.lag.ms clamp (issue #355). Both pin ExpectEmptyPlan.
	steps = append(steps, dp.TopicConfigSteps(fieldMutationVars)...)
	steps = append(steps, dp.TopicClampRegressionSteps(fieldMutationVars)...)

	zonesSentinelVars := make(map[string]config.Variable)
	maps.Copy(zonesSentinelVars, fieldMutationVars)
	zonesSentinelVars["zones"] = config.ListVariable(
		config.StringVariable("zzz9-az9"),
		config.StringVariable("zzz9-az8"),
	)
	steps = append(steps, resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(testFile),
		ConfigVariables:          zonesSentinelVars,
		PlanOnly:                 true,
		ExpectNonEmptyPlan:       true,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
	})

	regionSentinelVars := make(map[string]config.Variable)
	maps.Copy(regionSentinelVars, fieldMutationVars)
	regionSentinelVars["region"] = config.StringVariable("region-sentinel-zzz")
	steps = append(steps, resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(testFile),
		ConfigVariables:          regionSentinelVars,
		PlanOnly:                 true,
		ExpectNonEmptyPlan:       true,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
	})

	if dp.Topic {
		rfSentinelVars := make(map[string]config.Variable)
		maps.Copy(rfSentinelVars, fieldMutationVars)
		rfSentinelVars["replication_factor"] = config.IntegerVariable(1)
		steps = append(steps, resource.TestStep{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          rfSentinelVars,
			PlanOnly:                 true,
			ExpectNonEmptyPlan:       true,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	// Post-rename: every remaining dataplane import resolves the cluster by its
	// new name. Gated per resource, so a scope that dropped one drops its import.
	steps = append(steps, dp.ImportSteps(updateTestCaseVars, idAfterRename)...)

	steps = append(steps, dp.PasswordWoRotationStep(fieldMutationVars, func(password string) error {
		id, err := idAfterRename()
		if err != nil {
			return err
		}
		return acc.VerifySRAuth(ctx, c, id, name, password)
	})...)

	steps = append(steps, dp.PipelineSteps(updateTestCaseVars, idAfterRename)...)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps:    steps,
	},
	)
}
