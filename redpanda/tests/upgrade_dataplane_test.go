//go:build upgrade

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

package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_DataplaneResourcesNoChurn is the self-contained, CI-runnable guard
// that upgrading from the latest released provider to the local build produces
// no churn on the dataplane resources. It provisions its own public serverless
// cluster (no fixture dependency) with the released provider, then re-plans with
// the local build and asserts user/acl/topic plan as no-op. All three have
// RequiresReplace on cluster_api_url, so any drift in how the local build reads
// or normalizes that value shows up here as a destructive plan.
//
// This test previously asserted the schema-version-0-to-1 cluster_api_url format
// migration by writing the legacy host:443 form in step 1. That premise is dead:
// Version: 1 shipped in v2.0.0, so the released provider is already at version 1,
// Terraform sees no version delta, and UpgradeResourceState is never called —
// step 2 planned delete+create every run. The migration itself is covered by
// TestIntegration_{User,Topic,ACL}_UpgradeState_NormalizesClusterApiUrl, which
// drive the real UpgradeResourceState RPC at version 0, plus unit tests on each
// resource's UpgradeState. The URL is still derived from the cluster's own
// output, so this holds regardless of the format the live API returns.
//
// The assertion is per-resource rather than whole-plan empty: step 0 omits
// serverless_cluster's allow_deletion so the config still applies under
// REDPANDA_LAST_VERSION pins that predate the field (≤v1.9.0), and step 1
// setting it true (so teardown can destroy the cluster) then plans as a benign
// update on the managed cluster. acl is load-bearing since it has no
// ImportState.
//
// Requires REDPANDA_CLIENT_ID + REDPANDA_CLIENT_SECRET; self-provisions the
// cluster, so no KAFKA_CLUSTER_* fixture is needed.
func TestUpgrade_DataplaneResourcesNoChurn(t *testing.T) {
	n := dataplaneUpgradeNames{
		rg:      upgrade.RandomName("tfrp-upg-rg"),
		cluster: upgrade.RandomName("tfrp-upg-sl"),
		user:    upgrade.RandomName("tfrp-upg-user"),
		topic:   upgrade.RandomName("tfrp-upg-topic"),
		acl:     upgrade.RandomName("tfrp-upg-acl"),
	}
	upgrade.CreateAndRunMigrationApplyTest(t,
		dataplaneUpgradeConfig(n, true),  // released provider creates the resources
		dataplaneUpgradeConfig(n, false), // local build re-plans the same config
		[]plancheck.PlanCheck{
			plancheck.ExpectResourceAction("redpanda_user.test", plancheck.ResourceActionNoop),
			plancheck.ExpectResourceAction("redpanda_acl.test", plancheck.ResourceActionNoop),
			plancheck.ExpectResourceAction("redpanda_topic.test", plancheck.ResourceActionNoop),
		},
	)
}

type dataplaneUpgradeNames struct {
	rg, cluster, user, topic, acl string
}

func dataplaneUpgradeConfig(n dataplaneUpgradeNames, released bool) string {
	// the local build sets allow_deletion=true so teardown can destroy the
	// cluster; step 0 omits it so the config also applies under
	// REDPANDA_LAST_VERSION pins that predate the field. The cluster_api_url is
	// canonical in both steps — the two providers must agree on it or
	// RequiresReplace fires.
	apiURL, clusterAllowDeletion := `"https://${local.host}"`, "\n  allow_deletion    = true"
	if released {
		clusterAllowDeletion = ""
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = %q
}

resource "redpanda_serverless_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = "eu-west-1"%s
}

locals {
  host = replace(replace(redpanda_serverless_cluster.test.cluster_api_url, "https://", ""), ":443", "")
}

resource "redpanda_user" "test" {
  name            = %q
  password        = "upgrade-test-pw-123"
  mechanism       = "scram-sha-256"
  cluster_api_url = %s
  allow_deletion  = true
}

resource "redpanda_topic" "test" {
  name               = %q
  partition_count    = 1
  replication_factor = 3
  cluster_api_url    = %s
  allow_deletion     = true
}

resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = %q
  resource_pattern_type = "LITERAL"
  principal             = "User:%s"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = %s
  allow_deletion        = true
}
`, n.rg, n.cluster, clusterAllowDeletion, n.user, apiURL, n.topic, apiURL, n.acl, n.user, apiURL)
}
