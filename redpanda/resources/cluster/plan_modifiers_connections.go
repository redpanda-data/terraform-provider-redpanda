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
	"fmt"
	"maps"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
)

// connectionEndpointFromState is the connections[*].endpoint plan modifier
// referenced by the generated schema. Endpoint is server-owned, so a config-set
// connections list plans it unknown every time; that would churn "(known after
// apply)" on every plan. The modifier restores the state endpoint of the
// element with the SAME (type, auth.mode) identity, not the same index: the
// list may be reordered, and index-based UseStateForUnknown would pin a public
// endpoint onto a connection the user just flipped to private. An element with
// no identity match in state (new connection, auth switch) stays unknown so the
// server value lands.
func connectionEndpointFromState() planmodifier.String {
	return connectionEndpointStatePin{}
}

type connectionEndpointStatePin struct{}

func (connectionEndpointStatePin) Description(_ context.Context) string {
	return "Restores the prior endpoint of the connection with the same type and auth mode; a connection with no prior match stays unknown."
}

func (m connectionEndpointStatePin) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (connectionEndpointStatePin) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.PlanValue.IsUnknown() {
		return
	}

	elemPath := req.Path.ParentPath()
	var connType, authMode types.String
	if diags := req.Plan.GetAttribute(ctx, elemPath.AtName("type"), &connType); diags.HasError() {
		return
	}
	if diags := req.Plan.GetAttribute(ctx, elemPath.AtName("auth").AtName("mode"), &authMode); diags.HasError() {
		return
	}
	if connType.IsNull() || connType.IsUnknown() || authMode.IsNull() || authMode.IsUnknown() {
		return
	}

	var stateList types.List
	if diags := req.State.GetAttribute(ctx, elemPath.ParentPath(), &stateList); diags.HasError() {
		return
	}
	if stateList.IsNull() || stateList.IsUnknown() {
		return
	}
	for _, el := range stateList.Elements() {
		t, mode, endpoint, ok := stateConnectionParts(el)
		if ok && t == connType.ValueString() && mode == authMode.ValueString() {
			resp.PlanValue = endpoint
			return
		}
	}
}

// stateConnectionParts pulls type, auth.mode, and endpoint out of one state
// connections element. ok is false for malformed or unknown-bearing elements.
func stateConnectionParts(el attr.Value) (connType, authMode string, endpoint types.String, ok bool) {
	obj, isObj := el.(types.Object)
	if !isObj || obj.IsNull() || obj.IsUnknown() {
		return "", "", types.String{}, false
	}
	attrs := obj.Attributes()
	t, isStr := attrs["type"].(types.String)
	if !isStr || t.IsNull() || t.IsUnknown() {
		return "", "", types.String{}, false
	}
	auth, isAuthObj := attrs["auth"].(types.Object)
	if !isAuthObj || auth.IsNull() || auth.IsUnknown() {
		return "", "", types.String{}, false
	}
	mode, isModeStr := auth.Attributes()["mode"].(types.String)
	if !isModeStr || mode.IsNull() || mode.IsUnknown() {
		return "", "", types.String{}, false
	}
	ep, isEpStr := attrs["endpoint"].(types.String)
	if !isEpStr || ep.IsNull() || ep.IsUnknown() {
		return "", "", types.String{}, false
	}
	return t.ValueString(), mode.ValueString(), ep, true
}

// markConnectionsUnknownOnLegacyListenerChange marks a service's connections
// list unknown when its legacy sasl/mtls config changes and the user does not
// manage connections for it: the read projection re-derives from the listeners
// (an mtls enable adds an mTLS entry), so the carried prior echo cannot
// survive the apply. Services with connections in CONFIG are untouched: their
// plan is user intent, and sasl coexistence is rejected at validate anyway.
func markConnectionsUnknownOnLegacyListenerChange(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	for _, svc := range []struct {
		name      string
		elemTypes map[string]attr.Type
		hasSASL   bool
	}{
		{"kafka_api", clustermodel.KafkaAPIConnectionsAttrTypes(), true},
		{"http_proxy", clustermodel.HTTPProxyConnectionsAttrTypes(), true},
		{"schema_registry", clustermodel.SchemaRegistryConnectionsAttrTypes(), false},
	} {
		connsPath := path.Root(svc.name).AtName("connections")
		var cfgConns types.List
		if d := req.Config.GetAttribute(ctx, connsPath, &cfgConns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if !cfgConns.IsNull() {
			continue
		}

		changed := false
		subAttrs := []string{"mtls"}
		if svc.hasSASL {
			subAttrs = append(subAttrs, "sasl")
		}
		for _, sub := range subAttrs {
			var planV, stateV types.Object
			if d := req.Plan.GetAttribute(ctx, path.Root(svc.name).AtName(sub), &planV); d.HasError() {
				resp.Diagnostics.Append(d...)
				return
			}
			if d := req.State.GetAttribute(ctx, path.Root(svc.name).AtName(sub), &stateV); d.HasError() {
				resp.Diagnostics.Append(d...)
				return
			}
			if !planV.Equal(stateV) {
				changed = true
			}
		}
		if changed {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, connsPath,
				types.ListUnknown(types.ObjectType{AttrTypes: svc.elemTypes}))...)
		}
	}
}

