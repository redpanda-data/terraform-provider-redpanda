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

package validators_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

func TestPasswordValidator(t *testing.T) {
	testCases := []struct {
		name          string
		password      *string // nil = null, pointer to empty string = empty, pointer to value = value
		passwordWO    *string // nil = null, pointer to empty string = empty, pointer to value = value
		validateField string  // which field to validate: "password" or "password_wo"
		expectError   bool
		errorContains string
		errorSummary  string
	}{
		// Minimum length validation tests
		{
			name:          "password too short (1 char)",
			password:      strPtr("a"),
			passwordWO:    nil,
			validateField: "password",
			expectError:   true,
			errorSummary:  "Invalid Password Length",
			errorContains: "at least 3 characters",
		},
		{
			name:          "password too short (2 chars)",
			password:      strPtr("ab"),
			passwordWO:    nil,
			validateField: "password",
			expectError:   true,
			errorSummary:  "Invalid Password Length",
			errorContains: "at least 3 characters",
		},
		{
			name:          "password exactly 3 chars - valid",
			password:      strPtr("abc"),
			passwordWO:    nil,
			validateField: "password",
			expectError:   false,
		},
		{
			name:          "password longer than 3 chars - valid",
			password:      strPtr("secure-password-123"),
			passwordWO:    nil,
			validateField: "password",
			expectError:   false,
		},
		{
			name:          "password_wo too short (1 char)",
			password:      nil,
			passwordWO:    strPtr("a"),
			validateField: "password_wo",
			expectError:   true,
			errorSummary:  "Invalid Password Length",
			errorContains: "at least 3 characters",
		},
		{
			name:          "password_wo exactly 3 chars - valid",
			password:      nil,
			passwordWO:    strPtr("abc"),
			validateField: "password_wo",
			expectError:   false,
		},
		// At least one required tests
		{
			name:          "both fields null - error",
			password:      nil,
			passwordWO:    nil,
			validateField: "password",
			expectError:   true,
			errorSummary:  "Missing Required Password",
			errorContains: "must be set with a non-empty value",
		},
		{
			name:          "both fields empty string - error",
			password:      strPtr(""),
			passwordWO:    strPtr(""),
			validateField: "password",
			expectError:   true,
			errorSummary:  "Missing Required Password",
			errorContains: "must be set with a non-empty value",
		},
		{
			name:          "password empty, password_wo null - error",
			password:      strPtr(""),
			passwordWO:    nil,
			validateField: "password",
			expectError:   true,
			errorSummary:  "Missing Required Password",
			errorContains: "must be set with a non-empty value",
		},
		{
			name:          "password null, password_wo set - valid (validating password)",
			password:      nil,
			passwordWO:    strPtr("abc"),
			validateField: "password",
			expectError:   false,
		},
		{
			name:          "password set, password_wo null - valid (validating password_wo)",
			password:      strPtr("abc"),
			passwordWO:    nil,
			validateField: "password_wo",
			expectError:   false,
		},
		{
			name:          "both fields set - valid",
			password:      strPtr("abc"),
			passwordWO:    strPtr("def"),
			validateField: "password",
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build tftypes values
			passwordValue := tftypes.NewValue(tftypes.String, nil)
			if tc.password != nil {
				passwordValue = tftypes.NewValue(tftypes.String, *tc.password)
			}

			passwordWOValue := tftypes.NewValue(tftypes.String, nil)
			if tc.passwordWO != nil {
				passwordWOValue = tftypes.NewValue(tftypes.String, *tc.passwordWO)
			}

			// Create schema
			testSchema := schema.Schema{
				Attributes: map[string]schema.Attribute{
					"password":    schema.StringAttribute{Optional: true},
					"password_wo": schema.StringAttribute{Optional: true},
				},
			}

			// Create raw config value
			rawValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"password":    tftypes.String,
						"password_wo": tftypes.String,
					},
				},
				map[string]tftypes.Value{
					"password":    passwordValue,
					"password_wo": passwordWOValue,
				},
			)

			// Create tfsdk.Config
			config := tfsdk.Config{
				Schema: testSchema,
				Raw:    rawValue,
			}

			// Determine which field we're validating and its value
			var configValue types.String
			var attrPath path.Path
			if tc.validateField == "password" {
				attrPath = path.Root("password")
				if tc.password == nil {
					configValue = types.StringNull()
				} else {
					configValue = types.StringValue(*tc.password)
				}
			} else {
				attrPath = path.Root("password_wo")
				if tc.passwordWO == nil {
					configValue = types.StringNull()
				} else {
					configValue = types.StringValue(*tc.passwordWO)
				}
			}

			// Create request
			req := validator.StringRequest{
				Path:           attrPath,
				PathExpression: attrPath.Expression(),
				ConfigValue:    configValue,
				Config:         config,
			}
			var resp validator.StringResponse

			// Run validation
			v := validators.Password(
				path.MatchRoot("password"),
				path.MatchRoot("password_wo"),
			)
			v.ValidateString(context.Background(), req, &resp)

			// Check results
			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
				if tc.errorSummary != "" {
					found := false
					for _, d := range resp.Diagnostics {
						if d.Severity() == diag.SeverityError && d.Summary() == tc.errorSummary {
							found = true
							if tc.errorContains != "" {
								assert.Contains(t, d.Detail(), tc.errorContains, "error detail doesn't contain expected text")
							}
							break
						}
					}
					assert.True(t, found, "expected error with summary %q not found", tc.errorSummary)
				}
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestPasswordValidator_UnknownValue(t *testing.T) {
	// When the current value is unknown, validation should be skipped
	testSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"password":    schema.StringAttribute{Optional: true},
			"password_wo": schema.StringAttribute{Optional: true},
		},
	}

	rawValue := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"password":    tftypes.String,
				"password_wo": tftypes.String,
			},
		},
		map[string]tftypes.Value{
			"password":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			"password_wo": tftypes.NewValue(tftypes.String, nil),
		},
	)

	config := tfsdk.Config{
		Schema: testSchema,
		Raw:    rawValue,
	}

	req := validator.StringRequest{
		Path:           path.Root("password"),
		PathExpression: path.Root("password").Expression(),
		ConfigValue:    types.StringUnknown(),
		Config:         config,
	}
	var resp validator.StringResponse

	v := validators.Password(
		path.MatchRoot("password"),
		path.MatchRoot("password_wo"),
	)
	v.ValidateString(context.Background(), req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "unknown value should skip validation")
}

