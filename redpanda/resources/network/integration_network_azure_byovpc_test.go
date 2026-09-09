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

package network_test

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

// azureSubnetNames maps every subnet leaf of
// customer_managed_resources.azure.subnets to the name the config sets, in
// the order the proto declares them. rp_egress_vnet is the one the control
// plane stores in its public-subnet map; it must read back like the rest.
var azureSubnetNames = [][2]string{
	{"rp_agent", "tfrp-agent-private"},
	{"rp_0_pods", "tfrp-rp-0-pods"},
	{"rp_0_vnet", "tfrp-rp-0-vnet"},
	{"rp_1_pods", "tfrp-rp-1-pods"},
	{"rp_1_vnet", "tfrp-rp-1-vnet"},
	{"rp_2_pods", "tfrp-rp-2-pods"},
	{"rp_2_vnet", "tfrp-rp-2-vnet"},
	{"rp_connect_pods", "tfrp-connect-pod"},
	{"rp_connect_vnet", "tfrp-connect-vnet"},
	{"sys_pods", "tfrp-system-pod"},
	{"sys_vnet", "tfrp-system-vnet"},
	{"rp_egress_vnet", "tfrp-agent-public"},
	{"kafka_connect_pods", "tfrp-kafka-connect-pod"},
	{"kafka_connect_vnet", "tfrp-kafka-connect-vnet"},
}

// azureNetOpts parameterizes byovpcAzureConfig. mgmtRG "" omits the optional
// management_bucket.resource_group; cloudProvider and clusterType default to
// azure and byoc and exist for the envelope error paths.
type azureNetOpts struct {
	name, vnetName, mgmtRG, cloudProvider, clusterType string
}

func byovpcAzureConfig(name, vnetName, mgmtRG string) string {
	return byovpcAzureConfigWith(azureNetOpts{name: name, vnetName: vnetName, mgmtRG: mgmtRG})
}

