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

package utils

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestImportStateBoolFromSchemaDefault(t *testing.T) {
	makeSchema := func(attr schema.Attribute) schema.Schema {
		return schema.Schema{Attributes: map[string]schema.Attribute{"allow_deletion": attr}}
	}
	stateOf := func(sch schema.Schema) *tfsdk.State {
		return &tfsdk.State{
			Schema: sch,
			Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"allow_deletion": tftypes.Bool,
			}}, map[string]tftypes.Value{
				"allow_deletion": tftypes.NewValue(tftypes.Bool, nil),
			}),
		}
	}
	ctx := context.Background()

	t.Run("reads false default", func(t *testing.T) {
		sch := makeSchema(schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)})
		state := stateOf(sch)
		diags := ImportStateBoolFromSchemaDefault(ctx, sch, state, "allow_deletion")
		if diags.HasError() {
			t.Fatalf("unexpected diags: %v", diags)
		}
		var got types.Bool
		if d := state.GetAttribute(ctx, path.Root("allow_deletion"), &got); d.HasError() {
			t.Fatalf("get failed: %v", d)
		}
		if got.ValueBool() {
			t.Fatalf("want false, got %v", got)
		}
	})

	t.Run("reads true default", func(t *testing.T) {
		sch := makeSchema(schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)})
		state := stateOf(sch)
		diags := ImportStateBoolFromSchemaDefault(ctx, sch, state, "allow_deletion")
		if diags.HasError() {
			t.Fatalf("unexpected diags: %v", diags)
		}
		var got types.Bool
		if d := state.GetAttribute(ctx, path.Root("allow_deletion"), &got); d.HasError() {
			t.Fatalf("get failed: %v", d)
		}
		if !got.ValueBool() {
			t.Fatalf("want true, got %v", got)
		}
	})

	t.Run("missing attribute errors", func(t *testing.T) {
		sch := makeSchema(schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)})
		state := stateOf(sch)
		diags := ImportStateBoolFromSchemaDefault(ctx, sch, state, "does_not_exist")
		if !diags.HasError() {
			t.Fatal("expected error for missing attribute")
		}
	})

	t.Run("missing default errors", func(t *testing.T) {
		sch := makeSchema(schema.BoolAttribute{Optional: true})
		state := stateOf(sch)
		diags := ImportStateBoolFromSchemaDefault(ctx, sch, state, "allow_deletion")
		if !diags.HasError() {
			t.Fatal("expected error for missing default")
		}
	})

	t.Run("non-bool attribute errors", func(t *testing.T) {
		sch := schema.Schema{Attributes: map[string]schema.Attribute{
			"allow_deletion": schema.StringAttribute{Optional: true},
		}}
		state := stateOf(sch)
		diags := ImportStateBoolFromSchemaDefault(ctx, sch, state, "allow_deletion")
		if !diags.HasError() {
			t.Fatal("expected error for non-bool attribute")
		}
	})
}
