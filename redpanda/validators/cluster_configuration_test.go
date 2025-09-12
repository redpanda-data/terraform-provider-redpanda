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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestClusterConfigurationValidator(t *testing.T) {
	ctx := context.Background()
	v := ClusterConfiguration()

	tests := []struct {
		name        string
		objectValue types.Object
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_with_custom_properties_json",
			objectValue: types.ObjectValueMust(
				map[string]attr.Type{
					"custom_properties_json": types.StringType,
				},
				map[string]attr.Value{
					"custom_properties_json": types.StringValue(`{"schema_registry_enable_authorization":true}`),
				},
			),
			expectError: false,
		},
		{
			name: "valid_with_null_custom_properties_json",
			objectValue: types.ObjectValueMust(
				map[string]attr.Type{
					"custom_properties_json": types.StringType,
				},
				map[string]attr.Value{
					"custom_properties_json": types.StringNull(),
				},
			),
			expectError: false,
		},
		{
			name:        "valid_when_null",
			objectValue: types.ObjectNull(map[string]attr.Type{"custom_properties_json": types.StringType}),
			expectError: false,
		},
		{
			name:        "valid_when_unknown",
			objectValue: types.ObjectUnknown(map[string]attr.Type{"custom_properties_json": types.StringType}),
			expectError: false,
		},
		{
			name: "invalid_with_extra_field",
			objectValue: types.ObjectValueMust(
				map[string]attr.Type{
					"custom_properties_json":               types.StringType,
					"schema_registry_enable_authorization": types.BoolType,
				},
				map[string]attr.Value{
					"custom_properties_json":               types.StringNull(),
					"schema_registry_enable_authorization": types.BoolValue(true),
				},
			),
			expectError: true,
			errorMsg:    "schema_registry_enable_authorization",
		},
		{
			name: "invalid_with_only_wrong_field",
			objectValue: types.ObjectValueMust(
				map[string]attr.Type{
					"schema_registry_enable_authorization": types.BoolType,
				},
				map[string]attr.Value{
					"schema_registry_enable_authorization": types.BoolValue(true),
				},
			),
			expectError: true,
			errorMsg:    "schema_registry_enable_authorization",
		},
		{
			name: "invalid_multiple_wrong_fields",
			objectValue: types.ObjectValueMust(
				map[string]attr.Type{
					"custom_properties_json": types.StringType,
					"auto_create_topics":     types.BoolType,
					"retention_ms":           types.StringType,
				},
				map[string]attr.Value{
					"custom_properties_json": types.StringNull(),
					"auto_create_topics":     types.BoolValue(false),
					"retention_ms":           types.StringValue("604800000"),
				},
			),
			expectError: true,
			errorMsg:    "auto_create_topics",
		},
		{
			name: "invalid_empty_object",
			objectValue: types.ObjectValueMust(
				map[string]attr.Type{},
				map[string]attr.Value{},
			),
			expectError: true,
			errorMsg:    "custom_properties_json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.ObjectRequest{
				ConfigValue: tt.objectValue,
				Path:        path.Root("cluster_configuration"),
			}
			resp := &validator.ObjectResponse{
				Diagnostics: diag.Diagnostics{},
			}

			v.ValidateObject(ctx, req, resp)

			if tt.expectError {
				require.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
				if tt.errorMsg != "" {
					// Check that the error message contains the expected text
					errorFound := false
					for _, d := range resp.Diagnostics.Errors() {
						if strings.Contains(d.Detail(), tt.errorMsg) {
							errorFound = true
							break
						}
					}
					require.True(t, errorFound, "expected error message to contain '%s', but got: %v", tt.errorMsg, resp.Diagnostics.Errors())
				}
			} else {
				require.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestClusterConfigurationValidator_EdgeCases(t *testing.T) {
	ctx := context.Background()
	v := ClusterConfiguration()

	t.Run("handles_jsonencoded_string", func(t *testing.T) {
		// Test that the validator accepts a JSON-encoded string in custom_properties_json
		objectValue := types.ObjectValueMust(
			map[string]attr.Type{
				"custom_properties_json": types.StringType,
			},
			map[string]attr.Value{
				"custom_properties_json": types.StringValue(`{
					"schema_registry_enable_authorization": true,
					"auto.create.topics.enable": false,
					"log.retention.ms": "604800000"
				}`),
			},
		)

		req := validator.ObjectRequest{
			ConfigValue: objectValue,
			Path:        path.Root("cluster_configuration"),
		}
		resp := &validator.ObjectResponse{
			Diagnostics: diag.Diagnostics{},
		}

		v.ValidateObject(ctx, req, resp)
		require.False(t, resp.Diagnostics.HasError(), "should accept valid JSON in custom_properties_json")
	})

	t.Run("rejects_direct_properties", func(t *testing.T) {
		// Test that the validator rejects properties placed directly in cluster_configuration
		objectValue := types.ObjectValueMust(
			map[string]attr.Type{
				"log.retention.ms": types.StringType,
			},
			map[string]attr.Value{
				"log.retention.ms": types.StringValue("604800000"),
			},
		)

		req := validator.ObjectRequest{
			ConfigValue: objectValue,
			Path:        path.Root("cluster_configuration"),
		}
		resp := &validator.ObjectResponse{
			Diagnostics: diag.Diagnostics{},
		}

		v.ValidateObject(ctx, req, resp)
		require.True(t, resp.Diagnostics.HasError(), "should reject properties directly in cluster_configuration")
		require.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "log.retention.ms")
	})
}

func TestClusterConfigurationValidator_Methods(t *testing.T) {
	ctx := context.Background()
	v := ClusterConfiguration()

	t.Run("description", func(t *testing.T) {
		desc := v.Description(ctx)
		require.NotEmpty(t, desc)
		require.Contains(t, desc, "custom_properties_json")
	})

	t.Run("markdown_description", func(t *testing.T) {
		mdDesc := v.MarkdownDescription(ctx)
		require.NotEmpty(t, mdDesc)
		require.Contains(t, mdDesc, "`custom_properties_json`")
	})
}
