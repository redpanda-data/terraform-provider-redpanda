// Copyright 2026 Redpanda Data, Inc.
//
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

package cloudtest

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// ExpectKnownNotUnknown is the inverse of plancheck.ExpectUnknownValue.
// It asserts the planned value at attributePath is NOT Unknown by
// reading rc.Change.AfterUnknown directly, bypassing terraform's
// rendered-diff collapsing. Terraform 1.14+ omits isolated
// null-state+unknown-plan diffs from the human-readable plan; the
// protocol-level AfterUnknown flag remains accurate regardless of CLI
// version, so this plancheck catches plan-modifier bugs that
// ExpectNonEmptyPlan would silently pass.
//
// AfterUnknown encoding (per terraform-json/plan.go): a leaf bool
// `true` means the whole attribute is Unknown; `false` means known
// (including known-null); a nested map of bools means partial. An
// attribute missing from AfterUnknown entirely is fully known.
//
// Use this to lock in UseStateForUnknown-style plan modifier behavior:
// given an attribute the user didn't configure and whose state is
// null, the plan modifier must copy null forward (known-null) rather
// than leave it Unknown.
func ExpectKnownNotUnknown(resourceAddress string, attributePath tfjsonpath.Path) plancheck.PlanCheck {
	return expectKnownNotUnknown{
		resourceAddress: resourceAddress,
		attributePath:   attributePath,
	}
}

type expectKnownNotUnknown struct {
	resourceAddress string
	attributePath   tfjsonpath.Path
}

func (e expectKnownNotUnknown) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address != e.resourceAddress {
			continue
		}
		result, err := tfjsonpath.Traverse(rc.Change.AfterUnknown, e.attributePath)
		if err != nil {
			// Path absent from AfterUnknown ⇒ fully known.
			return
		}
		switch v := result.(type) {
		case bool:
			if v {
				resp.Error = fmt.Errorf("expected %q to be known in plan, but it is unknown", e.attributePath)
			}
		case map[string]any, []any:
			// Nested structure. The attribute itself is known (terraform
			// only emits a nested structure when some leaves are
			// unknown and the rest known). Callers assert nested leaves
			// via separate ExpectKnownNotUnknown calls with deeper paths.
		default:
			resp.Error = fmt.Errorf("unexpected AfterUnknown shape at %q: %T", e.attributePath, result)
		}
		return
	}
	resp.Error = fmt.Errorf("%s - resource not found in plan ResourceChanges", e.resourceAddress)
}
