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

package integration

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock/fakes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// emptyPlanChecks is the shared PostApplyPostRefresh assertion for every step
// that ends in a clean state. One allocation keeps the "ExpectEmptyPlan is
// load-bearing" discipline visible across all builders.
var emptyPlanChecks = []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}

// actionStep is the single construction point for steps that combine a
// PreApply ExpectResourceAction with the canonical PostApplyPostRefresh
// ExpectEmptyPlan assertion.
func actionStep(addr, config string, action plancheck.ResourceActionType, stateChecks []statecheck.StateCheck) resource.TestStep {
	return resource.TestStep{
		Config: config,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(addr, action),
			},
			PostApplyPostRefresh: emptyPlanChecks,
		},
		ConfigStateChecks: stateChecks,
	}
}

// CreateStep builds the canonical first step of a CreateAndRefresh scenario.
// PreApply asserts ResourceActionCreate. PostApplyPostRefresh asserts an
// empty plan. The caller supplies the HCL and any state-check assertions.
func CreateStep(addr, config string, stateChecks []statecheck.StateCheck) resource.TestStep {
	return actionStep(addr, config, plancheck.ResourceActionCreate, stateChecks)
}

// NoopReapplyStep builds a re-apply step with identical config. PreApply
// asserts ResourceActionNoop. PostApplyPostRefresh asserts an empty plan.
// Use this to validate that UseStateForUnknown leaves retain their prior
// value across a clean-config re-plan.
func NoopReapplyStep(addr, config string, stateChecks []statecheck.StateCheck) resource.TestStep {
	return actionStep(addr, config, plancheck.ResourceActionNoop, stateChecks)
}

// UpdateLeafStep builds a step that mutates one or more in-place leaves.
// PreApply asserts ResourceActionUpdate. PostApplyPostRefresh asserts an
// empty plan. The caller supplies the post-mutation state-check list; the
// PostApplyPostRefresh ExpectEmptyPlan acts as the cross-check that unmasked
// fields survived the partial update (FieldMask correctness).
func UpdateLeafStep(addr, config string, stateChecks []statecheck.StateCheck) resource.TestStep {
	return actionStep(addr, config, plancheck.ResourceActionUpdate, stateChecks)
}

// RequiresReplaceStep builds a step that mutates a RequiresReplace leaf.
// PreApply asserts ResourceActionDestroyBeforeCreate. PostApplyPostRefresh
// asserts an empty plan.
func RequiresReplaceStep(addr, config string, stateChecks []statecheck.StateCheck) resource.TestStep {
	return actionStep(addr, config, plancheck.ResourceActionDestroyBeforeCreate, stateChecks)
}

// RequiresReplaceIfStep builds a step that exercises a RequiresReplaceIf
// custom predicate. When triggersReplace is true, PreApply asserts
// ResourceActionDestroyBeforeCreate (predicate-positive branch); when false,
// PreApply asserts ResourceActionUpdate (predicate-negative branch). Callers
// must be explicit about which branch they are exercising so the test pins
// the predicate's behavior on both sides.
func RequiresReplaceIfStep(addr, config string, triggersReplace bool, stateChecks []statecheck.StateCheck) resource.TestStep {
	action := plancheck.ResourceActionUpdate
	if triggersReplace {
		action = plancheck.ResourceActionDestroyBeforeCreate
	}
	return actionStep(addr, config, action, stateChecks)
}

// ImportRoundTripStep builds an ImportState step that verifies post-import
// state matches post-Create state. Pass a non-nil idFunc for composite-ID
// resources (e.g. "<cluster_api_url>/<name>"); pass nil to use the resource's
// bearer "id" attribute. verifyIgnore lists attribute path prefixes to skip
// during the verify pass (typically write-only fields that are absent after
// import by design).
func ImportRoundTripStep(addr string, idFunc resource.ImportStateIdFunc, verifyIgnore []string) resource.TestStep {
	return resource.TestStep{
		ResourceName:            addr,
		ImportState:             true,
		ImportStateVerify:       true,
		ImportStateIdFunc:       idFunc,
		ImportStateVerifyIgnore: verifyIgnore,
	}
}

// ErrorPathStep injects a one-shot gRPC error via srv.OverrideOnce for the
// named method, then returns a step that drives that RPC. The provider must
// surface the error as a Terraform diagnostic matching errorPattern; state must
// remain unmodified (the apply fails before any partial write is committed).
func ErrorPathStep(srv *mock.Server, method string, code codes.Code, config, errorPattern string) resource.TestStep {
	srv.OverrideOnce(method, status.Error(code, errorPattern))
	return resource.TestStep{
		Config:      config,
		ExpectError: regexp.MustCompile(errorPattern),
	}
}

// RESTErrorPathStep is the REST analog of ErrorPathStep for the
// SchemaRegistryFake. It injects a one-shot HTTP error via sr.OverrideOnceHTTP
// and returns a step whose Config triggers that route. The step's ExpectError
// is set to a regexp built from errorPattern; the framework matches it against
// the resulting Terraform diagnostic.
//
// path is the EXPANDED literal URL path the client will send, e.g.
// "/subjects/my-subject/versions/3", NOT a mux pattern with placeholders like
// "/subjects/{subject}/versions/{version}". A path that never matches an
// incoming request is flagged at t.Cleanup as an unconsumed override.
//
// body is written verbatim as the response body (typically the Schema Registry
// error JSON shape, e.g. `{"error_code":50001,"message":"..."}`).
func RESTErrorPathStep(sr *fakes.SchemaRegistryFake, method, path string, statusCode int, body, config, errorPattern string) resource.TestStep {
	sr.OverrideOnceHTTP(method, path, statusCode, body)
	return resource.TestStep{
		Config:      config,
		ExpectError: regexp.MustCompile(errorPattern),
	}
}
