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
	"fmt"
	"strings"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithValidateConfig = &Cluster{}

// ValidateConfig enforces the dual-listener-mode cross-attribute rules the
// control plane implements only in server code (cloudv2
// dual_mode_connections.go) — none are expressible as buf.validate annotations,
// so without this they surface as opaque apply-time errors. Rules that depend
// on stored cluster state (e.g. an mTLS CA preserved from storage on update)
// are deliberately left to the API, which distinguishes create from update.
func (*Cluster) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateDualListenerConnections(ctx, req.Config, resp)
}

// connService is one service's config view for the cross-service rules.
type connService struct {
	name  string
	conns types.List
	sasl  types.Object
	mtls  types.Object
}

func validateDualListenerConnections(ctx context.Context, cfg tfsdk.Config, resp *resource.ValidateConfigResponse) {
	services := make([]connService, 0, 3)
	for _, name := range []string{"kafka_api", "http_proxy", "schema_registry"} {
		s := connService{name: name, sasl: types.ObjectNull(nil)}
		if d := cfg.GetAttribute(ctx, path.Root(name).AtName("connections"), &s.conns); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		// schema_registry exposes no sasl attribute in the TF schema (the
		// deprecated proto field was never implemented here).
		if name != "schema_registry" {
			if d := cfg.GetAttribute(ctx, path.Root(name).AtName("sasl"), &s.sasl); d.HasError() {
				resp.Diagnostics.Append(d...)
				return
			}
		}
		if d := cfg.GetAttribute(ctx, path.Root(name).AtName("mtls"), &s.mtls); d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		if s.conns.IsUnknown() {
			return // defer to apply when the topology itself is unresolved
		}
		services = append(services, s)
	}

	// type and auth.mode are schema-level optional+computed (see the yaml
	// rationale: a required leaf breaks core's prior-echo carry-through) but
	// semantically required on every configured element.
	for _, s := range services {
		for i, el := range s.conns.Elements() {
			obj, isObj := el.(types.Object)
			if !isObj || obj.IsNull() || obj.IsUnknown() {
				continue
			}
			attrs := obj.Attributes()
			if t, ok := attrs["type"].(types.String); ok && t.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("connections").AtListIndex(i).AtName("type"),
					"Missing Connection Type",
					fmt.Sprintf("%s.connections[%d] must set type to public or private", s.name, i))
			}
			auth, hasAuth := attrs["auth"].(types.Object)
			if !hasAuth || auth.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("connections").AtListIndex(i).AtName("auth"),
					"Missing Connection Auth",
					fmt.Sprintf("%s.connections[%d] must set auth.mode to sasl or mtls", s.name, i))
			} else if mode, ok := auth.Attributes()["mode"].(types.String); ok && mode.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("connections").AtListIndex(i).AtName("auth").AtName("mode"),
					"Missing Connection Auth Mode",
					fmt.Sprintf("%s.connections[%d] must set auth.mode to sasl or mtls", s.name, i))
			}
		}
	}

	anyConns := false
	for _, s := range services {
		if len(s.conns.Elements()) > 0 {
			anyConns = true
		}
	}

	// A connections element whose type or auth.mode is unknown (var-driven,
	// cross-resource reference) makes every rule that reads the topology
	// undecidable at plan time; those rules defer to apply instead of raising
	// false positives.
	unresolved := anyUnresolvedConnection(services)

	var connType types.String
	if !getAttr(ctx, cfg, path.Root("connection_type"), &connType, resp) {
		return
	}
	var cmr types.Object
	if !getAttr(ctx, cfg, path.Root("customer_managed_resources"), &cmr, resp) {
		return
	}

	if !cmr.IsNull() && !cmr.IsUnknown() && connType.IsNull() && !unresolved && !anyPrivateConnection(services) {
		resp.Diagnostics.AddAttributeError(path.Root("customer_managed_resources"),
			"Private Access Required",
			"customer_managed_resources requires private access: set a private connection on every service or connection_type = \"private\"")
	}

	if !anyConns {
		// connection_type lost its schema-level Required when connections
		// arrived; without this gate a config setting neither silently
		// creates a cluster whose exposure resolves server-side (UNSPECIFIED
		// leans public).
		if connType.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("connection_type"),
				"Missing Connection Topology",
				"set connection_type or configure connections on all services; the cluster's network exposure must be chosen explicitly")
		}
		return
	}

	if !connType.IsNull() && !connType.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("connection_type"),
			"Conflicting Connection Configuration",
			"connection_type cannot be set together with connections; connections define the cluster's network topology")
	}

	var missing []string
	for _, s := range services {
		if len(s.conns.Elements()) == 0 {
			missing = append(missing, s.name)
		}
		if !s.sasl.IsNull() && !s.sasl.IsUnknown() && len(s.conns.Elements()) > 0 {
			resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("sasl"),
				"Conflicting Listener Configuration",
				fmt.Sprintf("%s.sasl cannot be set together with %s.connections; connections define per-listener auth", s.name, s.name))
		}
	}
	if len(missing) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root(missing[0]).AtName("connections"),
			"Incomplete Connection Configuration",
			"when connections are set they must be set on all services; missing on "+strings.Join(missing, ", "))
	}

	if !unresolved {
		validateConnectionTopology(services, resp)
		for _, s := range services {
			validateConnectionMTLSCoupling(s, resp)
		}
	}

	var cloudProvider, clusterType types.String
	if !getAttr(ctx, cfg, path.Root("cloud_provider"), &cloudProvider, resp) {
		return
	}
	if !getAttr(ctx, cfg, path.Root("cluster_type"), &clusterType, resp) {
		return
	}
	// Certified envelope (cloudv2 #29143): AWS BYOC only. The cloud API
	// rejects only Azure, so an uncertified combination would otherwise pass
	// plan and apply and come up broken.
	if !cloudProvider.IsUnknown() && !clusterType.IsUnknown() &&
		(cloudProvider.ValueString() != "aws" || clusterType.ValueString() != "byoc") {
		resp.Diagnostics.AddAttributeError(path.Root("kafka_api").AtName("connections"),
			"Unsupported Cluster Envelope",
			"dual listener mode (connections) is supported only on AWS BYOC clusters")
	}
}

