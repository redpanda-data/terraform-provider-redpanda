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

func makeDriftProto() *ProtoMessage {
	return &ProtoMessage{
		Name: "Thing",
		Fields: []ProtoField{
			{Name: "name", Kind: KindString, Cardinality: "singular"},
			{Name: "nested", Kind: KindMessage, Cardinality: "singular", Nested: &ProtoMessage{
				Name: "ThingNested",
				Fields: []ProtoField{
					{Name: "value", Kind: KindString, Cardinality: "singular"},
				},
			}},
		},
	}
}

func TestMerge_StaleConfigKey_Errors(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":            {Required: true},
			"gone_from_proto": {Required: true},
		},
	}
	_, _, _, errs := Merge(makeDriftProto(), cfg, "resource", nil)
	if len(errs) == 0 {
		t.Fatal("expected an error for a stale yaml key with no matching proto field")
	}
	if !containsErr(errs, "gone_from_proto") {
		t.Fatalf("error should name the missing field; got %v", errs)
	}
}

func TestMerge_UnknownValidator_Errors(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name": {Required: true, Validator: "NotRegisteredAnywhere"},
		},
	}
	_, _, _, errs := Merge(makeDriftProto(), cfg, "resource", nil)
	if len(errs) == 0 {
		t.Fatal("expected an error for an unknown validator name")
	}
	if !containsErr(errs, "NotRegisteredAnywhere") {
		t.Fatalf("error should name the unknown validator; got %v", errs)
	}
}

func TestMerge_FromProtoMissingField_Errors(t *testing.T) {
	optTrue := true
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"id": {
				Extra:     true,
				Type:      "string",
				Optional:  &optTrue,
				FromProto: "vanished_field",
			},
		},
	}
	_, _, _, errs := Merge(makeDriftProto(), cfg, "resource", nil)
	if len(errs) == 0 {
		t.Fatal("expected an error for from_proto referencing a missing field")
	}
	if !containsErr(errs, "vanished_field") {
		t.Fatalf("error should name the missing from_proto target; got %v", errs)
	}
}

func TestMerge_StaleTodo_Errors(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":          {Required: true},
			"used_to_exist": {Todo: true},
		},
	}
	_, _, _, errs := Merge(makeDriftProto(), cfg, "resource", nil)
	if len(errs) == 0 {
		t.Fatal("expected an error for stale todo entry")
	}
	if !containsErr(errs, "stale todo") || !containsErr(errs, "used_to_exist") {
		t.Fatalf("error should flag the stale todo by name; got %v", errs)
	}
}

func TestMerge_LiveTodo_NoError(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":   {Required: true},
			"nested": {Todo: true},
		},
	}
	_, _, _, errs := Merge(makeDriftProto(), cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("live todo should not error; got %v", errs)
	}
}

func TestMerge_CleanCfg_NoErrors(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":   {Required: true},
			"nested": {},
		},
	}
	_, _, _, errs := Merge(makeDriftProto(), cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("expected zero errors for clean cfg; got %v", errs)
	}
}

func containsErr(errs []error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return true
		}
	}
	return false
}
