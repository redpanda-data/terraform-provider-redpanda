//go:build live_test

// Copyright 2023 Redpanda Data, Inc.
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

package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

// pinnedProbeConfig pins the registry source the lane example directories
// pin and declares the action, which only the local build can serve; the
// resource group is there so the plan is non-empty and nothing is applied.
func pinnedProbeConfig(name string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
  }
}

provider "redpanda" {}

resource "redpanda_resource_group" "probe" {
  name = %q
}

action "redpanda_byoc_agent_apply" "probe" {
  config {
    cluster_id = "probe"
  }
}
`, name)
}

// TestAcc_LocalBuildServesPinnedSource pins that a config pinning
// redpanda-data/redpanda resolves to the in-process build, not the released
// registry provider. Plan-only: the action schema is fetched at config load,
// so a released provider fails before anything is planned.
func TestAcc_LocalBuildServesPinnedSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{{
			Config:                   pinnedProbeConfig(acc.RandomName(acc.NamePrefix + "probe")),
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			PlanOnly:                 true,
			ExpectNonEmptyPlan:       true,
		}},
	})
}

// upgradeProbeConfig is the resource-group half of pinnedProbeConfig, as an
// inline config for the upgrade entry: the framework writes the
// required_providers block itself for inline configs, released source for
// step 0 and the reattached local build afterwards.
func upgradeProbeConfig(name string) string {
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "probe" {
  name = %q
}
`, name)
}

// TestAcc_UpgradeEntryThenLocalBuild pins the lane flow itself: step 0
// applies with the released registry provider, step 1 re-plans empty with
// the local build, and step 2 plans a config only the local build can parse,
// which holds only when the reattached build, not the provider step 0
// installed into the working directory, serves the pinned address.
func TestAcc_UpgradeEntryThenLocalBuild(t *testing.T) {
	name := acc.RandomName(acc.NamePrefix + "probe")
	steps := acc.UpgradeEntryStepsInline(t, upgradeProbeConfig(name), nil)
	if len(steps) == 0 {
		t.Skip("provider-upgrade entry disabled")
	}
	steps = append(steps, resource.TestStep{
		Config:                   pinnedProbeConfig(name),
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
		PlanOnly:                 true,
		ExpectNonEmptyPlan:       false,
	})
	resource.Test(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps:    steps,
	})
}

// writeProbeDir writes a directory config that pins the registry source and
// declares the action, so only the local build can serve it.
func writeProbeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(pinnedProbeConfig(acc.RandomName(acc.NamePrefix+"probe"))), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestAcc_DirectoryConfigResolvesLocalBuild pins the ConfigDirectory rule:
// plugin-testing injects no required_providers into a directory, so a lane
// directory must pin redpanda-data/redpanda itself to reach the local build.
// The negative cannot be asserted here: an unpinned directory resolves to
// hashicorp/redpanda and fails at terraform init, which the framework
// reports as a hard error rather than a matchable step error.
func TestAcc_DirectoryConfigResolvesLocalBuild(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps: []resource.TestStep{{
			ConfigDirectory:          config.StaticDirectory(writeProbeDir(t)),
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			PlanOnly:                 true,
			ExpectNonEmptyPlan:       true,
		}},
	})
}
