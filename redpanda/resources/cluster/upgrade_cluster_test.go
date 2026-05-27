//go:build upgrade

package cluster_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_Cluster guards the schemagen structural reorganization across
// v1.9.0..HEAD. A dedicated cluster created by v1.9.0 must produce an
// empty plan when HEAD re-plans against the same state.
func TestUpgrade_Cluster(t *testing.T) {
	name := upgrade.RandomName("tfrp-upgrade-cl")
	upgrade.CreateAndRunTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: upgradeClusterConfig(name),
			},
		},
	})
}

func upgradeClusterConfig(name string) string {
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

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = "tier-1-aws-v2-arm"
  zones             = ["use1-az2"]
  allow_deletion    = true
}
`, name, name, name)
}
