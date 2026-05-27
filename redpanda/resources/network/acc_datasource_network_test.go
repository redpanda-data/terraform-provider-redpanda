//go:build live_test && (all || network)

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

package network_test

import (
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_DataSource_Network(t *testing.T) {
	if !strings.Contains(acc.TestAgainstExistingCluster, "true") {
		t.Skip("skipping network datasource test")
	}

	networkIDEnv := os.Getenv("REDPANDA_NETWORK_ID")
	if networkIDEnv == "" {
		t.Skip("skipping network data source test: REDPANDA_NETWORK_ID not set")
	}

	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["network_id"] = config.StringVariable(networkIDEnv)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(acc.NetworkDataSourceDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(acc.NetworkDataSourceName, "id"),
					resource.TestCheckResourceAttrSet(acc.NetworkDataSourceName, "name"),
					resource.TestCheckResourceAttrSet(acc.NetworkDataSourceName, "cloud_provider"),
					resource.TestCheckResourceAttrSet(acc.NetworkDataSourceName, "region"),
					resource.TestCheckResourceAttrSet(acc.NetworkDataSourceName, "resource_group_id"),
					resource.TestCheckResourceAttrSet(acc.NetworkDataSourceName, "cluster_type"),
					resource.TestCheckResourceAttr(acc.NetworkDataSourceName, "id", networkIDEnv),
				),
			},
		},
	})
}
