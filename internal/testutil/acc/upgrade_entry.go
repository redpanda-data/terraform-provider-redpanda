// Copyright 2026 Redpanda Data, Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package acc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The provider-upgrade entry folds the former standalone upgrade tier into
// every acceptance test: step 0 applies the test's own first config with the
// RELEASED provider from the registry, step 1 re-plans it with the local build
// and requires an empty plan — the upgrade contract. The test's existing steps
// then run unchanged on the local build (the first of them applies a no-op).
//
// The step-1 assertion is deliberately strict (whole plan empty, plan-only, so
// a non-empty upgrade plan fails loudly instead of being applied). When a
// release legitimately introduces a defaulted attribute that trips it, the
// affected test may temporarily disable the entry via its skip parameter with
// a comment naming the release, removed after the next release ships.

// UpgradeEntryEnabled reports whether the provider-upgrade entry runs.
// Default on; REDPANDA_UPGRADE_ENTRY=off disables it — for local runs using
// dev_overrides, for configs the released provider cannot parse yet, and for
// release-validation runs that want the local build's create path exercised
// live instead.
func UpgradeEntryEnabled() bool {
	switch os.Getenv("REDPANDA_UPGRADE_ENTRY") {
	case "off", "0", "false":
		return false
	default:
		return true
	}
}

// UpgradeExternalProviders returns the step-0 provider map: the released
// redpanda-data/redpanda provider from the public registry, pinned by
// REDPANDA_LAST_VERSION (empty resolves to the latest published release, so
// the entry always validates latest → HEAD).
func UpgradeExternalProviders() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"redpanda": {
			Source:            "redpanda-data/redpanda",
			VersionConstraint: os.Getenv("REDPANDA_LAST_VERSION"),
		},
	}
}

// UpgradeEntrySteps returns the two entry steps for a test whose first config
// lives in dir with vars, or nil when the entry is disabled. checks (optional)
// are the test's create-time assertions, run against the released provider's
// apply. Callers prepend the result to their existing steps and must keep
// provider factories per-step (a TestCase-level factory map conflicts with
// step 0's ExternalProviders).
func UpgradeEntrySteps(t testing.TB, dir string, vars map[string]config.Variable, checks ...resource.TestCheckFunc) []resource.TestStep {
	t.Helper()
	if !UpgradeEntryEnabled() {
		return nil
	}
	// dev_overrides would silently mask step 0's registry fetch and make the
	// step-1 empty-plan assertion meaningless.
	if v := os.Getenv("TF_CLI_CONFIG_FILE"); v != "" {
		t.Fatalf(
			"TF_CLI_CONFIG_FILE=%q is set — the provider-upgrade entry fetches the "+
				"released provider from the public registry and dev_overrides would "+
				"silently mask it. Unset TF_CLI_CONFIG_FILE, or set "+
				"REDPANDA_UPGRADE_ENTRY=off to skip the entry.",
			v,
		)
	}
	alignReleasedProviderCloudEnv(t)

	constraint := os.Getenv("REDPANDA_LAST_VERSION")
	if constraint == "" {
		constraint = "latest (REDPANDA_LAST_VERSION unset)"
	}
	t.Logf("provider-upgrade entry: step 0 uses released redpanda-data/redpanda @ %s", constraint)

	// The framework rejects ExternalProviders on ConfigDirectory steps
	// ("Providers must only be specified within the terraform configuration
	// files when using TestStep.Config"), so the entry inlines the directory's
	// .tf files; ConfigVariables still applies. Subsequent steps may keep
	// using ConfigDirectory.
	return UpgradeEntryStepsInline(t, inlineConfigFromDir(t, dir), vars, checks...)
}

// inlineConfigFromDir concatenates a config directory's .tf files into one
// inline Config string, in lexical order.
func inlineConfigFromDir(t testing.TB, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("provider-upgrade entry: reading config dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tf") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		content, err := os.ReadFile(filepath.Join(dir, n)) // #nosec G304 -- dir comes from test fixture constants
		if err != nil {
			t.Fatalf("provider-upgrade entry: reading %s: %v", n, err)
		}
		b.Write(content)
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		t.Fatalf("provider-upgrade entry: no .tf files in %s", dir)
	}
	return b.String()
}

// UpgradeEntryStepsInline is UpgradeEntrySteps for tests that build their
// first config as an inline string instead of a directory.
func UpgradeEntryStepsInline(t testing.TB, cfg string, vars map[string]config.Variable, checks ...resource.TestCheckFunc) []resource.TestStep {
	t.Helper()
	if !UpgradeEntryEnabled() {
		return nil
	}
	if v := os.Getenv("TF_CLI_CONFIG_FILE"); v != "" {
		t.Fatalf(
			"TF_CLI_CONFIG_FILE=%q is set — the provider-upgrade entry fetches the "+
				"released provider from the public registry and dev_overrides would "+
				"silently mask it. Unset TF_CLI_CONFIG_FILE, or set "+
				"REDPANDA_UPGRADE_ENTRY=off to skip the entry.",
			v,
		)
	}
	alignReleasedProviderCloudEnv(t)
	step0 := resource.TestStep{
		Config:            cfg,
		ConfigVariables:   vars,
		ExternalProviders: UpgradeExternalProviders(),
	}
	if len(checks) > 0 {
		step0.Check = resource.ComposeAggregateTestCheckFunc(checks...)
	}
	return []resource.TestStep{
		step0,
		{
			Config:                   cfg,
			ConfigVariables:          vars,
			ProtoV6ProviderFactories: ProtoV6Factories,
			PlanOnly:                 true,
			ExpectNonEmptyPlan:       false,
		},
	}
}

// alignReleasedProviderCloudEnv exports REDPANDA_CLOUD_ENVIRONMENT for step
// 0's released provider binary when unset. The local build receives CloudEnv
// in-process via the provider factories, but the released binary reads only
// the environment and defaults to prod — pre-env credentials against the prod
// auth domain fail with oauth2 invalid_request. Process-wide on purpose: entry
// builders may run from parallel tests, but every caller writes the same
// value.
func alignReleasedProviderCloudEnv(t testing.TB) {
	t.Helper()
	if os.Getenv("REDPANDA_CLOUD_ENVIRONMENT") != "" {
		return
	}
	if err := os.Setenv("REDPANDA_CLOUD_ENVIRONMENT", CloudEnv); err != nil {
		t.Fatalf("provider-upgrade entry: exporting REDPANDA_CLOUD_ENVIRONMENT: %v", err)
	}
	t.Logf("provider-upgrade entry: exported REDPANDA_CLOUD_ENVIRONMENT=%s for the released provider", CloudEnv)
}
