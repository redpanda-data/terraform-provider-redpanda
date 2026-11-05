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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/clustermask"
)

var (
	cmrPath   = path.Root("customer_managed_resources")
	rpsqlPath = path.Root("rpsql")
	// operatorAccount gives the Redpanda operator its workload identity, which
	// secrets in cluster properties need; it is not part of Redpanda Connect.
	operatorAccount = cmrPath.AtName("gcp").AtName("redpanda_operator_service_account")
)

// connectPairs are the Redpanda Connect blocks per arm that the control plane
// needs together: its provisioner turns Connect on only when both are set, so
// one alone applies cleanly and leaves Connect off.
var connectPairs = []struct{ arm, a, b, noun string }{
	{"gcp", "redpanda_connect_service_account", "redpanda_connect_api_service_account", "accounts"},
	{"aws", "redpanda_connect_node_group_instance_profile", "redpanda_connect_security_group", "blocks"},
}

// connectBlocks lists the Connect pairs and the operator account: the plain
// Optional blocks under customer_managed_resources, which a configuration
// clears by leaving out.
func connectBlocks() []path.Path {
	var out []path.Path
	for _, pair := range connectPairs {
		out = append(out, cmrPath.AtName(pair.arm).AtName(pair.a), cmrPath.AtName(pair.arm).AtName(pair.b))
	}
	return append(out, operatorAccount)
}

// stateOnlySetting is a setting the state holds and the config leaves out,
// which only an Optional+Computed attribute can be.
type stateOnlySetting struct {
	Path string `json:"path"`
	// Warn marks a setting the control plane may put back on the
	// replacement, so losing it is a warning rather than a refusal.
	Warn bool `json:"warn,omitempty"`
}

// stateOnlyKey marks, in private state, the settings the prior state holds
// and the config leaves out. Core plans a replace twice, and only the second
// pass, which has no prior state and plans the new cluster from config alone,
// can tell it is a create; the planned private state is what crosses from
// the first pass to the second. The apply-time plan of the create has no
// prior private state at all, so a guard that reads the marker may only
// refuse on it, never allow something a markerless plan would refuse.
const stateOnlyKey = "settings_state_only"

func readStateOnlySettings(ctx context.Context, priv privateReader) ([]stateOnlySetting, diag.Diagnostics) {
	raw, diags := priv.GetKey(ctx, stateOnlyKey)
	if diags.HasError() || len(raw) == 0 {
		return nil, diags
	}
	var held []stateOnlySetting
	if err := json.Unmarshal(raw, &held); err != nil {
		diags.AddError("Unable to read recorded cluster settings", err.Error())
	}
	return held, diags
}

// writeStateOnlySettings stores held, or deletes the key when there is
// nothing to hold. It deletes as well as sets because the planned private
// state persists through an update.
func writeStateOnlySettings(ctx context.Context, priv privateWriter, held []stateOnlySetting) diag.Diagnostics {
	if len(held) == 0 {
		return priv.SetKey(ctx, stateOnlyKey, nil)
	}
	raw, err := json.Marshal(held)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Unable to record cluster settings", err.Error())
		return diags
	}
	return priv.SetKey(ctx, stateOnlyKey, raw)
}

