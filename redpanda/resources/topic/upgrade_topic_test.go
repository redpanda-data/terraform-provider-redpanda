//go:build upgrade

package topic_test

import (
	"fmt"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_Topic_ClusterAPIURLMigration guards the cluster_api_url format
// migration across v1.9.0..HEAD for redpanda_topic: the released provider stores
// the legacy host:443 form, and HEAD's schema-version-1 UpgradeState must
// rewrite it to the canonical https://host in place so the format change alone
// does not force replacement. Step 1's empty plan is the proof.
//
// Requires KAFKA_CLUSTER_API_URL.
func TestUpgrade_Topic_ClusterAPIURLMigration(t *testing.T) {
	legacy, canonical := upgrade.ClusterAPIURLForms(t)
	name := upgrade.RandomName("tfrp-upgrade-topic-url")
	upgrade.CreateAndRunMigrationTest(t,
		upgradeTopicConfig(name, legacy),
		upgradeTopicConfig(name, canonical),
	)
}

func upgradeTopicConfig(name, clusterAPIURL string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_topic" "test" {
  name               = %q
  partition_count    = 1
  replication_factor = 1
  cluster_api_url    = %q
  allow_deletion     = true
}
`, name, clusterAPIURL)
}
