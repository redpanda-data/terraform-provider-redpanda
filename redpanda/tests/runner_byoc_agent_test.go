//go:build live_test

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

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

// byocAgentApplyOverlay is the action plus a terraform_data resource whose
// creation triggers it, appended to a lane's config directory.
const byocAgentApplyOverlay = `
action "redpanda_byoc_agent_apply" "agent" {
  config {
    cluster_id = redpanda_cluster.test.id
  }
}

resource "terraform_data" "byoc_agent_apply" {
  input = "1"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.redpanda_byoc_agent_apply.agent]
    }
  }
}
`

// byocAgentApplyStep re-runs the byoc plugin's apply on the READY cluster the
// lane created, through the redpanda_byoc_agent_apply action, and proves the
// cluster still plans empty afterwards. The lane's config directory is copied
// into a temp dir and the action appended there rather than edited in place:
// the released provider the upgrade entry runs cannot parse an action block,
// and the next step's return to the original directory drops the trigger
// resource again without touching the plugin.
func byocAgentApplyStep(t *testing.T, testFile string, vars map[string]config.Variable) resource.TestStep {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(testFile)
	if err != nil {
		t.Fatalf("byoc agent apply step: read %s: %v", testFile, err)
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(testFile, e.Name()))
		if err != nil {
			t.Fatalf("byoc agent apply step: read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o600); err != nil {
			t.Fatalf("byoc agent apply step: write %s: %v", e.Name(), err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "byoc_agent_apply.tf"), []byte(byocAgentApplyOverlay), 0o600); err != nil {
		t.Fatalf("byoc agent apply step: write overlay: %v", err)
	}
	return resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(dir),
		ConfigVariables:          vars,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
		Check:                    resource.TestCheckResourceAttr(acc.ClusterResourceName, "state", "STATE_READY"),
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}