// guardCustomerManagedResourcesTransition rejects the customer_managed_resources
// changes the control plane refuses too late. Dropping the block in place is
// an update it accepts: it clears the in-place-updatable leaves and keeps the
// create-only rest, so the apply cannot converge and Connect goes off on the
// way; nested RequiresReplace markers do not fire under a null parent, so
// this is the only gate. A move to another network, or the block appearing on
// the current one, replaces the cluster, and the control plane checks the new
// cluster against its network only when it creates it, after the destroy; so
// the network is read here, and a network that cannot be read fails the plan.
// A network not known until apply cannot be checked.
func guardCustomerManagedResourcesTransition(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, networkHasCMR func(context.Context, string) (bool, error)) {
	var stateCMR, planCMR types.Object
	var stateNet, planNet types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, cmrPath, &stateCMR)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, cmrPath, &planCMR)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("network_id"), &stateNet)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("network_id"), &planNet)...)
	if resp.Diagnostics.HasError() || planCMR.IsUnknown() || planNet.IsUnknown() {
		return
	}
	planned, held, moved := !planCMR.IsNull(), !stateCMR.IsNull(), !planNet.Equal(stateNet)
	switch {
	case !moved && planned == held:
		return
	case !moved && held:
		resp.Diagnostics.AddAttributeError(cmrPath,
			"Customer-managed resources cannot be removed",
			"customer_managed_resources cannot be removed while the cluster stays on its network: the control "+
				"plane keeps the create-only resources and clears only the Redpanda Connect, Redpanda Operator, "+
				"Redpanda SQL and PSC NAT entries, which turns Redpanda Connect off and leaves a block Terraform "+
				"cannot reconcile. Keep the block, or move the cluster to a network without customer-managed "+
				"resources, which replaces it.")
		return
	}
	network := planNet.ValueString()
	has, err := networkHasCMR(ctx, network)
	switch {
	case err != nil:
		resp.Diagnostics.AddAttributeError(path.Root("network_id"),
			"Network could not be read",
			fmt.Sprintf("network %s could not be read, so whether a cluster on it needs customer_managed_resources is "+
				"unknown: %v. The control plane checks that only when it creates the replacement, after the current "+
				"cluster is destroyed.", network, err))
	case has && !planned:
		resp.Diagnostics.AddAttributeError(cmrPath,
			"Customer-managed resources required by the network",
			fmt.Sprintf("network %s has customer-managed resources, so a cluster on it needs customer_managed_resources "+
				"too. The control plane would refuse the new cluster only after the current one is destroyed.", network))
	case !has && planned:
		resp.Diagnostics.AddAttributeError(cmrPath,
			"Customer-managed resources refused by the network",
			fmt.Sprintf("network %s has no customer-managed resources, so a cluster on it cannot have "+
				"customer_managed_resources. The control plane would refuse the new cluster only after the current "+
				"one is destroyed.", network))
	}
}

// rpsqlCMRLeaves are the rpsql_* entries of the update mask table, keyed by
// arm and leaf attribute, so a leaf the provider can send is a leaf the guard
// checks.
var rpsqlCMRLeaves = func() []struct{ arm, leaf string } {
	var out []struct{ arm, leaf string }
	for _, p := range clustermask.CMRUpdatableLeafPaths() {
		if seg := strings.Split(p, "."); len(seg) >= 3 && strings.HasPrefix(seg[2], "rpsql_") {
			out = append(out, struct{ arm, leaf string }{seg[1], seg[2]})
		}
	}
	return out
}()

// guardRpsqlCMRWhileDisabled rejects rpsql_* customer-managed resources in a
// plan whose Redpanda SQL state is off. The validator on each leaf sees config
// only and defers when the rpsql block is omitted; the planned block here is
// the prior state on update and the server default (off) on create, which is
// what the control plane acts on.
func guardRpsqlCMRWhileDisabled(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var cfgRpsql, rpsql, cmr types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, rpsqlPath, &cfgRpsql)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, rpsqlPath, &rpsql)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, cmrPath, &cmr)...)
	create := req.State.Raw.IsNull()
	if resp.Diagnostics.HasError() || cfgRpsql.IsUnknown() || cmr.IsNull() || cmr.IsUnknown() || !rpsqlPlannedOff(rpsql, create) {
		return
	}
	for _, l := range rpsqlCMRLeaves {
		arm, ok := cmr.Attributes()[l.arm].(types.Object)
		if !ok || arm.IsNull() || arm.IsUnknown() {
			continue
		}
		v := arm.Attributes()[l.leaf]
		if v == nil || v.IsNull() || v.IsUnknown() {
			continue
		}
		if s, isString := v.(types.String); isString && s.ValueString() == "" {
			continue
		}
		leaf := cmrPath.AtName(l.arm).AtName(l.leaf)
		state := "The configuration leaves out the rpsql block, so the cluster keeps its current setting, which is off, and the control plane clears this value while Redpanda SQL is off."
		if create {
			state = "The configuration leaves out the rpsql block, so the cluster is created with Redpanda SQL off, and the control plane drops this value."
		}
		resp.Diagnostics.AddAttributeError(leaf, "Redpanda SQL resources need Redpanda SQL enabled",
			fmt.Sprintf("%q needs Redpanda SQL enabled. %s Add rpsql = { enabled = true }, or remove the value.", leaf.String(), state))
	}
}

