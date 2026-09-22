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

package tests

import (
	"context"
	"maps"
	"os"
	"strings"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// azureBYOVPCVars maps every AZ_* variable the byovpc:azure task exports from
// the producer stack's inputs.json onto the test config's variables; the two
// sides use the same names.
var azureBYOVPCVars = []string{
	"management_bucket_storage_account_name", "management_bucket_storage_container_name",
	"vnet_name", "network_resource_group_name",
	"rp_agent_subnet_name", "rp_0_pods_subnet_name", "rp_0_vnet_subnet_name",
	"rp_1_pods_subnet_name", "rp_1_vnet_subnet_name", "rp_2_pods_subnet_name", "rp_2_vnet_subnet_name",
	"rp_connect_pods_subnet_name", "rp_connect_vnet_subnet_name",
	"kafka_connect_pods_subnet_name", "kafka_connect_vnet_subnet_name",
	"sys_pods_subnet_name", "sys_vnet_subnet_name", "rp_egress_vnet_subnet_name",
	"redpanda_resource_group_name", "storage_resource_group_name", "iam_resource_group_name",
	"agent_user_assigned_identity_name", "aks_user_assigned_identity_name",
	"redpanda_cluster_assigned_identity_name", "cert_manager_assigned_identity_name",
	"external_dns_assigned_identity_name", "redpanda_console_assigned_identity_name",
	"kafka_connect_assigned_identity_name", "redpanda_connect_assigned_identity_name",
	"redpanda_connect_api_assigned_identity_name", "redpanda_operator_assigned_identity_name",
	"tiered_storage_account_name", "tiered_storage_container_name",
	"management_key_vault_name", "console_key_vault_name",
	"redpanda_security_group_name", "aks_service_cidr",
}

// TestAcc_Cluster_BYOVPC_Azure creates an Azure BYOVPC network and cluster on
// the infrastructure the byovpc:azure task provisions with
// terraform-azure-redpanda-byovnet. The provider-upgrade entry is skipped:
// no released provider carries customer_managed_resources.azure yet, and
// Terraform would silently drop the nested key and create a plain BYOC
// network. Re-enable once a release ships the schema.
func TestAcc_Cluster_BYOVPC_Azure(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + "testazure")
	rename := acc.RandomName(acc.NamePrefix + "testazure-rename")

	customVars := map[string]config.Variable{
		"region": config.StringVariable(os.Getenv("AZURE_REGION")),
	}
	for _, v := range azureBYOVPCVars {
		customVars[v] = config.StringVariable(os.Getenv("AZ_" + strings.ToUpper(v)))
	}
	if zones := os.Getenv("AZ_ZONES"); zones != "" {
		var list []config.Variable
		for _, z := range strings.Split(zones, ",") {
			list = append(list, config.StringVariable(strings.TrimSpace(z)))
		}
		customVars["zones"] = config.ListVariable(list...)
	}

	testRunnerCluster(ctx, name, rename, acc.RedpandaVersion, acc.AzureByoVpcClusterDir, customVars, t, withoutUpgradeEntry())
}

// azureNetworkCMRFromEnv builds the network's customer_managed_resources.azure
// from the AZ_* variables the byovpc:azure task exports.
func azureNetworkCMRFromEnv() *controlplanev1.Network_CustomerManagedResources {
	sub := func(v string) *controlplanev1.Network_CustomerManagedResources_Azure_Subnets_Subnet {
		return &controlplanev1.Network_CustomerManagedResources_Azure_Subnets_Subnet{Name: os.Getenv("AZ_" + v)}
	}
	return &controlplanev1.Network_CustomerManagedResources{
		CloudProvider: &controlplanev1.Network_CustomerManagedResources_Azure_{
			Azure: &controlplanev1.Network_CustomerManagedResources_Azure{
				ManagementBucket: &controlplanev1.CustomerManagedAzureBucketSpec{
					StorageAccountName:   os.Getenv("AZ_MANAGEMENT_BUCKET_STORAGE_ACCOUNT_NAME"),
					StorageContainerName: os.Getenv("AZ_MANAGEMENT_BUCKET_STORAGE_CONTAINER_NAME"),
					ResourceGroup:        &controlplanev1.CustomerManagedAzureResourceGroupSpec{Name: os.Getenv("AZ_REDPANDA_RESOURCE_GROUP_NAME")},
				},
				Vnet: &controlplanev1.Network_CustomerManagedResources_Azure_Vnet{
					Name:          os.Getenv("AZ_VNET_NAME"),
					ResourceGroup: &controlplanev1.CustomerManagedAzureResourceGroupSpec{Name: os.Getenv("AZ_NETWORK_RESOURCE_GROUP_NAME")},
				},
				Subnets: &controlplanev1.Network_CustomerManagedResources_Azure_Subnets{
					RpAgent:          sub("RP_AGENT_SUBNET_NAME"),
					Rp_0Pods:         sub("RP_0_PODS_SUBNET_NAME"),
					Rp_0Vnet:         sub("RP_0_VNET_SUBNET_NAME"),
					Rp_1Pods:         sub("RP_1_PODS_SUBNET_NAME"),
					Rp_1Vnet:         sub("RP_1_VNET_SUBNET_NAME"),
					Rp_2Pods:         sub("RP_2_PODS_SUBNET_NAME"),
					Rp_2Vnet:         sub("RP_2_VNET_SUBNET_NAME"),
					RpConnectPods:    sub("RP_CONNECT_PODS_SUBNET_NAME"),
					RpConnectVnet:    sub("RP_CONNECT_VNET_SUBNET_NAME"),
					KafkaConnectPods: sub("KAFKA_CONNECT_PODS_SUBNET_NAME"),
					KafkaConnectVnet: sub("KAFKA_CONNECT_VNET_SUBNET_NAME"),
					SysPods:          sub("SYS_PODS_SUBNET_NAME"),
					SysVnet:          sub("SYS_VNET_SUBNET_NAME"),
					RpEgressVnet:     sub("RP_EGRESS_VNET_SUBNET_NAME"),
				},
			},
		},
	}
}

