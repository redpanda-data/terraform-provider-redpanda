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