// rpsqlPlannedOff reports whether the planned rpsql block resolves to off. An
// unknown planned block whose config is omitted is the server default on
// create, off; on update the caller has already deferred on unknown config,
// and the state value is copied into the plan.
func rpsqlPlannedOff(rpsql types.Object, create bool) bool {
	if rpsql.IsUnknown() {
		return create
	}
	if rpsql.IsNull() {
		return true
	}
	enabled, ok := rpsql.Attributes()["enabled"].(types.Bool)
	if !ok || enabled.IsUnknown() {
		return false
	}
	return !enabled.ValueBool()
}

// guardConnectPairing judges a Connect pair on the plan. On an update, a
// half the config leaves out is cleared: leaving out both turns Connect off,
// which the plan says in a warning; leaving out one while the cluster holds
// it would clear it and strand the other, so that is an error. A lone block
// the config keeps unchanged is a warning, not an error: it is a state the
// control plane accepted, and a lone half never turned Connect on. A create,
// plain or the create pass of a replace, has no state to compare with and
// errors on any lone block in config; the apply-time plan of a replace's
// create would do the same after the destroy, so the plan-time one may not be
// more lenient.
func guardConnectPairing(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	create := req.State.Raw.IsNull()
	for _, pair := range connectPairs {
		aPath, bPath := cmrPath.AtName(pair.arm).AtName(pair.a), cmrPath.AtName(pair.arm).AtName(pair.b)
		a := plannedBlock(ctx, req, aPath, resp)
		b := plannedBlock(ctx, req, bPath, resp)
		if resp.Diagnostics.HasError() {
			continue
		}
		if a.sent == b.sent {
			if !create && a.known && b.known && !a.sent && a.held && b.held {
				resp.Diagnostics.AddAttributeWarning(aPath, "Redpanda Connect turns off",
					fmt.Sprintf("The configuration leaves out %s and %s, which the cluster holds. Applying clears both and turns "+
						"Redpanda Connect off. Declare both to keep Connect on.", pair.a, pair.b))
			}
			continue
		}
		missing, kept := aPath, pair.b
		missingBlock, keptBlock, missingName := a, b, pair.a
		if a.sent {
			missing, kept = bPath, pair.a
			missingBlock, keptBlock, missingName = b, a, pair.b
		}
		if !missingBlock.known {
			continue
		}
		detail := fmt.Sprintf("%s and %s must be set together: the control plane turns Redpanda Connect on only when "+
			"both are present, so one alone applies cleanly and leaves Connect off.", pair.a, pair.b)
		switch {
		case create:
			resp.Diagnostics.AddAttributeError(missing, "Redpanda Connect needs both "+pair.noun, detail+
				" Set both "+pair.noun+", or leave the lone one out of the configuration, which creates the cluster "+
				"with Connect off. If this plan replaces a cluster that has both, declare both, or the replacement "+
				"comes up with Connect off.")
		case missingBlock.held:
			resp.Diagnostics.AddAttributeError(missing, "Redpanda Connect needs both "+pair.noun, detail+
				fmt.Sprintf(" The configuration leaves out %s, which the cluster holds, so applying would clear it and "+
					"leave %s behind. Remove both %s to turn Redpanda Connect off, or keep both.", missingName, kept, pair.noun))
		case !keptBlock.known:
			continue
		case !a.changed && !b.changed:
			resp.Diagnostics.AddAttributeWarning(missing, "Redpanda Connect needs both "+pair.noun, detail)
		default:
			resp.Diagnostics.AddAttributeError(missing, "Redpanda Connect needs both "+pair.noun, detail)
		}
	}
}

