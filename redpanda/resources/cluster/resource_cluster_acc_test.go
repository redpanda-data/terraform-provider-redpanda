// Copyright 2024 Redpanda Data, Inc.
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

// Acceptance tests against the in-process fake at redpanda/cloud/cloudtest.
// Runs Terraform's full PlanResourceChange + ApplyResourceChange pipeline,
// so plan modifiers are exercised — unlike the unit harness in
// resource_cluster_test.go, which calls Create/Read/Update directly.

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud/cloudtest"
)

// newAccProtoV6Factories starts an in-process fake control plane and
// returns a provider factory wired to it. TF_ACC is set so resource.Test
// runs unconditionally — the framework's TF_ACC gate exists to prevent
// accidental runs against real backends, which doesn't apply here.
func newAccProtoV6Factories(t *testing.T) (map[string]func() (tfprotov6.ProviderServer, error), *cloudtest.Fake) {
	t.Helper()
	t.Setenv("TF_ACC", "1")
	fake, conn := cloudtest.Start(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": providerserver.NewProtocol6WithError(
			redpanda.NewWithTestConn(context.Background(), "test", "acc-test", conn)(),
		),
	}
	return factories, fake
}

// hclClusterWithAwsPrivateLink builds a minimal `redpanda_cluster` HCL
// config with the supplied aws_private_link block. resource_group_id and
// network_id are literals — the fake doesn't validate FKs.
func hclClusterWithAwsPrivateLink(plBlock string) string {
	return `
provider "redpanda" {}

resource "redpanda_cluster" "test" {
  name              = "fake-cluster"
  cluster_type      = "dedicated"
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v2-arm"
  resource_group_id = "rg-fake"
  network_id        = "net-fake"
  connection_type   = "private"
  allow_deletion    = true

  aws_private_link = ` + plBlock + `
}
`
}

const plEnabled = `{
    enabled            = true
    connect_console    = true
    allowed_principals = ["arn:aws:iam::123456789012:root"]
  }`

const plDisabled = `{
    enabled            = false
    connect_console    = false
    allowed_principals = []
  }`

// hclMinimalClusterTagged builds a minimal redpanda_cluster HCL config
// with no Optional+Computed nested blocks configured (no cloud_storage,
// no cluster_configuration, no private-link) and a single mutable
// `tags` attribute. Step 2 of
// TestAcc_Cluster_OptionalComputedParents_NonEmptyDiff swaps the tag
// value to force a real Update plan — without that, the framework
// short-circuits the Computed-null-config marking step at
// internal/fwserver/server_planresourcechange.go:200 and the
// regression can't surface.
func hclMinimalClusterTagged(tag string) string {
	return `
provider "redpanda" {}

resource "redpanda_cluster" "test" {
  name              = "fake-cluster"
  cluster_type      = "dedicated"
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v2-arm"
  resource_group_id = "rg-fake"
  network_id        = "net-fake"
  connection_type   = "public"
  allow_deletion    = true
  tags              = { key = "` + tag + `" }
}
`
}

// clusterNonNullStateForUnknownPaths is every attribute on
// redpanda_cluster whose parent schema block uses
// UseNonNullStateForUnknown (or where UseStateForUnknown sits
// alongside siblings using the non-null variant). Each path is
// asserted known — i.e. not Unknown — in the PreApply plan of Step 2.
//
// The list reflects the schema at schema_resource.go as of this
// writing. When a new UseNonNullStateForUnknown usage is added or
// removed, update this list accordingly.
var clusterNonNullStateForUnknownPaths = []tfjsonpath.Path{
	tfjsonpath.New("cloud_storage"),
	tfjsonpath.New("cluster_configuration"),
	tfjsonpath.New("http_proxy"),
	tfjsonpath.New("http_proxy").AtMapKey("all_urls"),
	tfjsonpath.New("http_proxy").AtMapKey("all_urls").AtMapKey("mtls"),
	tfjsonpath.New("http_proxy").AtMapKey("all_urls").AtMapKey("private_link_mtls"),
	tfjsonpath.New("http_proxy").AtMapKey("all_urls").AtMapKey("private_link_sasl"),
	tfjsonpath.New("http_proxy").AtMapKey("all_urls").AtMapKey("sasl"),
	tfjsonpath.New("http_proxy").AtMapKey("mtls"),
	tfjsonpath.New("http_proxy").AtMapKey("sasl"),
	tfjsonpath.New("http_proxy").AtMapKey("url"),
	tfjsonpath.New("kafka_api"),
	tfjsonpath.New("kafka_api").AtMapKey("all_seed_brokers"),
	tfjsonpath.New("kafka_api").AtMapKey("all_seed_brokers").AtMapKey("mtls"),
	tfjsonpath.New("kafka_api").AtMapKey("all_seed_brokers").AtMapKey("private_link_mtls"),
	tfjsonpath.New("kafka_api").AtMapKey("all_seed_brokers").AtMapKey("private_link_sasl"),
	tfjsonpath.New("kafka_api").AtMapKey("all_seed_brokers").AtMapKey("sasl"),
	tfjsonpath.New("kafka_api").AtMapKey("mtls"),
	tfjsonpath.New("kafka_api").AtMapKey("sasl"),
	tfjsonpath.New("kafka_api").AtMapKey("seed_brokers"),
	tfjsonpath.New("kafka_connect"),
	tfjsonpath.New("maintenance_window_config"),
	tfjsonpath.New("prometheus"),
	tfjsonpath.New("prometheus").AtMapKey("url"),
	tfjsonpath.New("redpanda_console"),
	tfjsonpath.New("redpanda_console").AtMapKey("url"),
	tfjsonpath.New("schema_registry"),
	tfjsonpath.New("schema_registry").AtMapKey("all_urls"),
	tfjsonpath.New("schema_registry").AtMapKey("all_urls").AtMapKey("mtls"),
	tfjsonpath.New("schema_registry").AtMapKey("all_urls").AtMapKey("private_link_mtls"),
	tfjsonpath.New("schema_registry").AtMapKey("all_urls").AtMapKey("private_link_sasl"),
	tfjsonpath.New("schema_registry").AtMapKey("all_urls").AtMapKey("sasl"),
	tfjsonpath.New("schema_registry").AtMapKey("mtls"),
	tfjsonpath.New("schema_registry").AtMapKey("url"),
	tfjsonpath.New("state_description"),
}

