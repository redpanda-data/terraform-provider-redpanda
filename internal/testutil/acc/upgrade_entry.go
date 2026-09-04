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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The provider-upgrade entry folds the former standalone upgrade tier into
// every acceptance test: step 0 applies the test's own first config with the
// RELEASED provider from the registry, step 1 re-plans it with the local build
// and requires an empty plan, the upgrade contract. The test's existing steps
// then run unchanged on the local build (the first of them applies a no-op).
//
// The step-1 assertion is deliberately strict (whole plan empty, plan-only, so
// a non-empty upgrade plan fails loudly instead of being applied). When a
// release legitimately introduces a defaulted attribute that trips it, the
// affected test may temporarily disable the entry via its skip parameter with
// a comment naming the release, removed after the next release ships.

// UpgradeEntryEnabled reports whether the provider-upgrade entry runs.
// Default on; REDPANDA_UPGRADE_ENTRY=off disables it: for local runs using
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
	// The framework rejects ExternalProviders on ConfigDirectory steps
	// ("Providers must only be specified within the terraform configuration
	// files when using TestStep.Config"), so the entry inlines the directory's
	// .tf files; ConfigVariables still applies. Subsequent steps may keep
	// using ConfigDirectory.
	return UpgradeEntryStepsInline(t, inlineConfigFromDir(t, dir), vars, checks...)
}

// guardReleasedProviderSource fails the test when something on this machine
// would replace step 0's registry fetch without a message: a CLI config that
// redirects provider installation, or a redpanda-data/redpanda package in one
// of Terraform's implied filesystem mirror directories (task build:install
// writes there), which Terraform prefers over the registry.
func guardReleasedProviderSource(t testing.TB) {
	t.Helper()
	if err := checkCLIConfig(os.Getenv("TF_CLI_CONFIG_FILE")); err != nil {
		t.Fatalf("provider-upgrade entry: %v. Remove the block, unset TF_CLI_CONFIG_FILE, or set REDPANDA_UPGRADE_ENTRY=off to skip the entry.", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("provider-upgrade entry: resolving the home directory: %v", err)
	}
	if mirror, found := staleImpliedMirror(impliedMirrorDirs(home)); found {
		t.Fatalf("provider-upgrade entry: %s is an implied Terraform filesystem mirror holding redpanda-data/redpanda, which Terraform would use for step 0 instead of the released provider. Move it aside, or set REDPANDA_UPGRADE_ENTRY=off to skip the entry.", mirror)
	}
}

// checkCLIConfig accepts an unset path and a config that only tunes caching;
// it rejects one carrying a provider_installation block (dev_overrides,
// filesystem or network mirrors), which would redirect step 0 away from the
// registry. A path that cannot be read is rejected rather than trusted.
func checkCLIConfig(path string) error {
	if path == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("TF_CLI_CONFIG_FILE=%q cannot be read: %w", path, err)
	}
	for _, block := range []string{"dev_overrides", "provider_installation"} {
		if strings.Contains(string(body), block) {
			return fmt.Errorf("TF_CLI_CONFIG_FILE=%q contains a %s block, which would silently replace the released provider the entry fetches from the registry", path, block)
		}
	}
	return nil
}

// impliedMirrorDirs lists the directories Terraform treats as implied
// filesystem mirrors on this platform, the ones it searches before the
// registry when no provider_installation block is configured.
func impliedMirrorDirs(home string) []string {
	dirs := []string{filepath.Join(home, ".terraform.d", "plugins")}
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs,
			filepath.Join(home, "Library", "Application Support", "io.terraform", "plugins"),
			"/Library/Application Support/io.terraform/plugins")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			dirs = append(dirs,
				filepath.Join(appData, "terraform.d", "plugins"),
				filepath.Join(appData, "HashiCorp", "Terraform", "plugins"))
		}
	default:
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		dirs = append(dirs, filepath.Join(dataHome, "terraform", "plugins"))
		dataDirs := os.Getenv("XDG_DATA_DIRS")
		if dataDirs == "" {
			dataDirs = "/usr/local/share:/usr/share"
		}
		for _, d := range strings.Split(dataDirs, ":") {
			if d != "" {
				dirs = append(dirs, filepath.Join(d, "terraform", "plugins"))
			}
		}
	}
	return dirs
}

// staleImpliedMirror reports the first mirror directory that holds a
// redpanda-data/redpanda package.
func staleImpliedMirror(dirs []string) (string, bool) {
	for _, d := range dirs {
		candidate := filepath.Join(d, "registry.terraform.io", "redpanda-data", "redpanda")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
	}
	return "", false
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
	fixtures := os.DirFS(dir)
	for _, n := range names {
		content, err := fs.ReadFile(fixtures, n)
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
	guardReleasedProviderSource(t)
	alignReleasedProviderCloudEnv(t)

	constraint := os.Getenv("REDPANDA_LAST_VERSION")
	if constraint == "" {
		constraint = "latest (REDPANDA_LAST_VERSION unset)"
	}
	t.Logf("provider-upgrade entry: step 0 uses released redpanda-data/redpanda @ %s", constraint)

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
// the environment and defaults to prod, so pre-env credentials against the prod
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
