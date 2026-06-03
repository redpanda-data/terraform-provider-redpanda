//go:build integration

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
	"regexp"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestIntegration_Cluster exercises redpanda_cluster (dedicated AWS variant) against
// the bufconn fake. Cluster's configurable surface is overwhelmingly
// RequiresReplace; the scenario is Create → no-op re-plan → refresh →
// implicit destroy. BYOC/BYOVPC variants are not covered at this tier.
func TestIntegration_Cluster(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	const addr = "redpanda_cluster.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mockClusterConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-cluster"),
					resource.TestCheckResourceAttr(addr, "cloud_provider", "aws"),
					resource.TestCheckResourceAttr(addr, "region", "us-east-1"),
					resource.TestCheckResourceAttr(addr, "cluster_type", "dedicated"),
					resource.TestCheckResourceAttr(addr, "connection_type", "public"),
					resource.TestCheckResourceAttr(addr, "throughput_tier", "tier-1-aws-v3-arm"),
					resource.TestCheckResourceAttrSet(addr, "id"),
					resource.TestCheckResourceAttrSet(addr, "cluster_api_url"),
					resource.TestCheckResourceAttrSet(addr, "current_redpanda_version"),
				),
			},
			{
				Config: mockClusterConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "tfrp-mock-cluster"),
					resource.TestCheckResourceAttrSet(addr, "id"),
				),
			},
		},
	})
}

func mockClusterConfig() string {
	return `
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cluster-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cluster-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = "tfrp-mock-cluster"
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true
}
`
}

const clusterAddr = "redpanda_cluster.test"

// clusterSetup returns a fresh mock server and provider factories.
func clusterSetup(t *testing.T) (srv *mock.Server, factories map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	return integration.Setup(t)
}

// awsDedicatedConfig returns the AWS dedicated baseline HCL with the given
// cluster name and optional extra cluster body lines. The HCL always includes
// allow_deletion=true so implicit destroy at end-of-TestCase succeeds.
func awsDedicatedConfig(name string, extra ...string) string {
	body := ""
	for _, e := range extra {
		body += "\n  " + e
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true%s
}
`, name, body)
}

// gcpDedicatedConfig returns the GCP dedicated baseline HCL.
func gcpDedicatedConfig(name string, extra ...string) string {
	body := ""
	for _, e := range extra {
		body += "\n  " + e
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  zones             = ["us-central1-a"]
  throughput_tier   = "tier-1-gcp-um4g"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true%s
}
`, name, body)
}

