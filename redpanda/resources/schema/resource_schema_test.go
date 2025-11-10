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

package schema

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	schemamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/sr"
)

func Test_parseImportID(t *testing.T) {
	tests := []struct {
		name        string
		importID    string
		want        *importIDComponents
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid import - basic schema",
			importID: "cluster-123:my-subject:1:myuser:mypass",
			want: &importIDComponents{
				clusterID: "cluster-123",
				subject:   "my-subject",
				version:   1,
				username:  "myuser",
				password:  "mypass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with hyphens",
			importID: "cluster-456:my-test-subject:5:testuser:testpass",
			want: &importIDComponents{
				clusterID: "cluster-456",
				subject:   "my-test-subject",
				version:   5,
				username:  "testuser",
				password:  "testpass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with dots",
			importID: "cluster-789:com.example.schema:10:admin:secret123",
			want: &importIDComponents{
				clusterID: "cluster-789",
				subject:   "com.example.schema",
				version:   10,
				username:  "admin",
				password:  "secret123",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with underscores",
			importID: "prod-cluster:user_profile_schema:2:svc_account:p@ssw0rd",
			want: &importIDComponents{
				clusterID: "prod-cluster",
				subject:   "user_profile_schema",
				version:   2,
				username:  "svc_account",
				password:  "p@ssw0rd",
			},
			wantErr: false,
		},
		{
			name:     "valid import - version zero",
			importID: "cluster-1:test-subject:0:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "test-subject",
				version:   0,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - large version number",
			importID: "cluster-1:schema:999999:admin:adminpass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "schema",
				version:   999999,
				username:  "admin",
				password:  "adminpass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with mixed case",
			importID: "cluster-1:MyTestSchema:1:TestUser:TestPass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "MyTestSchema",
				version:   1,
				username:  "TestUser",
				password:  "TestPass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with numbers",
			importID: "cluster-1:schema123:1:user123:pass123",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "schema123",
				version:   1,
				username:  "user123",
				password:  "pass123",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - missing password",
			importID:    "cluster-1:my-subject:1:myuser",
			want:        nil,
			wantErr:     true,
			errContains: "got 4 parts (expected 5)",
		},
		{
			name:        "invalid import - missing username and password",
			importID:    "cluster-1:my-subject:1",
			want:        nil,
			wantErr:     true,
			errContains: "got 3 parts (expected 5)",
		},
		{
			name:        "invalid import - missing version, username, and password",
			importID:    "cluster-1:my-subject",
			want:        nil,
			wantErr:     true,
			errContains: "got 2 parts (expected 5)",
		},
		{
			name:        "invalid import - only cluster_id",
			importID:    "cluster-1",
			want:        nil,
			wantErr:     true,
			errContains: "got 1 parts (expected 5)",
		},
		{
			name:        "invalid import - empty string",
			importID:    "",
			want:        nil,
			wantErr:     true,
			errContains: "got 1 parts (expected 5)",
		},
		{
			name:        "invalid import - too many parts",
			importID:    "cluster-1:subject:1:user:pass:extra",
			want:        nil,
			wantErr:     true,
			errContains: "got 6 parts (expected 5)",
		},
		{
			name:        "invalid import - too many colons",
			importID:    "cluster-1:subject:1:user:pass:extra:more",
			want:        nil,
			wantErr:     true,
			errContains: "got 7 parts (expected 5)",
		},
		{
			name:     "valid import - empty cluster_id",
			importID: ":my-subject:1:user:pass",
			want: &importIDComponents{
				clusterID: "",
				subject:   "my-subject",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - empty subject",
			importID: "cluster-1::1:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - empty version",
			importID:    "cluster-1:my-subject::user:pass",
			want:        nil,
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:     "valid import - empty username",
			importID: "cluster-1:my-subject:1::pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   1,
				username:  "",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - empty password",
			importID: "cluster-1:my-subject:1:user:",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   1,
				username:  "user",
				password:  "",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - all empty",
			importID:    "::::",
			want:        nil,
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:     "valid import - special characters in subject",
			importID: "cluster-1:test-subject-$special:1:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "test-subject-$special",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - special characters in password",
			importID: "cluster-1:subject:1:user:p@ss!w0rd#",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "subject",
				version:   1,
				username:  "user",
				password:  "p@ss!w0rd#",
			},
			wantErr: false,
		},
		{
			name:     "valid import - version with leading zeros",
			importID: "cluster-1:my-subject:001:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - non-numeric version",
			importID:    "cluster-1:my-subject:latest:user:pass",
			want:        nil,
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:     "valid import - negative version (parses successfully)",
			importID: "cluster-1:my-subject:-1:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   -1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:        "valid import - colons in password",
			importID:    "cluster-1:subject:1:user:pass:with:colons",
			want:        nil,
			wantErr:     true,
			errContains: "got 7 parts (expected 5)",
		},
		{
			name:     "valid import - UUID cluster ID",
			importID: "c7n1234567890ab:my-subject:1:admin:secretpass",
			want: &importIDComponents{
				clusterID: "c7n1234567890ab",
				subject:   "my-subject",
				version:   1,
				username:  "admin",
				password:  "secretpass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - complex subject name",
			importID: "cluster-1:com.example.avro.User-v2:42:svc_user:complex_P@ss123",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "com.example.avro.User-v2",
				version:   42,
				username:  "svc_user",
				password:  "complex_P@ss123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseImportID(tt.importID)

			if tt.wantErr {
				require.Error(t, err, "parseImportID should return error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "error message should contain expected string")
				}
				assert.Nil(t, got, "result should be nil on error")
				return
			}

			require.NoError(t, err, "parseImportID should not return error")
			require.NotNil(t, got, "result should not be nil")

			assert.Equal(t, tt.want.clusterID, got.clusterID, "clusterID should match")
			assert.Equal(t, tt.want.subject, got.subject, "subject should match")
			assert.Equal(t, tt.want.version, got.version, "version should match")
			assert.Equal(t, tt.want.username, got.username, "username should match")
			assert.Equal(t, tt.want.password, got.password, "password should match")
		})
	}
}

// TestImportTypeConversions validates that parseImportID returns correct types
// and that they convert properly to Terraform types (types.StringValue(), types.Int64Value()).
func TestImportTypeConversions(t *testing.T) {
	tests := []struct {
		name            string
		importID        string
		expectClusterID string
		expectSubject   string
		expectVersion   int64
		expectUsername  string
		expectPassword  string
		wantErr         bool
		errContains     string
	}{
		{
			name:            "basic types convert correctly",
			importID:        "cluster-123:test-subject:42:testuser:testpass",
			expectClusterID: "cluster-123",
			expectSubject:   "test-subject",
			expectVersion:   42,
			expectUsername:  "testuser",
			expectPassword:  "testpass",
			wantErr:         false,
		},
		{
			name:            "version zero",
			importID:        "cluster-1:subject:0:user:pass",
			expectClusterID: "cluster-1",
			expectSubject:   "subject",
			expectVersion:   0,
			expectUsername:  "user",
			expectPassword:  "pass",
			wantErr:         false,
		},
		{
			name:            "large version number",
			importID:        "cluster-1:subject:999999:user:pass",
			expectClusterID: "cluster-1",
			expectSubject:   "subject",
			expectVersion:   999999,
			expectUsername:  "user",
			expectPassword:  "pass",
			wantErr:         false,
		},
		{
			name:            "negative version",
			importID:        "cluster-1:subject:-1:user:pass",
			expectClusterID: "cluster-1",
			expectSubject:   "subject",
			expectVersion:   -1,
			expectUsername:  "user",
			expectPassword:  "pass",
			wantErr:         false,
		},
		{
			name:            "empty strings",
			importID:        "::1::",
			expectClusterID: "",
			expectSubject:   "",
			expectVersion:   1,
			expectUsername:  "",
			expectPassword:  "",
			wantErr:         false,
		},
		{
			name:            "special characters in strings",
			importID:        "cluster-123:test-$subject:1:user@example.com:p@ss!w0rd#",
			expectClusterID: "cluster-123",
			expectSubject:   "test-$subject",
			expectVersion:   1,
			expectUsername:  "user@example.com",
			expectPassword:  "p@ss!w0rd#",
			wantErr:         false,
		},
		{
			name:        "non-numeric version fails",
			importID:    "cluster-1:subject:latest:user:pass",
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:        "empty version fails",
			importID:    "cluster-1:subject::user:pass",
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components, err := parseImportID(tt.importID)

			if tt.wantErr {
				require.Error(t, err, "parseImportID should return error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, components)

			// Validate Go types
			assert.IsType(t, "", components.clusterID)
			assert.IsType(t, "", components.subject)
			assert.IsType(t, int64(0), components.version)
			assert.IsType(t, "", components.username)
			assert.IsType(t, "", components.password)

			// Validate values
			assert.Equal(t, tt.expectClusterID, components.clusterID)
			assert.Equal(t, tt.expectSubject, components.subject)
			assert.Equal(t, tt.expectVersion, components.version)
			assert.Equal(t, tt.expectUsername, components.username)
			assert.Equal(t, tt.expectPassword, components.password)

			// Validate Terraform type conversions
			clusterIDValue := types.StringValue(components.clusterID)
			assert.False(t, clusterIDValue.IsNull())
			assert.Equal(t, components.clusterID, clusterIDValue.ValueString())

			subjectValue := types.StringValue(components.subject)
			assert.False(t, subjectValue.IsNull())
			assert.Equal(t, components.subject, subjectValue.ValueString())

			versionValue := types.Int64Value(components.version)
			assert.False(t, versionValue.IsNull())
			assert.Equal(t, components.version, versionValue.ValueInt64())

			usernameValue := types.StringValue(components.username)
			assert.False(t, usernameValue.IsNull())
			assert.Equal(t, components.username, usernameValue.ValueString())

			passwordValue := types.StringValue(components.password)
			assert.False(t, passwordValue.IsNull())
			assert.Equal(t, components.password, passwordValue.ValueString())
		})
	}
}

func TestSchema_Create(t *testing.T) {
	tests := []struct {
		name               string
		input              schemamodel.ResourceModel
		clientFactoryError error
		createSchemaError  error
		setCompatError     error
		mockResponse       sr.SubjectSchema
		compatResults      []sr.CompatibilityResult
		wantErr            bool
		errorContains      string
	}{
		{
			name: "basic AVRO schema",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockResponse: sr.SubjectSchema{
				ID:      1,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
		},
		{
			name: "JSON schema with compatibility",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("json-subject"),
				Schema:        types.StringValue(`{"type": "object", "properties": {"name": {"type": "string"}}}`),
				SchemaType:    types.StringValue("JSON"),
				Compatibility: types.StringValue("FULL"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockResponse: sr.SubjectSchema{
				ID:      2,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
					Type:   sr.TypeJSON,
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatFull,
					Subject: "json-subject",
				},
			},
		},
		{
			name: "PROTOBUF schema",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("proto-subject"),
				Schema:     types.StringValue(`syntax = "proto3"; message Test { string name = 1; }`),
				SchemaType: types.StringValue("PROTOBUF"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockResponse: sr.SubjectSchema{
				ID:      3,
				Version: 1,
				Schema: sr.Schema{
					Schema: `syntax = "proto3"; message Test { string name = 1; }`,
					Type:   sr.TypeProtobuf,
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "proto-subject",
				},
			},
		},
		{
			name: "client factory fails",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			clientFactoryError: errors.New("failed to connect to cluster"),
			wantErr:            true,
			errorContains:      "Failed to create Schema Registry client",
		},
		{
			name: "CreateSchema API error",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			createSchemaError: errors.New("schema validation failed"),
			wantErr:           true,
			errorContains:     "Failed to create schema",
		},
		{
			name: "SetCompatibility fails after successful creation",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("FULL"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			setCompatError: errors.New("invalid compatibility level"),
			wantErr:        true,
			errorContains:  "Failed to set compatibility level",
		},
		{
			name: "create success - compatibility retrieval fails, uses default",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockResponse: sr.SubjectSchema{
				ID:      1,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Err: errors.New("failed to retrieve compatibility"),
				},
			},
			wantErr: false,
		},
		{
			name: "create success - schema with references",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Schema:     types.StringValue(`{"type": "record", "name": "Test", "fields": [{"name": "id", "type": "string"}]}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockResponse: sr.SubjectSchema{
				ID:      1,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "record", "name": "Test", "fields": [{"name": "id", "type": "string"}]}`,
					Type:   sr.TypeAvro,
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSRClienter(ctrl)

			var s *Schema
			if tt.clientFactoryError != nil {
				s = &Schema{
					clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
						return nil, tt.clientFactoryError
					},
				}
			} else {
				if tt.createSchemaError != nil {
					mockClient.EXPECT().
						CreateSchema(ctx, tt.input.Subject.ValueString(), gomock.Any()).
						Return(sr.SubjectSchema{}, tt.createSchemaError)
				} else {
					mockClient.EXPECT().
						CreateSchema(ctx, tt.input.Subject.ValueString(), gomock.Any()).
						Return(tt.mockResponse, nil)
				}

				if !tt.input.Compatibility.IsNull() && tt.createSchemaError == nil {
					if tt.setCompatError != nil {
						results := []sr.CompatibilityResult{
							{
								Err: tt.setCompatError,
							},
						}
						mockClient.EXPECT().
							SetCompatibility(ctx, gomock.Any(), tt.input.Subject.ValueString()).
							Return(results)
					} else {
						mockClient.EXPECT().
							SetCompatibility(ctx, gomock.Any(), tt.input.Subject.ValueString()).
							Return(tt.compatResults)
					}
				} else if tt.input.Compatibility.IsNull() && tt.createSchemaError == nil {
					mockClient.EXPECT().
						Compatibility(ctx, tt.input.Subject.ValueString()).
						Return(tt.compatResults)
				}

				s = &Schema{
					clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
						return mockClient, nil
					},
				}
			}

			schemaResp := resource.SchemaResponse{}
			s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := req.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			resp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Create(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				if tt.errorContains != "" {
					found := false
					for _, diag := range resp.Diagnostics.Errors() {
						if strings.Contains(diag.Summary(), tt.errorContains) || strings.Contains(diag.Detail(), tt.errorContains) {
							found = true
							break
						}
					}
					assert.True(t, found, "error should contain '%s'", tt.errorContains)
				}
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Create should not error: %v", resp.Diagnostics)

			var state schemamodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			assert.Equal(t, tt.input.ClusterID, state.ClusterID, "ClusterID should match plan")
			assert.Equal(t, tt.input.Subject, state.Subject, "Subject should match plan")
			assert.Equal(t, types.Int64Value(int64(tt.mockResponse.ID)), state.ID, "ID should be set from response")
			assert.Equal(t, types.Int64Value(int64(tt.mockResponse.Version)), state.Version, "Version should be set from response")
			assert.Equal(t, tt.input.Schema, state.Schema, "Schema should match plan")
			assert.Equal(t, tt.input.SchemaType, state.SchemaType, "SchemaType should match plan")
			assert.Equal(t, tt.input.Username, state.Username, "Username should be preserved")
			assert.Equal(t, tt.input.Password, state.Password, "Password should be preserved")
			assert.Equal(t, tt.input.References, state.References, "References should match plan")
			assert.Equal(t, tt.input.AllowDeletion, state.AllowDeletion, "AllowDeletion should match plan")
			assert.False(t, state.Compatibility.IsNull(), "Compatibility should be set")
		})
	}
}

func TestSchema_CreateReadConsistency(t *testing.T) {
	tests := []struct {
		name           string
		input          schemamodel.ResourceModel
		mockCreateResp sr.SubjectSchema
		mockReadResp   sr.SubjectSchema
		compatResults  []sr.CompatibilityResult
	}{
		{
			name: "basic AVRO schema - all fields persist through create and read",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      1,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      1,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
		},
		{
			name: "schema with explicit FULL compatibility persists",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("json-subject"),
				Schema:        types.StringValue(`{"type": "object", "properties": {"name": {"type": "string"}}}`),
				SchemaType:    types.StringValue("JSON"),
				Compatibility: types.StringValue("FULL"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      2,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
					Type:   sr.TypeJSON,
				},
				Subject: "json-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      2,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
					Type:   sr.TypeJSON,
				},
				Subject: "json-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatFull,
					Subject: "json-subject",
				},
			},
		},
		{
			name: "PROTOBUF schema with BACKWARD_TRANSITIVE compatibility",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("proto-subject"),
				Schema:        types.StringValue(`syntax = "proto3"; message Test { string name = 1; }`),
				SchemaType:    types.StringValue("PROTOBUF"),
				Compatibility: types.StringValue("BACKWARD_TRANSITIVE"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      3,
				Version: 1,
				Schema: sr.Schema{
					Schema: `syntax = "proto3"; message Test { string name = 1; }`,
					Type:   sr.TypeProtobuf,
				},
				Subject: "proto-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      3,
				Version: 1,
				Schema: sr.Schema{
					Schema: `syntax = "proto3"; message Test { string name = 1; }`,
					Type:   sr.TypeProtobuf,
				},
				Subject: "proto-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackwardTransitive,
					Subject: "proto-subject",
				},
			},
		},
		{
			name: "schema with references persists correctly",
			input: schemamodel.ResourceModel{
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("ref-subject"),
				Schema:     types.StringValue(`{"type": "record", "name": "Test", "fields": [{"name": "id", "type": "string"}]}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListValueMust(
					types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"name":    types.StringType,
							"subject": types.StringType,
							"version": types.Int64Type,
						},
					},
					[]attr.Value{
						types.ObjectValueMust(
							map[string]attr.Type{
								"name":    types.StringType,
								"subject": types.StringType,
								"version": types.Int64Type,
							},
							map[string]attr.Value{
								"name":    types.StringValue("Reference1"),
								"subject": types.StringValue("ref-dep-subject"),
								"version": types.Int64Value(1),
							},
						),
					},
				),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      4,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "record", "name": "Test", "fields": [{"name": "id", "type": "string"}]}`,
					Type:   sr.TypeAvro,
					References: []sr.SchemaReference{
						{
							Name:    "Reference1",
							Subject: "ref-dep-subject",
							Version: 1,
						},
					},
				},
				Subject: "ref-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      4,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "record", "name": "Test", "fields": [{"name": "id", "type": "string"}]}`,
					Type:   sr.TypeAvro,
					References: []sr.SchemaReference{
						{
							Name:    "Reference1",
							Subject: "ref-dep-subject",
							Version: 1,
						},
					},
				},
				Subject: "ref-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "ref-subject",
				},
			},
		},
		{
			name: "schema with allow_deletion true persists",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("deletable-subject"),
				Schema:        types.StringValue(`{"type": "int"}`),
				SchemaType:    types.StringValue("AVRO"),
				AllowDeletion: types.BoolValue(true),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      5,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "deletable-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      5,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "deletable-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "deletable-subject",
				},
			},
		},
		{
			name: "schema with all optional fields null persists correctly",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("minimal-subject"),
				Schema:        types.StringValue(`{"type": "boolean"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringNull(),
				AllowDeletion: types.BoolNull(),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      6,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "boolean"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "minimal-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      6,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "boolean"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "minimal-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "minimal-subject",
				},
			},
		},
		{
			name: "complex JSON schema with FORWARD compatibility",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("complex-json"),
				Schema:        types.StringValue(`{"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}, "active": {"type": "boolean"}}}`),
				SchemaType:    types.StringValue("JSON"),
				Compatibility: types.StringValue("FORWARD"),
				Username:      types.StringValue("admin"),
				Password:      types.StringValue("secret123"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      7,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}, "active": {"type": "boolean"}}}`,
					Type:   sr.TypeJSON,
				},
				Subject: "complex-json",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      7,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}, "active": {"type": "boolean"}}}`,
					Type:   sr.TypeJSON,
				},
				Subject: "complex-json",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatForward,
					Subject: "complex-json",
				},
			},
		},
		{
			name: "schema with NONE compatibility persists",
			input: schemamodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("no-compat-subject"),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("NONE"),
				AllowDeletion: types.BoolValue(false),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      8,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "no-compat-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      8,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "no-compat-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatNone,
					Subject: "no-compat-subject",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSRClienter(ctrl)

			// Mock Create operation
			mockClient.EXPECT().
				CreateSchema(ctx, tt.input.Subject.ValueString(), gomock.Any()).
				Return(tt.mockCreateResp, nil)

			// Mock compatibility for Create
			if !tt.input.Compatibility.IsNull() {
				mockClient.EXPECT().
					SetCompatibility(ctx, gomock.Any(), tt.input.Subject.ValueString()).
					Return(tt.compatResults)
			} else {
				mockClient.EXPECT().
					Compatibility(ctx, tt.input.Subject.ValueString()).
					Return(tt.compatResults)
			}

			// Mock Read operation that follows Create
			mockClient.EXPECT().
				SchemaByVersion(ctx, tt.input.Subject.ValueString(), tt.mockCreateResp.Version).
				Return(tt.mockReadResp, nil)

			// Mock compatibility for Read
			mockClient.EXPECT().
				Compatibility(ctx, tt.input.Subject.ValueString()).
				Return(tt.compatResults)

			s := &Schema{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Execute Create
			createReq := resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := createReq.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			createResp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Create(ctx, createReq, &createResp)
			require.False(t, createResp.Diagnostics.HasError(), "Create should not error: %v", createResp.Diagnostics)

			var stateAfterCreate schemamodel.ResourceModel
			diags = createResp.State.Get(ctx, &stateAfterCreate)
			require.False(t, diags.HasError(), "State.Get after Create should not error")

			// Execute Read using the state from Create
			readReq := resource.ReadRequest{
				State: createResp.State,
			}
			readResp := resource.ReadResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Read(ctx, readReq, &readResp)
			require.False(t, readResp.Diagnostics.HasError(), "Read should not error: %v", readResp.Diagnostics)

			var stateAfterRead schemamodel.ResourceModel
			diags = readResp.State.Get(ctx, &stateAfterRead)
			require.False(t, diags.HasError(), "State.Get after Read should not error")

			// CRITICAL: Verify ALL fields match between Create state and Read state
			assert.Equal(t, stateAfterCreate.ID, stateAfterRead.ID, "ID should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.Version, stateAfterRead.Version, "Version should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.ClusterID, stateAfterRead.ClusterID, "ClusterID should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.Subject, stateAfterRead.Subject, "Subject should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.Schema, stateAfterRead.Schema, "Schema should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.SchemaType, stateAfterRead.SchemaType, "SchemaType should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.Compatibility, stateAfterRead.Compatibility, "Compatibility should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.References, stateAfterRead.References, "References should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.Username, stateAfterRead.Username, "Username should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.Password, stateAfterRead.Password, "Password should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.AllowDeletion, stateAfterRead.AllowDeletion, "AllowDeletion should be consistent between Create and Read")
		})
	}
}

