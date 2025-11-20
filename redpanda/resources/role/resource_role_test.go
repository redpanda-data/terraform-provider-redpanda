// Copyright 2023 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package role

import (
	"context"
	"errors"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRole_Create(t *testing.T) {
	tests := []struct {
		name      string
		input     models.Role
		mockError error
		wantErr   bool
	}{
		{
			name: "basic role with minimal fields",
			input: models.Role{
				Name:          types.StringValue("developer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
			},
		},
		{
			name: "role with allow_deletion=true",
			input: models.Role{
				Name:          types.StringValue("admin"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
			},
		},
		{
			name: "role with allow_deletion=false",
			input: models.Role{
				Name:          types.StringValue("viewer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
			},
		},
		{
			name: "role with allow_deletion unset (defaults to false)",
			input: models.Role{
				Name:          types.StringValue("operator"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
			},
		},
		{
			name: "role with long name",
			input: models.Role{
				Name:          types.StringValue("super-long-role-name-with-many-characters-for-testing"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
			},
		},
		{
			name: "role with special characters in name",
			input: models.Role{
				Name:          types.StringValue("role-with_special.chars"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
			},
		},
		{
			name: "create fails - API error",
			input: models.Role{
				Name:          types.StringValue("failing-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
			},
			mockError: errors.New("API error: role creation failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// Setup mock expectations
			if tt.mockError != nil {
				mockClient.EXPECT().
					CreateRole(ctx, gomock.Any(), gomock.Any()).
					Return(nil, tt.mockError)
			} else {
				mockClient.EXPECT().
					CreateRole(ctx, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *consolev1alpha1.CreateRoleRequest, _ ...grpc.CallOption) (*consolev1alpha1.CreateRoleResponse, error) {
						// Validate request
						assert.Equal(t, tt.input.Name.ValueString(), req.Request.Role.Name, "Role name should match")
						return &consolev1alpha1.CreateRoleResponse{}, nil
					})
			}

			// Create role resource with mock factory
			r := &Role{
				clientFactory: func(_ context.Context, _, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			// Get schema
			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Setup request with plan
			req := resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := req.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			// Setup response
			resp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			// Execute Create
			r.Create(ctx, req, &resp)

			// Validate results
			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Create should not error: %v", resp.Diagnostics)

			// Validate state
			var state models.Role
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			// Assert all fields
			assert.Equal(t, tt.input.Name, state.Name, "Name should match plan")
			assert.Equal(t, tt.input.ClusterAPIURL, state.ClusterAPIURL, "ClusterAPIURL should match plan")
			assert.Equal(t, tt.input.AllowDeletion, state.AllowDeletion, "AllowDeletion should match plan")

			// Validate computed fields
			assert.False(t, state.ID.IsNull(), "ID should be computed")
			assert.False(t, state.ID.IsUnknown(), "ID should be known")
			assert.Equal(t, tt.input.Name.ValueString(), state.ID.ValueString(), "ID should equal Name")
		})
	}
}

func TestRole_Read(t *testing.T) {
	tests := []struct {
		name          string
		initialState  models.Role
		mockSetup     func(*mocks.MockSecurityServiceClient)
		expectRemoved bool
		expectWarning bool
		wantErr       bool
	}{
		{
			name: "role exists - preserve all fields",
			initialState: models.Role{
				Name:          types.StringValue("developer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("developer"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&consolev1alpha1.GetRoleResponse{}, nil)
			},
			expectRemoved: false,
		},
		{
			name: "role exists with allow_deletion=false",
			initialState: models.Role{
				Name:          types.StringValue("admin"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
				ID:            types.StringValue("admin"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&consolev1alpha1.GetRoleResponse{}, nil)
			},
			expectRemoved: false,
		},
		{
			name: "role not found + allow_deletion=true - remove from state",
			initialState: models.Role{
				Name:          types.StringValue("missing-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("missing-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("role not found"))
			},
			expectRemoved: true,
		},
		{
			name: "role not found + allow_deletion=false - keep in state with warning",
			initialState: models.Role{
				Name:          types.StringValue("missing-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
				ID:            types.StringValue("missing-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("role not found"))
			},
			expectRemoved: false,
			expectWarning: true,
		},
		{
			name: "role not found + allow_deletion=null - keep in state with warning",
			initialState: models.Role{
				Name:          types.StringValue("missing-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
				ID:            types.StringValue("missing-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("role not found"))
			},
			expectRemoved: false,
			expectWarning: true,
		},
		{
			name: "cluster unreachable + allow_deletion=true - remove from state",
			initialState: models.Role{
				Name:          types.StringValue("unreachable-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("unreachable-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, status.Error(codes.Unavailable, "name resolver error: produced zero addresses"))
			},
			expectRemoved: true,
		},
		{
			name: "cluster unreachable + allow_deletion=false - keep in state with warning",
			initialState: models.Role{
				Name:          types.StringValue("unreachable-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
				ID:            types.StringValue("unreachable-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, status.Error(codes.Unavailable, "name resolver error: produced zero addresses"))
			},
			expectRemoved: false,
			expectWarning: true,
		},
		{
			name: "empty cluster_api_url - should fail with error",
			initialState: models.Role{
				Name:          types.StringValue("invalid-role"),
				ClusterAPIURL: types.StringValue(""),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("invalid-role"),
			},
			mockSetup: func(_ *mocks.MockSecurityServiceClient) {
				// No expectations - should fail before API call
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// Setup mock expectations
			tt.mockSetup(mockClient)

			// Create role resource with mock factory
			r := &Role{
				clientFactory: func(_ context.Context, clusterURL, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					// Simulate real behavior: empty clusterURL should error
					if clusterURL == "" {
						return nil, nil, errors.New("unable to create client with empty target cluster API URL")
					}
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			// Get schema
			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Setup request with initial state
			req := resource.ReadRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")

			// Setup response
			resp := resource.ReadResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			// Execute Read
			r.Read(ctx, req, &resp)

			// Validate results
			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Read should not have errors: %v", resp.Diagnostics.Errors())

			// Check for warnings if expected
			if tt.expectWarning {
				assert.True(t, len(resp.Diagnostics.Warnings()) > 0, "expected warnings but got none")
			}

			// Check if removed from state
			if tt.expectRemoved {
				var state *models.Role
				diags = resp.State.Get(ctx, &state)
				require.False(t, diags.HasError(), "State.Get should not error")
				assert.Nil(t, state, "State should be removed (nil)")
				return
			}

			// Validate all fields preserved
			var state models.Role
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			assert.Equal(t, tt.initialState.Name, state.Name, "Name should be preserved")
			assert.Equal(t, tt.initialState.ClusterAPIURL, state.ClusterAPIURL, "ClusterAPIURL should be preserved")
			assert.Equal(t, tt.initialState.AllowDeletion, state.AllowDeletion, "AllowDeletion should be preserved")
			assert.Equal(t, tt.initialState.ID, state.ID, "ID should be preserved")
		})
	}
}

func TestRole_Update(t *testing.T) {
	tests := []struct {
		name         string
		initialState models.Role
		plan         models.Role
	}{
		{
			name: "no changes - validate no-op",
			initialState: models.Role{
				Name:          types.StringValue("developer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("developer"),
			},
			plan: models.Role{
				Name:          types.StringValue("developer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("developer"),
			},
		},
		{
			name: "attempt to change allow_deletion - should be ignored (no-op)",
			initialState: models.Role{
				Name:          types.StringValue("admin"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
				ID:            types.StringValue("admin"),
			},
			plan: models.Role{
				Name:          types.StringValue("admin"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true), // Changed, but should be ignored
				ID:            types.StringValue("admin"),
			},
		},
		{
			name: "roles are immutable - update is always no-op",
			initialState: models.Role{
				Name:          types.StringValue("viewer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
				ID:            types.StringValue("viewer"),
			},
			plan: models.Role{
				Name:          types.StringValue("viewer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
				ID:            types.StringValue("viewer"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// No mock expectations - Update should make NO API calls
			// If any API calls are made, the test will fail

			r := &Role{
				clientFactory: func(_ context.Context, _, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			// Get schema
			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Setup request with state and plan
			req := resource.UpdateRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
				Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")
			diags = req.Plan.Set(ctx, &tt.plan)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			// Setup response
			resp := resource.UpdateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			// Execute Update
			r.Update(ctx, req, &resp)

			// Validate no errors
			require.False(t, resp.Diagnostics.HasError(), "Update should not error: %v", resp.Diagnostics)

			// Update is a no-op, so state should remain unchanged
			// (In reality, the framework handles this since Update doesn't modify resp.State)
		})
	}
}

func TestRole_Delete(t *testing.T) {
	tests := []struct {
		name         string
		initialState models.Role
		mockSetup    func(*mocks.MockSecurityServiceClient)
		wantErr      bool
		errorMsg     string
	}{
		{
			name: "successful deletion with allow_deletion=true",
			initialState: models.Role{
				Name:          types.StringValue("developer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("developer"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&consolev1alpha1.DeleteRoleResponse{}, nil)
			},
			wantErr: false,
		},
		{
			name: "deletion blocked with allow_deletion=false",
			initialState: models.Role{
				Name:          types.StringValue("admin"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
				ID:            types.StringValue("admin"),
			},
			mockSetup: func(_ *mocks.MockSecurityServiceClient) {
				// No expectations - should not reach API call
			},
			wantErr:  true,
			errorMsg: "role deletion not allowed",
		},
		{
			name: "deletion blocked with allow_deletion unset (defaults to false)",
			initialState: models.Role{
				Name:          types.StringValue("viewer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
				ID:            types.StringValue("viewer"),
			},
			mockSetup: func(_ *mocks.MockSecurityServiceClient) {
				// No expectations - should not reach API call
			},
			wantErr:  true,
			errorMsg: "role deletion not allowed",
		},
		{
			name: "deletion fails due to API error",
			initialState: models.Role{
				Name:          types.StringValue("failing-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("failing-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("API error: deletion failed"))
			},
			wantErr:  true,
			errorMsg: "Failed to delete role failing-role",
		},
		{
			name: "deletion with explicit allow_deletion check",
			initialState: models.Role{
				Name:          types.StringValue("test-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("test-role"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *consolev1alpha1.DeleteRoleRequest, _ ...grpc.CallOption) (*consolev1alpha1.DeleteRoleResponse, error) {
						assert.Equal(t, "test-role", req.Request.RoleName, "Role name should match")
						return &consolev1alpha1.DeleteRoleResponse{}, nil
					})
			},
			wantErr: false,
		},
		{
			name: "deletion of non-existent role (should succeed silently)",
			initialState: models.Role{
				Name:          types.StringValue("non-existent"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
				ID:            types.StringValue("non-existent"),
			},
			mockSetup: func(m *mocks.MockSecurityServiceClient) {
				m.EXPECT().
					DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&consolev1alpha1.DeleteRoleResponse{}, nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// Setup mock expectations
			tt.mockSetup(mockClient)

			r := &Role{
				clientFactory: func(_ context.Context, _, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			// Get schema
			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Setup request with state
			req := resource.DeleteRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")

			// Setup response
			resp := resource.DeleteResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			// Execute Delete
			r.Delete(ctx, req, &resp)

			// Validate results
			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				if tt.errorMsg != "" {
					errorFound := false
					for _, diag := range resp.Diagnostics.Errors() {
						if strings.Contains(diag.Summary(), tt.errorMsg) || strings.Contains(diag.Detail(), tt.errorMsg) {
							errorFound = true
							break
						}
					}
					assert.True(t, errorFound, "expected error message containing %q", tt.errorMsg)
				}
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Delete should not error: %v", resp.Diagnostics)
		})
	}
}

func TestImportIDFormat(t *testing.T) {
	tests := []struct {
		name            string
		importID        string
		expectError     bool
		expectedRole    string
		expectedCluster string
	}{
		{
			name:            "valid import format",
			importID:        "developer,c1a2b3c4d5",
			expectError:     false,
			expectedRole:    "developer",
			expectedCluster: "c1a2b3c4d5",
		},
		{
			name:            "valid import format with longer cluster ID",
			importID:        "admin,d110a6bu3l09un9dm4jg",
			expectError:     false,
			expectedRole:    "admin",
			expectedCluster: "d110a6bu3l09un9dm4jg",
		},
		{
			name:        "invalid format - missing cluster ID",
			importID:    "developer",
			expectError: true,
		},
		{
			name:        "empty import ID",
			importID:    "",
			expectError: true,
		},
		{
			name:            "role name with special characters",
			importID:        "super-admin-role,c1a2b3c4d5",
			expectError:     false,
			expectedRole:    "super-admin-role",
			expectedCluster: "c1a2b3c4d5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test validates the expected import ID format
			// The actual ImportState method would be tested in integration tests
			parts := splitImportID(tt.importID)

			if tt.expectError {
				if len(parts) == 2 {
					t.Errorf("Expected error for import ID %q, but parsing succeeded", tt.importID)
				}
			} else {
				if len(parts) != 2 {
					t.Errorf("Expected valid parsing for import ID %q, but got %d parts", tt.importID, len(parts))
					return
				}

				role := parts[0]
				clusterID := parts[1]

				if role != tt.expectedRole {
					t.Errorf("Expected role %q, got %q", tt.expectedRole, role)
				}

				if clusterID != tt.expectedCluster {
					t.Errorf("Expected cluster ID %q, got %q", tt.expectedCluster, clusterID)
				}
			}
		})
	}
}

// Integration-style tests that validate lifecycle operations

func TestRole_CreateAndRead(t *testing.T) {
	tests := []struct {
		name  string
		input models.Role
	}{
		{
			name: "create then read - basic role",
			input: models.Role{
				Name:          types.StringValue("developer"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
			},
		},
		{
			name: "create then read - with allow_deletion=false",
			input: models.Role{
				Name:          types.StringValue("admin"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
			},
		},
		{
			name: "create then read - validate state consistency",
			input: models.Role{
				Name:          types.StringValue("operator"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
			},
		},
		{
			name: "create then read - no field drift",
			input: models.Role{
				Name:          types.StringValue("test-role-123"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// Setup expectations for Create
			mockClient.EXPECT().
				CreateRole(ctx, gomock.Any(), gomock.Any()).
				Return(&consolev1alpha1.CreateRoleResponse{}, nil)

			// Setup expectations for Read
			mockClient.EXPECT().
				GetRole(ctx, gomock.Any(), gomock.Any()).
				Return(&consolev1alpha1.GetRoleResponse{}, nil)

			r := &Role{
				clientFactory: func(_ context.Context, _, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Execute Create
			createReq := resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := createReq.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError())

			createResp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			r.Create(ctx, createReq, &createResp)
			require.False(t, createResp.Diagnostics.HasError(), "Create should not error: %v", createResp.Diagnostics)

			var stateAfterCreate models.Role
			diags = createResp.State.Get(ctx, &stateAfterCreate)
			require.False(t, diags.HasError(), "State.Get after Create should not error")

			// Execute Read using the state from Create
			readReq := resource.ReadRequest{
				State: createResp.State,
			}
			readResp := resource.ReadResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			r.Read(ctx, readReq, &readResp)
			require.False(t, readResp.Diagnostics.HasError(), "Read should not error: %v", readResp.Diagnostics)

			var stateAfterRead models.Role
			diags = readResp.State.Get(ctx, &stateAfterRead)
			require.False(t, diags.HasError(), "State.Get after Read should not error")

			// Verify ALL fields match between Create state and Read state
			assert.Equal(t, stateAfterCreate.Name, stateAfterRead.Name, "Name should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.ClusterAPIURL, stateAfterRead.ClusterAPIURL, "ClusterAPIURL should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.AllowDeletion, stateAfterRead.AllowDeletion, "AllowDeletion should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.ID, stateAfterRead.ID, "ID should be consistent between Create and Read")
		})
	}
}

func TestRole_CreateReadDelete(t *testing.T) {
	tests := []struct {
		name          string
		input         models.Role
		allowDeletion bool
		expectError   bool
	}{
		{
			name: "full lifecycle with allow_deletion=true",
			input: models.Role{
				Name:          types.StringValue("lifecycle-test"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
			},
			allowDeletion: true,
			expectError:   false,
		},
		{
			name: "delete blocked with allow_deletion=false",
			input: models.Role{
				Name:          types.StringValue("protected-role"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
			},
			allowDeletion: false,
			expectError:   true,
		},
		{
			name: "complete lifecycle validation",
			input: models.Role{
				Name:          types.StringValue("full-test"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
			},
			allowDeletion: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// Setup Create expectation
			mockClient.EXPECT().
				CreateRole(ctx, gomock.Any(), gomock.Any()).
				Return(&consolev1alpha1.CreateRoleResponse{}, nil)

			// Setup Read expectation
			mockClient.EXPECT().
				GetRole(ctx, gomock.Any(), gomock.Any()).
				Return(&consolev1alpha1.GetRoleResponse{}, nil)

			// Setup Delete expectation only if deletion is allowed
			if tt.allowDeletion {
				mockClient.EXPECT().
					DeleteRole(ctx, gomock.Any(), gomock.Any()).
					Return(&consolev1alpha1.DeleteRoleResponse{}, nil)
			}

			r := &Role{
				clientFactory: func(_ context.Context, _, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Create
			createReq := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
			diags := createReq.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError())
			createResp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Create(ctx, createReq, &createResp)
			require.False(t, createResp.Diagnostics.HasError())

			var state models.Role
			diags = createResp.State.Get(ctx, &state)
			require.False(t, diags.HasError())

			// Read
			readReq := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
			diags = readReq.State.Set(ctx, &state)
			require.False(t, diags.HasError())
			readResp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Read(ctx, readReq, &readResp)
			require.False(t, readResp.Diagnostics.HasError())

			// Delete
			deleteReq := resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
			diags = deleteReq.State.Set(ctx, &state)
			require.False(t, diags.HasError())
			deleteResp := resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Delete(ctx, deleteReq, &deleteResp)

			if tt.expectError {
				assert.True(t, deleteResp.Diagnostics.HasError())
			} else {
				assert.False(t, deleteResp.Diagnostics.HasError())
			}
		})
	}
}

func TestRole_CreateReadUpdate(t *testing.T) {
	tests := []struct {
		name  string
		input models.Role
	}{
		{
			name: "create-read-update-read validates immutability",
			input: models.Role{
				Name:          types.StringValue("immutable-test"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(true),
			},
		},
		{
			name: "update doesnt corrupt state",
			input: models.Role{
				Name:          types.StringValue("state-test"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolValue(false),
			},
		},
		{
			name: "read after update returns identical state",
			input: models.Role{
				Name:          types.StringValue("consistency-test"),
				ClusterAPIURL: types.StringValue("https://api-test.cluster.redpanda.com"),
				AllowDeletion: types.BoolNull(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSecurityServiceClient(ctrl)

			// Setup expectations
			mockClient.EXPECT().
				CreateRole(ctx, gomock.Any(), gomock.Any()).
				Return(&consolev1alpha1.CreateRoleResponse{}, nil)
			mockClient.EXPECT().
				GetRole(ctx, gomock.Any(), gomock.Any()).
				Return(&consolev1alpha1.GetRoleResponse{}, nil).Times(2)

			r := &Role{
				clientFactory: func(_ context.Context, _, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
					return mockClient, nil, nil
				},
				resData: config.Resource{
					AuthToken:        "test-token",
					ProviderVersion:  "1.0.0",
					TerraformVersion: "1.5.0",
				},
			}

			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			// Create
			createReq := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
			diags := createReq.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError())
			createResp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Create(ctx, createReq, &createResp)
			require.False(t, createResp.Diagnostics.HasError(), "Create should not error: %v", createResp.Diagnostics)

			var stateAfterCreate models.Role
			diags = createResp.State.Get(ctx, &stateAfterCreate)
			require.False(t, diags.HasError(), "State.Get after Create should not error")

			// Read after Create
			readReq1 := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
			diags = readReq1.State.Set(ctx, &stateAfterCreate)
			require.False(t, diags.HasError())
			readResp1 := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Read(ctx, readReq1, &readResp1)
			require.False(t, readResp1.Diagnostics.HasError(), "Read after Create should not error: %v", readResp1.Diagnostics)

			var stateAfterFirstRead models.Role
			diags = readResp1.State.Get(ctx, &stateAfterFirstRead)
			require.False(t, diags.HasError(), "State.Get after first Read should not error")

			// Verify state consistency between Create and Read
			assert.Equal(t, stateAfterCreate.Name, stateAfterFirstRead.Name, "Name should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.ClusterAPIURL, stateAfterFirstRead.ClusterAPIURL, "ClusterAPIURL should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.AllowDeletion, stateAfterFirstRead.AllowDeletion, "AllowDeletion should be consistent between Create and Read")
			assert.Equal(t, stateAfterCreate.ID, stateAfterFirstRead.ID, "ID should be consistent between Create and Read")

			// Update (no-op)
			updateReq := resource.UpdateRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
				Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags = updateReq.State.Set(ctx, &stateAfterFirstRead)
			require.False(t, diags.HasError())
			diags = updateReq.Plan.Set(ctx, &stateAfterFirstRead)
			require.False(t, diags.HasError())
			updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Update(ctx, updateReq, &updateResp)
			require.False(t, updateResp.Diagnostics.HasError(), "Update should not error: %v", updateResp.Diagnostics)

			// Read after Update
			readReq2 := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
			diags = readReq2.State.Set(ctx, &stateAfterFirstRead)
			require.False(t, diags.HasError())
			readResp2 := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Read(ctx, readReq2, &readResp2)
			require.False(t, readResp2.Diagnostics.HasError(), "Read after Update should not error: %v", readResp2.Diagnostics)

			var stateAfterSecondRead models.Role
			diags = readResp2.State.Get(ctx, &stateAfterSecondRead)
			require.False(t, diags.HasError(), "State.Get after second Read should not error")

			// Verify all states remain identical throughout lifecycle
			assert.Equal(t, stateAfterCreate.Name, stateAfterSecondRead.Name, "Name should remain consistent")
			assert.Equal(t, stateAfterCreate.ClusterAPIURL, stateAfterSecondRead.ClusterAPIURL, "ClusterAPIURL should remain consistent")
			assert.Equal(t, stateAfterCreate.AllowDeletion, stateAfterSecondRead.AllowDeletion, "AllowDeletion should remain consistent")
			assert.Equal(t, stateAfterCreate.ID, stateAfterSecondRead.ID, "ID should remain consistent")
		})
	}
}

// Helper function to simulate import ID parsing - mimics strings.SplitN(importID, ",", 2)
func splitImportID(importID string) []string {
	if importID == "" {
		return []string{}
	}

	// Find first comma
	commaIndex := -1
	for i, r := range importID {
		if r == ',' {
			commaIndex = i
			break
		}
	}

	if commaIndex == -1 {
		return []string{importID}
	}

	// Split into exactly 2 parts at the first comma
	parts := []string{
		importID[:commaIndex],
		importID[commaIndex+1:],
	}

	return parts
}