// awsBYOCConfig returns the AWS BYOC baseline HCL.
func awsBYOCConfig(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    aws = {
      management_bucket = { arn = "arn:aws:s3:::tfrp-bv-mgmt" }
      dynamodb_table    = { arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb" }
      vpc               = { arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a" }
      private_subnets   = { arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"] }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "byoc"
  connection_type   = "private"
  allow_deletion    = true
}
`, name)
}

// awsBYOVPCConfig returns the AWS BYOVPC (customer_managed_resources.aws) HCL.
// securityGroupARN allows the C9 scenario to mutate cluster_security_group.arn.
func awsBYOVPCConfig(name, securityGroupARN string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    aws = {
      management_bucket = { arn = "arn:aws:s3:::tfrp-bv-mgmt" }
      dynamodb_table    = { arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb" }
      vpc               = { arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a" }
      private_subnets   = { arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"] }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "byoc"
  connection_type   = "private"
  allow_deletion    = true
  customer_managed_resources = {
    aws = {
      agent_instance_profile                   = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-agent" }
      cloud_storage_bucket                     = { arn = "arn:aws:s3:::tfrp-cloud-storage" }
      cluster_security_group                   = { arn = %q }
      connectors_node_group_instance_profile   = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-connectors-ng" }
      connectors_security_group                = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-connectors" }
      k8s_cluster_role                         = { arn = "arn:aws:iam::123456789012:role/tfrp-k8s-cluster" }
      node_security_group                      = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-nodes" }
      permissions_boundary_policy              = { arn = "arn:aws:iam::123456789012:policy/tfrp-permissions-boundary" }
      redpanda_agent_security_group            = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-agent" }
      redpanda_node_group_instance_profile     = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-rp-ng" }
      redpanda_node_group_security_group       = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-ng" }
      utility_node_group_instance_profile      = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-utility-ng" }
      utility_security_group                   = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-utility" }
    }
  }
}
`, name, securityGroupARN)
}

// gcpBYOVPCConfig returns the GCP BYOVPC (customer_managed_resources.gcp) HCL.
// subnetName allows the C10 scenario to mutate cmr.gcp.subnet.name.
// psc_nat_subnet_name is always set to a non-empty value because conv_gen.go
// flattens the unset proto3 string as "" rather than null.
func gcpBYOVPCConfig(name, subnetName, pscNatSubnet string) string {
	pscLine := ""
	if pscNatSubnet != "" {
		pscLine = fmt.Sprintf("\n      psc_nat_subnet_name   = %q", pscNatSubnet)
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    gcp = {
      network_name       = "tfrp-bv-net"
      network_project_id = "tfrp-bv-proj"
      management_bucket  = { name = "tfrp-bv-bkt" }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "gcp"
  region            = "us-central1"
  zones             = ["us-central1-a"]
  throughput_tier   = "tier-1-gcp-um4g"
  cluster_type      = "byoc"
  connection_type   = "private"
  allow_deletion    = true
  customer_managed_resources = {
    gcp = {
      agent_service_account     = { email = "agent@tfrp-proj.iam.gserviceaccount.com" }
      connector_service_account = { email = "connector@tfrp-proj.iam.gserviceaccount.com" }
      console_service_account   = { email = "console@tfrp-proj.iam.gserviceaccount.com" }
      gke_service_account       = { email = "gke@tfrp-proj.iam.gserviceaccount.com" }
      redpanda_cluster_service_account = { email = "redpanda@tfrp-proj.iam.gserviceaccount.com" }
      subnet = {
        name                          = %q
        k8s_master_ipv4_range         = "172.16.0.0/28"
        secondary_ipv4_range_pods     = { name = "pods-range" }
        secondary_ipv4_range_services = { name = "svc-range" }
      }
      tiered_storage_bucket = { name = "tfrp-tiered-bucket" }%s
    }
  }
}
`, name, subnetName, pscLine)
}

// twoRGConfig declares two resource_groups; rgLabel selects which one the
// cluster binds to ("rg1" or "rg2"). Used by C5.
func twoRGConfig(name, rgLabel string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "rg1" {
  name = "tfrp-mock-cl-rg-1"
}

resource "redpanda_resource_group" "rg2" {
  name = "tfrp-mock-cl-rg-2"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.rg1.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.%s.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true
}
`, name, rgLabel)
}

// awsDedicatedConfigWith returns the AWS dedicated baseline HCL with the named
// fields substituted. It exists to avoid HCL duplicate-attribute errors that
// would occur if the caller used awsDedicatedConfig's variadic extra to "override"
// a field that the template already declares.
func awsDedicatedConfigWith(name, throughputTier, connectionType, region string, zones []string, extra ...string) string {
	zonesHCL := `["` + zones[0] + `"`
	for _, z := range zones[1:] {
		zonesHCL += `, "` + z + `"`
	}
	zonesHCL += `]`

	body := ""
	for _, e := range extra {
		body += "\n  " + e
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = %q
  zones             = %s
  throughput_tier   = %q
  cluster_type      = "dedicated"
  connection_type   = %q
  allow_deletion    = true%s
}
`, name, region, zonesHCL, throughputTier, connectionType, body)
}

// awsDedicatedConfigBYOC returns the AWS dedicated baseline HCL with cluster_type
// and connection_type overridden. Used by C7.
func awsDedicatedConfigBYOC(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    aws = {
      management_bucket = { arn = "arn:aws:s3:::tfrp-bv-mgmt" }
      dynamodb_table    = { arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb" }
      vpc               = { arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a" }
      private_subnets   = { arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"] }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "byoc"
  connection_type   = "private"
  allow_deletion    = true
}
`, name)
}

// twoNetConfig declares two networks; netLabel selects which one the cluster
// binds to ("net1" or "net2"). Used by C3.
func twoNetConfig(name, netLabel string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "net1" {
  name              = "tfrp-mock-cl-net-1"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_network" "net2" {
  name              = "tfrp-mock-cl-net-2"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.1.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.%s.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true
}
`, name, netLabel)
}

// --- Class A: Cloud-provider variant baselines ---

// TestIntegration_Cluster_CreateAndRefresh_AWS_Dedicated creates and no-op re-applies
// the AWS dedicated variant. Asserts key top-level, optional, and computed-only
// leaves. idPreserved across both steps validates UseStateForUnknown on id
// (regression guard against id reverting to Unknown post-refresh).
func TestIntegration_Cluster_CreateAndRefresh_AWS_Dedicated(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-a1"
	cfg := awsDedicatedConfig(name,
		`redpanda_node_count = 3`,
		`tags = { env = "test" }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("public")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("throughput_tier"), knownvalue.StringExact("tier-1-aws-v3-arm")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("dedicated")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("zones"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("use1-az1"),
				})),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("redpanda_node_count"), knownvalue.Int32Exact(3)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("test")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("current_redpanda_version"), knownvalue.StringExact("24.3.1")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("desired_redpanda_version"), knownvalue.StringExact("24.3.1")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("kafka_api").AtMapKey("seed_brokers"),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("mock-broker-0.mock.redpanda.cloud:9092"),
					})),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("http_proxy").AtMapKey("url"), knownvalue.StringExact("https://mock.http-proxy.redpanda.cloud")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("schema_registry").AtMapKey("url"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("redpanda_console").AtMapKey("url"), knownvalue.StringExact("https://mock.console.redpanda.cloud")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("prometheus").AtMapKey("url"), knownvalue.StringExact("https://mock.prometheus.redpanda.cloud")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_CreateAndRefresh_GCP_Dedicated creates and no-op re-applies
// the GCP dedicated variant. Validates that AWSZoneIDValidator is cloud-provider-gated
// (GCP zones are accepted when cloud_provider="gcp").
func TestIntegration_Cluster_CreateAndRefresh_GCP_Dedicated(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-a2"
	cfg := gcpDedicatedConfig(name,
		`redpanda_node_count = 3`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("gcp")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-central1")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("zones"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("us-central1-a"),
				})),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("throughput_tier"), knownvalue.StringExact("tier-1-gcp-um4g")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("dedicated")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("current_redpanda_version"), knownvalue.StringExact("24.3.1")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("gcp")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_CreateAndRefresh_AWS_BYOC creates and no-op re-applies the
