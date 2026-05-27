//go:build upgrade

package resourcegroup_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_ResourceGroup is a load-bearing regression guard for the
// id RequiresReplace -> UseStateForUnknown fix on redpanda_resource_group
// across v1.9.0..HEAD.
func TestUpgrade_ResourceGroup(t *testing.T) {
	name := upgrade.RandomName("tfrp-upgrade-rg")
	upgrade.CreateAndRunTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: upgradeResourceGroupConfig(name),
			},
		},
	})
}

func upgradeResourceGroupConfig(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = %q
}
`, name)
}
