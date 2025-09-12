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

var (
	_ validator.String = SchemaRegistryResourceTypeValidator{}
	_ validator.String = SchemaRegistryPatternTypeValidator{}
	_ validator.String = SchemaRegistryPermissionValidator{}
)

// SchemaRegistryResourceTypeValidator validates that a Schema Registry resource type is valid
type SchemaRegistryResourceTypeValidator struct{}

// Description provides a description of the validator
func (SchemaRegistryResourceTypeValidator) Description(_ context.Context) string {
	return "value must be one of: SUBJECT, REGISTRY"
}

// MarkdownDescription provides a description of the validator in markdown format
func (SchemaRegistryResourceTypeValidator) MarkdownDescription(_ context.Context) string {
	return "value must be one of: `SUBJECT`, `REGISTRY`"
}

// ValidateString validates that the string value is a valid Schema Registry resource type
func (SchemaRegistryResourceTypeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the value is unknown or null
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	value := req.ConfigValue.ValueString()
	validTypes := []string{"SUBJECT", "REGISTRY"}

	isValid := false
	for _, t := range validTypes {
		if value == t {
			isValid = true
			break
		}
	}

	if !isValid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Schema Registry Resource Type",
			fmt.Sprintf("Resource type must be one of [%s], got: %s",
				strings.Join(validTypes, ", "), value),
		)
	}
}

// SchemaRegistryResourceType returns a validator for Schema Registry resource types
func SchemaRegistryResourceType() validator.String {
	return SchemaRegistryResourceTypeValidator{}
}

// SchemaRegistryPatternTypeValidator validates that a Schema Registry pattern type is valid
type SchemaRegistryPatternTypeValidator struct{}

// Description provides a description of the validator
func (SchemaRegistryPatternTypeValidator) Description(_ context.Context) string {
	return "value must be one of: LITERAL, PREFIXED"
}

// MarkdownDescription provides a description of the validator in markdown format
func (SchemaRegistryPatternTypeValidator) MarkdownDescription(_ context.Context) string {
	return "value must be one of: `LITERAL`, `PREFIXED`"
}

// ValidateString validates that the string value is a valid Schema Registry pattern type
func (SchemaRegistryPatternTypeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the value is unknown or null
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	value := req.ConfigValue.ValueString()
	validTypes := []string{"LITERAL", "PREFIXED"}

	isValid := false
	for _, t := range validTypes {
		if value == t {
			isValid = true
			break
		}
	}

	if !isValid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Schema Registry Pattern Type",
			fmt.Sprintf("Pattern type must be one of [%s], got: %s",
				strings.Join(validTypes, ", "), value),
		)
	}
}

// SchemaRegistryPatternType returns a validator for Schema Registry pattern types
func SchemaRegistryPatternType() validator.String {
	return SchemaRegistryPatternTypeValidator{}
}

// SchemaRegistryPermissionValidator validates that a Schema Registry permission is valid
type SchemaRegistryPermissionValidator struct{}

// Description provides a description of the validator
func (SchemaRegistryPermissionValidator) Description(_ context.Context) string {
	return "value must be one of: ALLOW, DENY"
}

// MarkdownDescription provides a description of the validator in markdown format
func (SchemaRegistryPermissionValidator) MarkdownDescription(_ context.Context) string {
	return "value must be one of: `ALLOW`, `DENY`"
}

// ValidateString validates that the string value is a valid Schema Registry permission
func (SchemaRegistryPermissionValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the value is unknown or null
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	value := req.ConfigValue.ValueString()
	validPerms := []string{"ALLOW", "DENY"}

	isValid := false
	for _, p := range validPerms {
		if value == p {
			isValid = true
			break
		}
	}

	if !isValid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Schema Registry Permission",
			fmt.Sprintf("Permission must be one of [%s], got: %s",
				strings.Join(validPerms, ", "), value),
		)
	}
}

// SchemaRegistryPermission returns a validator for Schema Registry permissions
func SchemaRegistryPermission() validator.String {
	return SchemaRegistryPermissionValidator{}
}