// AWS BYOC variant. The fake returns STATE_READY immediately so
// Cluster.Create's RetryGetCluster terminates without ever entering
// STATE_CREATING_AGENT; c.Byoc.RunByoc is never called.
func TestIntegration_Cluster_CreateAndRefresh_AWS_BYOC(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-a3"
	cfg := awsBYOCConfig(name)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("private")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_api_url"), knownvalue.StringExact("bufnet")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_CreateAndRefresh_AWS_BYOVPC creates and no-op re-applies
// the AWS BYOVPC variant with a full customer_managed_resources.aws block.
func TestIntegration_Cluster_CreateAndRefresh_AWS_BYOVPC(t *testing.T) {
	_, factories := clusterSetup(t)

	const (
		name  = "tfrp-mock-cl-a4"
		sgARN = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-cluster"
	)
	cfg := awsBYOVPCConfig(name, sgARN)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("private")),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("agent_instance_profile").AtMapKey("arn"),
					knownvalue.StringExact("arn:aws:iam::123456789012:instance-profile/tfrp-agent")),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("cloud_storage_bucket").AtMapKey("arn"),
					knownvalue.StringExact("arn:aws:s3:::tfrp-cloud-storage")),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("cluster_security_group").AtMapKey("arn"),
					knownvalue.StringExact(sgARN)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("k8s_cluster_role").AtMapKey("arn"),
					knownvalue.StringExact("arn:aws:iam::123456789012:role/tfrp-k8s-cluster")),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("permissions_boundary_policy").AtMapKey("arn"),
					knownvalue.StringExact("arn:aws:iam::123456789012:policy/tfrp-permissions-boundary")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("cluster_security_group").AtMapKey("arn"),
					knownvalue.StringExact(sgARN)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_CreateAndRefresh_GCP_BYOVPC creates and no-op re-applies
// the GCP BYOVPC variant with a full customer_managed_resources.gcp block.
func TestIntegration_Cluster_CreateAndRefresh_GCP_BYOVPC(t *testing.T) {
	_, factories := clusterSetup(t)

	const (
		name       = "tfrp-mock-cl-a5"
		subnetName = "tfrp-subnet-a"
	)
	cfg := gcpBYOVPCConfig(name, subnetName, "psc-nat-subnet-test")

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("gcp")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("private")),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("agent_service_account").AtMapKey("email"),
					knownvalue.StringExact("agent@tfrp-proj.iam.gserviceaccount.com")),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("subnet").AtMapKey("name"),
					knownvalue.StringExact(subnetName)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("tiered_storage_bucket").AtMapKey("name"),
					knownvalue.StringExact("tfrp-tiered-bucket")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("subnet").AtMapKey("name"),
					knownvalue.StringExact(subnetName)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// --- Class B: In-place UpdateLeaf ---

// TestIntegration_Cluster_UpdateLeaf_Name renames the cluster in place. name is in
// the ClusterUpdate payload so the FieldMask emits "name". id stays the same
// (ValuesSame) confirming no destroy occurred.
func TestIntegration_Cluster_UpdateLeaf_Name(t *testing.T) {
	_, factories := clusterSetup(t)

	const (
		nameA = "tfrp-mock-cl-rename-a"
		nameB = "tfrp-mock-cl-rename-b"
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())
	nameChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, awsDedicatedConfig(nameA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				nameChanged.AddStateValue(clusterAddr, tfjsonpath.New("name")),
			}),
			integration.UpdateLeafStep(clusterAddr, awsDedicatedConfig(nameB), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameB)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				nameChanged.AddStateValue(clusterAddr, tfjsonpath.New("name")),
			}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_ThroughputTier mutates throughput_tier in place.
