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

package planmodifiers

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The fixture mirrors the shapes the classifier emits the modifier for: a
// collection leaf under an Optional+Computed object, and a collection leaf
// inside an element of a computed list of objects.
var (
	linkType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"conns":  tftypes.List{ElementType: tftypes.String},
		"labels": tftypes.Set{ElementType: tftypes.String},
		"meta":   tftypes.Map{ElementType: tftypes.String},
	}}
	itemType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"arns": tftypes.List{ElementType: tftypes.String},
	}}
	resourceType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"link":  linkType,
		"items": tftypes.List{ElementType: itemType},
	}}
)

func fixtureSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"link": schema.SingleNestedAttribute{
			Optional: true,
			Computed: true,
			Attributes: map[string]schema.Attribute{
				"conns":  schema.ListAttribute{Computed: true, ElementType: types.StringType},
				"labels": schema.SetAttribute{Computed: true, ElementType: types.StringType},
				"meta":   schema.MapAttribute{Computed: true, ElementType: types.StringType},
			},
		},
		"items": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"arns": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			}},
		},
	}}
}

func stateOf(link, items tftypes.Value) tfsdk.State {
	return tfsdk.State{
		Schema: fixtureSchema(),
		Raw: tftypes.NewValue(resourceType, map[string]tftypes.Value{
			"link":  link,
			"items": items,
		}),
	}
}

func linkWith(conns []string) tftypes.Value {
	var connsVal tftypes.Value
	if conns == nil {
		connsVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	} else {
		elems := make([]tftypes.Value, 0, len(conns))
		for _, c := range conns {
			elems = append(elems, tftypes.NewValue(tftypes.String, c))
		}
		connsVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, elems)
	}
	return tftypes.NewValue(linkType, map[string]tftypes.Value{
		"conns":  connsVal,
		"labels": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"meta":   tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
	})
}

