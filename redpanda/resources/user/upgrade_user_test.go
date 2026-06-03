//go:build upgrade

package user_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_User guards the password -> password_wo deprecation alias
// upgrade across v1.9.0..HEAD. v1.9.0 has `password` as a regular Optional
// field; HEAD's schemagen-driven deprecation aliases must accept v1.9.0
// state without a plan diff (the deprecated `password` field stays in state
// and is honored by HEAD until removed in a future major).
//
// Requires KAFKA_CLUSTER_API_URL to be set.
func TestUpgrade_User(t *testing.T) {
	clusterAPIURL := upgrade.ClusterAPIURL(t)
	name := upgrade.RandomName("tfrp-upgrade-user")
	upgrade.CreateAndRunTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: upgradeUserConfig(name, clusterAPIURL),
			},
		},
	})
}

func upgradeUserConfig(name, clusterAPIURL string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_user" "test" {
  name            = %q
  password        = "upgrade-test-pw-123"
  cluster_api_url = %q
  allow_deletion  = true
}
`, name, clusterAPIURL)
}

// TestUpgrade_User_ClusterAPIURLMigration guards the cluster_api_url format
// migration across v1.9.0..HEAD: the released provider stores the legacy
// host:443 form, and HEAD's schema-version-1 UpgradeState must rewrite it to
// the canonical https://host in place so the format change alone does not force
// replacement. Step 1's empty plan is the proof; it also exercises the
// UpgradeResourceState PriorSchema decode against real released-provider state.
//
// Requires KAFKA_CLUSTER_API_URL.
func TestUpgrade_User_ClusterAPIURLMigration(t *testing.T) {
	legacy, canonical := upgrade.ClusterAPIURLForms(t)
	name := upgrade.RandomName("tfrp-upgrade-user-url")
	upgrade.CreateAndRunMigrationTest(t,
		upgradeUserConfig(name, legacy),
		upgradeUserConfig(name, canonical),
	)
}
