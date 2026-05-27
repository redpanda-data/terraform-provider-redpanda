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
	"bytes"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "update golden fixtures in internal/schemagen/testdata/")

type goldenCase struct {
	name    string
	cfg     *Config
	proto   *ProtoMessage
	attrs   []SchemaAttr
	pkg     string
	alias   string
	import_ string

	extraProtos map[string]*ProtoMessage
}

// TestFlattenExpandGolden pins the conversion-generator output against
// fixtures in testdata/. Run `go test -run TestFlattenExpandGolden -update-golden`
// to regenerate when an expected change happens.
func TestFlattenExpandGolden(t *testing.T) {
	cases := []goldenCase{
		userGoldenCase(),
		scalarMixGoldenCase(),
		fromProtoGoldenCase(),
		multiFlatGoldenCase(),
		divergentPayloadGoldenCase(),
		pointerStringGoldenCase(),
		listScalarKindsGoldenCase(),
		listMessageGoldenCase(),
		numberAttrGoldenCase(),
		oneofVariantGoldenCase(),
		timestampScalarGoldenCase(),
		mapScalarGoldenCase(),
		perRPCDivergentNestedGoldenCase(),
		forceBoolOneofGoldenCase(),
		returnPayloadUpdateGoldenCase(),
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lookup := func(name string) (*ProtoMessage, error) {
				if msg, ok := c.extraProtos[name]; ok {
					return msg, nil
				}
				return nil, fmt.Errorf("test fixture missing flat request type %q", name)
			}
			data, err := PlanFlattenExpand(c.attrs, c.cfg, c.proto, c.pkg, c.alias, c.import_, "resource", lookup)
			if err != nil {
				t.Fatalf("PlanFlattenExpand: %v", err)
			}
			src, err := GenerateFlattenExpand(data)
			if err != nil {
				t.Fatalf("GenerateFlattenExpand: %v", err)
			}

			fset := token.NewFileSet()
			if _, perr := parser.ParseFile(fset, c.name+".go", src, parser.AllErrors); perr != nil {
				t.Fatalf("generated source does not parse: %v\n--- SOURCE ---\n%s", perr, src)
			}

			path := filepath.Join("testdata", c.name+".golden")
			if *updateGolden {
				if err := os.WriteFile(path, src, 0o600); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path) // #nosec G304 — path is fixed, in-tree
			if err != nil {
				t.Fatalf("read golden %s (run with -update-golden to create): %v", path, err)
			}
			if !bytes.Equal(want, src) {
				t.Fatalf("golden %s does not match generated output.\n--- WANT ---\n%s\n--- GOT ---\n%s",
					path, want, src)
			}
		})
	}
}

