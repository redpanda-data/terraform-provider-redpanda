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

// versionConstraint returns the registry pin for upgrade tests. Defaults to
// "1.9.0". Override with REDPANDA_LAST_VERSION when a new release ships.
func versionConstraint() string {
	if v := os.Getenv("REDPANDA_LAST_VERSION"); v != "" {
		return v
	}
	return "1.9.0"
}