func TestSchema_Read(t *testing.T) {
	tests := []struct {
		name               string
		initialState       schemamodel.ResourceModel
		clientFactoryError error
		schemaByVersionErr error
		schemasErr         error
		compatErr          error
		mockSchemaResp     []sr.SubjectSchema
		compatResults      []sr.CompatibilityResult
		expectRemoved      bool
		wantErr            bool
	}{
		{
			name: "schema exists and matches",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockSchemaResp: []sr.SubjectSchema{
				{
					ID:      1,
					Version: 1,
					Schema: sr.Schema{
						Schema: `{"type": "string"}`,
						Type:   sr.TypeAvro,
					},
					Subject: "test-subject",
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
			expectRemoved: false,
		},
		{
			name: "schema with all fields populated",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(2),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("json-subject"),
				Version:       types.Int64Value(2),
				Schema:        types.StringValue(`{"type": "object"}`),
				SchemaType:    types.StringValue("JSON"),
				Compatibility: types.StringValue("FULL"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				AllowDeletion: types.BoolValue(true),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockSchemaResp: []sr.SubjectSchema{
				{
					ID:      2,
					Version: 2,
					Schema: sr.Schema{
						Schema: `{"type": "object"}`,
						Type:   sr.TypeJSON,
					},
					Subject: "json-subject",
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatFull,
					Subject: "json-subject",
				},
			},
			expectRemoved: false,
		},
		{
			name: "schema not found removes from state",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			schemaByVersionErr: errors.New("not found"),
			expectRemoved:      true,
			wantErr:            false,
		},
		{
			name: "cluster unreachable with allow_deletion true removes from state",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				AllowDeletion: types.BoolValue(true),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			clientFactoryError: errors.New("name resolver error: produced zero addresses"),
			expectRemoved:      true,
			wantErr:            false,
		},
		{
			name: "cluster unreachable with allow_deletion false returns error",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				AllowDeletion: types.BoolValue(false),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			clientFactoryError: errors.New("name resolver error: produced zero addresses"),
			expectRemoved:      false,
			wantErr:            true,
		},
		{
			name: "cluster unreachable with allow_deletion null removes from state",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			clientFactoryError: errors.New("name resolver error: produced zero addresses"),
			expectRemoved:      true,
			wantErr:            false,
		},
		{
			name: "permission denied with allow_deletion true removes from state",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				AllowDeletion: types.BoolValue(true),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			clientFactoryError: errors.New("403 Forbidden"),
			expectRemoved:      true,
			wantErr:            false,
		},
		{
			name: "permission denied with allow_deletion false returns error",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				AllowDeletion: types.BoolValue(false),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			clientFactoryError: errors.New("403 Forbidden"),
			expectRemoved:      false,
			wantErr:            true,
		},
		{
			name: "null cluster_id removes from state",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringNull(),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			expectRemoved: true,
			wantErr:       false,
		},
		{
			name: "empty cluster_id removes from state",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue(""),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			expectRemoved: true,
			wantErr:       false,
		},
		{
			name: "client creation error returns error",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			clientFactoryError: errors.New("failed to connect"),
			expectRemoved:      false,
			wantErr:            true,
		},
		{
			name: "SchemaByVersion API error",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			schemaByVersionErr: errors.New("internal server error"),
			expectRemoved:      false,
			wantErr:            true,
		},
		{
			name: "Schemas API error when version is null",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Null(),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			schemasErr:    errors.New("internal server error"),
			expectRemoved: false,
			wantErr:       true,
		},
		{
			name: "compatibility retrieval fails, uses existing value",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("BACKWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			mockSchemaResp: []sr.SubjectSchema{
				{
					ID:      1,
					Version: 1,
					Schema: sr.Schema{
						Schema: `{"type": "string"}`,
						Type:   sr.TypeAvro,
					},
					Subject: "test-subject",
				},
			},
			compatErr:     errors.New("failed to get compatibility"),
			expectRemoved: false,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSRClienter(ctrl)

			var s *Schema
			//nolint:gocritic // if-else chain is clearer than switch for this test setup logic
			if tt.clientFactoryError != nil {
				s = &Schema{
					clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
						return nil, tt.clientFactoryError
					},
				}
			} else if tt.initialState.ClusterID.IsNull() || tt.initialState.ClusterID.ValueString() == "" {
				// Don't set up mock client for null/empty cluster ID
				s = &Schema{
					clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
						return mockClient, nil
					},
				}
			} else {
				// Set up mock client expectations
				if tt.initialState.Version.IsNull() {
					if tt.schemasErr != nil {
						mockClient.EXPECT().
							Schemas(ctx, tt.initialState.Subject.ValueString()).
							Return(nil, tt.schemasErr)
					} else {
						mockClient.EXPECT().
							Schemas(ctx, tt.initialState.Subject.ValueString()).
							Return(tt.mockSchemaResp, nil)
					}
				} else {
					if tt.schemaByVersionErr != nil {
						mockClient.EXPECT().
							SchemaByVersion(ctx, tt.initialState.Subject.ValueString(), int(tt.initialState.Version.ValueInt64())).
							Return(sr.SubjectSchema{}, tt.schemaByVersionErr)
					} else {
						mockClient.EXPECT().
							SchemaByVersion(ctx, tt.initialState.Subject.ValueString(), int(tt.initialState.Version.ValueInt64())).
							Return(tt.mockSchemaResp[0], nil)
					}
				}

				// Set up compatibility expectations if schema fetch succeeded
				if tt.schemaByVersionErr == nil && tt.schemasErr == nil {
					if tt.compatErr != nil {
						results := []sr.CompatibilityResult{
							{
								Err: tt.compatErr,
							},
						}
						mockClient.EXPECT().
							Compatibility(ctx, tt.initialState.Subject.ValueString()).
							Return(results)
					} else {
						mockClient.EXPECT().
							Compatibility(ctx, tt.initialState.Subject.ValueString()).
							Return(tt.compatResults)
					}
				}

				s = &Schema{
					clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
						return mockClient, nil
					},
				}
			}

			schemaResp := resource.SchemaResponse{}
			s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.ReadRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")

			resp := resource.ReadResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Read(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Read should not error: %v", resp.Diagnostics)

			if tt.expectRemoved {
				var state *schemamodel.ResourceModel
				diags = resp.State.Get(ctx, &state)
				require.False(t, diags.HasError(), "State.Get should not error")
				assert.Nil(t, state, "State should be removed")
				return
			}

			var state schemamodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			assert.Equal(t, tt.initialState.ID, state.ID, "ID should be preserved")
			assert.Equal(t, tt.initialState.ClusterID, state.ClusterID, "ClusterID should be preserved")
			assert.Equal(t, tt.initialState.Subject, state.Subject, "Subject should be preserved")
			assert.Equal(t, tt.initialState.Version, state.Version, "Version should be preserved")
			assert.Equal(t, tt.initialState.Schema, state.Schema, "Schema should be preserved")
			assert.Equal(t, tt.initialState.SchemaType, state.SchemaType, "SchemaType should be preserved")
			assert.Equal(t, tt.initialState.Username, state.Username, "Username should be preserved")
			assert.Equal(t, tt.initialState.Password, state.Password, "Password should be preserved")
			assert.Equal(t, tt.initialState.References, state.References, "References should be preserved")
			assert.Equal(t, tt.initialState.AllowDeletion, state.AllowDeletion, "AllowDeletion should be preserved")
			assert.False(t, state.Compatibility.IsNull(), "Compatibility should be set")
		})
	}
}