// guardOperatorAccountRemoved warns on an update whose configuration leaves
// the operator account out while the cluster holds it: applying clears it,
// and the cluster loses the identity secrets in cluster properties need.
func guardOperatorAccountRemoved(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() {
		return
	}
	b := plannedBlock(ctx, req, operatorAccount, resp)
	if resp.Diagnostics.HasError() || !b.known || b.sent || !b.held {
		return
	}
	resp.Diagnostics.AddAttributeWarning(operatorAccount, "Redpanda operator account removed",
		"The configuration leaves out redpanda_operator_service_account, which the cluster holds. Applying clears it, "+
			"and secrets in cluster properties lose the operator identity. Declare it to keep it.")
}

// plannedConnectBlock is what the plan does with one Connect block.
type plannedConnectBlock struct {
	sent       bool // the plan carries a non-empty value
	configured bool // the config names it
	changed    bool // the config names it with a value the state does not hold
	held       bool // the state carries a non-empty value, which a plan without one clears
	known      bool // changed is decidable now; false while its config is not known yet
}

// plannedBlock reports what the plan does with the block at p. A block the
// config leaves null plans null, so it is not sent. An empty value, the
// shape a leaf-only clear leaves on the control plane, is neither sent nor
// held. On the update pass a configured block with a leaf not known yet is
// sent, but whether it changes is left to the apply-time plan; a block whose
// config is not known at all is undecided throughout.
func plannedBlock(ctx context.Context, req resource.ModifyPlanRequest, p path.Path, resp *resource.ModifyPlanResponse) plannedConnectBlock {
	var cfg, plan, state types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, p, &cfg)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, p, &plan)...)
	update := !req.State.Raw.IsNull()
	if update {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, p, &state)...)
	}
	held := update && !state.IsNull() && !state.IsUnknown() && !emptyBlock(state)
	switch {
	case cfg.IsUnknown() || configUnknownAbove(ctx, req, p, resp):
		return plannedConnectBlock{}
	case !cfg.IsNull():
		if update && !fullyKnown(ctx, cfg) {
			return plannedConnectBlock{sent: true, configured: true, held: held}
		}
		return plannedConnectBlock{sent: true, configured: true, changed: !cfg.Equal(state), held: held, known: true}
	default:
		return plannedConnectBlock{sent: !plan.IsNull() && !plan.IsUnknown() && !emptyBlock(plan), held: held, known: true}
	}
}

// configUnknownAbove reports whether an ancestor of p is not known yet in
// config, in which case the block under it reads as null rather than unknown.
func configUnknownAbove(ctx context.Context, req resource.ModifyPlanRequest, p path.Path, resp *resource.ModifyPlanResponse) bool {
	for parent := p.ParentPath(); len(parent.Steps()) > 0; parent = parent.ParentPath() {
		var o types.Object
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, parent, &o)...)
		if resp.Diagnostics.HasError() || o.IsUnknown() {
			return true
		}
	}
	return false
}

func fullyKnown(ctx context.Context, o types.Object) bool {
	v, err := o.ToTerraformValue(ctx)
	return err == nil && v.IsFullyKnown()
}

// emptyBlock reports a known block whose every leaf is null or the empty
// string. A leaf-only clear on the control plane leaves such a block behind,
// and it carries nothing.
func emptyBlock(o types.Object) bool {
	if o.IsNull() || o.IsUnknown() {
		return false
	}
	for _, v := range o.Attributes() {
		if v.IsNull() {
			continue
		}
		s, ok := v.(types.String)
		if !ok || s.IsUnknown() || s.ValueString() != "" {
			return false
		}
	}
	return true
}

