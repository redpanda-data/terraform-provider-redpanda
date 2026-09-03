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

package cluster

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// rpsqlZonesStatePin is the rpsql.zones plan modifier referenced by the
// generated schema. It pins the prior state value (null included) over an
// unknown planned value (UseStateForUnknown semantics) UNLESS rpsql.enabled
// changes in a way that makes the server re-derive the leaf: a rise with no
// retained zones (the control plane assigns the first cluster zone on a fresh
// enable) or a fall (the control plane clears zones on disable). Both must stay
// "known after apply" for the server's value to land. Steady states keep the
// pin so an unrelated update plan does not churn the leaf.
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
// retained value for the server to keep (the fresh-enable defaulter case), or
// it is falling, which clears zones control-plane side.
func releasePinForServerAssign(planEnabled, stateEnabled types.Bool, stateValue types.List) bool {
	if planEnabled.IsUnknown() {
		return true
	}
	// Disabling replaces the whole server-side spec with a bare disabled one, so
	// zones come back empty; a pinned prior value would contradict that read.
	if stateEnabled.ValueBool() && !planEnabled.ValueBool() {
		return true
	}
	rise := planEnabled.ValueBool() && !stateEnabled.ValueBool()
	noRetained := stateValue.IsNull() || len(stateValue.Elements()) == 0
	return rise && noRetained
}

// rpsqlReplicasStatePin is the rpsql.replicas plan modifier. It derives replicas
// from the sibling rpsql.enabled instead of a static default, because the control
// plane reports replicas 0 while Redpanda SQL is disabled and defaults to 1 on
// enable: a static default of 1 churns every disabled cluster, and no default
// sends replicas 0 on enable and trips the CEL (replicas>=1 when enabled). When
// the config supplies replicas, that value wins. Otherwise: disabled → 0;
// enabling (create-enable or rise) → 1; steady enabled → prior state.
func rpsqlReplicasStatePin() planmodifier.Int32 {
	return pinInt32StateUnlessSiblingRises{sibling: path.Root("rpsql").AtName("enabled")}
}

type pinInt32StateUnlessSiblingRises struct {
	sibling path.Path
}

func (pinInt32StateUnlessSiblingRises) Description(_ context.Context) string {
	return "Derives replicas from the sibling enabled flag: 0 while disabled, 1 on enable, prior state while steady-enabled."
}

func (m pinInt32StateUnlessSiblingRises) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyInt32 implements planmodifier.Int32.
func (m pinInt32StateUnlessSiblingRises) PlanModifyInt32(ctx context.Context, req planmodifier.Int32Request, resp *planmodifier.Int32Response) {
	// Config-supplied replicas (known plan value) wins.
	if !req.PlanValue.IsUnknown() || req.ConfigValue.IsUnknown() {
		return
	}
	var planEnabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, m.sibling, &planEnabled)...)
	if resp.Diagnostics.HasError() || planEnabled.IsNull() || planEnabled.IsUnknown() {
		return
	}
	if !planEnabled.ValueBool() {
		resp.PlanValue = types.Int32Value(0) // disabled: server reports 0
		return
	}
	// Enabled and no config value: pin the prior count only while it was already
	// enabled (steady); on create-enable or a disabled->enabled rise, seed the
	// server default 1 so the payload satisfies the replicas>=1 CEL.
	if !req.State.Raw.IsNull() {
		var stateEnabled types.Bool
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, m.sibling, &stateEnabled)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if stateEnabled.ValueBool() && !req.StateValue.IsNull() {
			resp.PlanValue = req.StateValue
			return
		}
	}
	resp.PlanValue = types.Int32Value(1)
}

// privateLinkStatusPin is the plan modifier for a private-link block's computed
// `status` object. The control plane returns no private-link block at all when
// the block is disabled, so the block's computed children must plan as
// known-null while enabled=false; otherwise an unknown status is carried into
// state and the framework rejects it ("must be known after apply"). While
// enabled, it behaves as UseNonNullStateForUnknown (holds a non-null prior
// status over an unknown plan). Keyed on the sibling enabled via the modifier's
// own path, so one modifier serves aws/gcp/azure.
func privateLinkStatusPin() planmodifier.Object {
	return nullObjectWhenSiblingDisabled{}
}

type nullObjectWhenSiblingDisabled struct{}

func (nullObjectWhenSiblingDisabled) Description(_ context.Context) string {
	return "Plans a known-null value while the sibling enabled flag is false; otherwise holds non-null prior state over an unknown plan."
}

func (m nullObjectWhenSiblingDisabled) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (nullObjectWhenSiblingDisabled) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	var enabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, req.Path.ParentPath().AtName("enabled"), &enabled)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !enabled.IsNull() && !enabled.IsUnknown() && !enabled.ValueBool() {
		resp.PlanValue = types.ObjectNull(req.PlanValue.AttributeTypes(ctx))
		return
	}
	// Enabled (or unknown): UseNonNullStateForUnknown.
	if !req.PlanValue.IsUnknown() || req.State.Raw.IsNull() || req.StateValue.IsNull() {
		return
	}
	resp.PlanValue = req.StateValue
}

// privateLinkListPin is the plan modifier for a private-link block's optional
// computed list children (aws_private_link.supported_regions). Same rationale as
// privateLinkStatusPin: known-null while disabled, else UseStateForUnknown.
func privateLinkListPin() planmodifier.List {
	return nullListWhenSiblingDisabled{}
}

type nullListWhenSiblingDisabled struct{}

func (nullListWhenSiblingDisabled) Description(_ context.Context) string {
	return "Plans a known-null value while the sibling enabled flag is false; otherwise holds prior state over an unknown plan."
}

func (m nullListWhenSiblingDisabled) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (nullListWhenSiblingDisabled) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	var enabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, req.Path.ParentPath().AtName("enabled"), &enabled)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !enabled.IsNull() && !enabled.IsUnknown() && !enabled.ValueBool() {
		resp.PlanValue = types.ListNull(req.PlanValue.ElementType(ctx))
		return
	}
	// Enabled (or unknown): UseStateForUnknown.
	if !req.PlanValue.IsUnknown() || req.State.Raw.IsNull() {
		return
	}
	resp.PlanValue = req.StateValue
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