func TestSchema_Update(t *testing.T) {
	tests := []struct {
		name           string
		initialState   schemamodel.ResourceModel
		plan           schemamodel.ResourceModel
		mockCreateResp sr.SubjectSchema
		expectCreate   bool
		compatResults  []sr.CompatibilityResult
		wantErr        bool
	}{
		{
			name: "schema content changed - creates new version",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			plan: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`{"type": "int"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			mockCreateResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
			expectCreate: true,
		},
		{
			name: "only compatibility changed",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("BACKWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			plan: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("FULL"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatFull,
					Subject: "test-subject",
				},
			},
			expectCreate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSRClienter(ctrl)

			if tt.expectCreate {
				mockClient.EXPECT().
					CreateSchema(ctx, tt.plan.Subject.ValueString(), gomock.Any()).
					Return(tt.mockCreateResp, nil)
			}

			if !tt.initialState.Compatibility.Equal(tt.plan.Compatibility) && !tt.plan.Compatibility.IsNull() {
				mockClient.EXPECT().
					SetCompatibility(ctx, gomock.Any(), tt.plan.Subject.ValueString()).
					Return(tt.compatResults)
			}

			// After any operation, we now verify compatibility
			if len(tt.compatResults) > 0 {
				mockClient.EXPECT().
					Compatibility(ctx, tt.plan.Subject.ValueString()).
					Return(tt.compatResults)
			}

			s := &Schema{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.UpdateRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
				Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")
			diags = req.Plan.Set(ctx, &tt.plan)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			resp := resource.UpdateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Update(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Update should not error: %v", resp.Diagnostics)

			var state schemamodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			if tt.expectCreate {
				assert.Equal(t, types.Int64Value(int64(tt.mockCreateResp.Version)), state.Version, "Version should be updated")
				assert.Equal(t, types.Int64Value(int64(tt.mockCreateResp.ID)), state.ID, "ID should be updated")
				assert.Equal(t, tt.plan.Schema, state.Schema, "Schema should match new plan")
			} else {
				assert.Equal(t, tt.initialState.Version, state.Version, "Version should not change")
				assert.Equal(t, tt.initialState.ID, state.ID, "ID should not change")
				assert.Equal(t, tt.initialState.Schema, state.Schema, "Schema should not change")
			}

			// Common assertions - fields that should always match plan (or be preserved)
			assert.Equal(t, tt.plan.ClusterID, state.ClusterID, "ClusterID should match plan")
			assert.Equal(t, tt.plan.Subject, state.Subject, "Subject should match plan")
			assert.Equal(t, tt.plan.SchemaType, state.SchemaType, "SchemaType should match plan")
			assert.Equal(t, tt.plan.Username, state.Username, "Username should be preserved")
			assert.Equal(t, tt.plan.Password, state.Password, "Password should be preserved")
			assert.Equal(t, tt.plan.References, state.References, "References should match plan")
			assert.Equal(t, tt.plan.AllowDeletion, state.AllowDeletion, "AllowDeletion should match plan")

			// Compatibility check
			if !tt.plan.Compatibility.IsNull() {
				assert.Equal(t, tt.plan.Compatibility, state.Compatibility, "Compatibility should match plan")
			}
		})
	}
}

func TestSchema_UpdateReadConsistency(t *testing.T) {
	tests := []struct {
		name             string
		initialState     schemamodel.ResourceModel
		plan             schemamodel.ResourceModel
		mockUpdateResp   sr.SubjectSchema
		mockReadResp     sr.SubjectSchema
		compatResults    []sr.CompatibilityResult
		expectNewVersion bool
	}{
		{
			name: "schema content change creates new version and persists",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("BACKWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			plan: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "int"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("BACKWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockUpdateResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
			expectNewVersion: true,
		},
		{
			name: "only compatibility changed - no new version",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("BACKWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			plan: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				Compatibility: types.StringValue("FULL"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockReadResp: sr.SubjectSchema{
				ID:      1,
				Version: 1,
				Schema: sr.Schema{
					Schema: `{"type": "string"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatFull,
					Subject: "test-subject",
				},
			},
			expectNewVersion: false,
		},
		{
			name: "schema and compatibility both change",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("json-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "object"}`),
				SchemaType:    types.StringValue("JSON"),
				Compatibility: types.StringValue("BACKWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			plan: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("json-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "object", "properties": {"id": {"type": "integer"}}}`),
				SchemaType:    types.StringValue("JSON"),
				Compatibility: types.StringValue("FORWARD"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockUpdateResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"id": {"type": "integer"}}}`,
					Type:   sr.TypeJSON,
				},
				Subject: "json-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "object", "properties": {"id": {"type": "integer"}}}`,
					Type:   sr.TypeJSON,
				},
				Subject: "json-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatForward,
					Subject: "json-subject",
				},
			},
			expectNewVersion: true,
		},
		{
			name: "PROTOBUF schema evolves with references",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("proto-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`syntax = "proto3"; message Test { string name = 1; }`),
				SchemaType: types.StringValue("PROTOBUF"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			plan: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("proto-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`syntax = "proto3"; message Test { string name = 1; int32 id = 2; }`),
				SchemaType: types.StringValue("PROTOBUF"),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListValueMust(
					types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"name":    types.StringType,
							"subject": types.StringType,
							"version": types.Int64Type,
						},
					},
					[]attr.Value{
						types.ObjectValueMust(
							map[string]attr.Type{
								"name":    types.StringType,
								"subject": types.StringType,
								"version": types.Int64Type,
							},
							map[string]attr.Value{
								"name":    types.StringValue("Dependency"),
								"subject": types.StringValue("dep-subject"),
								"version": types.Int64Value(1),
							},
						),
					},
				),
			},
			mockUpdateResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `syntax = "proto3"; message Test { string name = 1; int32 id = 2; }`,
					Type:   sr.TypeProtobuf,
					References: []sr.SchemaReference{
						{
							Name:    "Dependency",
							Subject: "dep-subject",
							Version: 1,
						},
					},
				},
				Subject: "proto-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `syntax = "proto3"; message Test { string name = 1; int32 id = 2; }`,
					Type:   sr.TypeProtobuf,
					References: []sr.SchemaReference{
						{
							Name:    "Dependency",
							Subject: "dep-subject",
							Version: 1,
						},
					},
				},
				Subject: "proto-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "proto-subject",
				},
			},
			expectNewVersion: true,
		},
		{
			name: "allow_deletion flag persists through update",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "string"}`),
				SchemaType:    types.StringValue("AVRO"),
				AllowDeletion: types.BoolValue(false),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			plan: schemamodel.ResourceModel{
				ID:            types.Int64Value(1),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				Schema:        types.StringValue(`{"type": "int"}`),
				SchemaType:    types.StringValue("AVRO"),
				AllowDeletion: types.BoolValue(true),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockUpdateResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "int"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "test-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "test-subject",
				},
			},
			expectNewVersion: true,
		},
		{
			name: "immutable fields remain consistent",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("immutable-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`{"type": "string"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("original-user"),
				Password:   types.StringValue("original-pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			plan: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("immutable-subject"),
				Version:    types.Int64Value(1),
				Schema:     types.StringValue(`{"type": "boolean"}`),
				SchemaType: types.StringValue("AVRO"),
				Username:   types.StringValue("original-user"),
				Password:   types.StringValue("original-pass"),
				References: types.ListNull(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":    types.StringType,
						"subject": types.StringType,
						"version": types.Int64Type,
					},
				}),
			},
			mockUpdateResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "boolean"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "immutable-subject",
			},
			mockReadResp: sr.SubjectSchema{
				ID:      2,
				Version: 2,
				Schema: sr.Schema{
					Schema: `{"type": "boolean"}`,
					Type:   sr.TypeAvro,
				},
				Subject: "immutable-subject",
			},
			compatResults: []sr.CompatibilityResult{
				{
					Level:   sr.CompatBackward,
					Subject: "immutable-subject",
				},
			},
			expectNewVersion: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSRClienter(ctrl)

			// Mock Update operation
			if tt.expectNewVersion {
				mockClient.EXPECT().
					CreateSchema(ctx, tt.plan.Subject.ValueString(), gomock.Any()).
					Return(tt.mockUpdateResp, nil)

				// If compatibility also changed, expect SetCompatibility
				if !tt.initialState.Compatibility.Equal(tt.plan.Compatibility) {
					mockClient.EXPECT().
						SetCompatibility(ctx, gomock.Any(), tt.plan.Subject.ValueString()).
						Return(tt.compatResults)
				}

				mockClient.EXPECT().
					Compatibility(ctx, tt.plan.Subject.ValueString()).
					Return(tt.compatResults)
			} else {
				// Only compatibility changed
				mockClient.EXPECT().
					SetCompatibility(ctx, gomock.Any(), tt.plan.Subject.ValueString()).
					Return(tt.compatResults)

				mockClient.EXPECT().
					Compatibility(ctx, tt.plan.Subject.ValueString()).
					Return(tt.compatResults)
			}

			// Mock Read operation that follows Update
			mockClient.EXPECT().
				SchemaByVersion(ctx, tt.plan.Subject.ValueString(), tt.mockReadResp.Version).
				Return(tt.mockReadResp, nil)

			// Mock compatibility for Read
			mockClient.EXPECT().
				Compatibility(ctx, tt.plan.Subject.ValueString()).
				Return(tt.compatResults)

			s := &Schema{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Execute Update
			updateReq := resource.UpdateRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
				Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := updateReq.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")
			diags = updateReq.Plan.Set(ctx, &tt.plan)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			updateResp := resource.UpdateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Update(ctx, updateReq, &updateResp)
			require.False(t, updateResp.Diagnostics.HasError(), "Update should not error: %v", updateResp.Diagnostics)

			var stateAfterUpdate schemamodel.ResourceModel
			diags = updateResp.State.Get(ctx, &stateAfterUpdate)
			require.False(t, diags.HasError(), "State.Get after Update should not error")

			// Execute Read using the state from Update
			readReq := resource.ReadRequest{
				State: updateResp.State,
			}
			readResp := resource.ReadResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Read(ctx, readReq, &readResp)
			require.False(t, readResp.Diagnostics.HasError(), "Read should not error: %v", readResp.Diagnostics)

			var stateAfterRead schemamodel.ResourceModel
			diags = readResp.State.Get(ctx, &stateAfterRead)
			require.False(t, diags.HasError(), "State.Get after Read should not error")

			// CRITICAL: Verify ALL fields match between Update state and Read state
			assert.Equal(t, stateAfterUpdate.ID, stateAfterRead.ID, "ID should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.Version, stateAfterRead.Version, "Version should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.ClusterID, stateAfterRead.ClusterID, "ClusterID should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.Subject, stateAfterRead.Subject, "Subject should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.Schema, stateAfterRead.Schema, "Schema should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.SchemaType, stateAfterRead.SchemaType, "SchemaType should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.Compatibility, stateAfterRead.Compatibility, "Compatibility should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.References, stateAfterRead.References, "References should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.Username, stateAfterRead.Username, "Username should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.Password, stateAfterRead.Password, "Password should be consistent between Update and Read")
			assert.Equal(t, stateAfterUpdate.AllowDeletion, stateAfterRead.AllowDeletion, "AllowDeletion should be consistent between Update and Read")

			// Verify immutable fields haven't changed from initial state
			assert.Equal(t, tt.initialState.ClusterID, stateAfterRead.ClusterID, "ClusterID is immutable")
			assert.Equal(t, tt.initialState.Subject, stateAfterRead.Subject, "Subject is immutable")
			assert.Equal(t, tt.initialState.Username, stateAfterRead.Username, "Username is immutable")
			assert.Equal(t, tt.initialState.Password, stateAfterRead.Password, "Password is immutable")
		})
	}
}