// getAttr reads one config attribute, appending any diagnostics; ok is false
// when the read errored (the caller must return — a swallowed read error would
// silently disable every rule below it).
func getAttr[T any](ctx context.Context, cfg tfsdk.Config, p path.Path, target *T, resp *resource.ValidateConfigResponse) bool {
	d := cfg.GetAttribute(ctx, p, target)
	resp.Diagnostics.Append(d...)
	return !d.HasError()
}

// anyUnresolvedConnection reports whether any configured connections element
// has an unknown identity leaf (element, type, or auth.mode unknown).
func anyUnresolvedConnection(services []connService) bool {
	for _, s := range services {
		for _, el := range s.conns.Elements() {
			obj, isObj := el.(types.Object)
			if !isObj {
				continue
			}
			if obj.IsUnknown() {
				return true
			}
			if obj.IsNull() {
				continue
			}
			attrs := obj.Attributes()
			if t, ok := attrs["type"].(types.String); ok && t.IsUnknown() {
				return true
			}
			auth, hasAuth := attrs["auth"].(types.Object)
			if hasAuth && auth.IsUnknown() {
				return true
			}
			if hasAuth && !auth.IsNull() {
				if mode, ok := auth.Attributes()["mode"].(types.String); ok && mode.IsUnknown() {
					return true
				}
			}
		}
	}
	return false
}

// validateConnectionTopology requires the public/private topology to be
// identical across the three services (auth mode may differ per service).
func validateConnectionTopology(services []connService, resp *resource.ValidateConfigResponse) {
	type topology struct {
		name                  string
		hasPublic, hasPrivate bool
	}
	tops := make([]topology, 0, len(services))
	for _, s := range services {
		if len(s.conns.Elements()) == 0 {
			continue
		}
		t := topology{name: s.name}
		for _, el := range s.conns.Elements() {
			connType, _, ok := connectionConfigParts(el)
			if !ok {
				continue
			}
			if connType == "private" {
				t.hasPrivate = true
			} else {
				t.hasPublic = true
			}
		}
		tops = append(tops, t)
	}
	if len(tops) < 2 {
		return
	}
	ref := tops[0]
	for _, t := range tops[1:] {
		if t.hasPublic != ref.hasPublic || t.hasPrivate != ref.hasPrivate {
			resp.Diagnostics.AddAttributeError(path.Root(t.name).AtName("connections"),
				"Mismatched Connection Topology",
				fmt.Sprintf("all services must have the same connection network types; %s is %s but %s is %s",
					ref.name, describeTopology(ref.hasPublic, ref.hasPrivate), t.name, describeTopology(t.hasPublic, t.hasPrivate)))
			return
		}
	}
}

func describeTopology(hasPublic, hasPrivate bool) string {
	switch {
	case hasPublic && hasPrivate:
		return "public+private"
	case hasPrivate:
		return "private-only"
	default:
		return "public-only"
	}
}

