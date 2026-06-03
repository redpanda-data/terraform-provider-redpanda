//go:build upgrade

// Package upgrade provides the Atlas-style two-step provider-upgrade test
// infrastructure.
package upgrade

import (
	"os"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ExternalProviders returns the Step-0 provider map: pull the released
// redpanda-data/redpanda provider from the public registry at the pinned
// version.
func ExternalProviders() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"redpanda": {
			Source:            "redpanda-data/redpanda",
			VersionConstraint: versionConstraint(),
		},
	}
}

// versionConstraint returns the registry pin for the released ("before")
// provider. Empty by default, which resolves to the latest published release so
// upgrade tests always validate latest..HEAD. Override with REDPANDA_LAST_VERSION
// to pin a specific version (e.g. to reproduce a migration from a
// pre-format-change release).
func versionConstraint() string {
	return os.Getenv("REDPANDA_LAST_VERSION")
}