func TestIntegration_Cluster_UpdateLeaf_ThroughputTier(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b2"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("throughput_tier"), knownvalue.StringExact("tier-1-aws-v3-arm")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfigWith(name, "tier-2-aws-v3-arm", "public", "us-east-1", []string{"use1-az1"}),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("throughput_tier"), knownvalue.StringExact("tier-2-aws-v3-arm")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_Tags mutates the tags map. Exercises the
// cloud_provider_tags <-> tags round-trip and the redpanda-* filter in
// tagsFromProto.
func TestIntegration_Cluster_UpdateLeaf_Tags(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b3"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `tags = { env = "dev", team = "ingest" }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("dev")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("tags").AtMapKey("team"), knownvalue.StringExact("ingest")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `tags = { env = "prod", "cost-center" = "42" }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("tags").AtMapKey("env"), knownvalue.StringExact("prod")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("tags").AtMapKey("cost-center"), knownvalue.StringExact("42")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_RedpandaNodeCount scales up node count in place.
func TestIntegration_Cluster_UpdateLeaf_RedpandaNodeCount(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b4"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `redpanda_node_count = 3`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("redpanda_node_count"), knownvalue.Int32Exact(3)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `redpanda_node_count = 6`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("redpanda_node_count"), knownvalue.Int32Exact(6)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_APIGatewayAccess toggles the api_gateway_access
// enum from "public" to "private".
func TestIntegration_Cluster_UpdateLeaf_APIGatewayAccess(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b5"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `api_gateway_access = "NETWORK_ACCESS_MODE_PUBLIC"`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("api_gateway_access"), knownvalue.StringExact("NETWORK_ACCESS_MODE_PUBLIC")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `api_gateway_access = "NETWORK_ACCESS_MODE_PRIVATE"`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("api_gateway_access"), knownvalue.StringExact("NETWORK_ACCESS_MODE_PRIVATE")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_ClusterConfiguration_CustomPropertiesJSON
// mutates the cluster_configuration.custom_properties_json field.
func TestIntegration_Cluster_UpdateLeaf_ClusterConfiguration_CustomPropertiesJSON(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b6"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `cluster_configuration = { custom_properties_json = "{\"auto.create.topics.enable\":\"true\"}" }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("cluster_configuration").AtMapKey("custom_properties_json"),
						knownvalue.StringExact(`{"auto.create.topics.enable":"true"}`)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_KafkaAPI_SASL_Enabled toggles
// kafka_api.sasl.enabled from false to true.
func TestIntegration_Cluster_UpdateLeaf_KafkaAPI_SASL_Enabled(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b7"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `kafka_api = { sasl = { enabled = false } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("kafka_api").AtMapKey("sasl").AtMapKey("enabled"),
						knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `kafka_api = { sasl = { enabled = true } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("kafka_api").AtMapKey("sasl").AtMapKey("enabled"),
						knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_HTTPProxy_MTLS_Enabled toggles
// http_proxy.mtls.enabled from false to true.
func TestIntegration_Cluster_UpdateLeaf_HTTPProxy_MTLS_Enabled(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b8"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `http_proxy = { mtls = { enabled = false } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("http_proxy").AtMapKey("mtls").AtMapKey("enabled"),
						knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `http_proxy = { mtls = { enabled = true } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("http_proxy").AtMapKey("mtls").AtMapKey("enabled"),
						knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_KafkaConnect_Enabled proves that
// GenerateProtobufDiffAndUpdateMask correctly detects the presence change when
// the user adds a kafka_connect block (null → { enabled = false }). The
// const=false constraint on KafkaConnect.enabled means enabled=true is invalid;
// only the null→false transition is testable. With the proto3 zero-value
// presence fix the mask includes kafka_connect, the fake stores it, and the
// post-apply Read returns the block in state.
func TestIntegration_Cluster_UpdateLeaf_KafkaConnect_Enabled(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-kc"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("kafka_connect"), knownvalue.Null()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `kafka_connect = { enabled = false }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("kafka_connect").AtMapKey("enabled"),
						knownvalue.Bool(false)),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_Rpsql exercises the rpsql block in place:
// enable (null→enabled), enable-after-disable (re-derives the computed url),
// and a replicas scale — all as ResourceActionUpdate. The scale also proves
// expandRpsqlPath round-trips the granular mask paths.
func TestIntegration_Cluster_UpdateLeaf_Rpsql(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-rpsql"
	const mockURL = "https://mock.rpsql.redpanda.cloud"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql"), knownvalue.Null()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			// Add a disabled block: replicas defaults to 1, url stays empty.
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `rpsql = { enabled = false }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("replicas"), knownvalue.Int32Exact(1)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("url"), knownvalue.StringExact("")),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			// Enable in place: url is re-derived to the provisioned endpoint.
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `rpsql = { enabled = true }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("url"), knownvalue.StringExact(mockURL)),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			// Scale replicas in place.
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `rpsql = { enabled = true, replicas = 3 }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("replicas"), knownvalue.Int32Exact(3)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("url"), knownvalue.StringExact(mockURL)),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_MaintenanceWindow_DayHour sets
// maintenance_window_config.day_hour from unset to {MONDAY,3} then mutates to
// {FRIDAY,17}. Validates the day_hour oneof variant and the v10d band-aid
// UseStateForUnknown on the unspecified field.
func TestIntegration_Cluster_UpdateLeaf_MaintenanceWindow_DayHour(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b10"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `maintenance_window_config = { day_hour = { day_of_week = "MONDAY", hour_of_day = 3 } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("day_of_week"),
						knownvalue.StringExact("MONDAY")),
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("hour_of_day"),
						knownvalue.Int32Exact(3)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `maintenance_window_config = { day_hour = { day_of_week = "FRIDAY", hour_of_day = 17 } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("day_of_week"),
						knownvalue.StringExact("FRIDAY")),
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("hour_of_day"),
						knownvalue.Int32Exact(17)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `maintenance_window_config = { day_hour = { day_of_week = "FRIDAY", hour_of_day = 0 } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("hour_of_day"),
						knownvalue.Int32Exact(0)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_MaintenanceWindow_Anytime flips
// maintenance_window_config from day_hour to anytime=true, exercising the
// oneof variant flip. The v10d band-aid USFU on unspecified keeps it null
// rather than churning.
func TestIntegration_Cluster_UpdateLeaf_MaintenanceWindow_Anytime(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b11"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `maintenance_window_config = { day_hour = { day_of_week = "MONDAY", hour_of_day = 3 } }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("day_of_week"),
						knownvalue.StringExact("MONDAY")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `maintenance_window_config = { anytime = true }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("maintenance_window_config").AtMapKey("anytime"),
						knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_AWSPrivateLink_Enabled toggles
// aws_private_link.enabled and exercises Cluster.ModifyPlan's endpoint-Unknown
// marking.
func TestIntegration_Cluster_UpdateLeaf_AWSPrivateLink_Enabled(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b13"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `aws_private_link = {
  enabled            = false
  connect_console    = false
  allowed_principals = ["arn:aws:iam::123456789012:root"]
}`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("aws_private_link").AtMapKey("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `aws_private_link = {
  enabled            = true
  connect_console    = false
  allowed_principals = ["arn:aws:iam::123456789012:root"]
}`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("aws_private_link").AtMapKey("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_AWSPrivateLink_SupportedRegions is the regression
// guard. With aws_private_link enabled and supported_regions unset, the schema's
// computed_only makes the planned value "known after apply"; before the fix it
// was a plain Optional whose planned value was null, which mismatched the
// server's empty/absent list and tripped "Provider produced inconsistent result
// after apply: was null, but now cty.ListValEmpty". UseStateForUnknown then
// holds the value across replans — and because the value lands null, this also
// pins the override choice over the classifier default UseNonNullStateForUnknown
// (which would leave a null leaf perpetually "known after apply" and churn).
//
// A proto3 repeated field cannot distinguish empty from absent on the wire, so
// the server's "[]" arrives as a nil slice that the provider flattens to null;
// the nil->null boundary itself is pinned by the model-level
// TestFlattenAWSPrivateLink_SupportedRegions_* tests.
func TestIntegration_Cluster_AWSPrivateLink_SupportedRegions(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-344"
	cfg := awsDedicatedConfig(name, `aws_private_link = {
  enabled            = true
  connect_console    = false
  allowed_principals = ["arn:aws:iam::123456789012:root"]
}`)
	srPath := tfjsonpath.New("aws_private_link").AtMapKey("supported_regions")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// computed_only => known after apply (null pre-fix).
						plancheck.ExpectUnknownValue(clusterAddr, srPath),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, srPath, knownvalue.Null()),
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("aws_private_link").AtMapKey("enabled"), knownvalue.Bool(true)),
				},
			},
			// Re-plan: UseStateForUnknown holds the null value, no churn.
			integration.NoopReapplyStep(clusterAddr, cfg, nil),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_GCPPrivateServiceConnect_Toggle toggles
// gcp_private_service_connect.enabled from false to true.
func TestIntegration_Cluster_UpdateLeaf_GCPPrivateServiceConnect_Toggle(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b14"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				gcpDedicatedConfig(name, `gcp_private_service_connect = {
  enabled              = false
  global_access_enabled = false
  consumer_accept_list = [{ source = "my-gcp-project-id" }]
}`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("gcp_private_service_connect").AtMapKey("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				gcpDedicatedConfig(name, `gcp_private_service_connect = {
  enabled              = true
  global_access_enabled = false
  consumer_accept_list = [{ source = "my-gcp-project-id" }]
}`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("gcp_private_service_connect").AtMapKey("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_UpdateLeaf_ReadReplicaClusterIds mutates
// read_replica_cluster_ids from empty to a two-element list.
func TestIntegration_Cluster_UpdateLeaf_ReadReplicaClusterIds(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b15"

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `read_replica_cluster_ids = ["00000000000000000001", "00000000000000000002"]`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("read_replica_cluster_ids"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("00000000000000000001"),
							knownvalue.StringExact("00000000000000000002"),
						})),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// --- Class C: RequiresReplace ---

// TestIntegration_Cluster_RequiresReplace_CloudProvider mutates cloud_provider from
// "aws" to "gcp". Both region and zones must be consistent with the new
// provider. idChanged proves destroy-before-create occurred.
func TestIntegration_Cluster_RequiresReplace_CloudProvider(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c1"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("aws")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				gcpDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cloud_provider"), knownvalue.StringExact("gcp")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_ConnectionType mutates connection_type from
// "public" to "private". idChanged proves destroy-before-create.
func TestIntegration_Cluster_RequiresReplace_ConnectionType(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c2"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("public")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				awsDedicatedConfigWith(name, "tier-1-aws-v3-arm", "private", "us-east-1", []string{"use1-az1"}),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("private")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_NetworkID switches the cluster's network_id
// between two declared networks. idChanged proves destroy-before-create.
func TestIntegration_Cluster_RequiresReplace_NetworkID(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c3"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				twoNetConfig(name, "net1"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				twoNetConfig(name, "net2"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_Region mutates region from "us-east-1" to
// "us-west-2". idChanged proves destroy-before-create.
func TestIntegration_Cluster_RequiresReplace_Region(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c4"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				awsDedicatedConfigWith(name, "tier-1-aws-v3-arm", "public", "us-west-2", []string{"usw2-az1"}),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("region"), knownvalue.StringExact("us-west-2")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_ResourceGroupID switches the cluster's
// resource_group_id between two declared resource groups. idChanged proves
// destroy-before-create.
func TestIntegration_Cluster_RequiresReplace_ResourceGroupID(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c5"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				twoRGConfig(name, "rg1"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				twoRGConfig(name, "rg2"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_Zones mutates zones from ["use1-az1"] to
// ["use1-az2"]. idChanged proves destroy-before-create.
func TestIntegration_Cluster_RequiresReplace_Zones(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c6"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("zones"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("use1-az1")})),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				awsDedicatedConfigWith(name, "tier-1-aws-v3-arm", "public", "us-east-1", []string{"use1-az2"}),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("zones"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("use1-az2")})),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_ClusterType flips cluster_type from
// "dedicated" to "byoc". Also switches connection_type to "private" (best
// practice for BYOC). idChanged proves destroy-before-create. c.Byoc.RunByoc
// is never invoked because the fake stays in STATE_READY.
func TestIntegration_Cluster_RequiresReplace_ClusterType(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c7"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("dedicated")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				awsDedicatedConfigBYOC(name),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("connection_type"), knownvalue.StringExact("private")),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_RedpandaVersion explicitly sets
// redpanda_version (Optional+RR extra field) and then mutates it.
// idChanged proves destroy-before-create.
func TestIntegration_Cluster_RequiresReplace_RedpandaVersion(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-c8"

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsDedicatedConfig(name, `redpanda_version = "24.3.1"`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				awsDedicatedConfig(name, `redpanda_version = "24.3.2"`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_CMR_AWS_Block mutates one inner ARN inside
// customer_managed_resources.aws. The block-level RequiresReplace fires
// destroy-before-create. idChanged proves it.
func TestIntegration_Cluster_RequiresReplace_CMR_AWS_Block(t *testing.T) {
	_, factories := clusterSetup(t)

	const (
		name = "tfrp-mock-cl-c9"
		sgA  = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-cluster-a"
		sgB  = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-cluster-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				awsBYOVPCConfig(name, sgA),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("cluster_security_group").AtMapKey("arn"),
						knownvalue.StringExact(sgA)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				awsBYOVPCConfig(name, sgB),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("customer_managed_resources").AtMapKey("aws").AtMapKey("cluster_security_group").AtMapKey("arn"),
						knownvalue.StringExact(sgB)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_RequiresReplace_CMR_GCP_Block mutates cmr.gcp.subnet.name.
// The block-level RequiresReplace fires destroy-before-create. idChanged proves it.
// gcpBYOVPCConfig always sets psc_nat_subnet_name (see helper for why).
func TestIntegration_Cluster_RequiresReplace_CMR_GCP_Block(t *testing.T) {
	_, factories := clusterSetup(t)

	const (
		name        = "tfrp-mock-cl-c10"
		subnetNameA = "tfrp-subnet-a"
		subnetNameB = "tfrp-subnet-b"
	)

	idChanged := statecheck.CompareValue(compare.ValuesDiffer())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr,
				gcpBYOVPCConfig(name, subnetNameA, "psc-nat-subnet-test"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("subnet").AtMapKey("name"),
						knownvalue.StringExact(subnetNameA)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
			integration.RequiresReplaceStep(clusterAddr,
				gcpBYOVPCConfig(name, subnetNameB, "psc-nat-subnet-test"),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("subnet").AtMapKey("name"),
						knownvalue.StringExact(subnetNameB)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idChanged.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// --- Class D: ImportRoundTrip ---

// TestIntegration_Cluster_ImportRoundTrip exercises ImportStatePassthroughID.
// The config includes a timeouts {} block as a regression re-pin: if id's
// UseStateForUnknown modifier regresses, the post-import re-plan would show
// id as Unknown and plan a destroy+create, causing ImportStateVerify to fail.
func TestIntegration_Cluster_ImportRoundTrip(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-d1"

	importCfg := fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true

  maintenance_window_config = { day_hour = { day_of_week = "MONDAY", hour_of_day = 0 } }
  cloud_storage             = { skip_destroy = false }
}
`, name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, importCfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("hour_of_day"),
					knownvalue.Int32Exact(0)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("cloud_storage").AtMapKey("skip_destroy"), knownvalue.Bool(false)),
			}),
			integration.ImportRoundTripStep(clusterAddr, nil, []string{"allow_deletion"}),
		},
	})
}

// --- Class E: ErrorPath ---

// TestIntegration_Cluster_ErrorPath_CreateFails injects codes.Internal on
// CreateCluster. The provider surfaces the error as "failed to create cluster".
func TestIntegration_Cluster_ErrorPath_CreateFails(t *testing.T) {
	srv, factories := clusterSetup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.ErrorPathStep(srv,
				controlplanev1grpc.ClusterService_CreateCluster_FullMethodName,
				codes.Internal,
				awsDedicatedConfig("tfrp-mock-cl-ep-create"),
				"failed to create cluster",
			),
		},
	})
}

