//go:build upgrade

package schema_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/upgrade"
)

// TestUpgrade_Schema guards the bearer-primary import ID fix (commit 2d3e2b0).
// Creates a schema with v1.9.0, then imports + verifies with HEAD. If
// HEAD's import handler drops a field that v1.9.0 wrote to state, the
// ImportStateVerify step fails. If HEAD's Read normalizes a value
// differently than v1.9.0's Create did, the auto-appended re-plan step
// fails. Requires KAFKA_CLUSTER_ID.
func TestUpgrade_Schema(t *testing.T) {
	clusterID := upgrade.ClusterFixture(t)
	subject := upgrade.RandomName("tfrp-upgrade-schema")
	upgrade.CreateAndRunTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: upgradeSchemaConfig(clusterID, subject),
			},
			{
				Config:            upgradeSchemaConfig(clusterID, subject),
				ResourceName:      "redpanda_schema.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["redpanda_schema.test"]
					if !ok {
						return "", fmt.Errorf("resource redpanda_schema.test not found in state")
					}
					cid := rs.Primary.Attributes["cluster_id"]
					subj := rs.Primary.Attributes["subject"]
					ver := rs.Primary.Attributes["version"]
					return fmt.Sprintf("%s:%s:%s", cid, subj, ver), nil
				},
				ImportStateVerifyIgnore: []string{"allow_deletion"},
			},
		},
	})
}

func upgradeSchemaConfig(clusterID, subject string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_schema" "test" {
  cluster_id     = %q
  subject        = %q
  schema         = "{\"type\":\"record\",\"name\":\"Test\",\"fields\":[{\"name\":\"id\",\"type\":\"string\"}]}"
  schema_type    = "AVRO"
  allow_deletion = true
}
`, clusterID, subject)
}
