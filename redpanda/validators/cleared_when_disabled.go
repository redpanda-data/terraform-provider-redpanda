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

package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ validator.List   = ClearedWhenDisabled{}
	_ validator.Int32  = ClearedWhenDisabled{}
	_ validator.Object = ClearedWhenDisabled{}
	_ validator.String = ClearedWhenDisabled{}
)

// ClearedWhenDisabled rejects a configured value while the sibling enabled
// flag is false (or omitted with a false default): the control plane clears
// the field on disable, and Terraform requires plan==config for a known
// Optional value, so no plan modifier can reconcile the pair. An explicit 0 or
// "" stays legal on a scalar: it matches the value the control plane resets
// the field to.
type ClearedWhenDisabled struct {
	Enabled path.Path
}

// Description provides a description of the validator.
func (v ClearedWhenDisabled) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription provides a description of the validator in markdown format.
func (v ClearedWhenDisabled) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Cannot be set while %q is false: the control plane clears it on disable", v.Enabled)
}

// ValidateList rejects a non-null list while the enabled sibling is off.
func (v ClearedWhenDisabled) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v.check(ctx, req.Config, req.Path, &resp.Diagnostics)
}

// ValidateInt32 rejects a non-zero value while the enabled sibling is off.
func (v ClearedWhenDisabled) ValidateInt32(ctx context.Context, req validator.Int32Request, resp *validator.Int32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueInt32() == 0 {
		return
	}
	v.check(ctx, req.Config, req.Path, &resp.Diagnostics)
}

// ValidateObject rejects a non-null object while the enabled sibling is off.
func (v ClearedWhenDisabled) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v.check(ctx, req.Config, req.Path, &resp.Diagnostics)
}

// ValidateString rejects a non-empty value while the enabled sibling is off.
func (v ClearedWhenDisabled) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	v.check(ctx, req.Config, req.Path, &resp.Diagnostics)
}

func (v ClearedWhenDisabled) check(ctx context.Context, cfg tfsdk.Config, attr path.Path, diags *diag.Diagnostics) {
	if enabledOff(ctx, cfg, v.Enabled, diags) {
		diags.Append(clearedWhenDisabledDiag(attr, v.Enabled))
	}
}

// enabledOff reports whether the enabled flag at p resolves to off in config.
// A null flag inside a configured block counts as off (the schema defaults it
// to false). An unknown flag defers: Terraform re-validates the config at apply
// once it is known, and this validator runs again then. A null enclosing block
// also defers, because an Optional+Computed block omitted from config plans as
// its prior state on update and as the server default on create, neither of
// which config shows; a plan-time check on the planned flag covers that case.
func enabledOff(ctx context.Context, cfg tfsdk.Config, p path.Path, diags *diag.Diagnostics) bool {
	if parent := p.ParentPath(); !parent.Equal(path.Empty()) {
		var block types.Object
		d := cfg.GetAttribute(ctx, parent, &block)
		diags.Append(d...)
		if d.HasError() || block.IsNull() || block.IsUnknown() {
			return false
		}
	}
	var enabled types.Bool
	d := cfg.GetAttribute(ctx, p, &enabled)
	diags.Append(d...)
	if d.HasError() || enabled.IsUnknown() {
		return false
	}
	return !enabled.ValueBool()
}

// clearedWhenDisabledDiag is the error for a value set while the enabled flag
// it depends on is off.
func clearedWhenDisabledDiag(attr, enabled path.Path) diag.Diagnostic {
	return diag.NewAttributeErrorDiagnostic(
		attr,
		"Invalid Attribute Combination",
		fmt.Sprintf("%q cannot be set while %q is false: the control plane clears it when disabling", attr, enabled),
	)
}
