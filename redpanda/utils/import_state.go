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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// ImportStateBoolFromSchemaDefault writes the schema-level Default of a bool
// attribute into state. Use this in ImportState handlers for TF-only fields
// (no proto correspondence) whose Read() path doesn't populate them — without
// the bootstrap the post-import plan shows a null→default heal diff. Pulling
// the value from the schema avoids drift if the default ever changes.
func ImportStateBoolFromSchemaDefault(ctx context.Context, sch schema.Schema, state *tfsdk.State, attrName string) diag.Diagnostics {
	var diags diag.Diagnostics
	rawAttr, ok := sch.Attributes[attrName]
	if !ok {
		diags.AddError("import state bootstrap failed", fmt.Sprintf("attribute %q not found in schema", attrName))
		return diags
	}
	boolAttr, ok := rawAttr.(schema.BoolAttribute)
	if !ok {
		diags.AddError("import state bootstrap failed", fmt.Sprintf("attribute %q is not a BoolAttribute", attrName))
		return diags
	}
	if boolAttr.Default == nil {
		diags.AddError("import state bootstrap failed", fmt.Sprintf("attribute %q has no Default", attrName))
		return diags
	}
	var defResp defaults.BoolResponse
	boolAttr.Default.DefaultBool(ctx, defaults.BoolRequest{}, &defResp)
	diags.Append(defResp.Diagnostics...)
	if diags.HasError() {
		return diags
	}
	diags.Append(state.SetAttribute(ctx, path.Root(attrName), defResp.PlanValue)...)
	return diags
}
