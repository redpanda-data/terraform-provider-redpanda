//go:build live_test && (all || datasource_cluster)

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

package cluster_test

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

func TestAcc_DataSource_Cluster(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + "ds-cluster")

	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["name_prefix"] = config.StringVariable(name)
	if acc.ThroughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(acc.ThroughputTier)
	}

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	const (
		clusterAddr = "redpanda_cluster.test"
		dsAddr      = "data.redpanda_cluster.test"
	)
	checkFuncs := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(clusterAddr, "name", name),
		resource.TestCheckResourceAttrSet(dsAddr, "id"),
		resource.TestCheckResourceAttr(dsAddr, "name", name),
		resource.TestCheckResourceAttrSet(dsAddr, "cluster_type"),
		resource.TestCheckResourceAttrSet(dsAddr, "cloud_provider"),
		resource.TestCheckResourceAttrSet(dsAddr, "region"),
		resource.TestCheckResourceAttrSet(dsAddr, "state"),
		resource.TestCheckResourceAttrSet(dsAddr, "cluster_api_url"),
		resource.TestCheckResourceAttrSet(dsAddr, "kafka_api.seed_brokers.#"),
		resource.TestCheckResourceAttrSet(dsAddr, "schema_registry.url"),
		resource.TestCheckResourceAttrSet(dsAddr, "http_proxy.url"),
	}

	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: name, Client: c}.SweepCluster("")
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
				ConfigDirectory:          config.StaticDirectory(acc.ClusterDatasourceInfraDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
