//go:build upgrade

package network_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_Network is the load-bearing regression guard for the
// id RequiresReplace -> UseStateForUnknown fix on redpanda_network
// across v1.9.0..HEAD.
func TestUpgrade_Network(t *testing.T) {
	name := upgrade.RandomName("tfrp-upgrade-net")
	upgrade.CreateAndRunTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: upgradeNetworkConfig(name),
			},
		},
	})
}

func upgradeNetworkConfig(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = %q
}

resource "redpanda_network" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}
`, name, name)
}
