//go:build upgrade

package serverlesscluster_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_ServerlessCluster guards the cluster_api_url deprecation alias
// across v1.9.0..HEAD. v1.9.0 populates cluster_api_url in state; HEAD adds the
// dataplane_api nested block whose UseStateForUnknown must keep the upgrade from
// forcing a replacement. allow_deletion is a provider-only field absent from
// v1.9.0, so the local config sets it true (also required for teardown to
// delete the cluster); the resulting upgrade is an in-place update, asserted via
// ResourceActionUpdate rather than a whole-plan-empty check.
func TestUpgrade_ServerlessCluster(t *testing.T) {
	name := upgrade.RandomName("tfrp-upgrade-sl")
	rgName := upgrade.RandomName("tfrp-upgrade-sl-rg")
	upgrade.CreateAndRunMigrationApplyTest(t,
		upgradeServerlessClusterConfig(name, rgName, false), // released v1.9.0: no allow_deletion
		upgradeServerlessClusterConfig(name, rgName, true),  // local build: allow_deletion=true
		[]plancheck.PlanCheck{
			plancheck.ExpectResourceAction("redpanda_serverless_cluster.test", plancheck.ResourceActionUpdate),
		},
	)
}

func upgradeServerlessClusterConfig(name, rgName string, allowDeletion bool) string {
	deletion := ""
	if allowDeletion {
		deletion = "\n  allow_deletion    = true"
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
`, rgName, name, deletion)
}
