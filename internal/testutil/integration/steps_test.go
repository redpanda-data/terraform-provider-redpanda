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

package integration_test

import (
	"context"
	"reflect"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	testAddr   = "redpanda_resource_group.test"
	testConfig = "resource \"redpanda_resource_group\" \"test\" { name = \"x\" }\n"
)

func sampleStateChecks() []statecheck.StateCheck {
	return []statecheck.StateCheck{
		statecheck.ExpectKnownValue(testAddr, tfjsonpath.New("name"), knownvalue.StringExact("x")),
	}
}

// runPlanCheck invokes a single PlanCheck against a synthetic Plan and
// returns the resulting error, if any. Use this to assert that the
// PreApply check the library inserts is in fact the action we expect.
func runPlanCheck(check plancheck.PlanCheck, plan *tfjson.Plan) error {
	req := plancheck.CheckPlanRequest{Plan: plan}
	resp := plancheck.CheckPlanResponse{}
	check.CheckPlan(context.Background(), req, &resp)
	return resp.Error
}

func planWithAction(addr string, actions tfjson.Actions) *tfjson.Plan {
	return &tfjson.Plan{
		ResourceChanges: []*tfjson.ResourceChange{
			{
				Address: addr,
				Change:  &tfjson.Change{Actions: actions},
			},
		},
	}
}

// assertSingleActionPreApply asserts that step has exactly one PreApply check
// and that the check passes against a plan with the wantAction action but
// fails against a plan with a different (Create-vs-Delete) action.
func assertSingleActionPreApply(t *testing.T, preApply []plancheck.PlanCheck, wantAction, mismatchAction tfjson.Actions) {
	t.Helper()
	if len(preApply) != 1 {
		t.Fatalf("expected exactly 1 PreApply check, got %d", len(preApply))
	}
	check := preApply[0]
	if err := runPlanCheck(check, planWithAction(testAddr, wantAction)); err != nil {
		t.Fatalf("PreApply check rejected the expected action: %v", err)
	}
	if err := runPlanCheck(check, planWithAction(testAddr, mismatchAction)); err == nil {
		t.Fatalf("PreApply check accepted a mismatched action %v", mismatchAction)
	}
}

// assertEmptyPlanPostApply asserts that PostApplyPostRefresh has exactly one
// ExpectEmptyPlan-shaped check (passes on empty plan, fails on non-empty).
func assertEmptyPlanPostApply(t *testing.T, postApply []plancheck.PlanCheck) {
	t.Helper()
	if len(postApply) != 1 {
		t.Fatalf("expected exactly 1 PostApplyPostRefresh check, got %d", len(postApply))
	}
	check := postApply[0]
	emptyPlan := &tfjson.Plan{ResourceChanges: nil}
	if err := runPlanCheck(check, emptyPlan); err != nil {
		t.Fatalf("PostApplyPostRefresh check rejected an empty plan: %v", err)
	}
	nonEmpty := planWithAction(testAddr, tfjson.Actions{tfjson.ActionUpdate})
	if err := runPlanCheck(check, nonEmpty); err == nil {
		t.Fatal("PostApplyPostRefresh check accepted a non-empty plan")
	}
}

func TestCreateStep_Shape(t *testing.T) {
	checks := sampleStateChecks()
	step := integration.CreateStep(testAddr, testConfig, checks)

	if step.Config != testConfig {
		t.Fatalf("step.Config not threaded through: got %q", step.Config)
	}
	if !reflect.DeepEqual(step.ConfigStateChecks, checks) {
		t.Fatal("step.ConfigStateChecks not threaded through")
	}
	assertSingleActionPreApply(t, step.ConfigPlanChecks.PreApply,
		tfjson.Actions{tfjson.ActionCreate},
		tfjson.Actions{tfjson.ActionDelete})
	assertEmptyPlanPostApply(t, step.ConfigPlanChecks.PostApplyPostRefresh)
}

func TestNoopReapplyStep_Shape(t *testing.T) {
	step := integration.NoopReapplyStep(testAddr, testConfig, nil)

	if step.Config != testConfig {
		t.Fatal("step.Config not threaded through")
	}
	if step.ConfigStateChecks != nil {
		t.Fatalf("nil stateChecks must thread through as nil, got %v", step.ConfigStateChecks)
	}
	assertSingleActionPreApply(t, step.ConfigPlanChecks.PreApply,
		tfjson.Actions{tfjson.ActionNoop},
		tfjson.Actions{tfjson.ActionUpdate})
	assertEmptyPlanPostApply(t, step.ConfigPlanChecks.PostApplyPostRefresh)
}

func TestUpdateLeafStep_Shape(t *testing.T) {
	checks := sampleStateChecks()
	step := integration.UpdateLeafStep(testAddr, testConfig, checks)

	if step.Config != testConfig {
		t.Fatal("step.Config not threaded through")
	}
	if !reflect.DeepEqual(step.ConfigStateChecks, checks) {
		t.Fatal("step.ConfigStateChecks not threaded through")
	}
	assertSingleActionPreApply(t, step.ConfigPlanChecks.PreApply,
		tfjson.Actions{tfjson.ActionUpdate},
		tfjson.Actions{tfjson.ActionCreate})
	assertEmptyPlanPostApply(t, step.ConfigPlanChecks.PostApplyPostRefresh)
}

