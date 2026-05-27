//go:build live_test && (all || serverless_regions)

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

package serverlessregions_test

import (
	"maps"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_DataSource_ServerlessRegions(t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(acc.ServerlessRegionsDataSourceDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsAWSDataSourceName, "serverless_regions.#"),
					resource.TestMatchResourceAttr(acc.ServerlessRegionsAWSDataSourceName, "serverless_regions.#", regexp.MustCompile(`^[1-9]\d*$`)),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsAWSDataSourceName, "serverless_regions.0.name"),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsAWSDataSourceName, "serverless_regions.0.time_zone"),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsAWSDataSourceName, "serverless_regions.0.placement.enabled"),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsGCPDataSourceName, "serverless_regions.#"),
					resource.TestMatchResourceAttr(acc.ServerlessRegionsGCPDataSourceName, "serverless_regions.#", regexp.MustCompile(`^[1-9]\d*$`)),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsGCPDataSourceName, "serverless_regions.0.name"),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsGCPDataSourceName, "serverless_regions.0.time_zone"),
					resource.TestCheckResourceAttrSet(acc.ServerlessRegionsGCPDataSourceName, "serverless_regions.0.placement.enabled"),
				),
			},
		},
	})
}
