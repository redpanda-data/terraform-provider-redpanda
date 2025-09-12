// Copyright 2024 Redpanda Data, Inc.
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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = SchemaRegistryOperationValidator{}

// SchemaRegistryOperationValidator validates that a Schema Registry operation is one of the allowed values
type SchemaRegistryOperationValidator struct{}

// Description provides a description of the validator
func (SchemaRegistryOperationValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must be one of: %s", strings.Join(validOperations(), ", "))
}

// MarkdownDescription provides a description of the validator in markdown format
func (SchemaRegistryOperationValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("value must be one of: `%s`", strings.Join(validOperations(), "`, `"))
}

// ValidateString validates that the string value is a valid Schema Registry operation
func (SchemaRegistryOperationValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the value is unknown or null
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	value := req.ConfigValue.ValueString()
	validOps := validOperations()

	// Check if the value is in the list of valid operations
	isValid := false
	for _, op := range validOps {
		if value == op {
			isValid = true
			break
		}
	}

	if !isValid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Schema Registry Operation",
			fmt.Sprintf("Operation must be one of [%s], got: %s",
				strings.Join(validOps, ", "), value),
		)
	}
}

// validOperations returns the list of valid Schema Registry operations
func validOperations() []string {
	return []string{
		"ALL",
		"READ",
		"WRITE",
		"DELETE",
		"DESCRIBE",
		"DESCRIBE_CONFIGS",
		"ALTER",
		"ALTER_CONFIGS",
	}
}

// SchemaRegistryOperation returns a validator for Schema Registry operations
func SchemaRegistryOperation() validator.String {
	return SchemaRegistryOperationValidator{}
}
