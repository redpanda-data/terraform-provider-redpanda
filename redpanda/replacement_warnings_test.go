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

package redpanda

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// TestReplacementWarnings pins every registered resource, generated or
// hand-written, to the rule that an attribute documents replacement exactly
// when a plan modifier forces it. An unconditional modifier is recognised by
// its Description matching utils.ReplacementWarning; a framework
// RequiresReplaceIf by its type name, and its own message must appear. A
// custom modifier that sets RequiresReplace itself is invisible here and is
// covered by its schemagen registry entry instead.
func TestReplacementWarnings(t *testing.T) {
	ctx := context.Background()
	for _, newResource := range New(ctx, "pre", "test")().(*Redpanda).Resources(ctx) {
		r := newResource()
		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "redpanda"}, &meta)
		var sr resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sr)
		if sr.Diagnostics.HasError() {
			t.Fatalf("%s: schema: %s", meta.TypeName, sr.Diagnostics)
		}
		t.Run(meta.TypeName, func(t *testing.T) {
			checkReplacementWarnings(ctx, t, "", sr.Schema.Attributes)
		})
	}
}

func checkReplacementWarnings(ctx context.Context, t *testing.T, parent string, attrs map[string]schema.Attribute) {
	t.Helper()
	for name, attr := range attrs {
		path := name
		if parent != "" {
			path = parent + "." + name
		}
		desc := attr.GetMarkdownDescription()
		if desc == "" {
			desc = attr.GetDescription()
		}
		unconditional := false
		for _, m := range planModifiersOf(attr) {
			text := m.Description(ctx)
			if text == utils.ReplacementWarning {
				unconditional = true
				continue
			}
			if strings.Contains(reflect.TypeOf(m).String(), "requiresReplaceIf") && !strings.Contains(desc, strings.TrimSuffix(text, ".")) {
				t.Errorf("%s: conditional replace modifier says %q but the description does not: %q", path, text, desc)
			}
		}
		if unconditional && !strings.Contains(desc, utils.ReplacementWarning) {
			t.Errorf("%s: forces replacement but the description does not say so: %q", path, desc)
		}
		if !unconditional && strings.Contains(desc, utils.ReplacementWarning) {
			t.Errorf("%s: description warns of replacement but no modifier forces it: %q", path, desc)
		}
		checkReplacementWarnings(ctx, t, path, nestedAttributesOf(attr))
	}
}

type describer interface {
	Description(context.Context) string
}

// planModifiersOf reads the typed PlanModifiers slice through reflection:
// the framework has one slice type per attribute kind and no common accessor.
func planModifiersOf(attr schema.Attribute) []describer {
	f := reflect.ValueOf(attr).FieldByName("PlanModifiers")
	if !f.IsValid() || f.Kind() != reflect.Slice {
		return nil
	}
	out := make([]describer, 0, f.Len())
	for i := 0; i < f.Len(); i++ {
		if d, ok := f.Index(i).Interface().(describer); ok {
			out = append(out, d)
		}
	}
	return out
}

func nestedAttributesOf(attr schema.Attribute) map[string]schema.Attribute {
	switch a := attr.(type) {
	case schema.SingleNestedAttribute:
		return a.Attributes
	case schema.ListNestedAttribute:
		return a.NestedObject.Attributes
	case schema.SetNestedAttribute:
		return a.NestedObject.Attributes
	case schema.MapNestedAttribute:
		return a.NestedObject.Attributes
	}
	return nil
}