func TestRequiresReplaceStep_Shape(t *testing.T) {
	step := integration.RequiresReplaceStep(testAddr, testConfig, sampleStateChecks())

	assertSingleActionPreApply(t, step.ConfigPlanChecks.PreApply,
		tfjson.Actions{tfjson.ActionDelete, tfjson.ActionCreate},
		tfjson.Actions{tfjson.ActionUpdate})
	assertEmptyPlanPostApply(t, step.ConfigPlanChecks.PostApplyPostRefresh)
}

func TestRequiresReplaceIfStep_PositiveBranch(t *testing.T) {
	step := integration.RequiresReplaceIfStep(testAddr, testConfig, true, sampleStateChecks())

	assertSingleActionPreApply(t, step.ConfigPlanChecks.PreApply,
		tfjson.Actions{tfjson.ActionDelete, tfjson.ActionCreate},
		tfjson.Actions{tfjson.ActionUpdate})
	assertEmptyPlanPostApply(t, step.ConfigPlanChecks.PostApplyPostRefresh)
}

func TestRequiresReplaceIfStep_NegativeBranch(t *testing.T) {
	step := integration.RequiresReplaceIfStep(testAddr, testConfig, false, sampleStateChecks())

	assertSingleActionPreApply(t, step.ConfigPlanChecks.PreApply,
		tfjson.Actions{tfjson.ActionUpdate},
		tfjson.Actions{tfjson.ActionDelete, tfjson.ActionCreate})
	assertEmptyPlanPostApply(t, step.ConfigPlanChecks.PostApplyPostRefresh)
}

func TestImportRoundTripStep_NoIDFunc(t *testing.T) {
	step := integration.ImportRoundTripStep(testAddr, nil, nil)

	if step.ResourceName != testAddr {
		t.Fatalf("ResourceName: got %q want %q", step.ResourceName, testAddr)
	}
	if !step.ImportState {
		t.Fatal("ImportState must be true")
	}
	if !step.ImportStateVerify {
		t.Fatal("ImportStateVerify must be true")
	}
	if step.ImportStateIdFunc != nil {
		t.Fatal("nil idFunc must thread through as nil")
	}
	if step.ImportStateVerifyIgnore != nil {
		t.Fatalf("nil verifyIgnore must thread through as nil, got %v", step.ImportStateVerifyIgnore)
	}
	if step.Config != "" {
		t.Fatalf("Import step must have empty Config, got %q", step.Config)
	}
}

func TestImportRoundTripStep_WithIDFunc(t *testing.T) {
	called := false
	idFunc := func(_ *terraform.State) (string, error) {
		called = true
		return "synthetic-id", nil
	}
	ignore := []string{"password_wo"}
	step := integration.ImportRoundTripStep(testAddr, idFunc, ignore)

	if step.ImportStateIdFunc == nil {
		t.Fatal("ImportStateIdFunc must be non-nil when supplied")
	}
	if _, err := step.ImportStateIdFunc(nil); err != nil {
		t.Fatalf("idFunc returned error: %v", err)
	}
	if !called {
		t.Fatal("supplied idFunc was not invoked")
	}
	if !reflect.DeepEqual(step.ImportStateVerifyIgnore, ignore) {
		t.Fatalf("ImportStateVerifyIgnore not threaded through: got %v", step.ImportStateVerifyIgnore)
	}
}

func TestErrorPathStep_Shape(t *testing.T) {
	srv := mock.New(t)

	const method = "/redpanda.api.controlplane.v1.ResourceGroupService/CreateResourceGroup"
	const pattern = "synthetic boom"

	step := integration.ErrorPathStep(srv, method, codes.Unavailable, testConfig, pattern)

	if step.Config != testConfig {
		t.Fatal("step.Config not threaded through")
	}
	if step.ExpectError == nil {
		t.Fatal("ExpectError must be non-nil")
	}
	if !step.ExpectError.MatchString(pattern) {
		t.Fatalf("ExpectError regexp does not match its own pattern: %q", pattern)
	}
	if len(step.ConfigPlanChecks.PreApply) != 0 {
		t.Fatalf("ErrorPathStep must not attach PreApply checks, got %d", len(step.ConfigPlanChecks.PreApply))
	}
	if len(step.ConfigPlanChecks.PostApplyPostRefresh) != 0 {
		t.Fatalf("ErrorPathStep must not attach PostApplyPostRefresh checks, got %d", len(step.ConfigPlanChecks.PostApplyPostRefresh))
	}

	// Consume the pending override so the safety-net cleanup does not fire a
	// false-positive "never consumed" failure. ErrorPathStep registers the
	// override as a side effect; in a real test the framework would execute the
	// step and trigger the RPC. Here we drive it directly.
	conn, err := grpc.NewClient("passthrough:///bufnet", srv.Dialer()...)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_, _ = cloud.NewControlPlaneClientSet(conn).CreateResourceGroup(context.Background(), "shape-test-consume")
}