func byovpcAzureConfigWith(o azureNetOpts) string {
	name, vnetName, mgmtRG := o.name, o.vnetName, o.mgmtRG
	cp, ct := o.cloudProvider, o.clusterType
	if cp == "" {
		cp = "azure"
	}
	if ct == "" {
		ct = "byoc"
	}
	subnets := ""
	for _, s := range azureSubnetNames {
		subnets += fmt.Sprintf("\n        %-18s = { name = %q }", s[0], s[1])
	}
	mgmtRGLine := ""
	if mgmtRG != "" {
		mgmtRGLine = fmt.Sprintf("\n        resource_group         = { name = %q }", mgmtRG)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-net-rg"
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = %q
  region            = "eastus"
  cluster_type      = %q
  customer_managed_resources = {
    azure = {
      management_bucket = {
        storage_account_name   = "tfrpmgmtsa"
        storage_container_name = "tfrp-mgmt"%s
      }
      vnet = {
        name           = %q
        resource_group = { name = "tfrp-network-rg" }
      }
      subnets = {%s
      }
    }
  }
}
`, name, cp, ct, mgmtRGLine, vnetName, subnets)
}

func azureNetCMRPath(keys ...string) tfjsonpath.Path {
	p := tfjsonpath.New("customer_managed_resources").AtMapKey("azure")
	for _, k := range keys {
		p = p.AtMapKey(k)
	}
	return p
}

// TestIntegration_Network_CreateAndRefresh_CMR_Azure validates the Create +
// no-op cycle for the Azure BYOVPC variant: cidr_block is null, every one of
// the fourteen subnets reads back, the omitted management_bucket.resource_group
// stays null, and the datasource mirrors the block.
func TestIntegration_Network_CreateAndRefresh_CMR_Azure(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name     = "tfrp-mock-net-az-bv"
		vnetName = "tfrp-vnet"
		dsAddr   = "data.redpanda_network.test"
		dsBlock  = `
data "redpanda_network" "test" {
  id = redpanda_network.test.id
}
`
	)
	cfg := byovpcAzureConfig(name, vnetName, "")

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	createChecks := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("azure")),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.Null()),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("management_bucket", "storage_account_name"), knownvalue.StringExact("tfrpmgmtsa")),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("management_bucket", "storage_container_name"), knownvalue.StringExact("tfrp-mgmt")),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("management_bucket", "resource_group"), knownvalue.Null()),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("vnet", "name"), knownvalue.StringExact(vnetName)),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("vnet", "resource_group", "name"), knownvalue.StringExact("tfrp-network-rg")),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
		idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
	}
	for _, s := range azureSubnetNames {
		createChecks = append(createChecks,
			statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("subnets", s[0], "name"), knownvalue.StringExact(s[1])))
	}

	refreshChecks := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("cidr_block"), knownvalue.Null()),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("management_bucket", "resource_group"), knownvalue.Null()),
		statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("subnets", "rp_egress_vnet", "name"), knownvalue.StringExact("tfrp-agent-public")),
		statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
		idPreserved.AddStateValue(networkAddr, tfjsonpath.New("id")),
		statecheck.ExpectKnownValue(dsAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("azure")),
		statecheck.ExpectKnownValue(dsAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
		statecheck.ExpectKnownValue(dsAddr, azureNetCMRPath("vnet", "name"), knownvalue.StringExact(vnetName)),
		statecheck.ExpectKnownValue(dsAddr, azureNetCMRPath("management_bucket", "storage_container_name"), knownvalue.StringExact("tfrp-mgmt")),
	}
	for _, s := range azureSubnetNames {
		refreshChecks = append(refreshChecks,
			statecheck.ExpectKnownValue(dsAddr, azureNetCMRPath("subnets", s[0], "name"), knownvalue.StringExact(s[1])))
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, cfg, createChecks),
			integration.NoopReapplyStep(networkAddr, cfg+dsBlock, refreshChecks),
		},
	})
}

// TestIntegration_Network_RequiresReplace_CMR_Azure mutates vnet.name and
// asserts DestroyBeforeCreate. customer_managed_resources.azure carries
// RequiresReplace: the only customer-managed field the control plane updates
// after create is the AWS public_subnets list.
func TestIntegration_Network_RequiresReplace_CMR_Azure(t *testing.T) {
	_, factories := integration.Setup(t)

	const (
		name      = "tfrp-mock-net-rr-cmr-az"
		vnetNameA = "tfrp-vnet-a"
		vnetNameB = "tfrp-vnet-b"
		mgmtRG    = "tfrp-storage-rg"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(networkAddr, byovpcAzureConfig(name, vnetNameA, mgmtRG), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("vnet", "name"), knownvalue.StringExact(vnetNameA)),
				statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("management_bucket", "resource_group", "name"), knownvalue.StringExact(mgmtRG)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
			integration.RequiresReplaceStep(networkAddr, byovpcAzureConfig(name, vnetNameB, mgmtRG), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(networkAddr, azureNetCMRPath("vnet", "name"), knownvalue.StringExact(vnetNameB)),
				statecheck.ExpectKnownValue(networkAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idChanged.AddStateValue(networkAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Network_ErrorPath_CMR_Envelope pins the plan-time envelope
// rules on customer_managed_resources: the arm must match cloud_provider, and
// the block needs cluster_type "byoc" (the control plane's own rule, moved
// forward from apply to plan).
func TestIntegration_Network_ErrorPath_CMR_Envelope(t *testing.T) {
	_, factories := integration.Setup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      byovpcAzureConfigWith(azureNetOpts{name: "tfrp-mock-net-az-mismatch", vnetName: "tfrp-vnet", cloudProvider: "gcp"}),
				ExpectError: regexp.MustCompile(`customer_managed_resources.azure is set but cloud_provider is "gcp"`),
			},
			{
				Config:      byovpcAzureConfigWith(azureNetOpts{name: "tfrp-mock-net-az-dedicated", vnetName: "tfrp-vnet", clusterType: "dedicated"}),
				ExpectError: regexp.MustCompile(`customer_managed_resources is set but cluster_type is "dedicated"`),
			},
		},
	})
}
