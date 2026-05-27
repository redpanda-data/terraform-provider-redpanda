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
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_DataSource_Runner(t *testing.T) {
	if !strings.Contains(acc.TestAgainstExistingCluster, "true") {
		t.Skip("skipping cluster user-acl-topic tests")
	}
	name := acc.RandomName(acc.NamePrefix + "datasource")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["cluster_id"] = config.StringVariable(os.Getenv("CLUSTER_ID"))
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	if acc.ThroughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(acc.ThroughputTier)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["topic_config"] = config.MapVariable(map[string]config.Variable{
		"compression.type": config.StringVariable("gzip"),
		"flush.ms":         config.StringVariable("100"),
	})

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(acc.DataSourcesTestDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.UserResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.TopicResourceName, "name", name),
				),
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.DataSourcesTestDir),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.compression.type", "gzip"),
				),
			},
			{
				ConfigDirectory:          config.StaticDirectory(acc.DataSourcesTestDir),
				ConfigVariables:          origTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
		},
	},
	)
}