// connectionsManagedKey marks, in framework private state, a cluster whose
// listeners are managed through config-set connections. Once set it never
// clears: the control plane has no dual->legacy path. Imported dual clusters
// lack the marker until their first connections-managed plan stamps it, so
// the guard below degrades to today's silent behavior in that window rather
// than ever misfiring on a legacy cluster.
const connectionsManagedKey = "connections_managed"

// configManagesConnections reports whether any service sets connections in
// the given config.
func configManagesConnections(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) bool {
	managed := false
	for _, svc := range []string{"kafka_api", "http_proxy", "schema_registry"} {
		var conns types.List
		d := cfg.GetAttribute(ctx, path.Root(svc).AtName("connections"), &conns)
		diags.Append(d...)
		if d.HasError() {
			return false
		}
		if !conns.IsNull() {
			managed = true
		}
	}
	return managed
}

// guardConnectionsManaged rejects the dual->legacy attempt and maintains the
// connections-managed marker. A marked cluster whose config drops connections
// and sets connection_type would otherwise plan as a silent no-op: the
// conflict validator sees no connections, connection_type matches the
// projected state, and stripEchoedConnections empties the payload: the user's
// intent is discarded without a word. The bare removal (neither field) is
// rejected by ValidateConfig's topology gate instead.
func guardConnectionsManaged(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	cfgManaged := configManagesConnections(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfgManaged {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, connectionsManagedKey, []byte(`true`))...)
		return
	}
	marker, d := req.Private.GetKey(ctx, connectionsManagedKey)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() || string(marker) != "true" {
		return
	}
	var connType types.String
	if d := req.Config.GetAttribute(ctx, path.Root("connection_type"), &connType); d.HasError() {
		resp.Diagnostics.Append(d...)
		return
	}
	if !connType.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("connection_type"),
			"Cluster Managed Through Connections",
			"this cluster's listeners are managed through connections; returning to connection_type networking is not supported — remove connection_type and restore the connections blocks on kafka_api, http_proxy, and schema_registry")
	}
}

