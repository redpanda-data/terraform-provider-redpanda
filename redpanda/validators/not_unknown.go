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

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = NotUnknownValidator{}

// NotUnknown returns a NotUnknownValidator
func NotUnknown() NotUnknownValidator {
	return NotUnknownValidator{}
}

// NotUnknownValidator is a custom validator to ensure that an attribute is not unknown
// at the validation stage.
type NotUnknownValidator struct{}

// Description provides a description of the validator
func (v NotUnknownValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription provides a description of the validator in markdown format
func (NotUnknownValidator) MarkdownDescription(_ context.Context) string {
	return "Ensure that an attribute is not unknown"
}

// ValidateString validates a string
func (NotUnknownValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			fmt.Sprintf("Attribute %q must be known at validation time", req.Path),
			"",
		)
	}
}
