//go:build live_test && (all || serverless_aws_public || serverless_aws_private || serverless_aws_both || serverless_gcp)

// Copyright 2023 Redpanda Data, Inc.
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
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

func runServerlessClusterVariantTest(t *testing.T, testSuffix, region string, publicNetworking, privateNetworking bool, opts ...runnerOpt) {
	ctx := context.Background()

	// A public-disabled (private-only) serverless cluster is unreachable from
	// the external test runner for Schema Registry, so it uses a fixture
	// without SR ACL / schema resources.
	dir := acc.ServerlessClusterDir
	if privateNetworking && !publicNetworking {
		dir = acc.ServerlessClusterPrivateDir
	}

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
	allowDeleteVars := make(map[string]config.Variable)
	maps.Copy(allowDeleteVars, origTestCaseVars)
	allowDeleteVars["cluster_allow_deletion"] = config.BoolVariable(true)

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)
	updateTestCaseVars["cluster_allow_deletion"] = config.BoolVariable(true)

	checkFuncs, err := acc.BuildTestCheckFuncs(dir, name, privateNetworking)
	if err != nil {
		t.Fatal(err)
	}

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	// Dataplane lifecycle steps come from the same builder the dedicated runner
	// uses. Serverless is where dataplane coverage belongs: it stands up in
	// seconds, so a dataplane regression costs seconds, not a cluster-creation cycle.
	dp, err := newDataplaneFixture(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	// The cluster ID is stable across the rename; only the lookup name changes.
	serverlessIDByName := func(lookup string) clusterIDFunc {
		return func() (string, error) {
			cluster, err := c.ServerlessClusterForName(ctx, lookup)
			if err != nil {
				return "", errors.New("test error: unable to get serverless cluster by name")
			}
			return cluster.GetId(), nil
		}
	}
	idBeforeRename, idAfterRename := serverlessIDByName(name), serverlessIDByName(rename)

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

	var steps []resource.TestStep
	if !resolveRunnerOpts(opts).skipUpgradeEntry {
		steps = acc.UpgradeEntrySteps(t, dir, origTestCaseVars)
	}
	steps = append(steps, []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(dir),
			ConfigVariables:          origTestCaseVars,
			Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}...)
	// Pre-rename: the user import resolves the cluster by its original name.
	steps = append(steps, dp.UserImportSteps(origTestCaseVars, idBeforeRename)...)
	steps = append(steps, []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(dir),
			ConfigVariables:          allowDeleteVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
		{
			ConfigDirectory:          config.StaticDirectory(dir),
			ConfigVariables:          updateTestCaseVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}...)
	steps = append(steps, dp.TopicConfigSteps(updateTestCaseVars)...)
	// Post-rename: every remaining dataplane import resolves by the new name.
	steps = append(steps, dp.ImportSteps(updateTestCaseVars, idAfterRename)...)
	steps = append(steps, dp.PipelineSteps(updateTestCaseVars, idAfterRename)...)
	steps = append(steps, dp.PasswordWoRotationStep(updateTestCaseVars, func(password string) error {
		id, err := idAfterRename()
		if err != nil {
			return err
		}
		return acc.VerifySRAuth(ctx, c, id, name, password)
	})...)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps:    steps,
	})
}