// guardLegacyMTLSRemoval rejects dropping a service's mtls block while state
// has mTLS enabled and the service is not connections-managed. The block is
// optional+computed with UseStateForUnknown, so its removal plans as no change
// and the control plane treats a nil mtls as no change too: mTLS would stay on
// with no signal. An explicit enabled = false is the disable path.
func guardLegacyMTLSRemoval(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	for _, svc := range []string{"kafka_api", "http_proxy", "schema_registry"} {
		var cfgMTLS, stateMTLS types.Object
		var cfgConns types.List
		if d := req.Config.GetAttribute(ctx, path.Root(svc).AtName("mtls"), &cfgMTLS); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if d := req.Config.GetAttribute(ctx, path.Root(svc).AtName("connections"), &cfgConns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if d := req.State.GetAttribute(ctx, path.Root(svc).AtName("mtls"), &stateMTLS); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if !cfgMTLS.IsNull() || !cfgConns.IsNull() || stateMTLS.IsNull() || stateMTLS.IsUnknown() {
			continue
		}
		enabled, ok := stateMTLS.Attributes()["enabled"].(types.Bool)
		if !ok || !enabled.ValueBool() {
			continue
		}
		resp.Diagnostics.AddAttributeError(path.Root(svc).AtName("mtls"),
			"Ambiguous mTLS Removal",
			fmt.Sprintf("%s.mtls was removed from the configuration while mTLS is enabled on the cluster; removing the block does not disable mTLS. Set %s.mtls.enabled = false to disable it, or restore the block to keep it.", svc, svc))
	}
}

// guardPrivateOnlyGainsPublic rejects the one topology transition the control
// plane cannot perform in place: adding public listeners to a cluster whose
// stored topology is private-only (its network was provisioned without public
// infrastructure, and the rejection otherwise surfaces only at apply).
// ValidateConfig cannot see stored state, so the rule lives here. Deferred
// when config identities are not yet known.
func guardPrivateOnlyGainsPublic(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	cfgManaged, cfgKnown, cfgHasPublic := false, true, false
	stateHasAny, stateHasPublic := false, false
	for _, svc := range []string{"kafka_api", "http_proxy", "schema_registry"} {
		connsPath := path.Root(svc).AtName("connections")
		var cfgConns, stateConns types.List
		if d := req.Config.GetAttribute(ctx, connsPath, &cfgConns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if d := req.State.GetAttribute(ctx, connsPath, &stateConns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if !cfgConns.IsNull() {
			cfgManaged = true
			ids, ok := connIdentities(cfgConns)
			if !ok {
				cfgKnown = false
			}
			for id := range ids {
				if strings.HasPrefix(id, "public/") {
					cfgHasPublic = true
				}
			}
		}
		if sIDs, ok := connIdentities(stateConns); ok {
			for id := range sIDs {
				stateHasAny = true
				if strings.HasPrefix(id, "public/") {
					stateHasPublic = true
				}
			}
		}
	}
	if cfgManaged && cfgKnown && cfgHasPublic && stateHasAny && !stateHasPublic {
		resp.Diagnostics.AddAttributeError(path.Root("kafka_api").AtName("connections"),
			"Private-Only Cluster Cannot Gain Public Listeners",
			"this cluster's topology is private-only; it cannot gain public listeners in place — the network was provisioned without public infrastructure. Recreate the cluster with the desired topology, or keep every connection private.")
	}
}

// connIdentity returns the (type, auth.mode) identity of one connections
// element. ok is false for malformed or not-yet-known elements.
func connIdentity(el attr.Value) (string, bool) {
	obj, isObj := el.(types.Object)
	if !isObj || obj.IsNull() || obj.IsUnknown() {
		return "", false
	}
	attrs := obj.Attributes()
	t, isStr := attrs["type"].(types.String)
	if !isStr || t.IsNull() || t.IsUnknown() {
		return "", false
	}
	auth, isAuthObj := attrs["auth"].(types.Object)
	if !isAuthObj || auth.IsNull() || auth.IsUnknown() {
		return "", false
	}
	mode, isModeStr := auth.Attributes()["mode"].(types.String)
	if !isModeStr || mode.IsNull() || mode.IsUnknown() {
		return "", false
	}
	return t.ValueString() + "/" + mode.ValueString(), true
}

// connIdentities returns the multiset of element identities in a connections
// list. ok is false when the list or any element identity is not yet known.
func connIdentities(list types.List) (map[string]int, bool) {
	if list.IsNull() || list.IsUnknown() {
		return nil, false
	}
	out := map[string]int{}
	for _, el := range list.Elements() {
		id, ok := connIdentity(el)
		if !ok {
			return nil, false
		}
		out[id]++
	}
	return out, true
}

// markEchoesUnknownOnConnectionsChange is the mirror image of
// markConnectionsUnknownOnLegacyListenerChange: when a service's config-set
// connections change identity (its multiset of type/auth pairs), the server
// re-derives the per-service sasl echo and, for kafka, the root
// connection_type (any private kafka listener reads back "private"), so the
// carried prior values cannot survive the apply. Identity comparison, not
// value comparison: config elements never carry endpoints, and a value diff
// would mark echoes unknown on every no-op plan.
func markEchoesUnknownOnConnectionsChange(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	for _, svc := range []struct {
		name          string
		saslAttrTypes map[string]attr.Type
		derivesRoot   bool
	}{
		{"kafka_api", clustermodel.KafkaAPISaslAttrTypes(), true},
		{"http_proxy", clustermodel.HTTPProxySaslAttrTypes(), false},
		{"schema_registry", nil, false},
	} {
		connsPath := path.Root(svc.name).AtName("connections")
		var cfgConns, stateConns types.List
		if d := req.Config.GetAttribute(ctx, connsPath, &cfgConns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if cfgConns.IsNull() {
			continue
		}
		if d := req.State.GetAttribute(ctx, connsPath, &stateConns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}

		changed := cfgConns.IsUnknown()
		if !changed {
			cfgIDs, cfgOK := connIdentities(cfgConns)
			stateIDs, stateOK := connIdentities(stateConns)
			changed = !cfgOK || !stateOK || !maps.Equal(cfgIDs, stateIDs)
		}
		if !changed {
			continue
		}
		if svc.saslAttrTypes != nil {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(svc.name).AtName("sasl"),
				types.ObjectUnknown(svc.saslAttrTypes))...)
		}
		if svc.derivesRoot {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("connection_type"), types.StringUnknown())...)
		}
	}
}
