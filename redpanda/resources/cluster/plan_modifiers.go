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

package cluster

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// rpsqlZonesStatePin is the rpsql.zones plan modifier referenced by the
// generated schema. It pins the prior state value (null included) over an
// unknown planned value — UseStateForUnknown semantics — UNLESS rpsql.enabled
// is rising in this plan with no retained zones: the control plane assigns
// the first cluster zone on a fresh enable, so the leaf must stay "known
// after apply" for that value to land. Every other transition keeps the pin:
// the leaf-expanded update mask always carries rpsql.zones, and sending empty
// zones on disable or re-enable would let the defaulter pick a zone that
// trips the control plane's immutability check.
func rpsqlZonesStatePin() planmodifier.List {
	return pinStateUnlessSiblingRises{sibling: path.Root("rpsql").AtName("enabled")}
}

type pinStateUnlessSiblingRises struct {
	sibling path.Path
}

func (pinStateUnlessSiblingRises) Description(_ context.Context) string {
	return "Pins the prior state value unless the sibling enabled flag turns on, re-deriving this attribute server-side."
}

func (m pinStateUnlessSiblingRises) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyList implements planmodifier.List.
func (m pinStateUnlessSiblingRises) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.State.Raw.IsNull() {
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}
	var planEnabled, stateEnabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, m.sibling, &planEnabled)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, m.sibling, &stateEnabled)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if releasePinForServerAssign(planEnabled, stateEnabled, req.StateValue) {
		return
	}
	resp.PlanValue = req.StateValue
}

// releasePinForServerAssign reports whether the planned value should stay
// unknown: the sibling enabled flag is rising (or unknown) and there is no
// retained value for the server to keep — the fresh-enable defaulter case.
func releasePinForServerAssign(planEnabled, stateEnabled types.Bool, stateValue types.List) bool {
	if planEnabled.IsUnknown() {
		return true
	}
	rise := planEnabled.ValueBool() && !stateEnabled.ValueBool()
	noRetained := stateValue.IsNull() || len(stateValue.Elements()) == 0
	return rise && noRetained
}

// gcpGatewayStatePin is the gcp_global_access_api_gateway_enabled plan modifier
// referenced by the generated schema. The status field is server-reported and
// coupled to the gcp_enable_global_access_api_gateway intent input: it pins the
// prior state value (UseStateForUnknown semantics) UNLESS that input differs
// between plan and state, in which case the value is re-derived server-side and
// must stay "known after apply". Plain UseStateForUnknown would pin the stale
// value and trip an inconsistent-result error when the input is toggled.
func gcpGatewayStatePin() planmodifier.Bool {
	return pinBoolStateUnlessSiblingDiffers{sibling: path.Root("gcp_enable_global_access_api_gateway")}
}

type pinBoolStateUnlessSiblingDiffers struct {
	sibling path.Path
}

func (pinBoolStateUnlessSiblingDiffers) Description(_ context.Context) string {
	return "Pins the prior state value unless the sibling input differs, re-deriving this attribute server-side."
}

func (m pinBoolStateUnlessSiblingDiffers) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyBool implements planmodifier.Bool.
func (m pinBoolStateUnlessSiblingDiffers) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.State.Raw.IsNull() {
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	var planSib, stateSib types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, m.sibling, &planSib)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, m.sibling, &stateSib)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if planSib.IsUnknown() || !planSib.Equal(stateSib) {
		return
	}
	resp.PlanValue = req.StateValue
}

// rpsqlStringStatePin is the rpsql.url / rpsql.version plan modifier referenced
// by the generated schema. Same fresh-enable release as rpsqlZonesStatePin: the
// control plane derives these server-side on enable, so the leaf must stay
// "known after apply" across the rise. Every other plan pins prior state, so an
// enabled cluster reaches an empty steady-state plan instead of churning the
// computed value to unknown on every refresh.
func rpsqlStringStatePin() planmodifier.String {
	return pinStringStateUnlessSiblingRises{sibling: path.Root("rpsql").AtName("enabled")}
}

type pinStringStateUnlessSiblingRises struct {
	sibling path.Path
}

func (pinStringStateUnlessSiblingRises) Description(_ context.Context) string {
	return "Pins the prior state value unless the sibling enabled flag turns on, re-deriving this attribute server-side."
}

func (m pinStringStateUnlessSiblingRises) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyString implements planmodifier.String.
func (m pinStringStateUnlessSiblingRises) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() {
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	var planEnabled, stateEnabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, m.sibling, &planEnabled)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, m.sibling, &stateEnabled)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if releaseStringPinForServerAssign(planEnabled, stateEnabled, req.StateValue) {
		return
	}
	resp.PlanValue = req.StateValue
}

// releaseStringPinForServerAssign decides the rpsql.url / rpsql.version release.
// Unlike zones (immutable once set), the control plane re-derives these on
// enable and clears them on disable, so the pin releases on any enabled change.
// A null prior also releases: the server returns a concrete "" for a disabled
// block, which a pinned null would contradict (inconsistent-result). A retained
// "" on a steady disabled block pins, so a disabled cluster reaches an empty
// steady-state plan too.
func releaseStringPinForServerAssign(planEnabled, stateEnabled types.Bool, stateValue types.String) bool {
	if planEnabled.IsUnknown() {
		return true
	}
	if stateValue.IsNull() {
		return true
	}
	return planEnabled.ValueBool() != stateEnabled.ValueBool()
}
