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

package cluster

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

// TestUnit_Cluster_Azure_SchemaCompiles exercises the Azure-specific
// Flatten/Expand code paths (azure_private_link, cloud_storage.azure,
// cloud_provider="azure")
func TestUnit_Cluster_Azure_SchemaCompiles(t *testing.T) {
	ctx := context.Background()

	proto := &controlplanev1.Cluster{
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		Region:        "westus2",
		AzurePrivateLink: &controlplanev1.Cluster_AzurePrivateLink{
			Enabled:              true,
			ConnectConsole:       true,
			AllowedSubscriptions: []string{"00000000-0000-0000-0000-000000000001"},
		},
		CloudStorage: &controlplanev1.Cluster_CloudStorage{
			CloudProvider: &controlplanev1.Cluster_CloudStorage_Azure_{
				Azure: &controlplanev1.Cluster_CloudStorage_Azure{
					ContainerName:      "rp-container",
					StorageAccountName: "rpstorageacct",
					AllowedIps:         []string{"203.0.113.0/24"},
				},
			},
		},
	}

	m, diags := Flatten(ctx, proto, nil)
	if diags.HasError() {
		t.Fatalf("Flatten: unexpected diagnostics: %v", diags.Errors())
	}

	if got := m.CloudProvider.ValueString(); got != enums.CloudProviderStringAzure {
		t.Errorf("CloudProvider: got %q, want %q", got, enums.CloudProviderStringAzure)
	}
	if m.AzurePrivateLink.IsNull() {
		t.Error("AzurePrivateLink: got Null, want populated object")
	}
	if m.CloudStorage.IsNull() {
		t.Error("CloudStorage: got Null, want populated object (Azure oneof should land in the model)")
	}

	req, diags := ExpandCreate(ctx, m)
	if diags.HasError() {
		t.Fatalf("ExpandCreate: unexpected diagnostics: %v", diags.Errors())
	}
	if req.GetCluster().GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE {
		t.Errorf("ExpandCreate CloudProvider: got %v, want CLOUD_PROVIDER_AZURE",
			req.GetCluster().GetCloudProvider())
	}
	if req.GetCluster().GetAzurePrivateLink() == nil {
		t.Error("ExpandCreate AzurePrivateLink: got nil, want non-nil")
	}
	if req.GetCluster().GetCloudStorage().GetAzure() == nil {
		t.Error("ExpandCreate CloudStorage.Azure oneof: got nil, want non-nil")
	}
}