// TestIntegration_Cluster_ErrorPath_ReadFails_NotFound injects codes.NotFound on
// GetCluster after a successful Create. The provider's Read calls
// RemoveResource; the next plan re-creates.
func TestIntegration_Cluster_ErrorPath_ReadFails_NotFound(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-ep-notfound"
	cfg := awsDedicatedConfig(name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ClusterService_GetCluster_FullMethodName,
						status.Error(codes.NotFound, "not found"),
					)
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_ReadFails_Internal injects codes.Internal on
// GetCluster after a successful Create. Non-NotFound errors surface as
// "failed to read cluster" diagnostics.
func TestIntegration_Cluster_ErrorPath_ReadFails_Internal(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-ep-readfail"
	cfg := awsDedicatedConfig(name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ClusterService_GetCluster_FullMethodName,
						status.Error(codes.Internal, "synthetic read failure"),
					)
				},
				Config:      cfg,
				ExpectError: regexp.MustCompile("failed to read cluster"),
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_UpdateFails injects codes.Internal on
// UpdateCluster after a successful Create. The provider surfaces "failed to
// send cluster update request".
func TestIntegration_Cluster_ErrorPath_UpdateFails(t *testing.T) {
	srv, factories := clusterSetup(t)

	const (
		nameA = "tfrp-mock-cl-ep-update-a"
		nameB = "tfrp-mock-cl-ep-update-b"
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, awsDedicatedConfig(nameA), []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(nameA)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ClusterService_UpdateCluster_FullMethodName,
						status.Error(codes.Internal, "synthetic update failure"),
					)
				},
				Config:      awsDedicatedConfig(nameB),
				ExpectError: regexp.MustCompile("failed to send cluster update request"),
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_DeleteFails injects codes.Internal on
// DeleteCluster. The provider surfaces "failed to delete cluster".
func TestIntegration_Cluster_ErrorPath_DeleteFails(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-ep-delfail"
	cfg := awsDedicatedConfig(name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
			}),
			{
				PreConfig: func() {
					srv.OverrideOnce(
						controlplanev1grpc.ClusterService_DeleteCluster_FullMethodName,
						status.Error(codes.Internal, "synthetic delete failure"),
					)
				},
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("failed to delete cluster"),
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_AllowDeletionBlocked creates a cluster with
// allow_deletion=false. Destroy fails at the provider-side guard with "cluster
// deletion not allowed". A third step flips to allow_deletion=true so the
// framework cleanup destroy can proceed.
func TestIntegration_Cluster_ErrorPath_AllowDeletionBlocked(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-ep-nodeletion"

	noDeleteCfg := fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = false
}
`, name)

	allowDeleteCfg := fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "dedicated"
  connection_type   = "public"
  allow_deletion    = true
}
`, name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: noDeleteCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttrSet(clusterAddr, "id"),
			},
			{
				Config:      noDeleteCfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile("cluster deletion not allowed"),
			},
			{
				Config: allowDeleteCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.TestCheckResourceAttrSet(clusterAddr, "id"),
			},
		},
	})
}

