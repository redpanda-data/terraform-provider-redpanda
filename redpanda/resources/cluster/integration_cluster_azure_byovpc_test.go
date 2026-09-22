//go:build integration

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

package cluster_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
)

// azureBYOVPCOpts parameterizes azureBYOVPCConfig. Every Azure
// customer_managed_resources leaf is create-only on the control plane
// (CustomerManagedResourcesUpdate has no Azure arm), so the varying fields
// exist to prove RequiresReplace per block, not in-place updates. Blank
// fields fall back to their default; tieredRG "" omits the optional
// tiered_cloud_storage.resource_group.
type azureBYOVPCOpts struct {
	name           string
	redpandaRG     string // resource_groups.redpanda_resource_group.name
	agentUAI       string // user_assigned_identities.agent_user_assigned_identity.name
	storageAccount string // tiered_cloud_storage.storage_account_name
	mgmtVault      string // key_vaults.management_vault.name; set explicitly to test the empty case
	mgmtVaultSet   bool
	securityGroup  string // security_groups.redpanda_security_group.name
	aksCIDR        string // cidrs.aks_service_cidr
	tieredRG       string // tiered_cloud_storage.resource_group.name; "" omits it
	connectionType string
	cloudProvider  string // both resources; "" is azure
	clusterType    string // both resources; "" is byoc
}

