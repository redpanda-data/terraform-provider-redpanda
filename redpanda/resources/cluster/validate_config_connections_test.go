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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// clusterConfig builds a resource config with every attribute null except the
// given top-level strings, so a test states only the leaves a rule reads.
func clusterConfig(t *testing.T, set map[string]string) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	objType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", s.Type().TerraformType(ctx))
	}
	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, at := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	for name, v := range set {
		vals[name] = tftypes.NewValue(tftypes.String, v)
	}
	return tfsdk.Config{Schema: s, Raw: tftypes.NewValue(objType, vals)}
}

// nullsExcept builds an object value with every attribute null except set.
func nullsExcept(obj tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	vals := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, at := range obj.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	for name, v := range set {
		vals[name] = v
	}
	return tftypes.NewValue(obj, vals)
}

// mustTFType narrows a schema-derived tftypes.Type, failing the test on a
// schema shape the helper does not expect.
func mustTFType[T tftypes.Type](t *testing.T, v tftypes.Type) T {
	t.Helper()
	typed, ok := v.(T)
	if !ok {
		t.Fatalf("schema type is %T, want %T", v, *new(T))
	}
	return typed
}

// clusterConfigWithConnections builds a config whose three services each carry
// one public SASL connection and no connection_type, on the given envelope.
func clusterConfigWithConnections(t *testing.T, cloudProvider, clusterType string) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	objType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", s.Type().TerraformType(ctx))
	}
	service := func(name string) tftypes.Value {
		svc := mustTFType[tftypes.Object](t, objType.AttributeTypes[name])
		list := mustTFType[tftypes.List](t, svc.AttributeTypes["connections"])
		elem := mustTFType[tftypes.Object](t, list.ElementType)
		auth := mustTFType[tftypes.Object](t, elem.AttributeTypes["auth"])
		conn := nullsExcept(elem, map[string]tftypes.Value{
			"type": tftypes.NewValue(tftypes.String, "public"),
			"auth": nullsExcept(auth, map[string]tftypes.Value{"mode": tftypes.NewValue(tftypes.String, "sasl")}),
		})
		return nullsExcept(svc, map[string]tftypes.Value{"connections": tftypes.NewValue(list, []tftypes.Value{conn})})
	}
	return tfsdk.Config{Schema: s, Raw: nullsExcept(objType, map[string]tftypes.Value{
		"cloud_provider":  tftypes.NewValue(tftypes.String, cloudProvider),
		"cluster_type":    tftypes.NewValue(tftypes.String, clusterType),
		"kafka_api":       service("kafka_api"),
		"http_proxy":      service("http_proxy"),
		"schema_registry": service("schema_registry"),
	})}
}

// TestConnectionsEnvelopeAndDeprecationAgree pins that the envelope gate on
// connections and the connection_type deprecation warning read the same
// certification scope: on every known envelope exactly one of them fires.
func TestConnectionsEnvelopeAndDeprecationAgree(t *testing.T) {
	for _, cp := range []string{"aws", "gcp", "azure"} {
		for _, ct := range []string{"byoc", "dedicated"} {
			t.Run(cp+"/"+ct, func(t *testing.T) {
				var gate resource.ValidateConfigResponse
				(&Cluster{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: clusterConfigWithConnections(t, cp, ct)}, &gate)
				rejected := false
				for _, d := range gate.Diagnostics.Errors() {
					if d.Summary() == "Unsupported Cluster Envelope" {
						rejected = true
					}
				}

				var legacy resource.ValidateConfigResponse
				(&Cluster{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: clusterConfig(t, map[string]string{
					"connection_type": "public", "cloud_provider": cp, "cluster_type": ct,
				})}, &legacy)
				warned := false
				for _, d := range legacy.Diagnostics.Warnings() {
					if d.Summary() == "Deprecated Attribute" {
						warned = true
					}
				}

				if rejected == warned {
					t.Errorf("envelope %s/%s: connections rejected = %v, connection_type deprecated = %v; exactly one must hold", cp, ct, rejected, warned)
				}
			})
		}
	}
}

// TestConnectionTypeIsNotDeprecatedSchemaWide pins that connection_type carries
// no schema-level deprecation. connections, its replacement, is accepted only
// on AWS BYOC, so a static deprecation would warn every other cluster about a
// migration the provider itself rejects.
func TestConnectionTypeIsNotDeprecatedSchemaWide(t *testing.T) {
	attr, ok := ResourceClusterSchema(context.Background()).Attributes["connection_type"]
	if !ok {
		t.Fatal("connection_type missing from the cluster schema")
	}
	if msg := attr.GetDeprecationMessage(); msg != "" {
		t.Errorf("connection_type has a schema-wide deprecation message %q; the deprecation is scoped to the AWS BYOC envelope in ValidateConfig", msg)
	}
}

// TestValidateConfigWarnsLegacyConnectionTypeOnAWSBYOC pins the envelope scope
// of the connection_type deprecation: a warning on AWS BYOC, where connections
// is available, and silence everywhere else.
func TestValidateConfigWarnsLegacyConnectionTypeOnAWSBYOC(t *testing.T) {
	cases := []struct {
		name     string
		set      map[string]string
		wantWarn bool
	}{
		{"aws byoc warns", map[string]string{"connection_type": "private", "cloud_provider": "aws", "cluster_type": "byoc"}, true},
		{"aws byoc public warns", map[string]string{"connection_type": "public", "cloud_provider": "aws", "cluster_type": "byoc"}, true},
		{"gcp byoc silent", map[string]string{"connection_type": "private", "cloud_provider": "gcp", "cluster_type": "byoc"}, false},
		{"azure byoc silent", map[string]string{"connection_type": "private", "cloud_provider": "azure", "cluster_type": "byoc"}, false},
		{"aws dedicated silent", map[string]string{"connection_type": "public", "cloud_provider": "aws", "cluster_type": "dedicated"}, false},
		{"aws byoc without connection_type silent", map[string]string{"cloud_provider": "aws", "cluster_type": "byoc"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp resource.ValidateConfigResponse
			(&Cluster{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: clusterConfig(t, tc.set)}, &resp)

			var warned bool
			for _, d := range resp.Diagnostics.Warnings() {
				if strings.Contains(d.Detail(), "connections") {
					warned = true
				}
			}
			if warned != tc.wantWarn {
				t.Errorf("connection_type deprecation warning = %v, want %v; diagnostics: %v", warned, tc.wantWarn, resp.Diagnostics)
			}
		})
	}
}