// TestAcc_Cluster_BYOVPC_Azure_ExistingNetwork creates the resource group and
// the BYOVNet network through the control-plane API and hands the cluster
// config their ids, so only the cluster's customer_managed_resources.azure
// goes through Terraform. It is the cluster-side live check that does not
// depend on the network resource's read path, and the "cluster on an
// existing network" scenario in its own right.
func TestAcc_Cluster_BYOVPC_Azure_ExistingNetwork(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + "testazurenet")
	rename := acc.RandomName(acc.NamePrefix + "testazurenet-rename")

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

	rg, err := c.CreateResourceGroup(ctx, name)
	if err != nil {
		t.Fatalf("create resource group: %v", err)
	}
	region := os.Getenv("AZURE_REGION")
	op, err := c.Network.CreateNetwork(ctx, &controlplanev1.CreateNetworkRequest{Network: &controlplanev1.NetworkCreate{
		Name:                     name,
		ResourceGroupId:          rg.GetId(),
		CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		Region:                   region,
		ClusterType:              controlplanev1.Cluster_TYPE_BYOC,
		CustomerManagedResources: azureNetworkCMRFromEnv(),
	}})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	if err := utils.AreWeDoneYet(ctx, op.GetOperation(), 15*time.Minute, c.Operation); err != nil {
		t.Fatalf("network create operation: %v", err)
	}
	network, err := c.NetworkForName(ctx, name)
	if err != nil {
		t.Fatalf("find network: %v", err)
	}
	t.Logf("created network %s (%s) through the API", name, network.GetId())

	vars := map[string]config.Variable{
		"resource_group_id": config.StringVariable(rg.GetId()),
		"network_id":        config.StringVariable(network.GetId()),
		"region":            config.StringVariable(region),
		"cluster_name":      config.StringVariable(name),
	}
	maps.Copy(vars, acc.ProviderCfgIDSecretVars)
	if acc.ThroughputTier != "" {
		vars["throughput_tier"] = config.StringVariable(acc.ThroughputTier)
	}
	for _, v := range azureBYOVPCVars {
		if strings.HasSuffix(v, "_subnet_name") || v == "vnet_name" || v == "network_resource_group_name" || strings.HasPrefix(v, "management_bucket_") {
			continue
		}
		vars[v] = config.StringVariable(os.Getenv("AZ_" + strings.ToUpper(v)))
	}
	if zones := os.Getenv("AZ_ZONES"); zones != "" {
		var list []config.Variable
		for _, z := range strings.Split(zones, ",") {
			list = append(list, config.StringVariable(strings.TrimSpace(z)))
		}
		vars["zones"] = config.ListVariable(list...)
	}
	renamed := make(map[string]config.Variable)
	maps.Copy(renamed, vars)
	renamed["cluster_name"] = config.StringVariable(rename)

	dir := config.StaticDirectory(acc.AzureByoVpcClusterOnNetworkDir)
	cmr := "customer_managed_resources.azure."
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          dir,
				ConfigVariables:          vars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", name),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "network_id", network.GetId()),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "cluster_type", "byoc"),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, cmr+"cidrs.aks_service_cidr", os.Getenv("AZ_AKS_SERVICE_CIDR")),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, cmr+"key_vaults.management_vault.name", os.Getenv("AZ_MANAGEMENT_KEY_VAULT_NAME")),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, cmr+"user_assigned_identities.redpanda_operator_assigned_identity.name", os.Getenv("AZ_REDPANDA_OPERATOR_ASSIGNED_IDENTITY_NAME")),
					resource.TestCheckResourceAttrSet(acc.ClusterResourceName, "state"),
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
				ResourceName:             acc.ClusterResourceName,
				ConfigDirectory:          dir,
				ConfigVariables:          vars,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"tags", "allow_deletion"},
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			{
				ConfigDirectory:          dir,
				ConfigVariables:          renamed,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
					resource.TestCheckResourceAttr(acc.ClusterResourceName, "network_id", network.GetId()),
				),
			},
		},
	})
}
