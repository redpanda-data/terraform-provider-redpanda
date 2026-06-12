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

package schemagen

import (
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/apidesc"
)

func descTestIndex() *apidesc.Index {
	return apidesc.FromFile(&apidesc.File{
		Schemas: map[string]*apidesc.Node{
			"Foo": {
				Fields: map[string]*apidesc.Node{
					"bounded": {Description: "API text for bounded."},
					"plain":   {Description: "API text for plain."},
				},
			},
		},
	}, "test-fixture")
}

func withScopedDescription(t *testing.T, key, text string) {
	t.Helper()
	prev, had := scopedDescriptions[key]
	scopedDescriptions[key] = text
	t.Cleanup(func() {
		if had {
			scopedDescriptions[key] = prev
		} else {
			delete(scopedDescriptions, key)
		}
	})
}

// TestMerge_ScopedDescriptionBeatsAPIDesc — scoped table entries describe
// provider behavior the API cannot know; they must win over apidesc text.
func TestMerge_ScopedDescriptionBeatsAPIDesc(t *testing.T) {
	withScopedDescription(t, "Foo.bounded", "Provider-behavior text.")
	proto := &ProtoMessage{
		Name: "Foo",
		Fields: []ProtoField{
			{Name: "bounded", Kind: KindString, Cardinality: "singular"},
			{Name: "plain", Kind: KindString, Cardinality: "singular"},
		},
	}
	cfg := &Config{APISchema: "Foo", API: &APIConfig{}}
	attrs, _, _, errs := Merge(proto, cfg, "resource", descTestIndex())
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, a := range attrs {
		switch a.Name {
		case "bounded":
			if a.Description != "Provider-behavior text." {
				t.Errorf("bounded: scoped table must beat apidesc; got %q", a.Description)
			}
		case "plain":
			if a.Description != "API text for plain." {
				t.Errorf("plain: apidesc must apply; got %q", a.Description)
			}
		default:
		}
	}
}

// TestMerge_CommonDescriptionOnExtraField — TF-only fields take the shared
// plain-name table text, including nested extras.
func TestMerge_CommonDescriptionOnExtraField(t *testing.T) {
	tru := true
	proto := &ProtoMessage{
		Name: "Foo",
		Fields: []ProtoField{
			{Name: "service_account", Kind: KindMessage, Cardinality: "singular", Nested: &ProtoMessage{
				Name:   "SA",
				Fields: []ProtoField{{Name: "client_id", Kind: KindString, Cardinality: "singular"}},
			}},
		},
	}
	cfg := &Config{
		APISchema: "Foo",
		Fields: map[string]FieldConfig{
			"allow_deletion": {Extra: true, Type: "bool", Optional: &tru},
			"service_account": {Fields: map[string]FieldConfig{
				"secret_version": {Extra: true, Type: "string", Optional: &tru},
			}},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, a := range attrs {
		switch a.Name {
		case "allow_deletion":
			if a.Description != commonDescriptions["allow_deletion"] {
				t.Errorf("allow_deletion: want shared table text; got %q", a.Description)
			}
		case "service_account":
			for _, n := range a.NestedAttrs {
				if n.Name == "secret_version" && n.Description != commonDescriptions["secret_version"] {
					t.Errorf("nested secret_version: want shared table text; got %q", n.Description)
				}
			}
		default:
		}
	}
}

// TestMerge_ScopedDescriptionOnExtraField — the Cluster.tags shape: an extra
// field with a scoped entry uses it over the plain-name/mechanical text.
func TestMerge_ScopedDescriptionOnExtraField(t *testing.T) {
	withScopedDescription(t, "Foo.tags", "Tags with provider-side filtering.")
	tru := true
	proto := &ProtoMessage{
		Name:   "Foo",
		Fields: []ProtoField{{Name: "cloud_provider_tags", Kind: KindMap, MapValKind: KindString, Cardinality: KindMap}},
	}
	cfg := &Config{
		APISchema: "Foo",
		Fields: map[string]FieldConfig{
			"cloud_provider_tags": {ProtoOnly: true},
			"tags": {
				Extra: true, Type: "map", ElementType: "string",
				FromProto: "cloud_provider_tags", Optional: &tru, Computed: &tru,
			},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, a := range attrs {
		if a.Name == "tags" && a.Description != "Tags with provider-side filtering." {
			t.Errorf("tags: want scoped text; got %q", a.Description)
		}
	}
}
