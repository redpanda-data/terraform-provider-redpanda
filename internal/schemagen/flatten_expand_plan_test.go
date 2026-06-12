// Copyright 2023 Redpanda Data, Inc.
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

package schemagen

import (
	"strings"
	"testing"
)

// TestPlanFlattenExpandUser exercises the user-resource shape against the
// planner: name (string), mechanism (enum SASLMechanism), password
// (expand_via + flatten_skip), id (extra+from_proto), allow_deletion
// (contingent), plus pure-tf fields.
func TestPlanFlattenExpandUser(t *testing.T) {
	cfg := &Config{
		TFName: "User",
		Fields: map[string]FieldConfig{
			"name": {Required: true},
			"password": {
				Optional:    boolPtr(true),
				ExpandVia:   "GetEffectivePassword",
				FlattenSkip: true,
			},
			"mechanism": {
				Optional: boolPtr(true),
			},
			"id": {
				Extra:        true,
				Type:         "string",
				ComputedOnly: true,
				FromProto:    "name",
			},
			"cluster_api_url": {
				Extra:    true,
				Type:     "string",
				Required: true,
			},
			"password_wo": {
				Extra:    true,
				Type:     "string",
				Optional: boolPtr(true),
			},
			"password_wo_version": {
				Extra:    true,
				Type:     "int64",
				Optional: boolPtr(true),
			},
			"allow_deletion": {
				Extra:    true,
				Type:     "bool",
				Optional: boolPtr(true),
				Computed: boolPtr(true),
				Default:  false,
			},
		},
		API: &APIConfig{
			Create: &RPCConfig{
				RPC:          "CreateUser",
				Request:      "CreateUserRequest",
				PayloadField: "user",
				PayloadType:  "CreateUserRequest_User",
			},
			Update: &RPCConfig{
				RPC:          "UpdateUser",
				Request:      "UpdateUserRequest",
				PayloadField: "user",
				PayloadType:  "UpdateUserRequest_User",
			},
			Delete: &RPCConfig{
				RPC:     "DeleteUser",
				Request: "DeleteUserRequest",
			},
			ResponseInterface: &ResponseInterfaceConfig{
				Name: "UserResponse",
			},
		},
	}
	proto := &ProtoMessage{
		Name: "User",
		Fields: []ProtoField{
			{Name: "name", Kind: "string", Cardinality: "singular"},
			{Name: "password", Kind: "string", Cardinality: "singular"},
			{Name: "mechanism", Kind: "enum", Cardinality: "singular", IsOptional: true, EnumName: "redpanda.api.dataplane.v1.SASLMechanism"},
		},
	}

	attrs := []SchemaAttr{
		{Name: "name", AttrType: AttrTypeString, Required: true},
		{Name: "password", AttrType: AttrTypeString, Optional: true, Sensitive: true},
		{Name: "mechanism", AttrType: AttrTypeString, Optional: true},
		{Name: "id", AttrType: AttrTypeString, Computed: true},
		{Name: "cluster_api_url", AttrType: AttrTypeString, Required: true},
		{Name: "password_wo", AttrType: AttrTypeString, Optional: true, WriteOnly: true},
		{Name: "password_wo_version", AttrType: AttrTypeInt64, Optional: true},
		{Name: "allow_deletion", AttrType: AttrTypeBool, Optional: true, Computed: true},
	}

	lookup := func(name string) (*ProtoMessage, error) {
		if name == "DeleteUserRequest" {
			return &ProtoMessage{
				Name: "DeleteUserRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			}, nil
		}
		return nil, nil
	}
	data, err := PlanFlattenExpand(attrs, cfg, proto, "user", "dataplanev1",
		"buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1",
		"resource", lookup)
	if err != nil {
		t.Fatalf("PlanFlattenExpand: %v", err)
	}

	src, err := GenerateFlattenExpand(data)
	if err != nil {
		t.Fatalf("GenerateFlattenExpand: %v\nGOT:\n%s", err, src)
	}

	got := string(src)
	mustContain(t, got, "package user")
	mustContain(t, got, "type UserResponse interface")
	mustContain(t, got, "GetName() string")
	mustContain(t, got, "HasMechanism() bool")
	mustNotContain(t, got, "type ContingentFields struct")

	mustContain(t, got, "func Flatten(_ context.Context, proto UserResponse, prev *ResourceModel)")
	mustContain(t, got, "m.Name = types.StringValue(proto.GetName())")
	mustContain(t, got, "m.ID = types.StringValue(proto.GetName())")

	mustContain(t, got, "m.AllowDeletion = prev.AllowDeletion")
	mustContain(t, got, "if m.AllowDeletion.IsNull() || m.AllowDeletion.IsUnknown()")
	mustContain(t, got, "m.AllowDeletion = types.BoolValue(false)")

	mustNotContain(t, got, "m.Password = types.StringValue")
	mustNotContain(t, got, "proto.GetPassword()")

	mustContain(t, got, "m.Password = prev.Password")

	mustContain(t, got, "if proto.HasMechanism()")
	mustContain(t, got, "} else if prev != nil && !prev.Mechanism.IsUnknown() {")
	mustContain(t, got, "m.Mechanism = prev.Mechanism")
	mustContain(t, got, "enums.SASLMechanismToString")
	mustContain(t, got, "GetMechanism() dataplanev1.SASLMechanism")

	mustContain(t, got, "func ExpandCreate(_ context.Context, m *ResourceModel) (*dataplanev1.CreateUserRequest")
	mustContain(t, got, "func ExpandUpdate(_ context.Context, m *ResourceModel) (*dataplanev1.UpdateUserRequest")
	mustContain(t, got, "func ExpandDelete(_ context.Context, m *ResourceModel) (*dataplanev1.DeleteUserRequest")

	mustContain(t, normalize(got), normalize("Name: m.Name.ValueString(),"))

	mustContain(t, got, "payload := &dataplanev1.CreateUserRequest_User{")
	mustContain(t, normalize(got), normalize("Password: m.GetEffectivePassword()"))
	mustContain(t, got, "enums.StringToSASLMechanism(m.Mechanism.ValueString())")
}

func mustContain(t *testing.T, src, sub string) {
	t.Helper()
	if !strings.Contains(src, sub) {
		t.Errorf("expected output to contain %q\n--- GOT ---\n%s\n--- END ---", sub, src)
	}
}

func mustNotContain(t *testing.T, src, sub string) {
	t.Helper()
	if strings.Contains(src, sub) {
		t.Errorf("expected output NOT to contain %q\n--- GOT ---\n%s\n--- END ---", sub, src)
	}
}

func boolPtr(b bool) *bool { return &b }

func normalize(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// TestFromProtoEnumWithProtoOnly pins the combination used to rename a
// proto enum field to a different TF attribute name (e.g. proto `type`
// → TF `cluster_type`). The proto field is suppressed from TF emission
// via proto_only:, but its getter must stay on the response interface
// so the auto-emitted enum Flatten/Expand can call it.
func TestFromProtoEnumWithProtoOnly(t *testing.T) {
	cfg := &Config{
		TFName:            "Cluster",
		ExcludeOperations: []string{"update", "delete"},
		Fields: map[string]FieldConfig{
			"type": {
				ProtoOnly: true,
			},
			"cluster_type": {
				Extra:     true,
				Type:      "string",
				Optional:  boolPtr(true),
				Computed:  boolPtr(true),
				FromProto: "type",
			},
		},
		API: &APIConfig{
			Create: &RPCConfig{
				RPC:          "CreateCluster",
				Request:      "CreateClusterRequest",
				PayloadField: "cluster",
				PayloadType:  "ClusterCreate",
			},
			ResponseInterface: &ResponseInterfaceConfig{Name: "ClusterResponse"},
		},
	}
	proto := &ProtoMessage{
		Name: "Cluster",
		Fields: []ProtoField{
			{Name: "type", Kind: "enum", Cardinality: "singular", EnumName: "redpanda.api.controlplane.v1.ClusterType"},
		},
	}
	attrs := []SchemaAttr{
		{Name: "cluster_type", AttrType: AttrTypeString, Optional: true, Computed: true},
	}

	lookup := func(name string) (*ProtoMessage, error) {
		if name == "CreateClusterRequest" {
			return &ProtoMessage{
				Name: "CreateClusterRequest",
				Fields: []ProtoField{
					{Name: "cluster", Kind: KindMessage, Nested: &ProtoMessage{
						Name:   "ClusterCreate",
						GoName: "ClusterCreate",
						Fields: []ProtoField{
							{Name: "type", Kind: "enum", Cardinality: "singular", EnumName: "redpanda.api.controlplane.v1.ClusterType"},
						},
					}},
				},
			}, nil
		}
		return nil, nil
	}
	data, err := PlanFlattenExpand(attrs, cfg, proto, "cluster", "controlplanev1",
		"buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1",
		"resource", lookup)
	if err != nil {
		t.Fatalf("PlanFlattenExpand: %v", err)
	}

	src, err := GenerateFlattenExpand(data)
	if err != nil {
		t.Fatalf("GenerateFlattenExpand: %v\nGOT:\n%s", err, src)
	}
	got := string(src)

	mustContain(t, got, "type ClusterResponse interface")
	mustContain(t, got, "GetType() controlplanev1.ClusterType")
	mustContain(t, got, "m.ClusterType = types.StringValue(enums.ClusterTypeToString(proto.GetType()))")
	mustContain(t, got, "Type: enums.StringToClusterType(m.ClusterType.ValueString())")
}

func TestInferRPCPayload(t *testing.T) {
	clusterUpdate := &ProtoMessage{Name: "ClusterUpdate", GoName: "ClusterUpdate"}
	fieldMask := &ProtoMessage{Name: "FieldMask"}
	awsConfig := &ProtoMessage{Name: "AWS", GoName: "UpdateServerlessPrivateLinkRequest_AWS"}

	lookup := func(messages map[string]*ProtoMessage) ProtoLookup {
		return func(name string) (*ProtoMessage, error) {
			if m, ok := messages[name]; ok {
				return m, nil
			}
			return nil, nil
		}
	}

	t.Run("infers single-message request", func(t *testing.T) {
		rpc := &RPCConfig{RPC: "UpdateCluster", Request: "UpdateClusterRequest"}
		l := lookup(map[string]*ProtoMessage{
			"UpdateClusterRequest": {Name: "UpdateClusterRequest", Fields: []ProtoField{
				{Name: "cluster", Kind: KindMessage, Nested: clusterUpdate},
				{Name: "update_mask", Kind: KindMessage, Nested: fieldMask},
			}},
		})
		if err := inferRPCPayload(rpc, l); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "cluster" || rpc.PayloadType != "ClusterUpdate" {
			t.Fatalf("want cluster/ClusterUpdate, got %s/%s", rpc.PayloadField, rpc.PayloadType)
		}
	})

	t.Run("skips flat request with scalar siblings", func(t *testing.T) {
		rpc := &RPCConfig{RPC: "UpdateSPL", Request: "UpdateServerlessPrivateLinkRequest"}
		l := lookup(map[string]*ProtoMessage{
			"UpdateServerlessPrivateLinkRequest": {Name: "UpdateServerlessPrivateLinkRequest", Fields: []ProtoField{
				{Name: "id", Kind: KindString},
				{Name: "aws_config", Kind: KindMessage, Nested: awsConfig, OneofName: "config"},
			}},
		})
		if err := inferRPCPayload(rpc, l); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "" || rpc.PayloadType != "" {
			t.Fatalf("should have skipped inference, got %s/%s", rpc.PayloadField, rpc.PayloadType)
		}
	})

	t.Run("respects explicit declaration", func(t *testing.T) {
		rpc := &RPCConfig{RPC: "UpdateCluster", Request: "UpdateClusterRequest", PayloadField: "custom", PayloadType: "Custom"}
		if err := inferRPCPayload(rpc, lookup(nil)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "custom" || rpc.PayloadType != "Custom" {
			t.Fatalf("explicit values were clobbered: %s/%s", rpc.PayloadField, rpc.PayloadType)
		}
	})

	t.Run("noop without lookup", func(t *testing.T) {
		rpc := &RPCConfig{RPC: "X", Request: "XRequest"}
		if err := inferRPCPayload(rpc, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "" {
			t.Fatal("should not have inferred without lookup")
		}
	})

	t.Run("noop on scalar-only flat request", func(t *testing.T) {
		rpc := &RPCConfig{RPC: "DeleteX", Request: "DeleteXRequest"}
		l := lookup(map[string]*ProtoMessage{
			"DeleteXRequest": {Name: "DeleteXRequest", Fields: []ProtoField{
				{Name: "id", Kind: KindString},
			}},
		})
		if err := inferRPCPayload(rpc, l); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "" {
			t.Fatalf("should not have inferred scalar-only flat: got %s", rpc.PayloadField)
		}
	})

	t.Run("infers payload alongside scalar siblings", func(t *testing.T) {
		pipelineUpdate := &ProtoMessage{Name: "PipelineUpdate", GoName: "PipelineUpdate"}
		rpc := &RPCConfig{RPC: "UpdatePipeline", Request: "UpdatePipelineRequest"}
		l := lookup(map[string]*ProtoMessage{
			"UpdatePipelineRequest": {Name: "UpdatePipelineRequest", Fields: []ProtoField{
				{Name: "id", Kind: KindString},
				{Name: "pipeline", Kind: KindMessage, Nested: pipelineUpdate},
				{Name: "update_mask", Kind: KindMessage, Nested: fieldMask},
			}},
		})
		if err := inferRPCPayload(rpc, l); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "pipeline" || rpc.PayloadType != "PipelineUpdate" {
			t.Fatalf("want pipeline/PipelineUpdate, got %s/%s", rpc.PayloadField, rpc.PayloadType)
		}
	})

	t.Run("infers payload alongside boolean flag", func(t *testing.T) {
		topic := &ProtoMessage{Name: "Topic", GoName: "CreateTopicRequest_Topic"}
		rpc := &RPCConfig{RPC: "CreateTopic", Request: "CreateTopicRequest"}
		l := lookup(map[string]*ProtoMessage{
			"CreateTopicRequest": {Name: "CreateTopicRequest", Fields: []ProtoField{
				{Name: "topic", Kind: KindMessage, Nested: topic},
				{Name: "validate_only", Kind: KindBool},
			}},
		})
		if err := inferRPCPayload(rpc, l); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "topic" || rpc.PayloadType != "CreateTopicRequest_Topic" {
			t.Fatalf("want topic/CreateTopicRequest_Topic, got %s/%s", rpc.PayloadField, rpc.PayloadType)
		}
	})

	t.Run("skips sub-component message that doesn't match request convention", func(t *testing.T) {
		nwConfig := &ProtoMessage{Name: "ServerlessNetworkingConfig", GoName: "ServerlessNetworkingConfig"}
		rpc := &RPCConfig{RPC: "UpdateServerlessCluster", Request: "UpdateServerlessClusterRequest"}
		l := lookup(map[string]*ProtoMessage{
			"UpdateServerlessClusterRequest": {Name: "UpdateServerlessClusterRequest", Fields: []ProtoField{
				{Name: "id", Kind: KindString},
				{Name: "networking_config", Kind: KindMessage, Nested: nwConfig},
				{Name: "private_link_id", Kind: KindString},
			}},
		})
		if err := inferRPCPayload(rpc, l); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rpc.PayloadField != "" || rpc.PayloadType != "" {
			t.Fatalf("should not infer non-convention sub-component: got %s/%s",
				rpc.PayloadField, rpc.PayloadType)
		}
	})
}

// TestFlattenFromPrev_TopLevelDirective covers the per-field directive on a
// top-level proto-backed field — the planner sets OverrideFromPrev=true on
// the conversion.
func TestFlattenFromPrev_TopLevelDirective(t *testing.T) {
	cfg := flattenFromPrevTestCfg(map[string]FieldConfig{
		"name":        {Required: true},
		"description": {Optional: boolPtr(true), FlattenFromPrev: true},
	})
	attrs := []SchemaAttr{
		{Name: "name", AttrType: AttrTypeString, Required: true},
		{Name: "description", AttrType: AttrTypeString, Optional: true},
	}
	data, err := PlanFlattenExpand(attrs, cfg, flattenFromPrevTestProto(), "pipelinemodel", "dataplanev1", "buf.build/example", "resource", nil)
	if err != nil {
		t.Fatalf("PlanFlattenExpand: %v", err)
	}
	var found bool
	for _, c := range data.RootFieldConversions {
		if c.GoName == "Description" {
			if !c.OverrideFromPrev {
				t.Error("expected OverrideFromPrev=true on Description")
			}
			found = true
		}
		if c.GoName == "Name" && c.OverrideFromPrev {
			t.Error("Name without directive should not have OverrideFromPrev=true")
		}
	}
	if !found {
		t.Error("Description conversion missing from RootFieldConversions")
	}
}

// TestFlattenFromPrev_ConflictsWithComputedOnly verifies the directive
// errors when paired with computed_only (no user-supplied prev value).
func TestFlattenFromPrev_ConflictsWithComputedOnly(t *testing.T) {
	cfg := flattenFromPrevTestCfg(map[string]FieldConfig{
		"description": {ComputedOnly: true, FlattenFromPrev: true},
	})
	attrs := []SchemaAttr{
		{Name: "description", AttrType: AttrTypeString, Computed: true},
	}
	_, err := PlanFlattenExpand(attrs, cfg, flattenFromPrevTestProto(), "pipelinemodel", "dataplanev1", "buf.build/example", "resource", nil)
	if err == nil || !strings.Contains(err.Error(), "computed_only") {
		t.Errorf("expected computed_only conflict, got: %v", err)
	}
}

// TestFlattenFromPrev_ConflictsWithFlattenVia verifies the directive errors
// when paired with flatten_via.
func TestFlattenFromPrev_ConflictsWithFlattenVia(t *testing.T) {
	cfg := flattenFromPrevTestCfg(map[string]FieldConfig{
		"description": {Optional: boolPtr(true), FlattenVia: "DescribeProto", FlattenFromPrev: true},
	})
	attrs := []SchemaAttr{
		{Name: "description", AttrType: AttrTypeString, Optional: true},
	}
	_, err := PlanFlattenExpand(attrs, cfg, flattenFromPrevTestProto(), "pipelinemodel", "dataplanev1", "buf.build/example", "resource", nil)
	if err == nil || !strings.Contains(err.Error(), "flatten_via") {
		t.Errorf("expected flatten_via conflict, got: %v", err)
	}
}

// TestNestedPreserveBlocks_WriteOnlyAndExtra covers the generator's
// nested-auto-preserve emission. SingleNested objects whose children include
// write_only or extra fields produce a NestedPreserveBlock.
func TestNestedPreserveBlocks_WriteOnlyAndExtra(t *testing.T) {
	cfg := flattenFromPrevTestCfg(map[string]FieldConfig{
		"service_account": {
			Optional: boolPtr(true),
			Fields: map[string]FieldConfig{
				"client_id":     {Required: true},
				"client_secret": {Required: true, WriteOnly: true, FlattenSkip: true},
				"secret_version": {
					Extra:    true,
					Type:     "int64",
					Optional: boolPtr(true),
				},
			},
		},
	})
	attrs := []SchemaAttr{
		{
			Name:     "service_account",
			AttrType: AttrTypeSingleNested,
			Optional: true,
			NestedAttrs: []SchemaAttr{
				{Name: "client_id", AttrType: AttrTypeString, Required: true},
				{Name: "client_secret", AttrType: AttrTypeString, Required: true, WriteOnly: true},
				{Name: "secret_version", AttrType: AttrTypeInt64, Optional: true},
			},
		},
	}
	data, err := PlanFlattenExpand(attrs, cfg, flattenFromPrevTestProtoNested(), "pipelinemodel", "dataplanev1", "buf.build/example", "resource", nil)
	if err != nil {
		t.Fatalf("PlanFlattenExpand: %v", err)
	}
	if len(data.NestedPreserveBlocks) != 1 {
		t.Fatalf("expected 1 NestedPreserveBlock, got %d", len(data.NestedPreserveBlocks))
	}
	block := data.NestedPreserveBlocks[0]
	if block.ContainerGoName != "ServiceAccount" {
		t.Errorf("ContainerGoName: got %q, want %q", block.ContainerGoName, "ServiceAccount")
	}
	if block.AsFunc != "AsServiceAccount" {
		t.Errorf("AsFunc: got %q, want %q", block.AsFunc, "AsServiceAccount")
	}
	if block.ToObjectFunc != "ServiceAccountToObject" {
		t.Errorf("ToObjectFunc: got %q, want %q", block.ToObjectFunc, "ServiceAccountToObject")
	}
	got := map[string]bool{}
	for _, c := range block.Children {
		got[c.GoName] = true
	}
	if !got["ClientSecret"] {
		t.Error("expected ClientSecret (write_only) in preserve children")
	}
	if !got["SecretVersion"] {
		t.Error("expected SecretVersion (extra) in preserve children")
	}
	if got["ClientId"] {
		t.Error("ClientID is neither write_only nor extra — should not be in preserve children")
	}
}

// TestNestedPreserveBlocks_NoEligibleChildren verifies that a SingleNested
// without any eligible children produces no NestedPreserveBlock.
func TestNestedPreserveBlocks_NoEligibleChildren(t *testing.T) {
	cfg := flattenFromPrevTestCfg(map[string]FieldConfig{
		"resources": {
			Optional: boolPtr(true),
			Fields: map[string]FieldConfig{
				"memory_shares": {Optional: boolPtr(true)},
				"cpu_shares":    {Optional: boolPtr(true)},
			},
		},
	})
	attrs := []SchemaAttr{
		{
			Name:     "resources",
			AttrType: AttrTypeSingleNested,
			Optional: true,
			NestedAttrs: []SchemaAttr{
				{Name: "memory_shares", AttrType: AttrTypeString, Optional: true},
				{Name: "cpu_shares", AttrType: AttrTypeString, Optional: true},
			},
		},
	}
	data, err := PlanFlattenExpand(attrs, cfg, flattenFromPrevTestProtoWithResources(), "pipelinemodel", "dataplanev1", "buf.build/example", "resource", nil)
	if err != nil {
		t.Fatalf("PlanFlattenExpand: %v", err)
	}
	if len(data.NestedPreserveBlocks) != 0 {
		t.Errorf("expected 0 NestedPreserveBlocks, got %d", len(data.NestedPreserveBlocks))
	}
}

func flattenFromPrevTestCfg(fields map[string]FieldConfig) *Config {
	return &Config{
		TFName:            "Pipeline",
		Fields:            fields,
		ExcludeOperations: []string{"update", "delete"},
		API: &APIConfig{
			Create: &RPCConfig{
				RPC:          "CreatePipeline",
				Request:      "CreatePipelineRequest",
				PayloadField: "pipeline",
				PayloadType:  "PipelineCreate",
			},
			ResponseInterface: &ResponseInterfaceConfig{Name: "PipelineResponse"},
		},
	}
}

func flattenFromPrevTestProto() *ProtoMessage {
	return &ProtoMessage{
		Name: "Pipeline",
		Fields: []ProtoField{
			{Name: "name", Kind: "string", Cardinality: "singular"},
			{Name: "description", Kind: "string", Cardinality: "singular"},
		},
	}
}

func flattenFromPrevTestProtoNested() *ProtoMessage {
	return &ProtoMessage{
		Name: "Pipeline",
		Fields: []ProtoField{
			{
				Name:        "service_account",
				Kind:        KindMessage,
				Cardinality: "singular",
				Nested: &ProtoMessage{
					Name: "Pipeline_ServiceAccount",
					Fields: []ProtoField{
						{Name: "client_id", Kind: "string", Cardinality: "singular"},
						{Name: "client_secret", Kind: "string", Cardinality: "singular"},
					},
				},
			},
		},
	}
}

func flattenFromPrevTestProtoWithResources() *ProtoMessage {
	return &ProtoMessage{
		Name: "Pipeline",
		Fields: []ProtoField{
			{
				Name:        "resources",
				Kind:        KindMessage,
				Cardinality: "singular",
				Nested: &ProtoMessage{
					Name: "Pipeline_Resources",
					Fields: []ProtoField{
						{Name: "memory_shares", Kind: "string", Cardinality: "singular"},
						{Name: "cpu_shares", Kind: "string", Cardinality: "singular"},
					},
				},
			},
		},
	}
}

// TestUsesUtils_StmtPaths ensures usesUtils detects "utils." in FlattenStmt
// and ExpandStmt, not just FlattenExpr/ExpandExpr.
func TestUsesUtils_StmtPaths(t *testing.T) {
	cases := []struct {
		name string
		data ConversionData
		want bool
	}{
		{
			name: "FlattenStmt contains utils.",
			data: ConversionData{
				RootFieldConversions: []FieldConversion{
					{FlattenStmt: "m.X = utils.SomeHelper(proto.GetX())"},
				},
			},
			want: true,
		},
		{
			name: "ExpandStmt contains utils. (in Expander)",
			data: ConversionData{
				Expanders: []Expander{
					{Conversions: []FieldConversion{
						{ExpandStmt: "out.X = utils.OtherHelper(m.X.ValueString())"},
					}},
				},
			},
			want: true,
		},
		{
			name: "ExpandStmt contains utils. (in NestedFlattener)",
			data: ConversionData{
				NestedFlatteners: []NestedFlattener{
					{EmitExpand: true, Conversions: []FieldConversion{
						{ExpandStmt: "out.Y = utils.ToProto(m.Y.ValueString())"},
					}},
				},
			},
			want: true,
		},
		{
			name: "no utils. anywhere",
			data: ConversionData{
				RootFieldConversions: []FieldConversion{
					{FlattenStmt: "m.X = types.StringValue(proto.GetX())"},
				},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dataReferencesPkg(tc.data, "utils."); got != tc.want {
				t.Errorf("dataReferencesPkg(utils.) = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWrapperSuffix covers the helper used by emitScalarFlattenExpand to
// suffix .GetValue() onto proto getters for scalar wrapper types
// (google.protobuf.{Bool,String,Int32,Int64,Float,Double}Value).
func TestWrapperSuffix(t *testing.T) {
	cases := []struct {
		name string
		pf   *ProtoField
		want string
	}{
		{name: "nil field", pf: nil, want: ""},
		{name: "plain scalar", pf: &ProtoField{Kind: KindString}, want: ""},
		{name: "proto3 optional scalar (not wrapper)", pf: &ProtoField{Kind: KindBool, IsOptional: true}, want: ""},
		{name: "wrapped bool", pf: &ProtoField{Kind: KindBool, IsOptional: true, IsScalarWrapper: true}, want: ".GetValue()"},
		{name: "wrapped string", pf: &ProtoField{Kind: KindString, IsOptional: true, IsScalarWrapper: true}, want: ".GetValue()"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wrapperSuffix(tc.pf); got != tc.want {
				t.Errorf("wrapperSuffix = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRenderScalarHeuristicFlatten covers the three-arm FlattenStmt emitted
// for TF-Optional + proto-no-presence scalar leaves — the path that resolves
// Bugs 004 / 009 / 023 (null-vs-zero drift). Cases drawn from
// scalar_mix_conv golden: description (String), enabled (Bool), max_count
// (Int32), weight (Int64), price (Float64).
func TestRenderScalarHeuristicFlatten(t *testing.T) {
	cases := []struct {
		name     string
		conv     *FieldConversion
		sh       scalarShape
		contains []string
	}{
		{
			name: "String (description)",
			conv: &FieldConversion{GoName: "Description", ProtoGoName: "Description"},
			sh:   scalarShapes[AttrTypeString],
			contains: []string{
				`if v := proto.GetDescription(); v != "" {`,
				`m.Description = types.StringValue(v)`,
				`} else if prev != nil && !prev.Description.IsUnknown() {`,
				`m.Description = prev.Description`,
				`m.Description = types.StringNull()`,
			},
		},
		{
			name: "Bool (enabled)",
			conv: &FieldConversion{GoName: "Enabled", ProtoGoName: "Enabled"},
			sh:   scalarShapes[AttrTypeBool],
			contains: []string{
				`if v := proto.GetEnabled(); v != false {`,
				`m.Enabled = types.BoolValue(v)`,
				`m.Enabled = prev.Enabled`,
				`m.Enabled = types.BoolNull()`,
			},
		},
		{
			name: "Int32 (max_count)",
			conv: &FieldConversion{GoName: "MaxCount", ProtoGoName: "MaxCount"},
			sh:   scalarShapes[AttrTypeInt32],
			contains: []string{
				`if v := proto.GetMaxCount(); v != 0 {`,
				`m.MaxCount = types.Int32Value(v)`,
				`m.MaxCount = types.Int32Null()`,
			},
		},
		{
			name: "Int64 (weight)",
			conv: &FieldConversion{GoName: "Weight", ProtoGoName: "Weight"},
			sh:   scalarShapes[AttrTypeInt64],
			contains: []string{
				`if v := proto.GetWeight(); v != 0 {`,
				`m.Weight = types.Int64Value(v)`,
				`m.Weight = types.Int64Null()`,
			},
		},
		{
			name: "Float64 (price)",
			conv: &FieldConversion{GoName: "Price", ProtoGoName: "Price"},
			sh:   scalarShapes[AttrTypeFloat64],
			contains: []string{
				`if v := proto.GetPrice(); v != 0.0 {`,
				`m.Price = types.Float64Value(v)`,
				`m.Price = types.Float64Null()`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderScalarHeuristicFlatten(tc.conv, tc.sh)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("rendered output missing %q\n--- GOT ---\n%s", want, got)
				}
			}
		})
	}
}

// TestScalarExpandExpr covers the Expand-side emission for scalar leaves.
// Cases drawn from pointer_string_conv golden (Required String fields whose
// proto carries presence → utils.PointerOrNil) plus the wrapper-recognition
// path added by Change 1 (wrappers always go through PointerOrNil).
func TestScalarExpandExpr(t *testing.T) {
	cases := []struct {
		name string
		conv *FieldConversion
		pf   *ProtoField
		sh   scalarShape
		want string
	}{
		{
			name: "Required String, proto no presence → bare ValueString",
			conv: &FieldConversion{GoName: "Name"},
			pf:   &ProtoField{Kind: KindString},
			sh:   scalarShapes[AttrTypeString],
			want: "m.Name.ValueString()",
		},
		{
			name: "Required String, proto3 optional → PointerOrNil (pointer_string_conv principal/host)",
			conv: &FieldConversion{GoName: "Principal"},
			pf:   &ProtoField{Kind: KindString, IsOptional: true},
			sh:   scalarShapes[AttrTypeString],
			want: "utils.PointerOrNil(m.Principal, types.String.ValueString)",
		},
		{
			name: "Optional Bool, proto no presence → bare ValueBool",
			conv: &FieldConversion{GoName: "Enabled"},
			pf:   &ProtoField{Kind: KindBool},
			sh:   scalarShapes[AttrTypeBool],
			want: "m.Enabled.ValueBool()",
		},
		{
			name: "Bool, proto3 optional → PointerOrNil",
			conv: &FieldConversion{GoName: "Flag"},
			pf:   &ProtoField{Kind: KindBool, IsOptional: true},
			sh:   scalarShapes[AttrTypeBool],
			want: "utils.PointerOrNil(m.Flag, types.Bool.ValueBool)",
		},
		{
			name: "Int32 wrapper → PointerOrNil",
			conv: &FieldConversion{GoName: "Count"},
			pf:   &ProtoField{Kind: KindInt32, IsOptional: true, IsScalarWrapper: true},
			sh:   scalarShapes[AttrTypeInt32],
			want: "utils.PointerOrNil(m.Count, types.Int32.ValueInt32)",
		},
		{
			name: "Int64 plain → bare ValueInt64",
			conv: &FieldConversion{GoName: "Size"},
			pf:   &ProtoField{Kind: KindInt64},
			sh:   scalarShapes[AttrTypeInt64],
			want: "m.Size.ValueInt64()",
		},
		{
			name: "Float64 wrapper → PointerOrNil",
			conv: &FieldConversion{GoName: "Ratio"},
			pf:   &ProtoField{Kind: KindDouble, IsOptional: true, IsScalarWrapper: true},
			sh:   scalarShapes[AttrTypeFloat64],
			want: "utils.PointerOrNil(m.Ratio, types.Float64.ValueFloat64)",
		},
		{
			name: "nil pf → bare ValueString (defensive)",
			conv: &FieldConversion{GoName: "X"},
			pf:   nil,
			sh:   scalarShapes[AttrTypeString],
			want: "m.X.ValueString()",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scalarExpandExpr(tc.conv, tc.pf, tc.sh); got != tc.want {
				t.Errorf("scalarExpandExpr = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEmitScalarFlattenExpand_FromGoldenCases drives the whole emitter against
// the (attr, proto) combos that scalar_mix_conv + pointer_string_conv goldens
// exercised. Each case asserts both Flatten and Expand emission; failures
// here would have surfaced as golden diffs.
func TestEmitScalarFlattenExpand_FromGoldenCases(t *testing.T) {
	cases := []struct {
		name            string
		conv            *FieldConversion
		pf              *ProtoField
		a               *SchemaAttr
		shKey           string
		wantFlattenStmt []string
		wantFlattenExpr string
		wantExpandExpr  string
	}{
		{
			name:  "scalar_mix description (TF Optional, proto no presence) → heuristic stmt + bare Expand",
			conv:  &FieldConversion{GoName: "Description", ProtoGoName: "Description"},
			pf:    &ProtoField{Kind: KindString},
			a:     &SchemaAttr{Name: "description", AttrType: AttrTypeString, Optional: true},
			shKey: AttrTypeString,
			wantFlattenStmt: []string{
				`if v := proto.GetDescription(); v != ""`,
				`m.Description = prev.Description`,
				`m.Description = types.StringNull()`,
			},
			wantExpandExpr: "m.Description.ValueString()",
		},
		{
			name:            "scalar_mix label (TF Optional, proto IsOptional via proto3 optional) → bare types.StringValue + PointerOrNil Expand",
			conv:            &FieldConversion{GoName: "Label", ProtoGoName: "Label"},
			pf:              &ProtoField{Kind: KindString, IsOptional: true},
			a:               &SchemaAttr{Name: "label", AttrType: AttrTypeString, Optional: true},
			shKey:           AttrTypeString,
			wantFlattenExpr: "types.StringValue(proto.GetLabel())",
			wantExpandExpr:  "utils.PointerOrNil(m.Label, types.String.ValueString)",
		},
		{
			name:  "scalar_mix enabled Bool (Optional, no presence) → heuristic stmt + bare Expand",
			conv:  &FieldConversion{GoName: "Enabled", ProtoGoName: "Enabled"},
			pf:    &ProtoField{Kind: KindBool},
			a:     &SchemaAttr{Name: "enabled", AttrType: AttrTypeBool, Optional: true},
			shKey: AttrTypeBool,
			wantFlattenStmt: []string{
				`if v := proto.GetEnabled(); v != false`,
				`m.Enabled = types.BoolNull()`,
			},
			wantExpandExpr: "m.Enabled.ValueBool()",
		},
		{
			name:  "scalar_mix max_count Int32 (Optional, no presence) → heuristic stmt",
			conv:  &FieldConversion{GoName: "MaxCount", ProtoGoName: "MaxCount"},
			pf:    &ProtoField{Kind: KindInt32},
			a:     &SchemaAttr{Name: "max_count", AttrType: AttrTypeInt32, Optional: true},
			shKey: AttrTypeInt32,
			wantFlattenStmt: []string{
				`if v := proto.GetMaxCount(); v != 0`,
				`m.MaxCount = types.Int32Null()`,
			},
			wantExpandExpr: "m.MaxCount.ValueInt32()",
		},
		{
			name:  "scalar_mix price Float64 (Optional, no presence) → heuristic stmt",
			conv:  &FieldConversion{GoName: "Price", ProtoGoName: "Price"},
			pf:    &ProtoField{Kind: KindDouble},
			a:     &SchemaAttr{Name: "price", AttrType: AttrTypeFloat64, Optional: true},
			shKey: AttrTypeFloat64,
			wantFlattenStmt: []string{
				`if v := proto.GetPrice(); v != 0.0`,
				`m.Price = types.Float64Null()`,
			},
			wantExpandExpr: "m.Price.ValueFloat64()",
		},
		{
			name:            "pointer_string principal (Required String, proto IsOptional) → bare Flatten + PointerOrNil Expand",
			conv:            &FieldConversion{GoName: "Principal", ProtoGoName: "Principal"},
			pf:              &ProtoField{Kind: KindString, IsOptional: true},
			a:               &SchemaAttr{Name: "principal", AttrType: AttrTypeString, Required: true},
			shKey:           AttrTypeString,
			wantFlattenExpr: "types.StringValue(proto.GetPrincipal())",
			wantExpandExpr:  "utils.PointerOrNil(m.Principal, types.String.ValueString)",
		},
		{
			name:            "wrapper-typed bool (Change 1 path) → bare types.BoolValue with .GetValue() suffix + PointerOrNil Expand",
			conv:            &FieldConversion{GoName: "Flag", ProtoGoName: "Flag"},
			pf:              &ProtoField{Kind: KindBool, IsOptional: true, IsScalarWrapper: true},
			a:               &SchemaAttr{Name: "flag", AttrType: AttrTypeBool, Optional: true},
			shKey:           AttrTypeBool,
			wantFlattenExpr: "types.BoolValue(proto.GetFlag().GetValue())",
			wantExpandExpr:  "utils.PointerOrNil(m.Flag, types.Bool.ValueBool)",
		},
		{
			name:            "Required scalar, proto no presence → bare Flatten + bare Expand",
			conv:            &FieldConversion{GoName: "Name", ProtoGoName: "Name"},
			pf:              &ProtoField{Kind: KindString},
			a:               &SchemaAttr{Name: "name", AttrType: AttrTypeString, Required: true},
			shKey:           AttrTypeString,
			wantFlattenExpr: "types.StringValue(proto.GetName())",
			wantExpandExpr:  "m.Name.ValueString()",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			emitScalarFlattenExpand(tc.conv, tc.pf, tc.a, scalarShapes[tc.shKey])

			if len(tc.wantFlattenStmt) > 0 {
				if tc.conv.FlattenStmt == "" {
					t.Errorf("expected FlattenStmt to be set, got empty (FlattenExpr=%q)", tc.conv.FlattenExpr)
				}
				for _, want := range tc.wantFlattenStmt {
					if !strings.Contains(tc.conv.FlattenStmt, want) {
						t.Errorf("FlattenStmt missing %q\n--- GOT ---\n%s", want, tc.conv.FlattenStmt)
					}
				}
			}
			if tc.wantFlattenExpr != "" {
				if tc.conv.FlattenExpr != tc.wantFlattenExpr {
					t.Errorf("FlattenExpr = %q, want %q", tc.conv.FlattenExpr, tc.wantFlattenExpr)
				}
			}
			if tc.conv.ExpandExpr != tc.wantExpandExpr {
				t.Errorf("ExpandExpr = %q, want %q", tc.conv.ExpandExpr, tc.wantExpandExpr)
			}
		})
	}
}

// TestUsesEnumsPkg_StmtPaths ensures usesEnumsPkg detects "enums." in
// FlattenStmt and ExpandStmt, not just FlattenExpr/ExpandExpr.
func TestUsesEnumsPkg_StmtPaths(t *testing.T) {
	cases := []struct {
		name string
		data ConversionData
		want bool
	}{
		{
			name: "FlattenStmt contains enums.",
			data: ConversionData{
				RootFieldConversions: []FieldConversion{
					{FlattenStmt: "m.State = enums.StateToString(proto.GetState())"},
				},
			},
			want: true,
		},
		{
			name: "ExpandStmt contains enums. (in Expander)",
			data: ConversionData{
				Expanders: []Expander{
					{Conversions: []FieldConversion{
						{ExpandStmt: "out.State = enums.StringToState(m.State.ValueString())"},
					}},
				},
			},
			want: true,
		},
		{
			name: "ExpandStmt contains enums. (in NestedFlattener)",
			data: ConversionData{
				NestedFlatteners: []NestedFlattener{
					{EmitExpand: true, Conversions: []FieldConversion{
						{ExpandStmt: "out.T = enums.StringToType(m.T.ValueString())"},
					}},
				},
			},
			want: true,
		},
		{
			name: "no enums. anywhere",
			data: ConversionData{
				RootFieldConversions: []FieldConversion{
					{FlattenStmt: "m.X = types.StringValue(proto.GetX())"},
				},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dataReferencesPkg(tc.data, "enums."); got != tc.want {
				t.Errorf("dataReferencesPkg(enums.) = %v, want %v", got, tc.want)
			}
		})
	}
}
