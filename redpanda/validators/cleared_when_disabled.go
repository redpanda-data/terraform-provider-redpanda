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
	_ validator.List  = ClearedWhenDisabledListValidator{}
	_ validator.Int32 = ClearedWhenDisabledInt32Validator{}
)

// ClearedWhenDisabledListValidator rejects a configured list while the sibling
// enabled flag is false (or omitted with a false default): the control plane
// clears the field on disable, and Terraform requires plan==config for a known
// Optional value, so no plan modifier can reconcile the pair.
type ClearedWhenDisabledListValidator struct {
	Enabled path.Path
}

// Description provides a description of the validator.
func (v ClearedWhenDisabledListValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription provides a description of the validator in markdown format.
func (v ClearedWhenDisabledListValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Cannot be set while %q is false: the control plane clears it on disable", v.Enabled)
}

// ValidateList rejects a non-null value while the enabled sibling is off.
func (v ClearedWhenDisabledListValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if enabledOff(ctx, req.Config, v.Enabled, &resp.Diagnostics) {
		resp.Diagnostics.Append(clearedWhenDisabledDiag(req.Path, v.Enabled))
	}
}

// ClearedWhenDisabledInt32Validator is the int32 variant of
// ClearedWhenDisabledListValidator. An explicit 0 stays legal: it matches the
// value the control plane resets the field to on disable.
type ClearedWhenDisabledInt32Validator struct {
	Enabled path.Path
}

// Description provides a description of the validator.
func (v ClearedWhenDisabledInt32Validator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription provides a description of the validator in markdown format.
func (v ClearedWhenDisabledInt32Validator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Cannot be set to a non-zero value while %q is false: the control plane resets it on disable", v.Enabled)
}

// ValidateInt32 rejects a non-zero value while the enabled sibling is off.
func (v ClearedWhenDisabledInt32Validator) ValidateInt32(ctx context.Context, req validator.Int32Request, resp *validator.Int32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueInt32() == 0 {
		return
	}
	if enabledOff(ctx, req.Config, v.Enabled, &resp.Diagnostics) {
		resp.Diagnostics.Append(clearedWhenDisabledDiag(req.Path, v.Enabled))
	}
}

// enabledOff reports whether the enabled flag at p resolves to off in config.
// A null flag counts as off (the schema defaults it to false); an unknown flag
// defers to apply-time server validation.
func enabledOff(ctx context.Context, cfg tfsdk.Config, p path.Path, diags *diag.Diagnostics) bool {
	var enabled types.Bool
	d := cfg.GetAttribute(ctx, p, &enabled)
	diags.Append(d...)
	if d.HasError() || enabled.IsUnknown() {
		return false
	}
	return enabled.IsNull() || !enabled.ValueBool()
}

func clearedWhenDisabledDiag(attr, enabled path.Path) diag.Diagnostic {
	return diag.NewAttributeErrorDiagnostic(
		attr,
		"Invalid Attribute Combination",
		fmt.Sprintf("%q cannot be set while %q is false: the control plane clears it when disabling", attr, enabled),
	)
}
