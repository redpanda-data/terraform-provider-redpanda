//go:build upgrade

package serverlesscluster_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_ServerlessCluster guards the cluster_api_url deprecation alias
// across v1.9.0..HEAD. v1.9.0 populates cluster_api_url in state; HEAD
// adds the dataplane_api nested block. HEAD's UseStateForUnknown on
// dataplane_api must prevent a plan diff when upgrading from state that
// has no dataplane_api key.
func TestUpgrade_ServerlessCluster(t *testing.T) {
	name := upgrade.RandomName("tfrp-upgrade-sl")
	rgName := upgrade.RandomName("tfrp-upgrade-sl-rg")
	upgrade.CreateAndRunTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: upgradeServerlessClusterConfig(name, rgName),
			},
		},
	})
}

func upgradeServerlessClusterConfig(name, rgName string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = %q
}

resource "redpanda_serverless_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = "pro-us-east-1"
}
`, rgName, name)
}