func azureBYOVPCConfig(o azureBYOVPCOpts) string {
	mgmtVault := o.mgmtVault
	if !o.mgmtVaultSet {
		mgmtVault = "tfrp-kv-mgmt"
	}
	cp := orDefault(o.cloudProvider, "azure")
	ct := orDefault(o.clusterType, "byoc")
	tieredRG := ""
	if o.tieredRG != "" {
		tieredRG = fmt.Sprintf("\n        resource_group         = { name = %q }", o.tieredRG)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = %q
  region            = "eastus"
  cluster_type      = %q
  customer_managed_resources = {
    azure = {
      management_bucket = {
        storage_account_name   = "tfrpmgmtsa"
        storage_container_name = "tfrp-mgmt"
        resource_group         = { name = "tfrp-storage-rg" }
      }
      vnet = {
        name           = "tfrp-vnet"
        resource_group = { name = "tfrp-network-rg" }
      }
      subnets = {
        rp_agent           = { name = "tfrp-agent-private" }
        rp_0_pods          = { name = "tfrp-rp-0-pods" }
        rp_0_vnet          = { name = "tfrp-rp-0-vnet" }
        rp_1_pods          = { name = "tfrp-rp-1-pods" }
        rp_1_vnet          = { name = "tfrp-rp-1-vnet" }
        rp_2_pods          = { name = "tfrp-rp-2-pods" }
        rp_2_vnet          = { name = "tfrp-rp-2-vnet" }
        rp_connect_pods    = { name = "tfrp-connect-pod" }
        rp_connect_vnet    = { name = "tfrp-connect-vnet" }
        kafka_connect_pods = { name = "tfrp-kafka-connect-pod" }
        kafka_connect_vnet = { name = "tfrp-kafka-connect-vnet" }
        sys_pods           = { name = "tfrp-system-pod" }
        sys_vnet           = { name = "tfrp-system-vnet" }
        rp_egress_vnet     = { name = "tfrp-agent-public" }
      }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = %q
  region            = "eastus"
  zones             = ["eastus-az1"]
  throughput_tier   = "tier-1-azure-v3-x86"
  cluster_type      = %q
  connection_type   = %q
  allow_deletion    = true
  customer_managed_resources = {
    azure = {
      resource_groups = {
        redpanda_resource_group = { name = %q }
        storage_resource_group  = { name = "tfrp-storage-rg" }
        iam_resource_group      = { name = "tfrp-iam-rg" }
      }
      user_assigned_identities = {
        agent_user_assigned_identity          = { name = %q }
        aks_user_assigned_identity            = { name = "tfrp-aks-uai" }
        redpanda_cluster_assigned_identity    = { name = "tfrp-cluster-uai" }
        cert_manager_assigned_identity        = { name = "tfrp-cert-manager-uai" }
        external_dns_assigned_identity        = { name = "tfrp-external-dns-uai" }
        redpanda_console_assigned_identity    = { name = "tfrp-console-uai" }
        kafka_connect_assigned_identity       = { name = "tfrp-kafka-connect-uai" }
        redpanda_connect_assigned_identity    = { name = "tfrp-connect-uai" }
        redpanda_connect_api_assigned_identity = { name = "tfrp-connect-api-uai" }
        redpanda_operator_assigned_identity   = { name = "tfrp-operator-uai" }
      }
      tiered_cloud_storage = {
        storage_account_name   = %q
        storage_container_name = "tfrp-tiered"%s
      }
      key_vaults = {
        management_vault = { name = %q }
        console_vault    = { name = "tfrp-kv-console" }
      }
      security_groups = {
        redpanda_security_group = { name = %q }
      }
      cidrs = {
        aks_service_cidr = %q
      }
    }
  }
}
`, cp, ct, o.name, cp, ct, orDefault(o.connectionType, "private"),
		orDefault(o.redpandaRG, "tfrp-redpanda-rg"),
		orDefault(o.agentUAI, "tfrp-agent-uai"),
		orDefault(o.storageAccount, "tfrptieredsa"), tieredRG,
		mgmtVault,
		orDefault(o.securityGroup, "tfrp-redpanda-nsg"),
		orDefault(o.aksCIDR, "10.0.15.0/24"))
}

func azureCMRPath(keys ...string) tfjsonpath.Path {
	p := tfjsonpath.New("customer_managed_resources").AtMapKey("azure")
	for _, k := range keys {
		p = p.AtMapKey(k)
	}
	return p
}

// TestIntegration_Cluster_CreateAndRefresh_Azure_BYOVPC creates and no-op
// re-applies an Azure BYOVPC cluster with a full customer_managed_resources.azure
// block, then reads it back through the datasource. The omitted optional
// tiered_cloud_storage.resource_group must stay null across the refresh.
func TestIntegration_Cluster_CreateAndRefresh_Azure_BYOVPC(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-az-bv"
	cfg := azureBYOVPCConfig(azureBYOVPCOpts{name: name})
	const dsBlock = `
data "redpanda_cluster" "test" {
  id = redpanda_cluster.test.id
}
`

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("azure")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("private")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("resource_groups", "redpanda_resource_group", "name"), knownvalue.StringExact("tfrp-redpanda-rg")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("resource_groups", "iam_resource_group", "name"), knownvalue.StringExact("tfrp-iam-rg")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("user_assigned_identities", "agent_user_assigned_identity", "name"), knownvalue.StringExact("tfrp-agent-uai")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("user_assigned_identities", "redpanda_operator_assigned_identity", "name"), knownvalue.StringExact("tfrp-operator-uai")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "storage_account_name"), knownvalue.StringExact("tfrptieredsa")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "storage_container_name"), knownvalue.StringExact("tfrp-tiered")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "resource_group"), knownvalue.Null()),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("key_vaults", "management_vault", "name"), knownvalue.StringExact("tfrp-kv-mgmt")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("key_vaults", "console_vault", "name"), knownvalue.StringExact("tfrp-kv-console")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("security_groups", "redpanda_security_group", "name"), knownvalue.StringExact("tfrp-redpanda-nsg")),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("cidrs", "aks_service_cidr"), knownvalue.StringExact("10.0.15.0/24")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg+dsBlock, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "resource_group"), knownvalue.Null()),
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("cidrs", "aks_service_cidr"), knownvalue.StringExact("10.0.15.0/24")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", tfjsonpath.New("cloud_provider"), knownvalue.StringExact("azure")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", azureCMRPath("resource_groups", "redpanda_resource_group", "name"), knownvalue.StringExact("tfrp-redpanda-rg")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", azureCMRPath("user_assigned_identities", "aks_user_assigned_identity", "name"), knownvalue.StringExact("tfrp-aks-uai")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", azureCMRPath("tiered_cloud_storage", "storage_account_name"), knownvalue.StringExact("tfrptieredsa")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", azureCMRPath("key_vaults", "console_vault", "name"), knownvalue.StringExact("tfrp-kv-console")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", azureCMRPath("security_groups", "redpanda_security_group", "name"), knownvalue.StringExact("tfrp-redpanda-nsg")),
				statecheck.ExpectKnownValue("data.redpanda_cluster.test", azureCMRPath("cidrs", "aks_service_cidr"), knownvalue.StringExact("10.0.15.0/24")),
			}),
		},
	})
}

// TestIntegration_Cluster_CMR_Azure pins the Azure customer_managed_resources
// contract: the control plane has no Azure arm on CustomerManagedResourcesUpdate,
// so a change to any leaf in any block plans destroy-before-create. One leaf per
// block is mutated in turn, and the cluster id must differ after each step.
func TestIntegration_Cluster_CMR_Azure(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-az-cmr"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	base := azureBYOVPCOpts{name: name}
	step := func(o azureBYOVPCOpts, path tfjsonpath.Path, want string) resource.TestStep {
		return integration.RequiresReplaceStep(clusterAddr, azureBYOVPCConfig(o), []statecheck.StateCheck{
			statecheck.ExpectKnownValue(clusterAddr, path, knownvalue.StringExact(want)),
			statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
		})
	}

	withRG := base
	withRG.redpandaRG = "tfrp-redpanda-rg-b"
	withUAI := withRG
	withUAI.agentUAI = "tfrp-agent-uai-b"
	withSA := withUAI
	withSA.storageAccount = "tfrptieredsab"
	withVault := withSA
	withVault.mgmtVault, withVault.mgmtVaultSet = "tfrp-kv-mgmt-b", true
	withSG := withVault
	withSG.securityGroup = "tfrp-redpanda-nsg-b"
	withCIDR := withSG
	withCIDR.aksCIDR = "10.0.16.0/24"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, azureBYOVPCConfig(base), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("resource_groups", "redpanda_resource_group", "name"), knownvalue.StringExact("tfrp-redpanda-rg")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			step(withRG, azureCMRPath("resource_groups", "redpanda_resource_group", "name"), "tfrp-redpanda-rg-b"),
			step(withUAI, azureCMRPath("user_assigned_identities", "agent_user_assigned_identity", "name"), "tfrp-agent-uai-b"),
			step(withSA, azureCMRPath("tiered_cloud_storage", "storage_account_name"), "tfrptieredsab"),
			step(withVault, azureCMRPath("key_vaults", "management_vault", "name"), "tfrp-kv-mgmt-b"),
			step(withSG, azureCMRPath("security_groups", "redpanda_security_group", "name"), "tfrp-redpanda-nsg-b"),
			step(withCIDR, azureCMRPath("cidrs", "aks_service_cidr"), "10.0.16.0/24"),
		},
	})
}

// TestIntegration_Cluster_ImportRoundTrip_Azure_BYOVPC imports an Azure BYOVPC
// cluster and verifies the customer_managed_resources.azure block survives the
// round trip byte for byte.
func TestIntegration_Cluster_ImportRoundTrip_Azure_BYOVPC(t *testing.T) {
	_, factories := clusterSetup(t)

	cfg := azureBYOVPCConfig(azureBYOVPCOpts{name: "tfrp-mock-cl-az-import", tieredRG: "tfrp-storage-rg"})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "resource_group", "name"), knownvalue.StringExact("tfrp-storage-rg")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			integration.ImportRoundTripStep(clusterAddr, nil, []string{"allow_deletion"}),
		},
	})
}

// TestIntegration_Cluster_CMR_Azure_OptionalBlockRemoved pins the one Azure
// transition a leaf-level RequiresReplace cannot see: dropping the optional
// tiered_cloud_storage.resource_group block plans the block to null, and the
// framework skips nested plan modifiers when the planned parent object is
// null. The arm-level RequiresReplace is what catches the removal; without it
// the change would plan an in-place update the control plane has no arm for.
func TestIntegration_Cluster_CMR_Azure_OptionalBlockRemoved(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-az-optrm"
	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, azureBYOVPCConfig(azureBYOVPCOpts{name: name, tieredRG: "tfrp-storage-rg"}), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "resource_group", "name"), knownvalue.StringExact("tfrp-storage-rg")),
				idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(clusterAddr, azureBYOVPCConfig(azureBYOVPCOpts{name: name}), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, azureCMRPath("tiered_cloud_storage", "resource_group"), knownvalue.Null()),
				idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_ErrorPath_Azure_BYOVPC_NonPrivateConnectionType pins
// that RequirePrivateConnectionValidator has no provider switch: an Azure
// customer_managed_resources block with connection_type="public" is rejected
// at plan time.
func TestIntegration_Cluster_ErrorPath_Azure_BYOVPC_NonPrivateConnectionType(t *testing.T) {
	_, factories := clusterSetup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      azureBYOVPCConfig(azureBYOVPCOpts{name: "tfrp-mock-cl-az-pub", connectionType: "public"}),
				ExpectError: regexp.MustCompile(`connection_type must be "private"`),
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_Azure_BYOVPC_EmptyKeyVaultName pins the
// buf.validate name rule on KeyVault.name (min 3 characters) at plan time. The
// byovnet module emits "" for a vault it did not create, so this is the config
// a user hits first when the module inputs are incomplete.
func TestIntegration_Cluster_ErrorPath_Azure_BYOVPC_EmptyKeyVaultName(t *testing.T) {
	_, factories := clusterSetup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      azureBYOVPCConfig(azureBYOVPCOpts{name: "tfrp-mock-cl-az-kv", mgmtVault: "", mgmtVaultSet: true}),
				ExpectError: regexp.MustCompile(`management_vault`),
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_CMR_Envelope pins the plan-time envelope
// rules on customer_managed_resources: the arm must match cloud_provider, and
// the block needs cluster_type "byoc". The control plane enforces the second
// only at apply and the first only for GCP.
func TestIntegration_Cluster_ErrorPath_CMR_Envelope(t *testing.T) {
	_, factories := clusterSetup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      azureBYOVPCConfig(azureBYOVPCOpts{name: "tfrp-mock-cl-az-mismatch", cloudProvider: "aws"}),
				ExpectError: regexp.MustCompile(`customer_managed_resources.azure is set but cloud_provider is "aws"`),
			},
			{
				Config:      azureBYOVPCConfig(azureBYOVPCOpts{name: "tfrp-mock-cl-az-dedicated", clusterType: "dedicated"}),
				ExpectError: regexp.MustCompile(`customer_managed_resources is set but cluster_type is "dedicated"`),
			},
		},
	})
}
