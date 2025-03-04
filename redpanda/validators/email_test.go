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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

func TestEmailValidator(t *testing.T) {
	testCases := []struct {
		name          string
		input         types.String
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid simple email",
			input:       types.StringValue("user@example.com"),
			expectError: false,
		},
		{
			name:        "valid email with dots",
			input:       types.StringValue("first.last@example.com"),
			expectError: false,
		},
		{
			name:        "valid email with plus",
			input:       types.StringValue("user+tag@example.com"),
			expectError: false,
		},
		{
			name:        "valid email with numbers",
			input:       types.StringValue("user123@example.com"),
			expectError: false,
		},
		{
			name:        "valid email with subdomain",
			input:       types.StringValue("user@sub.example.com"),
			expectError: false,
		},
		{
			name:        "valid email with uppercase",
			input:       types.StringValue("USER@EXAMPLE.COM"),
			expectError: false,
		},
		{
			name:        "valid email with .org TLD",
			input:       types.StringValue("user@example.org"),
			expectError: false,
		},
		{
			name:        "valid email with .io TLD",
			input:       types.StringValue("user@example.io"),
			expectError: false,
		},
		{
			name:        "valid email with .dev TLD",
			input:       types.StringValue("user@example.dev"),
			expectError: false,
		},
		{
			name:        "valid email with .co.uk TLD",
			input:       types.StringValue("user@example.co.uk"),
			expectError: false,
		},
		{
			name:        "valid email with .cloud TLD",
			input:       types.StringValue("user@redpanda.cloud"),
			expectError: false,
		},
		{
			name:        "null value - should skip validation",
			input:       types.StringNull(),
			expectError: false,
		},
		{
			name:        "unknown value - should skip validation",
			input:       types.StringUnknown(),
			expectError: false,
		},
		{
			name:          "invalid - missing @",
			input:         types.StringValue("userexample.com"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
		{
			name:          "invalid - missing domain",
			input:         types.StringValue("user@"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
		{
			name:          "invalid - missing username",
			input:         types.StringValue("@example.com"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
		{
			name:          "invalid - missing tld",
			input:         types.StringValue("user@example"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
		{
			name:          "invalid - spaces",
			input:         types.StringValue("user @example.com"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
		{
			name:          "invalid - special chars in domain",
			input:         types.StringValue("user@example!.com"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
		{
			name:          "invalid - too short tld",
			input:         types.StringValue("user@example.c"),
			expectError:   true,
			errorContains: "not a valid email address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup request and response
			req := validator.StringRequest{
				Path:        path.Root("test_attribute"),
				ConfigValue: tc.input,
			}
			var resp validator.StringResponse

			// Run validation
			validators.EmailValidator{}.ValidateString(context.Background(), req, &resp)

			// Check results
			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
				if tc.errorContains != "" {
					errMsg := resp.Diagnostics.Errors()[0].Detail()
					assert.Contains(t, errMsg, tc.errorContains, "error message doesn't contain expected text")
				}
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}
