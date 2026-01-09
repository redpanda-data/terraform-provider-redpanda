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

package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const minPasswordLength = 3

var _ validator.String = PasswordValidator{}

// Password returns a PasswordValidator that validates password fields.
// It ensures:
// 1. At least one of password or password_wo must be set and non-empty
// 2. If set, password must be at least 3 characters (Redpanda API requirement)
func Password(passwordPath, passwordWOPath path.Expression) PasswordValidator {
	return PasswordValidator{
		PasswordPath:   passwordPath,
		PasswordWOPath: passwordWOPath,
	}
}

// PasswordValidator validates password fields for minimum length and ensures
// at least one password field is populated.
type PasswordValidator struct {
	PasswordPath   path.Expression
	PasswordWOPath path.Expression
}

// Description provides a description of the validator
func (v PasswordValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription provides a description of the validator in markdown format
func (v PasswordValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Ensures at least one of %q or %q is set with a minimum of %d characters",
		v.PasswordPath, v.PasswordWOPath, minPasswordLength)
}

// ValidateString validates a string password field
func (v PasswordValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the current value is unknown (computed values not yet known)
	if req.ConfigValue.IsUnknown() {
		return
	}

	// Get the current field's value
	currentValue := ""
	currentIsSet := false
	if !req.ConfigValue.IsNull() {
		currentValue = req.ConfigValue.ValueString()
		currentIsSet = currentValue != ""
	}

	// If current field has a value, validate minimum length
	if currentIsSet && len(currentValue) < minPasswordLength {
		resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			req.Path,
			"Invalid Password Length",
			fmt.Sprintf("Password must be at least %d characters, got %d characters", minPasswordLength, len(currentValue)),
		))
		return
	}

	// Check if the other password field has a value
	otherPath := v.PasswordWOPath
	if req.Path.Equal(path.Root("password_wo")) {
		otherPath = v.PasswordPath
	}

	matchedPaths, diags := req.Config.PathMatches(ctx, otherPath)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	for _, mp := range matchedPaths {
		var otherValue types.String
		diags = req.Config.GetAttribute(ctx, mp, &otherValue)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			continue
		}

		// If other value is unknown, delay validation
		if otherValue.IsUnknown() {
			return
		}

		// Check if other field has a non-empty value
		if !otherValue.IsNull() && otherValue.ValueString() != "" {
			// Other field is set and non-empty, validation passes
			return
		}
	}

	// If we get here, check if current field is set
	if currentIsSet {
		// Current field is set and non-empty, validation passes
		return
	}

	// Neither field has a non-empty value
	resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
		req.Path,
		"Missing Required Password",
		fmt.Sprintf("At least one of %q or %q must be set with a non-empty value",
			v.PasswordPath, v.PasswordWOPath),
	))
}
