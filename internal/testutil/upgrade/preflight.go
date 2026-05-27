//go:build upgrade

package upgrade

import (
	"os"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// PreCheck fails the test if the environment is not set up for an upgrade
// run: TF_CLI_CONFIG_FILE must be unset (dev-overrides would silently mask
// Step 0's registry fetch), and the Redpanda client credentials must be
// present so both the released and local providers can authenticate.
func PreCheck(t *testing.T) {
	t.Helper()
	if v := os.Getenv("TF_CLI_CONFIG_FILE"); v != "" {
		t.Fatalf(
			"TF_CLI_CONFIG_FILE=%q is set — upgrade tests must fetch the "+
				"released provider from the public registry; dev_overrides would "+
				"silently mask Step 0 and make the empty-plan assertion in Step 1 "+
				"meaningless. Unset TF_CLI_CONFIG_FILE and re-run.",
			v,
		)
	}
	if os.Getenv(redpanda.ClientIDEnv) == "" {
		t.Fatalf("%s must be set for upgrade tests", redpanda.ClientIDEnv)
	}
	if os.Getenv(redpanda.ClientSecretEnv) == "" {
		t.Fatalf("%s must be set for upgrade tests", redpanda.ClientSecretEnv)
	}
}