// TestUnit_Cluster_AzureCMR_RoundTrip drives a full customer_managed_resources.azure
// block through Flatten and ExpandCreate. The optional
// tiered_cloud_storage.resource_group is left nil on the proto and must stay
// null in the model, then absent again on the create payload.
func TestUnit_Cluster_AzureCMR_RoundTrip(t *testing.T) {
	ctx := context.Background()

	rg := func(n string) *controlplanev1.CustomerManagedAzureResourceGroupSpec {
		return &controlplanev1.CustomerManagedAzureResourceGroupSpec{Name: n}
	}
	uai := func(n string) *controlplanev1.CustomerManagedResources_Azure_UserAssignedIdentities_UAISpec {
		return &controlplanev1.CustomerManagedResources_Azure_UserAssignedIdentities_UAISpec{Name: n}
	}
	kv := func(n string) *controlplanev1.CustomerManagedResources_Azure_KeyVaults_KeyVault {
		return &controlplanev1.CustomerManagedResources_Azure_KeyVaults_KeyVault{Name: n}
	}
	proto := &controlplanev1.Cluster{
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		Type:          controlplanev1.Cluster_TYPE_BYOC,
		Region:        "eastus",
		CustomerManagedResources: &controlplanev1.CustomerManagedResources{
			CloudProvider: &controlplanev1.CustomerManagedResources_Azure_{
				Azure: &controlplanev1.CustomerManagedResources_Azure{
					ResourceGroups: &controlplanev1.CustomerManagedResources_Azure_ResourceGroups{
						RedpandaResourceGroup: rg("rp-rg"), StorageResourceGroup: rg("storage-rg"), IamResourceGroup: rg("iam-rg"),
					},
					UserAssignedIdentities: &controlplanev1.CustomerManagedResources_Azure_UserAssignedIdentities{
						AgentUserAssignedIdentity: uai("agent"), AksUserAssignedIdentity: uai("aks"),
						RedpandaClusterAssignedIdentity: uai("cluster"), CertManagerAssignedIdentity: uai("cert-manager"),
						ExternalDnsAssignedIdentity: uai("external-dns"), RedpandaConsoleAssignedIdentity: uai("console"),
						KafkaConnectAssignedIdentity: uai("kafka-connect"), RedpandaConnectAssignedIdentity: uai("connect"),
						RedpandaConnectApiAssignedIdentity: uai("connect-api"), RedpandaOperatorAssignedIdentity: uai("operator"),
					},
					TieredCloudStorage: &controlplanev1.CustomerManagedAzureBucketSpec{
						StorageAccountName: "tieredsa", StorageContainerName: "tiered",
					},
					KeyVaults: &controlplanev1.CustomerManagedResources_Azure_KeyVaults{
						ManagementVault: kv("kv-mgmt"), ConsoleVault: kv("kv-console"),
					},
					SecurityGroups: &controlplanev1.CustomerManagedResources_Azure_SecurityGroups{
						RedpandaSecurityGroup: &controlplanev1.CustomerManagedResources_Azure_SecurityGroups_SecurityGroup{Name: "rp-nsg"},
					},
					Cidrs: &controlplanev1.CustomerManagedResources_Azure_CIDR{AksServiceCidr: "10.0.15.0/24"},
				},
			},
		},
	}

	m, diags := Flatten(ctx, proto, nil)
	if diags.HasError() {
		t.Fatalf("Flatten: unexpected diagnostics: %v", diags.Errors())
	}
	if m.CustomerManagedResources.IsNull() {
		t.Fatal("CustomerManagedResources: got Null, want populated object")
	}
	azure := m.CustomerManagedResources.Attributes()["azure"]
	if azure == nil || azure.IsNull() {
		t.Fatal("customer_managed_resources.azure: got Null, want populated object")
	}
	if aws := m.CustomerManagedResources.Attributes()["aws"]; aws != nil && !aws.IsNull() {
		t.Error("customer_managed_resources.aws: want Null on an Azure cluster")
	}

	req, diags := ExpandCreate(ctx, m)
	if diags.HasError() {
		t.Fatalf("ExpandCreate: unexpected diagnostics: %v", diags.Errors())
	}
	got := req.GetCluster().GetCustomerManagedResources().GetAzure()
	if got == nil {
		t.Fatal("ExpandCreate CustomerManagedResources.Azure oneof: got nil, want non-nil")
	}
	if got.GetResourceGroups().GetIamResourceGroup().GetName() != "iam-rg" {
		t.Errorf("iam_resource_group.name: got %q", got.GetResourceGroups().GetIamResourceGroup().GetName())
	}
	if got.GetUserAssignedIdentities().GetRedpandaOperatorAssignedIdentity().GetName() != "operator" {
		t.Errorf("redpanda_operator_assigned_identity.name: got %q", got.GetUserAssignedIdentities().GetRedpandaOperatorAssignedIdentity().GetName())
	}
	if got.GetTieredCloudStorage().GetStorageContainerName() != "tiered" {
		t.Errorf("tiered_cloud_storage.storage_container_name: got %q", got.GetTieredCloudStorage().GetStorageContainerName())
	}
	if got.GetTieredCloudStorage().GetResourceGroup() != nil {
		t.Errorf("tiered_cloud_storage.resource_group: got %v, want nil for an omitted optional block", got.GetTieredCloudStorage().GetResourceGroup())
	}
	if got.GetKeyVaults().GetConsoleVault().GetName() != "kv-console" {
		t.Errorf("key_vaults.console_vault.name: got %q", got.GetKeyVaults().GetConsoleVault().GetName())
	}
	if got.GetSecurityGroups().GetRedpandaSecurityGroup().GetName() != "rp-nsg" {
		t.Errorf("security_groups.redpanda_security_group.name: got %q", got.GetSecurityGroups().GetRedpandaSecurityGroup().GetName())
	}
	if got.GetCidrs().GetAksServiceCidr() != "10.0.15.0/24" {
		t.Errorf("cidrs.aks_service_cidr: got %q", got.GetCidrs().GetAksServiceCidr())
	}
}