func TestPasswordValidator_OtherUnknown(t *testing.T) {
	// When the other password field is unknown, validation should be delayed
	testSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"password":    schema.StringAttribute{Optional: true},
			"password_wo": schema.StringAttribute{Optional: true},
		},
	}

	rawValue := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"password":    tftypes.String,
				"password_wo": tftypes.String,
			},
		},
		map[string]tftypes.Value{
			"password":    tftypes.NewValue(tftypes.String, nil),
			"password_wo": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		},
	)

	config := tfsdk.Config{
		Schema: testSchema,
		Raw:    rawValue,
	}

	req := validator.StringRequest{
		Path:           path.Root("password"),
		PathExpression: path.Root("password").Expression(),
		ConfigValue:    types.StringNull(),
		Config:         config,
	}
	var resp validator.StringResponse

	v := validators.Password(
		path.MatchRoot("password"),
		path.MatchRoot("password_wo"),
	)
	v.ValidateString(context.Background(), req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "validation should be delayed when other field is unknown")
}

func TestPasswordValidator_Description(t *testing.T) {
	v := validators.Password(
		path.MatchRoot("password"),
		path.MatchRoot("password_wo"),
	)

	desc := v.Description(context.Background())
	assert.Contains(t, desc, "password")
	assert.Contains(t, desc, "password_wo")
	assert.Contains(t, desc, "3")

	mdDesc := v.MarkdownDescription(context.Background())
	assert.Equal(t, desc, mdDesc)
}

// strPtr is a helper to create string pointers for test cases
func strPtr(s string) *string {
	return &s
}