// TestAcc_Cluster_OptionalComputedParents_NonEmptyDiff locks in the
// build 1284 fix for `+ cloud_storage = (known after apply)` and
// `+ cluster_configuration = (known after apply)` drift, and extends
// the guarantee to every UseNonNullStateForUnknown attribute on the
// cluster schema.
//
// Step 2 mutates `tags` so ProposedNewState != PriorState: this
// forces the framework's Computed-null-config marking step
// (internal/fwserver/server_planresourcechange.go:200) to fire, which
// invokes each attribute's plan modifier. The PreApply plancheck
// then reads rc.Change.AfterUnknown directly and fails on any path
// whose modifier left the plan Unknown when state was null.
//
// Reading AfterUnknown is required: terraform 1.14+'s rendered-diff
// collapses isolated null-state+unknown-plan entries to no-op,
// hiding the regression. Protocol-level AfterUnknown does not
// collapse, so this test is terraform-CLI-version-independent.
func TestAcc_Cluster_OptionalComputedParents_NonEmptyDiff(t *testing.T) {
	factories, _ := newAccProtoV6Factories(t)

	checks := make([]plancheck.PlanCheck, 0, len(clusterNonNullStateForUnknownPaths))
	for _, p := range clusterNonNullStateForUnknownPaths {
		checks = append(checks, cloudtest.ExpectKnownNotUnknown("redpanda_cluster.test", p))
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: hclMinimalClusterTagged("v1")},
			{
				Config: hclMinimalClusterTagged("v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: checks,
				},
			},
		},
	})
}

// Toggle PL disabled→enabled. Pre-fix the parent UseStateForUnknown on
// aws_private_link.status copies prior-null into the plan, then apply
// returns populated and the framework errors "inconsistent result after apply".
func TestAcc_Cluster_AwsPrivateLinkStatusNullToPopulated(t *testing.T) {
	factories, _ := newAccProtoV6Factories(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: hclClusterWithAwsPrivateLink(plDisabled)},
			{Config: hclClusterWithAwsPrivateLink(plEnabled)},
			{
				Config:             hclClusterWithAwsPrivateLink(plEnabled),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// PL on→off must not trip "inconsistent result after apply"
// (terraform-plugin-framework#1211). The PreApply plancheck asserts
// ModifyPlan marked the three endpoint objects Unknown.
func TestAcc_Cluster_AwsPrivateLinkToggle(t *testing.T) {
	factories, _ := newAccProtoV6Factories(t)

	expectEndpointsUnknown := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectUnknownValue("redpanda_cluster.test", tfjsonpath.New("kafka_api").AtMapKey("all_seed_brokers")),
			plancheck.ExpectUnknownValue("redpanda_cluster.test", tfjsonpath.New("http_proxy").AtMapKey("all_urls")),
			plancheck.ExpectUnknownValue("redpanda_cluster.test", tfjsonpath.New("schema_registry").AtMapKey("all_urls")),
		},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: hclClusterWithAwsPrivateLink(plEnabled)},
			{
				Config:             hclClusterWithAwsPrivateLink(plEnabled),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config:           hclClusterWithAwsPrivateLink(plDisabled),
				ConfigPlanChecks: expectEndpointsUnknown,
			},
			{
				Config:             hclClusterWithAwsPrivateLink(plDisabled),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