// validateConnectionMTLSCoupling rejects config-pinned mtls contradictions for
// a service with connections set:
//   - mtls.enabled explicitly false alongside an mTLS connection
//   - an explicitly empty mtls.ca_certificates_pem alongside an mTLS connection
//   - a meaningful mtls block (enabled or CA set in config) with no mTLS
//     connection to apply it to
//
// A config-omitted mtls block or CA is NOT rejected: on update the control
// plane preserves the stored CA, and only the API can tell create from update.
func validateConnectionMTLSCoupling(s connService, resp *resource.ValidateConfigResponse) {
	if len(s.conns.Elements()) == 0 {
		return
	}
	hasMTLSConn := false
	for _, el := range s.conns.Elements() {
		if _, authMode, ok := connectionConfigParts(el); ok && authMode == "mtls" {
			hasMTLSConn = true
		}
	}

	enabled, caList := mtlsConfigParts(s.mtls)

	if hasMTLSConn {
		if !enabled.IsNull() && !enabled.IsUnknown() && !enabled.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("mtls").AtName("enabled"),
				"Conflicting mTLS Configuration",
				fmt.Sprintf("%s declares an mTLS connection but %s.mtls.enabled is false; set it to true instead of relying on an implicit override", s.name, s.name))
		}
		if !caList.IsNull() && !caList.IsUnknown() && len(caList.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("mtls").AtName("ca_certificates_pem"),
				"Missing mTLS CA Bundle",
				fmt.Sprintf("%s declares an mTLS connection but %s.mtls.ca_certificates_pem is empty; provide the trusted client CA bundle", s.name, s.name))
		}
		return
	}

	meaningful := (!enabled.IsNull() && !enabled.IsUnknown() && enabled.ValueBool()) ||
		(!caList.IsNull() && !caList.IsUnknown() && len(caList.Elements()) > 0)
	if meaningful {
		resp.Diagnostics.AddAttributeError(path.Root(s.name).AtName("mtls"),
			"Orphaned mTLS Configuration",
			fmt.Sprintf("%s.mtls cannot be set when no connection uses mTLS auth; add an mTLS connection or remove the mtls block", s.name))
	}
}

// anyPrivateConnection reports whether any service's config declares a private
// connection.
func anyPrivateConnection(services []connService) bool {
	for _, s := range services {
		for _, el := range s.conns.Elements() {
			if connType, _, ok := connectionConfigParts(el); ok && connType == "private" {
				return true
			}
		}
	}
	return false
}

// connectionConfigParts pulls type and auth.mode out of one config connections
// element. Unlike the state-side helper it ignores endpoint, which is computed
// and absent from config.
func connectionConfigParts(el attr.Value) (connType, authMode string, ok bool) {
	obj, isObj := el.(types.Object)
	if !isObj || obj.IsNull() || obj.IsUnknown() {
		return "", "", false
	}
	attrs := obj.Attributes()
	t, isStr := attrs["type"].(types.String)
	if !isStr || t.IsNull() || t.IsUnknown() {
		return "", "", false
	}
	auth, isAuthObj := attrs["auth"].(types.Object)
	if !isAuthObj || auth.IsNull() || auth.IsUnknown() {
		return "", "", false
	}
	mode, isModeStr := auth.Attributes()["mode"].(types.String)
	if !isModeStr || mode.IsNull() || mode.IsUnknown() {
		return "", "", false
	}
	return t.ValueString(), mode.ValueString(), true
}

// mtlsConfigParts pulls enabled and ca_certificates_pem out of a service's
// config mtls object; null object yields null parts.
func mtlsConfigParts(mtls types.Object) (enabled types.Bool, caList types.List) {
	if mtls.IsNull() || mtls.IsUnknown() {
		return types.BoolNull(), types.ListNull(types.StringType)
	}
	attrs := mtls.Attributes()
	enabled, ok := attrs["enabled"].(types.Bool)
	if !ok {
		enabled = types.BoolNull()
	}
	caList, ok = attrs["ca_certificates_pem"].(types.List)
	if !ok {
		caList = types.ListNull(types.StringType)
	}
	return enabled, caList
}

// stripEchoedConnections clears connections from the update payload for
// services whose CONFIG leaves connections null: the plan value there is the
// computed read echo, not user intent, and sending it under a mask would
// silently adopt dual listener mode on a legacy cluster.
func stripEchoedConnections(ctx context.Context, cfg tfsdk.Config, payload *controlplanev1.ClusterUpdate, diags *diag.Diagnostics) {
	for _, svc := range []struct {
		name  string
		clear func()
	}{
		{"kafka_api", func() {
			if payload.GetKafkaApi() != nil {
				payload.GetKafkaApi().Connections = nil
			}
		}},
		{"http_proxy", func() {
			if payload.GetHttpProxy() != nil {
				payload.GetHttpProxy().Connections = nil
			}
		}},
		{"schema_registry", func() {
			if payload.GetSchemaRegistry() != nil {
				payload.GetSchemaRegistry().Connections = nil
			}
		}},
	} {
		var conns types.List
		if d := cfg.GetAttribute(ctx, path.Root(svc.name).AtName("connections"), &conns); d.HasError() {
			diags.Append(d...)
			return
		}
		if conns.IsNull() {
			svc.clear()
		}
	}
}
