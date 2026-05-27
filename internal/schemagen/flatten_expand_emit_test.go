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
	"go/parser"
	"go/token"
	"testing"
)

// TestGenerateFlattenExpandParses verifies the user-resource emission produces
// valid Go that parses cleanly. Catches template/format regressions even when
// the project doesn't compile against the real proto types yet.
func TestGenerateFlattenExpandParses(t *testing.T) {
	cfg := &Config{
		TFName: "User",
		Fields: map[string]FieldConfig{
			"name":     {Required: true},
			"password": {Optional: boolPtr(true), ExpandVia: "GetEffectivePassword", FlattenSkip: true},
			"mechanism": {
				Optional: boolPtr(true),
			},
			"id":              {Extra: true, Type: "string", ComputedOnly: true, FromProto: "name"},
			"cluster_api_url": {Extra: true, Type: "string", Required: true},
			"allow_deletion":  {Extra: true, Type: "bool", Optional: boolPtr(true), Default: false},
		},
		API: &APIConfig{
			Create: &RPCConfig{Request: "CreateUserRequest", PayloadField: "user", PayloadType: "CreateUserRequest_User"},
			Update: &RPCConfig{Request: "UpdateUserRequest", PayloadField: "user", PayloadType: "UpdateUserRequest_User"},
			Delete: &RPCConfig{Request: "DeleteUserRequest"},
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
		{Name: "name", AttrType: AttrTypeString},
		{Name: "password", AttrType: AttrTypeString},
		{Name: "mechanism", AttrType: AttrTypeString},
		{Name: "id", AttrType: AttrTypeString},
		{Name: "cluster_api_url", AttrType: AttrTypeString},
		{Name: "allow_deletion", AttrType: AttrTypeBool},
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
		t.Fatalf("GenerateFlattenExpand: %v\n--- GOT ---\n%s", err, src)
	}

	fset := token.NewFileSet()
	if _, perr := parser.ParseFile(fset, "flatten_expand_gen.go", src, parser.AllErrors); perr != nil {
		t.Fatalf("generated source does not parse: %v\n--- SOURCE ---\n%s", perr, src)
	}
}