func userGoldenCase() goldenCase {
	return goldenCase{
		name:    "user_conv",
		pkg:     "user",
		alias:   "dataplanev1",
		import_: "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1",
		cfg: &Config{
			TFName: "User",
			Fields: map[string]FieldConfig{
				"name":     {Required: true},
				"password": {Optional: boolPtrG(true), ExpandVia: "GetEffectivePassword", FlattenSkip: true},
				"mechanism": {
					Optional:    boolPtrG(true),
					HasPresence: true,
				},
				"id":              {Extra: true, Type: "string", ComputedOnly: true, FromProto: "name"},
				"cluster_api_url": {Extra: true, Type: "string", Required: true},
				"password_wo":     {Extra: true, Type: "string", Optional: boolPtrG(true)},
				"password_wo_version": {
					Extra: true, Type: "int64", Optional: boolPtrG(true),
				},
				"allow_deletion": {
					Extra: true, Type: "bool", Optional: boolPtrG(true), Computed: boolPtrG(true),
					Default: false,
				},
			},
			API: &APIConfig{
				Create: &RPCConfig{RPC: "CreateUser", Request: "CreateUserRequest", PayloadField: "user", PayloadType: "CreateUserRequest_User"},
				Update: &RPCConfig{RPC: "UpdateUser", Request: "UpdateUserRequest", PayloadField: "user", PayloadType: "UpdateUserRequest_User"},
				Delete: &RPCConfig{RPC: "DeleteUser", Request: "DeleteUserRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "UserResponse",
				},
			},
		},
		proto: &ProtoMessage{
			Name: "User",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{Name: "password", Kind: "string", Cardinality: "singular"},
				{Name: "mechanism", Kind: "enum", Cardinality: "singular", IsOptional: false, EnumName: "redpanda.api.dataplane.v1.SASLMechanism"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "password", AttrType: AttrTypeString, Optional: true, Sensitive: true},
			{Name: "mechanism", AttrType: AttrTypeString, Optional: true},
			{Name: "id", AttrType: AttrTypeString, Computed: true},
			{Name: "cluster_api_url", AttrType: AttrTypeString, Required: true},
			{Name: "password_wo", AttrType: AttrTypeString, Optional: true, WriteOnly: true},
			{Name: "password_wo_version", AttrType: AttrTypeInt64, Optional: true},
			{Name: "allow_deletion", AttrType: AttrTypeBool, Optional: true, Computed: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteUserRequest": {
				Name: "DeleteUserRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func scalarMixGoldenCase() goldenCase {
	return goldenCase{
		name:    "scalar_mix_conv",
		pkg:     "scalarmix",
		alias:   "scalarmixv1",
		import_: "buf.build/gen/go/example/scalarmix/protocolbuffers/go/example/scalarmix/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"id":          {Required: true},
				"enabled":     {Optional: boolPtrG(true)},
				"max_count":   {Optional: boolPtrG(true)},
				"price":       {Optional: boolPtrG(true)},
				"description": {Optional: boolPtrG(true)},
				"label":       {Optional: boolPtrG(true)},
			},
			API: &APIConfig{
				Create: &RPCConfig{RPC: "Create", Request: "CreateThingRequest", PayloadField: "thing", PayloadType: "CreateThingRequest_Thing"},
				Delete: &RPCConfig{RPC: "Delete", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "id", Kind: "string", Cardinality: "singular"},
				{Name: "enabled", Kind: "bool", Cardinality: "singular"},
				{Name: "max_count", Kind: "int32", Cardinality: "singular"},
				{Name: "weight", Kind: "int64", Cardinality: "singular"},
				{Name: "price", Kind: "double", Cardinality: "singular"},
				{Name: "description", Kind: "string", Cardinality: "singular"},
				{Name: "label", Kind: "string", Cardinality: "singular", IsOptional: true},
			},
		},
		attrs: []SchemaAttr{
			{Name: "id", AttrType: AttrTypeString, Required: true},
			{Name: "enabled", AttrType: AttrTypeBool, Optional: true},
			{Name: "max_count", AttrType: AttrTypeInt32, Optional: true},
			{Name: "weight", AttrType: AttrTypeInt64, Optional: true},
			{Name: "price", AttrType: AttrTypeFloat64, Optional: true},
			{Name: "description", AttrType: AttrTypeString, Optional: true},
			{Name: "label", AttrType: AttrTypeString, Optional: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "id", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func fromProtoGoldenCase() goldenCase {
	return goldenCase{
		name:    "from_proto_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name":  {Required: true},
				"id":    {Extra: true, Type: "string", ComputedOnly: true, FromProto: "name"},
				"state": {Extra: true, Type: "string", ComputedOnly: true},
			},
			API: &APIConfig{
				Create: &RPCConfig{Request: "CreateThingRequest", PayloadField: "thing", PayloadType: "CreateThingRequest_Thing"},
				Delete: &RPCConfig{Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "id", AttrType: AttrTypeString, Computed: true},
			{Name: "state", AttrType: AttrTypeString, Computed: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func divergentPayloadGoldenCase() goldenCase {
	return goldenCase{
		name:    "divergent_payload_conv",
		pkg:     "rg",
		alias:   "controlplanev1",
		import_: "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1",
		cfg: &Config{
			TFName: "ResourceGroup",
			Fields: map[string]FieldConfig{
				"name": {Required: true},
				"id":   {ComputedOnly: true},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateResourceGroup", Request: "CreateResourceGroupRequest",
					PayloadField: "resource_group", PayloadType: "ResourceGroup",
				},
				Update: &RPCConfig{
					RPC: "UpdateResourceGroup", Request: "UpdateResourceGroupRequest",
					PayloadField: "resource_group", PayloadType: "ResourceGroupUpdate",
				},
				Delete: &RPCConfig{RPC: "DeleteResourceGroup", Request: "DeleteResourceGroupRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ResourceGroupResponse",
				},
			},
		},
		proto: &ProtoMessage{
			Name: "ResourceGroup",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "id", AttrType: AttrTypeString, Computed: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"ResourceGroupUpdate": {
				Name: "ResourceGroupUpdate",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
					{Name: "id", Kind: "string", Cardinality: "singular"},
				},
			},

			"DeleteResourceGroupRequest": {
				Name: "DeleteResourceGroupRequest",
				Fields: []ProtoField{
					{Name: "id", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func multiFlatGoldenCase() goldenCase {
	return goldenCase{
		name:    "multi_flat_conv",
		pkg:     "role",
		alias:   "dataplanev1",
		import_: "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1",
		cfg: &Config{
			TFName: "Role",
			Fields: map[string]FieldConfig{
				"name":        {Required: true},
				"id":          {Extra: true, Type: "string", ComputedOnly: true, FromProto: "name"},
				"delete_acls": {Extra: true, Type: "bool", Optional: boolPtrG(true), Default: false},
			},
			API: &APIConfig{
				Create: &RPCConfig{RPC: "CreateRole", Request: "CreateRoleRequest", PayloadField: "role", PayloadType: "Role"},
				Delete: &RPCConfig{
					RPC:          "DeleteRole",
					Request:      "DeleteRoleRequest",
					FlatFieldMap: map[string]string{"role_name": "name"},
				},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "RoleResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Role",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "id", AttrType: AttrTypeString, Computed: true},
			{Name: "delete_acls", AttrType: AttrTypeBool, Optional: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteRoleRequest": {
				Name: "DeleteRoleRequest",
				Fields: []ProtoField{
					{Name: "role_name", Kind: "string", Cardinality: "singular"},
					{Name: "delete_acls", Kind: "bool", Cardinality: "singular"},
				},
			},
		},
	}
}

func pointerStringGoldenCase() goldenCase {
	return goldenCase{
		name:    "pointer_string_conv",
		pkg:     "filterable",
		alias:   "filterablev1",
		import_: "buf.build/gen/go/example/filterable/protocolbuffers/go/example/filterable/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"resource_type": {Required: true},
				"resource_name": {Required: true},
				"principal":     {Required: true},
				"host":          {Required: true},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "DeleteThings", Request: "DeleteThingsRequest",
					PayloadField: "filter", PayloadType: "DeleteThingsRequest_Filter",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},

		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "resource_type", Kind: "string", Cardinality: "singular"},
				{Name: "resource_name", Kind: "string", Cardinality: "singular"},
				{Name: "principal", Kind: "string", Cardinality: "singular"},
				{Name: "host", Kind: "string", Cardinality: "singular"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "resource_type", AttrType: AttrTypeString, Required: true},
			{Name: "resource_name", AttrType: AttrTypeString, Required: true},
			{Name: "principal", AttrType: AttrTypeString, Required: true},
			{Name: "host", AttrType: AttrTypeString, Required: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingsRequest_Filter": {
				Name: "DeleteThingsRequest_Filter",
				Fields: []ProtoField{
					{Name: "resource_type", Kind: "string", Cardinality: "singular"},
					{Name: "resource_name", Kind: "string", Cardinality: "singular", IsOptional: true},
					{Name: "principal", Kind: "string", Cardinality: "singular", IsOptional: true},
					{Name: "host", Kind: "string", Cardinality: "singular", IsOptional: true},
				},
			},
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "resource_type", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func listScalarKindsGoldenCase() goldenCase {
	return goldenCase{
		name:    "list_scalar_kinds_conv",
		pkg:     "scalarlists",
		alias:   "scalarlistsv1",
		import_: "buf.build/gen/go/example/scalarlists/protocolbuffers/go/example/scalarlists/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name":    {Required: true},
				"tags":    {Optional: boolPtrG(true)},
				"ids":     {Optional: boolPtrG(true)},
				"counts":  {Optional: boolPtrG(true)},
				"flags":   {Optional: boolPtrG(true)},
				"weights": {Optional: boolPtrG(true)},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "Thing",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{Name: "tags", Kind: "string", Cardinality: "repeated"},
				{Name: "ids", Kind: "int32", Cardinality: "repeated"},
				{Name: "counts", Kind: "int64", Cardinality: "repeated"},
				{Name: "flags", Kind: "bool", Cardinality: "repeated"},
				{Name: "weights", Kind: "double", Cardinality: "repeated"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "tags", AttrType: AttrTypeList, ElementType: "types.StringType", Optional: true},
			{Name: "ids", AttrType: AttrTypeList, ElementType: "types.Int32Type", Optional: true},
			{Name: "counts", AttrType: AttrTypeList, ElementType: "types.Int64Type", Optional: true},
			{Name: "flags", AttrType: AttrTypeList, ElementType: "types.BoolType", Optional: true},
			{Name: "weights", AttrType: AttrTypeList, ElementType: "types.Float64Type", Optional: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func listMessageGoldenCase() goldenCase {
	return goldenCase{
		name:    "list_message_conv",
		pkg:     "topic",
		alias:   "dataplanev1",
		import_: "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1",
		cfg: &Config{
			TFName: "Topic",
			Fields: map[string]FieldConfig{
				"name": {Required: true},
				"replica_assignments": {
					Optional: boolPtrG(true),
					Fields: map[string]FieldConfig{
						"partition_id": {Required: true},
						"replica_ids":  {Required: true},
					},
				},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateTopic", Request: "CreateTopicRequest",
					PayloadField: "topic", PayloadType: "CreateTopicRequest_Topic",
				},
				Delete: &RPCConfig{
					RPC: "DeleteTopic", Request: "DeleteTopicRequest",
					FlatFieldMap: map[string]string{"topic_name": "name"},
				},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "TopicResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name:   "Topic",
			GoName: "CreateTopicRequest_Topic",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{
					Name:        "replica_assignments",
					Kind:        "message",
					Cardinality: "repeated",
					Nested: &ProtoMessage{
						Name:   "ReplicaAssignment",
						GoName: "CreateTopicRequest_Topic_ReplicaAssignment",
						Fields: []ProtoField{
							{Name: "partition_id", Kind: "int32", Cardinality: "singular"},
							{Name: "replica_ids", Kind: "int32", Cardinality: "repeated"},
						},
					},
				},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{
				Name:     "replica_assignments",
				AttrType: AttrTypeListNested,
				Optional: true,
				NestedAttrs: []SchemaAttr{
					{Name: "partition_id", AttrType: AttrTypeInt32, Required: true},
					{Name: "replica_ids", AttrType: AttrTypeList, ElementType: "types.Int32Type", Required: true},
				},
			},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteTopicRequest": {
				Name: "DeleteTopicRequest",
				Fields: []ProtoField{
					{Name: "topic_name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func numberAttrGoldenCase() goldenCase {
	return goldenCase{
		name:    "number_attr_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name":               {Required: true},
				"partition_count":    {Optional: boolPtrG(true), Computed: boolPtrG(true), ForceType: AttrTypeNumber},
				"replication_factor": {Optional: boolPtrG(true), Computed: boolPtrG(true), ForceType: AttrTypeNumber},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "Thing",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{Name: "partition_count", Kind: "int32", Cardinality: "singular", IsOptional: true},
				{Name: "replication_factor", Kind: "int32", Cardinality: "singular", IsOptional: true},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "partition_count", AttrType: AttrTypeNumber, Optional: true, Computed: true},
			{Name: "replication_factor", AttrType: AttrTypeNumber, Optional: true, Computed: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func oneofVariantGoldenCase() goldenCase {
	return goldenCase{
		name:    "oneof_variant_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name": {Required: true},
				"customer_managed_resources": {
					Optional: boolPtrG(true),
					Fields: map[string]FieldConfig{
						"aws": {Optional: boolPtrG(true)},
						"gcp": {Optional: boolPtrG(true)},
					},
				},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "ThingCreate",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name:   "Thing",
			GoName: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{
					Name:        "customer_managed_resources",
					Kind:        "message",
					Cardinality: "singular",
					IsOptional:  true,
					Nested: &ProtoMessage{
						Name:   "CustomerManagedResources",
						GoName: "Thing_CustomerManagedResources",
						Fields: []ProtoField{
							{
								Name:        "aws",
								Kind:        "message",
								Cardinality: "singular",
								OneofName:   "cloud_provider",
								Nested: &ProtoMessage{
									Name:   "Aws",
									GoName: "Thing_CustomerManagedResources_AWS",
									Fields: []ProtoField{
										{Name: "arn", Kind: "string", Cardinality: "singular"},
									},
								},
							},
							{
								Name:        "gcp",
								Kind:        "message",
								Cardinality: "singular",
								OneofName:   "cloud_provider",
								Nested: &ProtoMessage{
									Name:   "Gcp",
									GoName: "Thing_CustomerManagedResources_GCP",
									Fields: []ProtoField{
										{Name: "name", Kind: "string", Cardinality: "singular"},
									},
								},
							},
						},
					},
				},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{
				Name:     "customer_managed_resources",
				AttrType: AttrTypeSingleNested,
				Optional: true,
				NestedAttrs: []SchemaAttr{
					{
						Name:     "aws",
						AttrType: AttrTypeSingleNested,
						Optional: true,
						NestedAttrs: []SchemaAttr{
							{Name: "arn", AttrType: AttrTypeString, Required: true},
						},
					},
					{
						Name:     "gcp",
						AttrType: AttrTypeSingleNested,
						Optional: true,
						NestedAttrs: []SchemaAttr{
							{Name: "name", AttrType: AttrTypeString, Required: true},
						},
					},
				},
			},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func timestampScalarGoldenCase() goldenCase {
	return goldenCase{
		name:    "timestamp_scalar_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name":         {Required: true},
				"delete_after": {Optional: boolPtrG(true), Computed: boolPtrG(true)},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "Thing",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{Name: "delete_after", Kind: "timestamp", Cardinality: "singular"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "delete_after", AttrType: AttrTypeString, Optional: true, Computed: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func mapScalarGoldenCase() goldenCase {
	return goldenCase{
		name:    "map_scalar_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name": {Required: true},
				"tags": {Optional: boolPtrG(true)},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "Thing",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{
					Name:        "tags",
					Kind:        "string",
					Cardinality: "map",
					MapKeyKind:  "string",
					MapValKind:  "string",
				},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{Name: "tags", AttrType: AttrTypeMap, ElementType: "types.StringType", Optional: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func perRPCDivergentNestedGoldenCase() goldenCase {
	return goldenCase{
		name:    "per_rpc_divergent_nested_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name": {Required: true},
				"kafka_api": {
					Optional: boolPtrG(true),
					Computed: boolPtrG(true),
					Fields: map[string]FieldConfig{
						"seed_brokers": {ComputedOnly: true},
					},
				},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "ThingCreate",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},

		proto: &ProtoMessage{
			Name:   "Thing",
			GoName: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{
					Name:        "kafka_api",
					Kind:        "message",
					Cardinality: "singular",
					Nested: &ProtoMessage{
						Name:   "KafkaAPI",
						GoName: "Thing_KafkaAPI",
						Fields: []ProtoField{
							{Name: "seed_brokers", Kind: "string", Cardinality: "repeated"},
						},
					},
				},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{
				Name: "kafka_api", AttrType: AttrTypeSingleNested,
				Optional: true, Computed: true,
				NestedAttrs: []SchemaAttr{
					{Name: "seed_brokers", AttrType: AttrTypeList, ElementType: "types.StringType", Computed: true},
				},
			},
		},
		extraProtos: map[string]*ProtoMessage{
			"ThingCreate": {
				Name:   "ThingCreate",
				GoName: "ThingCreate",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
					{
						Name:        "kafka_api",
						Kind:        "message",
						Cardinality: "singular",
						Nested: &ProtoMessage{
							Name:   "KafkaAPISpec",
							GoName: "KafkaAPISpec",
							Fields: []ProtoField{
								{Name: "name", Kind: "string", Cardinality: "singular"},
							},
						},
					},
				},
			},
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func forceBoolOneofGoldenCase() goldenCase {
	return goldenCase{
		name:    "force_bool_oneof_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"name": {Required: true},
				"window": {
					Optional: boolPtrG(true),
					Fields: map[string]FieldConfig{
						"anytime":     {Optional: boolPtrG(true), ForceType: AttrTypeBool},
						"unspecified": {Optional: boolPtrG(true), ForceType: AttrTypeBool},
					},
				},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "Thing",
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
			ExcludeOperations: []string{"update"},
		},
		proto: &ProtoMessage{
			Name:   "Thing",
			GoName: "Thing",
			Fields: []ProtoField{
				{Name: "name", Kind: "string", Cardinality: "singular"},
				{
					Name:        "window",
					Kind:        "message",
					Cardinality: "singular",
					IsOptional:  true,
					Nested: &ProtoMessage{
						Name:   "Window",
						GoName: "Thing_Window",
						Fields: []ProtoField{
							{
								Name:        "anytime",
								Kind:        "message",
								Cardinality: "singular",
								OneofName:   "kind",
								Nested: &ProtoMessage{
									Name:   "Anytime",
									GoName: "Thing_Window_Anytime",
								},
							},
							{
								Name:        "unspecified",
								Kind:        "message",
								Cardinality: "singular",
								OneofName:   "kind",
								Nested: &ProtoMessage{
									Name:   "Unspecified",
									GoName: "Thing_Window_Unspecified",
								},
							},
						},
					},
				},
			},
		},
		attrs: []SchemaAttr{
			{Name: "name", AttrType: AttrTypeString, Required: true},
			{
				Name:     "window",
				AttrType: AttrTypeSingleNested,
				Optional: true,
				NestedAttrs: []SchemaAttr{
					{Name: "anytime", AttrType: AttrTypeBool, Optional: true},
					{Name: "unspecified", AttrType: AttrTypeBool, Optional: true},
				},
			},
		},
		extraProtos: map[string]*ProtoMessage{
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func returnPayloadUpdateGoldenCase() goldenCase {
	return goldenCase{
		name:    "return_payload_update_conv",
		pkg:     "thing",
		alias:   "thingv1",
		import_: "buf.build/gen/go/example/thing/protocolbuffers/go/example/thing/v1",
		cfg: &Config{
			TFName: "Thing",
			Fields: map[string]FieldConfig{
				"id":   {ComputedOnly: true},
				"name": {Required: true},
			},
			API: &APIConfig{
				Create: &RPCConfig{
					RPC: "CreateThing", Request: "CreateThingRequest",
					PayloadField: "thing", PayloadType: "ThingCreate",
				},
				Update: &RPCConfig{
					RPC: "UpdateThing", Request: "UpdateThingRequest",
					PayloadField: "thing", PayloadType: "ThingUpdate",
					ReturnPayload: true,
				},
				Delete: &RPCConfig{RPC: "DeleteThing", Request: "DeleteThingRequest"},
				ResponseInterface: &ResponseInterfaceConfig{
					Name: "ThingResponse",
				},
			},
		},
		proto: &ProtoMessage{
			Name:   "Thing",
			GoName: "Thing",
			Fields: []ProtoField{
				{Name: "id", Kind: "string", Cardinality: "singular"},
				{Name: "name", Kind: "string", Cardinality: "singular"},
			},
		},
		attrs: []SchemaAttr{
			{Name: "id", AttrType: AttrTypeString, Computed: true},
			{Name: "name", AttrType: AttrTypeString, Required: true},
		},
		extraProtos: map[string]*ProtoMessage{
			"ThingCreate": {
				Name: "ThingCreate", GoName: "ThingCreate",
				Fields: []ProtoField{
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
			"ThingUpdate": {
				Name: "ThingUpdate", GoName: "ThingUpdate",
				Fields: []ProtoField{
					{Name: "id", Kind: "string", Cardinality: "singular"},
					{Name: "name", Kind: "string", Cardinality: "singular"},
				},
			},
			"DeleteThingRequest": {
				Name: "DeleteThingRequest",
				Fields: []ProtoField{
					{Name: "id", Kind: "string", Cardinality: "singular"},
				},
			},
		},
	}
}

func boolPtrG(b bool) *bool { return &b }