func TestSchema_Delete(t *testing.T) {
	tests := []struct {
		name         string
		initialState schemamodel.ResourceModel
		wantErr      bool
	}{
		{
			name: "successful deletion",
			initialState: schemamodel.ResourceModel{
				ID:         types.Int64Value(1),
				ClusterID:  types.StringValue("cluster-1"),
				Subject:    types.StringValue("test-subject"),
				Version:    types.Int64Value(1),
				Username:   types.StringValue("user"),
				Password:   types.StringValue("pass"),
				References: types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
		},
		{
			name: "deletion with allow_deletion=true",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(2),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				AllowDeletion: types.BoolValue(true),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
		},
		{
			name: "deletion with allow_deletion=false",
			initialState: schemamodel.ResourceModel{
				ID:            types.Int64Value(3),
				ClusterID:     types.StringValue("cluster-1"),
				Subject:       types.StringValue("test-subject"),
				Version:       types.Int64Value(1),
				AllowDeletion: types.BoolValue(false),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				References:    types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "subject": types.StringType, "version": types.Int64Type}}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSRClienter(ctrl)

			mockClient.EXPECT().
				DeleteSubject(ctx, tt.initialState.Subject.ValueString(), sr.SoftDelete).
				Return([]int{int(tt.initialState.Version.ValueInt64())}, nil)

			s := &Schema{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (SRClienter, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.DeleteRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")

			resp := resource.DeleteResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			s.Delete(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Delete should not error: %v", resp.Diagnostics)

			var state *schemamodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")
			assert.Nil(t, state, "State should be removed after delete")
		})
	}
}
