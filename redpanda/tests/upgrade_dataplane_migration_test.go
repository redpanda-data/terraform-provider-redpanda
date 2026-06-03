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

// TestUpgrade_DataplaneClusterAPIURLMigration is the self-contained, CI-runnable
// guard for the cluster_api_url format migration. It provisions its own public
// serverless cluster (no fixture dependency) with the latest released provider,
// storing the dataplane resources' cluster_api_url in the legacy host:443 form;
// the local build then re-plans with the canonical https://host form. The three
// dataplane resources planning as no-op proves schema-version-1 UpgradeState
// rewrote the value in place rather than forcing replacement, and exercises the
// UpgradeResourceState PriorSchema decode against real released-provider state.
//
// host:443 is the cluster's real endpoint in legacy notation (host + the HTTPS
// port); the dataplane dialer accepts a scheme-less host:port, so the released
// provider creates the resources for real. Both URL forms are derived
// symmetrically from the cluster's own output so the assertion holds regardless
// of the format the live API returns.
//
// The assertion is per-resource (user/acl/topic no-op) rather than whole-plan
// empty: serverless_cluster gained the provider-only allow_deletion field after
// the released version, so the managed cluster shows a benign update on upgrade.
// Step 1 applies that update (allow_deletion=true) so teardown can destroy the
// cluster. Covers all three RequiresReplace dataplane resources; acl is
// load-bearing since it has no ImportState.
//
// Requires REDPANDA_CLIENT_ID + REDPANDA_CLIENT_SECRET; self-provisions the
// cluster, so no KAFKA_CLUSTER_* fixture is needed.
func TestUpgrade_DataplaneClusterAPIURLMigration(t *testing.T) {
	n := dataplaneUpgradeNames{
		rg:      upgrade.RandomName("tfrp-upg-rg"),
		cluster: upgrade.RandomName("tfrp-upg-sl"),
		user:    upgrade.RandomName("tfrp-upg-user"),
		topic:   upgrade.RandomName("tfrp-upg-topic"),
		acl:     upgrade.RandomName("tfrp-upg-acl"),
	}
	upgrade.CreateAndRunMigrationApplyTest(t,
		dataplaneMigrationConfig(n, true),  // released provider stores legacy host:443
		dataplaneMigrationConfig(n, false), // local build re-plans with canonical https://host
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

func dataplaneMigrationConfig(n dataplaneUpgradeNames, legacy bool) string {
	// canonical (local build) sets allow_deletion=true so teardown can destroy
	// the cluster; the released provider has no such field, so legacy omits it.
	apiURL, clusterAllowDeletion := `"https://${local.host}"`, "\n  allow_deletion    = true"
	if legacy {
		apiURL, clusterAllowDeletion = `"${local.host}:443"`, ""
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
