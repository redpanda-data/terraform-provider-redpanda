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

package schemagen

import (
	"strings"
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

// TestMerge_ScopedDescriptionBeatsAPIDesc: scoped table entries describe
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

func withResourceScopedDescription(t *testing.T, key, text string) {
	t.Helper()
	prev, had := resourceScopedDescriptions[key]
	resourceScopedDescriptions[key] = text
	t.Cleanup(func() {
		if had {
			resourceScopedDescriptions[key] = prev
		} else {
			delete(resourceScopedDescriptions, key)
		}
	})
}

// TestMerge_ResourceScopedDescriptionSkipsDatasource: text about configuring
// an attribute belongs to the resource; the data source, where the attribute
// is read-only, keeps the shared text.
func TestMerge_ResourceScopedDescriptionSkipsDatasource(t *testing.T) {
	withScopedDescription(t, "Foo.bounded", "Shared text.")
	withResourceScopedDescription(t, "Foo.bounded", "Shared text. Set it together with its partner.")
	proto := &ProtoMessage{
		Name:   "Foo",
		Fields: []ProtoField{{Name: "bounded", Kind: KindString, Cardinality: "singular"}},
	}
	cfg := &Config{APISchema: "Foo", API: &APIConfig{}}
	for schemaType, want := range map[string]string{
		"resource":           "Shared text. Set it together with its partner.",
		SchemaTypeDatasource: "Shared text.",
	} {
		attrs, _, _, errs := Merge(proto, cfg, schemaType, descTestIndex())
		if len(errs) != 0 {
			t.Fatalf("%s: unexpected errors: %v", schemaType, errs)
		}
		if got := attrs[0].Description; got != want {
			t.Errorf("%s: description = %q, want %q", schemaType, got, want)
		}
	}
}

// TestMerge_CommonDescriptionOnExtraField: TF-only fields take the shared
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

// descTestIndexWithWriteRoot mirrors descTestIndex but adds a write-shape root
// ("FooCreate") carrying a write-only field plus a field also present on the
// read root, to exercise read-primary/write-fallback resolution.
func descTestIndexWithWriteRoot() *apidesc.Index {
	return apidesc.FromFile(&apidesc.File{
		Schemas: map[string]*apidesc.Node{
			"Foo": {
				Fields: map[string]*apidesc.Node{
					"name":   {Description: "Read text for name."},
					"shared": {Description: "Read text for shared."},
				},
			},
			"FooCreate": {
				Fields: map[string]*apidesc.Node{
					"enable_widget": {Description: "Controls whether the widget is enabled. Applicable only for GCP"},
					"shared":        {Description: "Write text for shared."},
				},
			},
		},
	}, "test-fixture")
}

// TestMerge_WriteShapeDescriptionFallback: a write-only input field
// (expand_proto_name, absent from the read shape) sources its description from
// the api_write_schemas fallback root, not the humanize fallback.
func TestMerge_WriteShapeDescriptionFallback(t *testing.T) {
	tru := true
	proto := &ProtoMessage{
		Name: "Foo",
		Fields: []ProtoField{
			{Name: "name", Kind: KindString, Cardinality: "singular"},
			{Name: "shared", Kind: KindString, Cardinality: "singular"},
		},
	}
	cfg := &Config{
		APISchema:       "Foo",
		APIWriteSchemas: []string{"FooCreate"},
		Fields: map[string]FieldConfig{
			"enable_widget": {
				Extra: true, Type: "bool", Optional: &tru,
				ExpandProtoName: "enable_widget", FlattenSkip: true,
			},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", descTestIndexWithWriteRoot())
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	got := map[string]string{}
	for _, a := range attrs {
		got[a.Name] = a.Description
	}
	if got["enable_widget"] != "Controls whether the widget is enabled. Applicable only for GCP" {
		t.Errorf("enable_widget: want write-root fallback text; got %q", got["enable_widget"])
	}
	// Read root wins for a field present on both.
	if got["shared"] != "Read text for shared." {
		t.Errorf("shared: read root must win over write fallback; got %q", got["shared"])
	}
}

// TestMerge_ScopedDescriptionOnExtraField pins the Cluster.tags shape: an extra
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

// TestResourceScopedDescriptionsExtendShared pins that every resource-only
// entry overlays a shared entry and starts with its text, so the data source
// and the resource never describe an attribute differently in substance.
func TestResourceScopedDescriptionsExtendShared(t *testing.T) {
	for key, text := range resourceScopedDescriptions {
		shared, ok := scopedDescriptions[key]
		if !ok {
			t.Errorf("%s: resource-only text without a shared entry", key)
			continue
		}
		if !strings.HasPrefix(text, shared) {
			t.Errorf("%s: resource text does not start with the shared text %q", key, shared)
		}
	}
}
