//go:build live_test && (all || byovpc_azure)

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

package network_test

import (
	"context"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

// azureNetworkVars are the AZ_* variables the byovpc:azure task exports from
// the producer stack's inputs.json that the network-only config reads.
var azureNetworkVars = []string{
	"management_bucket_storage_account_name", "management_bucket_storage_container_name",
	"redpanda_resource_group_name", "vnet_name", "network_resource_group_name",
	"rp_agent_subnet_name", "rp_0_pods_subnet_name", "rp_0_vnet_subnet_name",
	"rp_1_pods_subnet_name", "rp_1_vnet_subnet_name", "rp_2_pods_subnet_name", "rp_2_vnet_subnet_name",
	"rp_connect_pods_subnet_name", "rp_connect_vnet_subnet_name",
	"kafka_connect_pods_subnet_name", "kafka_connect_vnet_subnet_name",
	"sys_pods_subnet_name", "sys_vnet_subnet_name", "rp_egress_vnet_subnet_name",
}

// TestAcc_Network_BYOVPC_Azure is the cheapest live check of the Azure
// customer_managed_resources contract: create the network on the byovnet
// module's infrastructure, require an empty re-plan (every leaf, the egress
// subnet included, must read back as written), import it, destroy it. No
// cluster, so minutes rather than an hour. The provider-upgrade entry is
// skipped: no released provider carries the Azure arm yet.
func TestAcc_Network_BYOVPC_Azure(t *testing.T) {
	ctx := context.Background()

	name := acc.RandomName(acc.NamePrefix + "testazurenet")
	vars := make(map[string]config.Variable)
	maps.Copy(vars, acc.ProviderCfgIDSecretVars)
	vars["resource_group_name"] = config.StringVariable(name)
	vars["network_name"] = config.StringVariable(name)
	vars["region"] = config.StringVariable(os.Getenv("AZURE_REGION"))
	for _, v := range azureNetworkVars {
		vars[v] = config.StringVariable(os.Getenv("AZ_" + strings.ToUpper(v)))
	}

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}
	acc.Register(acc.KindNetwork, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Network{NetworkName: name, Client: c}.SweepNetworks("")
	}))
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
	}))

	dir := config.StaticDirectory(acc.AzureByoVpcNetworkDir)
	cmr := "customer_managed_resources.azure."
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          dir,
				ConfigVariables:          vars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, "cloud_provider", "azure"),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, "cluster_type", "byoc"),
					resource.TestCheckNoResourceAttr(acc.NetworkResourceName, "cidr_block"),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, cmr+"vnet.name", os.Getenv("AZ_VNET_NAME")),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, cmr+"subnets.rp_egress_vnet.name", os.Getenv("AZ_RP_EGRESS_VNET_SUBNET_NAME")),
					resource.TestCheckResourceAttr(acc.NetworkResourceName, cmr+"subnets.sys_pods.name", os.Getenv("AZ_SYS_PODS_SUBNET_NAME")),
					resource.TestCheckResourceAttrSet(acc.NetworkResourceName, "state"),
				),
			},
			{
				ConfigDirectory:          dir,
				ConfigVariables:          vars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:             acc.NetworkResourceName,
				ConfigDirectory:          dir,
				ConfigVariables:          vars,
				ImportState:              true,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			{
				ConfigDirectory:          dir,
				ConfigVariables:          vars,
				Destroy:                  true,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
		},
	})
}