// TestIntegration_Cluster_ErrorPath_BYOVPC_NonPrivateConnectionType exercises the
// RequirePrivateConnectionValidator: customer_managed_resources set +
// connection_type="public" is rejected at plan time.
func TestIntegration_Cluster_ErrorPath_BYOVPC_NonPrivateConnectionType(t *testing.T) {
	_, factories := clusterSetup(t)

	invalidCfg := `
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-cl-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-cl-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "byoc"
  customer_managed_resources = {
    aws = {
      management_bucket = { arn = "arn:aws:s3:::tfrp-bv-mgmt" }
      dynamodb_table    = { arn = "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb" }
      vpc               = { arn = "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a" }
      private_subnets   = { arns = ["arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"] }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = "tfrp-mock-cl-ep-byovpc-pub"
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = "byoc"
  connection_type   = "public"
  allow_deletion    = true
  customer_managed_resources = {
    aws = {
      agent_instance_profile                   = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-agent" }
      cloud_storage_bucket                     = { arn = "arn:aws:s3:::tfrp-cloud-storage" }
      cluster_security_group                   = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-cluster" }
      connectors_node_group_instance_profile   = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-connectors-ng" }
      connectors_security_group                = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-connectors" }
      k8s_cluster_role                         = { arn = "arn:aws:iam::123456789012:role/tfrp-k8s-cluster" }
      node_security_group                      = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-nodes" }
      permissions_boundary_policy              = { arn = "arn:aws:iam::123456789012:policy/tfrp-permissions-boundary" }
      redpanda_agent_security_group            = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-agent" }
      redpanda_node_group_instance_profile     = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-rp-ng" }
      redpanda_node_group_security_group       = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-ng" }
      utility_node_group_instance_profile      = { arn = "arn:aws:iam::123456789012:instance-profile/tfrp-utility-ng" }
      utility_security_group                   = { arn = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-utility" }
    }
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      invalidCfg,
				ExpectError: regexp.MustCompile(`connection_type must be`),
			},
		},
	})
}

// --- Class F: NestedMatrix Dense ---

// TestIntegration_Cluster_NestedMatrix_AWSPrivateLink_Dense fully populates the
// aws_private_link block (Required: allowed_principals, connect_console,
// enabled) and verifies it round-trips via Create + NoopReapply. The empty
// plan on the Noop step is the load-bearing v10d-override regression guard
// for aws_private_link.supported_regions (UseStateForUnknown overrides
// classifier default to prevent plan churn on null server-side field).
func TestIntegration_Cluster_NestedMatrix_AWSPrivateLink_Dense(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-f1"
	cfg := awsDedicatedConfig(name,
		`aws_private_link = {
    enabled            = true
    connect_console    = true
    allowed_principals = ["arn:aws:iam::123456789012:user/test-user", "arn:aws:iam::123456789012:role/test-role"]
  }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("aws_private_link").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("aws_private_link").AtMapKey("connect_console"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("aws_private_link").AtMapKey("allowed_principals"),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::123456789012:user/test-user"),
						knownvalue.StringExact("arn:aws:iam::123456789012:role/test-role"),
					})),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("aws_private_link").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("aws_private_link").AtMapKey("connect_console"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_NestedMatrix_GCPPrivateServiceConnect_Dense fully populates
// the gcp_private_service_connect block. The empty plan on the Noop step is
// the load-bearing assertion that ModifyPlan and UseNonNullStateForUnknown on
// status sub-fields keep the plan clean.
func TestIntegration_Cluster_NestedMatrix_GCPPrivateServiceConnect_Dense(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-f2"
	cfg := gcpDedicatedConfig(name,
		`gcp_private_service_connect = {
    enabled               = true
    global_access_enabled = true
    consumer_accept_list  = [{ source = "my-gcp-project-id" }, { source = "another-project" }]
  }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("gcp_private_service_connect").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("gcp_private_service_connect").AtMapKey("global_access_enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("gcp_private_service_connect").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_NestedMatrix_HTTPProxy_Dense fully populates http_proxy
// including mtls and sasl sub-blocks.
func TestIntegration_Cluster_NestedMatrix_HTTPProxy_Dense(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-f3"
	cfg := awsDedicatedConfig(name,
		`http_proxy = {
    mtls = { enabled = true }
    sasl = { enabled = true }
  }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("http_proxy").AtMapKey("mtls").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("http_proxy").AtMapKey("sasl").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("http_proxy").AtMapKey("mtls").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("http_proxy").AtMapKey("sasl").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_NestedMatrix_KafkaAPI_Dense fully populates kafka_api
// including mtls and sasl sub-blocks.
func TestIntegration_Cluster_NestedMatrix_KafkaAPI_Dense(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-f4"
	cfg := awsDedicatedConfig(name,
		`kafka_api = {
    mtls = { enabled = true }
    sasl = { enabled = true }
  }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("kafka_api").AtMapKey("mtls").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("kafka_api").AtMapKey("sasl").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("kafka_api").AtMapKey("mtls").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("kafka_api").AtMapKey("sasl").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_NestedMatrix_SchemaRegistry_Dense fully populates
// schema_registry including the mtls sub-block.
func TestIntegration_Cluster_NestedMatrix_SchemaRegistry_Dense(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-f5"
	cfg := awsDedicatedConfig(name,
		`schema_registry = {
    mtls = { enabled = true }
  }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("schema_registry").AtMapKey("mtls").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("schema_registry").AtMapKey("mtls").AtMapKey("enabled"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
		},
	})
}

// TestIntegration_Cluster_NestedMatrix_CloudStorage_Dense populates
// cloud_storage.skip_destroy=true (the only mutable leaf; aws/gcp sub-objects
// are Computed-only). Verifies the flag round-trips via Create + Noop.
func TestIntegration_Cluster_NestedMatrix_CloudStorage_Dense(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-f6"
	cfg := awsDedicatedConfig(name,
		`cloud_storage = { skip_destroy = true }`,
	)

	idPreserved := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("cloud_storage").AtMapKey("skip_destroy"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr,
					tfjsonpath.New("cloud_storage").AtMapKey("skip_destroy"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
				idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.UpdateLeafStep(clusterAddr,
				awsDedicatedConfig(name, `cloud_storage = { skip_destroy = false }`),
				[]statecheck.StateCheck{
					statecheck.ExpectKnownValue(clusterAddr,
						tfjsonpath.New("cloud_storage").AtMapKey("skip_destroy"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("id"), knownvalue.NotNull()),
					idPreserved.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				}),
		},
	})
}

// TestIntegration_Cluster_OptionalComputed_MaintenanceWindowHour covers the
// Optional+Computed contract for hour_of_day: a server-supplied default is
// adopted when config omits the field and stays stable across re-apply, and a
// later config value overrides a differing remote default.
func TestIntegration_Cluster_OptionalComputed_MaintenanceWindowHour(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-oc1"

	srv.Cluster.CreateMutator = func(cl *controlplanev1.Cluster) {
		if dh := cl.GetMaintenanceWindowConfig().GetDayHour(); dh != nil {
			dh.HourOfDay = 9
		}
	}

	hourPath := tfjsonpath.New("maintenance_window_config").AtMapKey("day_hour").AtMapKey("hour_of_day")
	nullHour := awsDedicatedConfig(name, `maintenance_window_config = { day_hour = { day_of_week = "MONDAY" } }`)
	setHour := awsDedicatedConfig(name, `maintenance_window_config = { day_hour = { day_of_week = "MONDAY", hour_of_day = 3 } }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, nullHour, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, hourPath, knownvalue.Int32Exact(9)),
			}),
			integration.NoopReapplyStep(clusterAddr, nullHour, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, hourPath, knownvalue.Int32Exact(9)),
			}),
			integration.UpdateLeafStep(clusterAddr, setHour, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, hourPath, knownvalue.Int32Exact(3)),
			}),
		},
	})
}

// TestIntegration_Cluster_OptionalComputed_CloudStorageSkipDestroy covers the
// Optional+Computed contract for skip_destroy: a server-supplied default is
// adopted when config omits the field and stays stable across re-apply, and a
// later config value overrides a differing remote default.
func TestIntegration_Cluster_OptionalComputed_CloudStorageSkipDestroy(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-oc2"

	srv.Cluster.CreateMutator = func(cl *controlplanev1.Cluster) {
		cl.CloudStorage = &controlplanev1.Cluster_CloudStorage{SkipDestroy: true}
	}

	skipPath := tfjsonpath.New("cloud_storage").AtMapKey("skip_destroy")
	noStorage := awsDedicatedConfig(name)
	setFalse := awsDedicatedConfig(name, `cloud_storage = { skip_destroy = false }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, noStorage, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, skipPath, knownvalue.Bool(true)),
			}),
			integration.NoopReapplyStep(clusterAddr, noStorage, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, skipPath, knownvalue.Bool(true)),
			}),
			integration.UpdateLeafStep(clusterAddr, setFalse, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, skipPath, knownvalue.Bool(false)),
			}),
		},
	})
}

// TestIntegration_Cluster_NullPscNatSubnetName_Repro omits psc_nat_subnet_name
// from the GCP CMR block; the Optional-only string leaf must stay null across
// create and re-apply rather than materializing an empty string.
func TestIntegration_Cluster_NullPscNatSubnetName_Repro(t *testing.T) {
	_, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-nullpsc"
	cfg := gcpBYOVPCConfig(name, "tfrp-subnet-nullpsc", "")

	pscPath := tfjsonpath.New("customer_managed_resources").AtMapKey("gcp").AtMapKey("psc_nat_subnet_name")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, pscPath, knownvalue.Null()),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, pscPath, knownvalue.Null()),
			}),
		},
	})
}
