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
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// EmailValidator is a custom validator to ensure a string attribute is a valid email address
type EmailValidator struct{}

// Description provides a description of the validator
func (EmailValidator) Description(_ context.Context) string {
	return "string value must be a valid email address"
}

// MarkdownDescription provides a description of the validator in markdown format
func (EmailValidator) MarkdownDescription(_ context.Context) string {
	return "String value must be a valid email address"
}

// ValidateString validates a string attribute
func (EmailValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the value is null or unknown
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	// Regular expression pattern for validating email addresses
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Email Validation Error",
			fmt.Sprintf("Error validating email format: %s", err),
		)
		return
	}

	if !matched {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Email Format",
			fmt.Sprintf("The value %q is not a valid email address. Email addresses must follow the standard format (example@domain.com).", value),
		)
	}
}