var (
	nullLink  = tftypes.NewValue(linkType, nil)
	nullItems = tftypes.NewValue(tftypes.List{ElementType: itemType}, nil)
	oneItem   = tftypes.NewValue(tftypes.List{ElementType: itemType}, []tftypes.Value{
		tftypes.NewValue(itemType, map[string]tftypes.Value{
			"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		}),
	})
)

func TestUseStateForUnknownIfParentInState_List(t *testing.T) {
	ctx := context.Background()
	nullList := types.ListNull(types.StringType)
	unknownList := types.ListUnknown(types.StringType)
	known, d := types.ListValueFrom(ctx, types.StringType, []string{"a"})
	if d.HasError() {
		t.Fatalf("ListValueFrom: %v", d.Errors())
	}
	empty, d := types.ListValueFrom(ctx, types.StringType, []string{})
	if d.HasError() {
		t.Fatalf("ListValueFrom: %v", d.Errors())
	}
	connsPath := path.Root("link").AtName("conns")

	cases := []struct {
		name   string
		path   path.Path
		state  tfsdk.State
		stateV types.List
		planV  types.List
		confV  types.List
		want   types.List
	}{
		{
			name: "parent in state, leaf null: holds null",
			path: connsPath, state: stateOf(linkWith(nil), nullItems),
			stateV: nullList, planV: unknownList, confV: nullList, want: nullList,
		},
		{
			name: "parent in state, leaf known: holds value",
			path: connsPath, state: stateOf(linkWith([]string{"a"}), nullItems),
			stateV: known, planV: unknownList, confV: nullList, want: known,
		},
		{
			name: "parent null in state: server fills the fresh block, stays unknown",
			path: connsPath, state: stateOf(nullLink, nullItems),
			stateV: nullList, planV: unknownList, confV: nullList, want: unknownList,
		},
		{
			name: "create (no prior state): stays unknown",
			path: connsPath, state: tfsdk.State{Schema: fixtureSchema(), Raw: tftypes.NewValue(resourceType, nil)},
			stateV: nullList, planV: unknownList, confV: nullList, want: unknownList,
		},
		{
			name: "known plan value is left alone",
			path: connsPath, state: stateOf(linkWith(nil), nullItems),
			stateV: nullList, planV: empty, confV: nullList, want: empty,
		},
		{
			name: "unknown config value is left alone",
			path: connsPath, state: stateOf(linkWith(nil), nullItems),
			stateV: nullList, planV: unknownList, confV: unknownList, want: unknownList,
		},
		{
			name: "list element parent in state: holds null",
			path: path.Root("items").AtListIndex(0).AtName("arns"), state: stateOf(nullLink, oneItem),
			stateV: nullList, planV: unknownList, confV: nullList, want: nullList,
		},
		{
			name: "list element parent absent from state: stays unknown",
			path: path.Root("items").AtListIndex(1).AtName("arns"), state: stateOf(nullLink, oneItem),
			stateV: nullList, planV: unknownList, confV: nullList, want: unknownList,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.ListRequest{
				Path:        tc.path,
				State:       tc.state,
				StateValue:  tc.stateV,
				PlanValue:   tc.planV,
				ConfigValue: tc.confV,
			}
			resp := &planmodifier.ListResponse{PlanValue: req.PlanValue}
			ListUseStateForUnknownIfParentInState().PlanModifyList(ctx, req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			if !resp.PlanValue.Equal(tc.want) {
				t.Fatalf("plan value = %v, want %v", resp.PlanValue, tc.want)
			}
		})
	}
}

func TestUseStateForUnknownIfParentInState_SetAndMap(t *testing.T) {
	ctx := context.Background()
	withParent := stateOf(linkWith(nil), nullItems)
	withoutParent := stateOf(nullLink, nullItems)

	t.Run("set holds null under a parent in state", func(t *testing.T) {
		req := planmodifier.SetRequest{
			Path:        path.Root("link").AtName("labels"),
			State:       withParent,
			StateValue:  types.SetNull(types.StringType),
			PlanValue:   types.SetUnknown(types.StringType),
			ConfigValue: types.SetNull(types.StringType),
		}
		resp := &planmodifier.SetResponse{PlanValue: req.PlanValue}
		SetUseStateForUnknownIfParentInState().PlanModifySet(ctx, req, resp)
		if !resp.PlanValue.Equal(types.SetNull(types.StringType)) {
			t.Fatalf("plan value = %v, want null set", resp.PlanValue)
		}
	})
	t.Run("set stays unknown under a null parent", func(t *testing.T) {
		req := planmodifier.SetRequest{
			Path:        path.Root("link").AtName("labels"),
			State:       withoutParent,
			StateValue:  types.SetNull(types.StringType),
			PlanValue:   types.SetUnknown(types.StringType),
			ConfigValue: types.SetNull(types.StringType),
		}
		resp := &planmodifier.SetResponse{PlanValue: req.PlanValue}
		SetUseStateForUnknownIfParentInState().PlanModifySet(ctx, req, resp)
		if !resp.PlanValue.IsUnknown() {
			t.Fatalf("plan value = %v, want unknown", resp.PlanValue)
		}
	})
	t.Run("map holds null under a parent in state", func(t *testing.T) {
		req := planmodifier.MapRequest{
			Path:        path.Root("link").AtName("meta"),
			State:       withParent,
			StateValue:  types.MapNull(types.StringType),
			PlanValue:   types.MapUnknown(types.StringType),
			ConfigValue: types.MapNull(types.StringType),
		}
		resp := &planmodifier.MapResponse{PlanValue: req.PlanValue}
		MapUseStateForUnknownIfParentInState().PlanModifyMap(ctx, req, resp)
		if !resp.PlanValue.Equal(types.MapNull(types.StringType)) {
			t.Fatalf("plan value = %v, want null map", resp.PlanValue)
		}
	})
	t.Run("map stays unknown under a null parent", func(t *testing.T) {
		req := planmodifier.MapRequest{
			Path:        path.Root("link").AtName("meta"),
			State:       withoutParent,
			StateValue:  types.MapNull(types.StringType),
			PlanValue:   types.MapUnknown(types.StringType),
			ConfigValue: types.MapNull(types.StringType),
		}
		resp := &planmodifier.MapResponse{PlanValue: req.PlanValue}
		MapUseStateForUnknownIfParentInState().PlanModifyMap(ctx, req, resp)
		if !resp.PlanValue.IsUnknown() {
			t.Fatalf("plan value = %v, want unknown", resp.PlanValue)
		}
	})
}
