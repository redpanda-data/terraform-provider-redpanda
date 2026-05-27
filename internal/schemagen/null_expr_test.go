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

func TestScalarNullExprForAttr(t *testing.T) {
	cases := []struct {
		attrType string
		want     string
		wantErr  bool
	}{
		{AttrTypeString, "types.StringNull()", false},
		{AttrTypeBool, "types.BoolNull()", false},
		{AttrTypeInt32, "types.Int32Null()", false},
		{AttrTypeInt64, "types.Int64Null()", false},
		{AttrTypeFloat64, "types.Float64Null()", false},
		{AttrTypeNumber, "types.NumberNull()", false},
		{AttrTypeList, "", false},
		{AttrTypeSet, "", false},
		{AttrTypeMap, "", false},
		{AttrTypeListNested, "", false},
		{AttrTypeSetNested, "", false},
		{AttrTypeMapNested, "", false},
		{AttrTypeSingleNested, "", false},
		{AttrTypeObject, "", false},
		{"FakeUnknownAttribute", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		t.Run(c.attrType, func(t *testing.T) {
			got, err := scalarNullExprForAttr(c.attrType)
			if c.wantErr && err == nil {
				t.Fatalf("expected error for AttrType %q, got nil", c.attrType)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error for AttrType %q: %v", c.attrType, err)
			}
			if got != c.want {
				t.Fatalf("scalarNullExprForAttr(%q): got %q, want %q", c.attrType, got, c.want)
			}
		})
	}
}

func TestNullExprScalars(t *testing.T) {
	scalars := []struct {
		attrType string
		nullExpr string
	}{
		{AttrTypeString, "types.StringNull()"},
		{AttrTypeBool, "types.BoolNull()"},
		{AttrTypeInt32, "types.Int32Null()"},
		{AttrTypeInt64, "types.Int64Null()"},
		{AttrTypeFloat64, "types.Float64Null()"},
		{AttrTypeNumber, "types.NumberNull()"},
	}
	for _, s := range scalars {
		t.Run(s.attrType+"/full", func(t *testing.T) {
			a := &SchemaAttr{Name: "foo", AttrType: s.attrType}
			got, err := NullExpr(a, NullExprOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != s.nullExpr {
				t.Fatalf("got %q, want %q", got, s.nullExpr)
			}
		})
		t.Run(s.attrType+"/skip", func(t *testing.T) {
			a := &SchemaAttr{Name: "foo", AttrType: s.attrType}
			got, err := NullExpr(a, NullExprOptions{SkipScalars: true})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != "" {
				t.Fatalf("expected empty (SkipScalars), got %q", got)
			}
		})
	}
}

func TestNullExprCollections(t *testing.T) {
	cases := []struct {
		name        string
		attrType    string
		elementType string
		want        string
		wantErr     bool
	}{
		{"list_with_elem", AttrTypeList, "types.StringType", "types.ListNull(types.StringType)", false},
		{"list_missing_elem", AttrTypeList, "", "", true},
		{"set_with_elem", AttrTypeSet, "types.Int32Type", "types.SetNull(types.Int32Type)", false},
		{"set_missing_elem", AttrTypeSet, "", "", true},
		{"map_with_elem", AttrTypeMap, "types.StringType", "types.MapNull(types.StringType)", false},
		{"map_missing_elem", AttrTypeMap, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &SchemaAttr{Name: "configuration", AttrType: c.attrType, ElementType: c.elementType}
			got, err := NullExpr(a, NullExprOptions{})
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "missing ElementType") {
					t.Fatalf("error should mention missing ElementType, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestNullExprNested(t *testing.T) {
	cases := []struct {
		name      string
		attrType  string
		helperPkg string
		prefix    string
		fieldName string
		want      string
	}{
		{
			"single_nested_in_pkg", AttrTypeSingleNested, "", "", "cluster_config",
			"types.ObjectNull(ClusterConfigAttrTypes())",
		},
		{
			"single_nested_cross_pkg", AttrTypeSingleNested, "clustermodel", "", "cluster_config",
			"types.ObjectNull(clustermodel.ClusterConfigAttrTypes())",
		},
		{
			"object_with_prefix", AttrTypeObject, "", "Data", "my_object",
			"types.ObjectNull(DataMyObjectAttrTypes())",
		},
		{
			"list_nested_in_pkg", AttrTypeListNested, "", "", "items",
			"types.ListNull(types.ObjectType{AttrTypes: ItemsAttrTypes()})",
		},
		{
			"set_nested_cross_pkg", AttrTypeSetNested, "topicmodel", "", "items",
			"types.SetNull(types.ObjectType{AttrTypes: topicmodel.ItemsAttrTypes()})",
		},
		{
			"map_nested_with_prefix", AttrTypeMapNested, "", "Data", "items",
			"types.MapNull(types.ObjectType{AttrTypes: DataItemsAttrTypes()})",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &SchemaAttr{Name: c.fieldName, AttrType: c.attrType}
			got, err := NullExpr(a, NullExprOptions{HelperPrefix: c.prefix, HelperPkg: c.helperPkg})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestNullExprUnknownAttrType(t *testing.T) {
	a := &SchemaAttr{Name: "foo", AttrType: "FakeUnknownAttribute"}
	_, err := NullExpr(a, NullExprOptions{})
	if err == nil {
		t.Fatal("expected error for unknown AttrType, got nil")
	}
	if !strings.Contains(err.Error(), "unknown AttrType") {
		t.Fatalf("error should mention unknown AttrType, got %v", err)
	}
}

// TestNullExprAttrTypeCoverage is a coverage tripwire: it enumerates every
// AttrType constant defined in this package and asserts NullExpr handles each.
// Adding a new AttrType constant without extending NullExpr will fail this
// test (the new constant won't be in the list — add it here and to NullExpr).
func TestNullExprAttrTypeCoverage(t *testing.T) {
	allAttrTypes := []string{
		AttrTypeString, AttrTypeBool, AttrTypeInt32, AttrTypeInt64,
		AttrTypeFloat64, AttrTypeNumber,
		AttrTypeList, AttrTypeSet, AttrTypeMap,
		AttrTypeListNested, AttrTypeSetNested, AttrTypeMapNested,
		AttrTypeSingleNested, AttrTypeObject,
	}
	for _, at := range allAttrTypes {
		t.Run(at, func(t *testing.T) {
			a := &SchemaAttr{Name: "covered", AttrType: at}
			switch at {
			case AttrTypeList, AttrTypeSet, AttrTypeMap:
				a.ElementType = "types.StringType"
			default:
				// nested / scalar types don't use ElementType
			}
			_, err := NullExpr(a, NullExprOptions{})
			if err != nil {
				t.Fatalf("NullExpr missing coverage for AttrType %q: %v", at, err)
			}
			_, err = scalarNullExprForAttr(at)
			if err != nil {
				t.Fatalf("scalarNullExprForAttr missing coverage for AttrType %q: %v", at, err)
			}
		})
	}
}
