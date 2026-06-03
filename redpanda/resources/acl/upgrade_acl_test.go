//go:build upgrade

package acl_test

import (
	"fmt"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_ACL_ClusterAPIURLMigration guards the cluster_api_url format
// migration across v1.9.0..HEAD for redpanda_acl: the released provider stores
// the legacy host:443 form, and HEAD's schema-version-1 UpgradeState must
// rewrite it to the canonical https://host in place so the format change alone
// does not force replacement. redpanda_acl has no ImportState, so the upgrader
// is its only in-place recovery path — the rm/reimport workaround the
// maintainers posted cannot apply here, making this the load-bearing case.
//
// Requires KAFKA_CLUSTER_API_URL.
func TestUpgrade_ACL_ClusterAPIURLMigration(t *testing.T) {
	legacy, canonical := upgrade.ClusterAPIURLForms(t)
	name := upgrade.RandomName("tfrp-upgrade-acl-url")
	upgrade.CreateAndRunMigrationTest(t,
		upgradeACLConfig(name, legacy),
		upgradeACLConfig(name, canonical),
	)
}

func upgradeACLConfig(principalName, clusterAPIURL string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = %q
  resource_pattern_type = "LITERAL"
  principal             = "User:%s"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = %q
  allow_deletion        = true
}
`, principalName, principalName, clusterAPIURL)
}
