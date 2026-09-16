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

// Package planmodifiers holds plan modifiers shared by generated schemas
// across resources.
package planmodifiers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ListUseStateForUnknownIfParentInState holds the prior state value of a
// computed collection, null included, whenever the enclosing object already
// existed in prior state. It sits between the framework's two pins: a proto3
// repeated or map field arrives as nil when the server holds nothing, which
// flattens to null, and UseNonNullStateForUnknown then leaves the leaf
// "known after apply" on every update plan; plain UseStateForUnknown would
// pin that null even when the parent block is being created by this plan and
// the server is about to fill the leaf, which fails as inconsistent result.
func ListUseStateForUnknownIfParentInState() planmodifier.List {
	return useStateForUnknownIfParentInState{}
}

// SetUseStateForUnknownIfParentInState is the set form of
// ListUseStateForUnknownIfParentInState.
func SetUseStateForUnknownIfParentInState() planmodifier.Set {
	return useStateForUnknownIfParentInState{}
}

// MapUseStateForUnknownIfParentInState is the map form of
// ListUseStateForUnknownIfParentInState.
func MapUseStateForUnknownIfParentInState() planmodifier.Map {
	return useStateForUnknownIfParentInState{}
}

type useStateForUnknownIfParentInState struct{}

func (useStateForUnknownIfParentInState) Description(_ context.Context) string {
	return "Holds the prior state value, null included, while the enclosing object already existed in state."
}

func (m useStateForUnknownIfParentInState) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyList implements planmodifier.List.
func (useStateForUnknownIfParentInState) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.PlanValue.IsUnknown() || req.ConfigValue.IsUnknown() {
		return
	}
	hold, diags := parentInState(ctx, req.State, req.Path)
	resp.Diagnostics.Append(diags...)
	if hold {
		resp.PlanValue = req.StateValue
	}
}

// PlanModifySet implements planmodifier.Set.
func (useStateForUnknownIfParentInState) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if !req.PlanValue.IsUnknown() || req.ConfigValue.IsUnknown() {
		return
	}
	hold, diags := parentInState(ctx, req.State, req.Path)
	resp.Diagnostics.Append(diags...)
	if hold {
		resp.PlanValue = req.StateValue
	}
}

// PlanModifyMap implements planmodifier.Map.
func (useStateForUnknownIfParentInState) PlanModifyMap(ctx context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if !req.PlanValue.IsUnknown() || req.ConfigValue.IsUnknown() {
		return
	}
	hold, diags := parentInState(ctx, req.State, req.Path)
	resp.Diagnostics.Append(diags...)
	if hold {
		resp.PlanValue = req.StateValue
	}
}

// parentInState reports whether the object enclosing leaf is known in prior
// state. A list element index past the stored length reads back as a null
// object, so a leaf inside a not-yet-stored element stays unknown.
func parentInState(ctx context.Context, state tfsdk.State, leaf path.Path) (bool, diag.Diagnostics) {
	if state.Raw.IsNull() {
		return false, nil
	}
	var parent types.Object
	diags := state.GetAttribute(ctx, leaf.ParentPath(), &parent)
	if diags.HasError() {
		return false, diags
	}
	return !parent.IsNull() && !parent.IsUnknown(), nil
}