// stateOnlySettings lists an enabled rpsql block that state holds and the
// config leaves out. The block is Optional+Computed, so a config without it
// keeps it on an update and loses it on a replace. An enabled rpsql block on
// a cluster without customer-managed resources may be the control plane's
// own doing, as on an Agentic Data Plane cluster, where it enables Redpanda
// SQL again on the replacement and rejects a declared block; that one is a
// warning.
func stateOnlySettings(ctx context.Context, state tfsdk.State, cfg tfsdk.Config) ([]stateOnlySetting, diag.Diagnostics) {
	var diags diag.Diagnostics
	var stRpsql, cfgRpsql, stCMR types.Object
	diags.Append(state.GetAttribute(ctx, rpsqlPath, &stRpsql)...)
	diags.Append(cfg.GetAttribute(ctx, rpsqlPath, &cfgRpsql)...)
	diags.Append(state.GetAttribute(ctx, cmrPath, &stCMR)...)
	if diags.HasError() || !cfgRpsql.IsNull() || stRpsql.IsNull() || stRpsql.IsUnknown() {
		return nil, diags
	}
	if enabled, ok := stRpsql.Attributes()["enabled"].(types.Bool); ok && enabled.ValueBool() {
		return []stateOnlySetting{{Path: rpsqlPath.String(), Warn: stCMR.IsNull()}}, diags
	}
	return nil, diags
}

// recordStateOnlySettings runs on every plan with a prior state and records,
// or clears, the settings stateOnlySettings finds.
func recordStateOnlySettings(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	held, diags := stateOnlySettings(ctx, req.State, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(writeStateOnlySettings(ctx, resp.Private, held)...)
}

// guardReplaceWithStateOnlySettings runs on the create pass of a plan. A plain
// create has no prior private state and passes. A tainted cluster is planned
// without its private state, so its replace is not covered: the replacement
// of a cluster whose enabled rpsql block the config leaves out comes up with
// Redpanda SQL off, without a word.
func guardReplaceWithStateOnlySettings(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	refuseReplaceWithStateOnlySettings(ctx, req.Private, resp)
}

// refuseReplaceWithStateOnlySettings rejects a replace while settings live
// only in state: the new cluster is planned from config alone, so those
// settings would be left off the create. A setting the control plane may put
// back on the replacement is a warning.
func refuseReplaceWithStateOnlySettings(ctx context.Context, priv privateReader, resp *resource.ModifyPlanResponse) {
	held, d := readStateOnlySettings(ctx, priv)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, s := range held {
		if s.Warn {
			resp.Diagnostics.AddWarning(
				"Cluster settings not in the configuration",
				fmt.Sprintf("%s is set on the cluster but not in the configuration. Replacing the cluster creates "+
					"the new one from the configuration alone, so it comes up with %s unless the control plane turns "+
					"it on itself, as it does on an Agentic Data Plane cluster. Declare it in the configuration to "+
					"keep it on any other cluster; on an Agentic Data Plane cluster leave it out, as the control "+
					"plane rejects it there.", s.Path, lostOnReplace(s.Path)))
			continue
		}
		resp.Diagnostics.AddError(
			"Cluster settings not in the configuration",
			fmt.Sprintf("%s is set on the cluster but not in the configuration. Replacing the cluster creates "+
				"the new one from the configuration alone, so it would come up with %s. Declare it in the "+
				"configuration before replacing the cluster.", s.Path, lostOnReplace(s.Path)))
	}
}

// lostOnReplace names what a replacement planned without the setting at p
// comes up without.
func lostOnReplace(p string) string {
	if p == rpsqlPath.String() {
		return "Redpanda SQL off"
	}
	return "the setting off"
}

// planGuard is one ModifyPlan check; runPlanGuards stops at the first that
// reports an error.
type planGuard func(context.Context, resource.ModifyPlanRequest, *resource.ModifyPlanResponse)

func runPlanGuards(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, guards ...planGuard) bool {
	for _, guard := range guards {
		guard(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			return false
		}
	}
	return true
}
