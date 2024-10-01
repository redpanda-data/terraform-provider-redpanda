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

var _ validator.String = AlsoRequiresOneOfValidator{}

// AlsoRequiresOneOf returns an AlsoRequiresOneOfValidator
func AlsoRequiresOneOf(expressions ...path.Expression) AlsoRequiresOneOfValidator {
	return AlsoRequiresOneOfValidator{
		Expressions: expressions,
	}
}

// AlsoRequiresOneOfValidator checks that at least one of a set of path.Expression has a
// non-null value, if the current attribute also has a non-null value.
type AlsoRequiresOneOfValidator struct {
	Expressions []path.Expression
}

// Description provides a description of the validator
func (v AlsoRequiresOneOfValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription provides a description of the validator in markdown format
func (v AlsoRequiresOneOfValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Ensure that if an attribute is set, then one of these is also set: %q", v.Expressions)
}

// ValidateString validates a string
func (v AlsoRequiresOneOfValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	for _, expression := range v.Expressions {
		matchedPaths, diags := req.Config.PathMatches(ctx, expression)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}

		for _, mp := range matchedPaths {
			if mp.Equal(req.Path) {
				continue
			}

			var value types.String
			diags = req.Config.GetAttribute(ctx, mp, &value)
			resp.Diagnostics.Append(diags...)

			if diags.HasError() {
				continue
			}

			if value.IsUnknown() {
				// Delay validation until all involved attributes have a known value
				return
			}

			if !value.IsNull() {
				return
			}
		}
	}

	description := ""
	for i, expression := range v.Expressions {
		if i != 0 {
			if len(v.Expressions) > 2 {
				description += ","
			}
			description += " "
		}
		if i == len(v.Expressions)-1 {
			description += "or "
		}
		description += fmt.Sprintf("%q", expression)
	}
	resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
		req.Path,
		"Invalid Attribute Combination", // this title matches stringvalidator.AlsoRequires
		fmt.Sprintf("One of the attributes %v must be specified when %q is specified", description, req.Path),
	))
}
