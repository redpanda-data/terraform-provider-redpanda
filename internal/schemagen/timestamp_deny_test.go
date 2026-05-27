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
	"testing"
)

// TestApplyTimestampDeny_RecursesIntoNested verifies the deny recurses into
// nested messages, not just the root.
func TestApplyTimestampDeny_RecursesIntoNested(t *testing.T) {
	attrs := []SchemaAttr{
		{Name: "name", AttrType: AttrTypeString},
		{Name: "created_at", AttrType: AttrTypeString},
		{
			Name:     "metadata",
			AttrType: AttrTypeSingleNested,
			NestedAttrs: []SchemaAttr{
				{Name: "owner", AttrType: AttrTypeString},
				{Name: "created_at", AttrType: AttrTypeString},
				{Name: "deleted_at", AttrType: AttrTypeString},
				{Name: "create_time", AttrType: AttrTypeString},
			},
		},
		{
			Name:     "items",
			AttrType: AttrTypeListNested,
			NestedAttrs: []SchemaAttr{
				{Name: "id", AttrType: AttrTypeString},
				{Name: "updated_at", AttrType: AttrTypeString},
				{Name: "update_time", AttrType: AttrTypeString},
			},
		},
	}
	applyTimestampDeny(&attrs)

	if hasAttrNamed(attrs, "created_at") {
		t.Error("root created_at should have been removed")
	}
	metadata := findAttrNamed(attrs, "metadata")
	if metadata == nil {
		t.Fatal("metadata attr missing after deny")
	}
	for _, banned := range []string{"created_at", "deleted_at", "create_time"} {
		if hasAttrNamed(metadata.NestedAttrs, banned) {
			t.Errorf("metadata.%s should have been removed", banned)
		}
	}
	if !hasAttrNamed(metadata.NestedAttrs, "owner") {
		t.Error("metadata.owner should still be present")
	}
	items := findAttrNamed(attrs, "items")
	if items == nil {
		t.Fatal("items attr missing after deny")
	}
	for _, banned := range []string{"updated_at", "update_time"} {
		if hasAttrNamed(items.NestedAttrs, banned) {
			t.Errorf("items.%s should have been removed", banned)
		}
	}
	if !hasAttrNamed(items.NestedAttrs, "id") {
		t.Error("items.id should still be present")
	}
}

// TestApplyFieldConfigs_BlocksTimestampExtras locks the strict policy:
// yaml entries for deny-listed names — extras, synthetics, regular
// overrides — must never produce a SchemaAttr.
func TestApplyFieldConfigs_BlocksTimestampExtras(t *testing.T) {
	attrs := []SchemaAttr{{Name: "name", AttrType: AttrTypeString}}
	extras := true
	cfg := map[string]FieldConfig{
		"created_at": {Extra: true, Type: "string", ComputedOnly: true},
		"updated_at": {Synthetic: true, Type: "string", ComputedOnly: true},
		"delete_time": {
			Required: true,
			Optional: &extras,
		},
	}
	var imports []string
	var errs []error
	applyFieldConfigs(&attrs, cfg, nil, "", &mergeCtx{extraImports: &imports, errs: &errs})

	for _, banned := range []string{"created_at", "updated_at", "delete_time"} {
		if hasAttrNamed(attrs, banned) {
			t.Errorf("yaml entry %q must not produce a SchemaAttr — strict deny", banned)
		}
	}
	if !hasAttrNamed(attrs, "name") {
		t.Error("non-banned attr should still be present")
	}
}

// TestFindUncovered_SkipsNestedTimestamps mirrors the deny — uncovered
// warnings should not fire for deny-listed timestamp fields at any
// nesting level.
func TestFindUncovered_SkipsNestedTimestamps(t *testing.T) {
	proto := &ProtoMessage{
		Name: "Root",
		Fields: []ProtoField{
			{Name: "name", Kind: "string", Cardinality: "singular"},
			{Name: "created_at", Kind: "timestamp", Cardinality: "singular"},
			{
				Name:        "metadata",
				Kind:        "message",
				Cardinality: "singular",
				Nested: &ProtoMessage{
					Name: "Metadata",
					Fields: []ProtoField{
						{Name: "owner", Kind: "string", Cardinality: "singular"},
						{Name: "created_at", Kind: "timestamp", Cardinality: "singular"},
					},
				},
			},
		},
	}
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name": {Required: true},
			"metadata": {
				Fields: map[string]FieldConfig{
					"owner": {Required: true},
				},
			},
		},
	}

	got := FindUncoveredFields(proto, cfg)
	for _, uf := range got {
		if uf.Path == "created_at" || uf.Path == "metadata.created_at" {
			t.Errorf("deny-listed field reported as uncovered: %s", uf.Path)
		}
	}
}

func hasAttrNamed(attrs []SchemaAttr, name string) bool {
	return findAttrNamed(attrs, name) != nil
}

func findAttrNamed(attrs []SchemaAttr, name string) *SchemaAttr {
	for i := range attrs {
		if attrs[i].Name == name {
			return &attrs[i]
		}
	}
	return nil
}
