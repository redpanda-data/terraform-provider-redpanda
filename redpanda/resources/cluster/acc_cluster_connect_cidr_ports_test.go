//go:build live_test && (all || cluster_connect_cidr_ports)

// Copyright 2026 Redpanda Data, Inc.
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
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

// TestAcc_Cluster_RedpandaConnect_CidrPorts creates a dedicated cluster with
// allowed_destination_cidr_ports, updates the list, then clears it, verifying
// each step round-trips the field values correctly and no cluster recreation
// occurs (in-place update via LeafExpansion).
func TestAcc_Cluster_RedpandaConnect_CidrPorts(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + "rc-cidr")

	resourceGroupID := os.Getenv("RESOURCE_GROUP_ID")
	networkID := os.Getenv("NETWORK_ID")

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: name, Client: c}.SweepCluster("")
	}))
	if networkID == "" {
		acc.Register(acc.KindNetwork, acc.CleanupFunc(func(_ context.Context) error {
			return sweep.Network{NetworkName: name, Client: c}.SweepNetworks("")
		}))
	}
	if resourceGroupID == "" {
		acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
			return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
		}))
	}

	const clusterAddr = "redpanda_cluster.test"

	throughputTier := "tier-1-aws-v2-x86"
	if acc.ThroughputTier != "" {
		throughputTier = acc.ThroughputTier
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
		Steps: []resource.TestStep{
			// Step 1: create cluster with two CIDR+port rules.
			{
				Config: cidrPortsConfig(name, throughputTier, resourceGroupID, networkID, `
    redpanda_connect = {
      allowed_destination_cidr_ports = [
        { cidr = "10.0.0.0/16", port_start = 5432 },
        { cidr = "20.0.0.0/16", port_start = 5432, port_end = 5500 },
      ]
    }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.#", "2"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.0.cidr", "10.0.0.0/16"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.0.port_start", "5432"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.0.port_end", "5432"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.1.cidr", "20.0.0.0/16"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.1.port_start", "5432"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.1.port_end", "5500"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Step 2: explicit port_end=0 is rejected at plan time (live CP
			// echoes port_end=port_start for a stored 0, so a known 0 can
			// never survive apply). Plan-only failure; cluster untouched.
			{
				Config: cidrPortsConfig(name, throughputTier, resourceGroupID, networkID, `
    redpanda_connect = {
      allowed_destination_cidr_ports = [
        { cidr = "10.0.0.0/16", port_start = 5432, port_end = 0 },
      ]
    }`),
				ExpectError: regexp.MustCompile(`port_end value\s+must be at least 1`),
			},
			// Step 3: remove one rule — in-place update, no cluster recreation.
			{
				Config: cidrPortsConfig(name, throughputTier, resourceGroupID, networkID, `
    redpanda_connect = {
      allowed_destination_cidr_ports = [
        { cidr = "10.0.0.0/16", port_start = 5432 },
      ]
    }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.#", "1"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.0.cidr", "10.0.0.0/16"),
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.0.port_start", "5432"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Step 3: clear all rules.
			{
				Config: cidrPortsConfig(name, throughputTier, resourceGroupID, networkID, `
    redpanda_connect = {
      allowed_destination_cidr_ports = []
    }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(clusterAddr, "redpanda_connect.allowed_destination_cidr_ports.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Step 4: import — verify redpanda_connect survives state import.
			{
				ResourceName:            clusterAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"allow_deletion"},
			},
		},
	})
}

// cidrPortsConfig builds the HCL config for the test. If resourceGroupID or
// networkID are non-empty, those resources are referenced by literal ID instead
// of being created (allowing reuse of pre-existing infra to speed up runs).
func cidrPortsConfig(name, throughputTier, resourceGroupID, networkID, extra string) string {
	var rgBlock, networkBlock, rgRef, networkRef string

	if resourceGroupID == "" {
		rgBlock = fmt.Sprintf(`
resource "redpanda_resource_group" "test" {
  name = %q
}`, name)
		rgRef = "redpanda_resource_group.test.id"
	} else {
		rgRef = fmt.Sprintf("%q", resourceGroupID)
	}

	if networkID == "" {
		networkBlock = fmt.Sprintf(`
resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = %s
  cloud_provider    = "aws"
  region            = "us-east-2"
  cluster_type      = "byoc"
  cidr_block        = "10.0.0.0/20"
  timeouts = { create = "20m", delete = "20m" }
}`, name, rgRef)
		networkRef = "redpanda_network.test.id"
	} else {
		networkRef = fmt.Sprintf("%q", networkID)
	}

	return fmt.Sprintf(`
provider "redpanda" {}
%s
%s
resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = %s
  network_id        = %s
  cloud_provider    = "aws"
  region            = "us-east-2"
  zones             = ["use2-az1", "use2-az2", "use2-az3"]
  throughput_tier   = %q
  cluster_type      = "byoc"
  connection_type   = "public"
  allow_deletion    = true
  timeouts          = { create = "90m", update = "60m" }
%s
}
`, rgBlock, networkBlock, name, rgRef, networkRef, throughputTier, extra)
}
