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

package schemaregistryacl

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	schemaregistryaclmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schemaregistryacl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSchemaRegistryACL_Create(t *testing.T) {
	tests := []struct {
		name         string
		input        schemaregistryaclmodel.ResourceModel
		mockResponse []kclients.SchemaRegistryACLResponse
		wantErr      bool
	}{
		{
			name: "basic subject ACL with READ permission",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:alice"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test-subject"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:alice",
					ResourceType: "SUBJECT",
					Resource:     "test-subject",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "READ",
					Permission:   "ALLOW",
				},
			},
		},
		{
			name: "registry-level ACL with ALL operation",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:admin"),
				ResourceType: types.StringValue("REGISTRY"),
				ResourceName: types.StringValue("*"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("ALL"),
				Permission:   types.StringValue("ALLOW"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:admin",
					ResourceType: "REGISTRY",
					Resource:     "*",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "ALL",
					Permission:   "ALLOW",
				},
			},
		},
		{
			name: "prefixed pattern ACL",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:bob"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("prod-"),
				PatternType:  types.StringValue("PREFIXED"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("ALLOW"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:bob",
					ResourceType: "SUBJECT",
					Resource:     "prod-",
					PatternType:  "PREFIXED",
					Host:         "*",
					Operation:    "WRITE",
					Permission:   "ALLOW",
				},
			},
		},
		{
			name: "ACL with username and password",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:carol"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("private-subject"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("testuser"),
				Password:     types.StringValue("testpass"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:carol",
					ResourceType: "SUBJECT",
					Resource:     "private-subject",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "READ",
					Permission:   "ALLOW",
				},
			},
		},
		{
			name: "ACL with allow_deletion set to true",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:dave"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("temp-subject"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("DELETE"),
				Permission:    types.StringValue("ALLOW"),
				AllowDeletion: types.BoolValue(true),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:dave",
					ResourceType: "SUBJECT",
					Resource:     "temp-subject",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "DELETE",
					Permission:   "ALLOW",
				},
			},
		},
		{
			name: "DENY permission ACL",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:eve"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("restricted"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("DENY"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:eve",
					ResourceType: "SUBJECT",
					Resource:     "restricted",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "WRITE",
					Permission:   "DENY",
				},
			},
		},
		{
			name: "DESCRIBE operation ACL",
			input: schemaregistryaclmodel.ResourceModel{
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:frank"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("metrics"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("192.168.1.1"),
				Operation:    types.StringValue("DESCRIBE"),
				Permission:   types.StringValue("ALLOW"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:frank",
					ResourceType: "SUBJECT",
					Resource:     "metrics",
					PatternType:  "LITERAL",
					Host:         "192.168.1.1",
					Operation:    "DESCRIBE",
					Permission:   "ALLOW",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)

			mockClient.EXPECT().
				CreateACL(ctx, gomock.Any()).
				Return(nil)

			mockClient.EXPECT().
				ListACLs(ctx, gomock.Any()).
				Return(tt.mockResponse, nil)

			sr := &SchemaRegistryACL{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (kclients.SchemaRegistryACLClientInterface, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			sr.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: schemaResp.Schema},
			}
			diags := req.Plan.Set(ctx, &tt.input)
			require.False(t, diags.HasError(), "Plan.Set should not error")

			resp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			sr.Create(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Create should not error: %v", resp.Diagnostics)

			var state schemaregistryaclmodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")
			assert.Equal(t, tt.input.ClusterID, state.ClusterID, "ClusterID should match plan")
			assert.Equal(t, tt.input.Principal, state.Principal, "Principal should match plan")
			assert.Equal(t, tt.input.ResourceType, state.ResourceType, "ResourceType should match plan")
			assert.Equal(t, tt.input.ResourceName, state.ResourceName, "ResourceName should match plan")
			assert.Equal(t, tt.input.PatternType, state.PatternType, "PatternType should match plan")
			assert.Equal(t, tt.input.Host, state.Host, "Host should match plan")
			assert.Equal(t, tt.input.Operation, state.Operation, "Operation should match plan")
			assert.Equal(t, tt.input.Permission, state.Permission, "Permission should match plan")
			assert.Equal(t, tt.input.Username, state.Username, "Username should match plan")
			assert.Equal(t, tt.input.Password, state.Password, "Password should match plan")
			assert.Equal(t, tt.input.AllowDeletion, state.AllowDeletion, "AllowDeletion should match plan")
			assert.False(t, state.ID.IsNull(), "ID should be computed")
			assert.False(t, state.ID.IsUnknown(), "ID should be known")
		})
	}
}

func TestSchemaRegistryACL_Read(t *testing.T) {
	tests := []struct {
		name          string
		initialState  schemaregistryaclmodel.ResourceModel
		mockResponse  []kclients.SchemaRegistryACLResponse
		mockError     error
		expectRemoved bool
		wantErr       bool
	}{
		{
			name: "ACL exists and matches",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:alice:SUBJECT:test-subject:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:alice"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test-subject"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("testuser"),
				Password:     types.StringValue("testpass"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:alice",
					ResourceType: "SUBJECT",
					Resource:     "test-subject",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "READ",
					Permission:   "ALLOW",
				},
			},
			expectRemoved: false,
		},
		{
			name: "ACL with all optional fields set",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:bob:REGISTRY:*:LITERAL:*:ALL:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:bob"),
				ResourceType:  types.StringValue("REGISTRY"),
				ResourceName:  types.StringValue("*"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("ALL"),
				Permission:    types.StringValue("ALLOW"),
				Username:      types.StringValue("admin"),
				Password:      types.StringValue("secret"),
				AllowDeletion: types.BoolValue(true),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:bob",
					ResourceType: "REGISTRY",
					Resource:     "*",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "ALL",
					Permission:   "ALLOW",
				},
			},
			expectRemoved: false,
		},
		{
			name: "ACL not found - should remove from state",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:carol:SUBJECT:missing:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:carol"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("missing"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
			},
			mockResponse:  []kclients.SchemaRegistryACLResponse{},
			expectRemoved: true,
		},
		{
			name: "ACL not found with allow_deletion=false - should keep in state and error",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:carol:SUBJECT:missing:LITERAL:*:READ:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:carol"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("missing"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("READ"),
				Permission:    types.StringValue("ALLOW"),
				AllowDeletion: types.BoolValue(false),
			},
			mockResponse:  []kclients.SchemaRegistryACLResponse{},
			expectRemoved: false,
			wantErr:       true,
		},
		{
			name: "multiple ACLs returned - finds correct one",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:dave:SUBJECT:target:LITERAL:*:WRITE:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:dave"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("target"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("ALLOW"),
			},
			mockResponse: []kclients.SchemaRegistryACLResponse{
				{
					Principal:    "User:other",
					ResourceType: "SUBJECT",
					Resource:     "other-subject",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "READ",
					Permission:   "ALLOW",
				},
				{
					Principal:    "User:dave",
					ResourceType: "SUBJECT",
					Resource:     "target",
					PatternType:  "LITERAL",
					Host:         "*",
					Operation:    "WRITE",
					Permission:   "ALLOW",
				},
			},
			expectRemoved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)

			if tt.mockError != nil {
				mockClient.EXPECT().
					ListACLs(ctx, gomock.Any()).
					Return(nil, tt.mockError)
			} else {
				mockClient.EXPECT().
					ListACLs(ctx, gomock.Any()).
					Return(tt.mockResponse, nil)
			}

			sr := &SchemaRegistryACL{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (kclients.SchemaRegistryACLClientInterface, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			sr.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.ReadRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")

			resp := resource.ReadResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			sr.Read(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Read should not error: %v", resp.Diagnostics)

			if tt.expectRemoved {
				var state *schemaregistryaclmodel.ResourceModel
				diags = resp.State.Get(ctx, &state)
				require.False(t, diags.HasError(), "State.Get should not error")
				assert.Nil(t, state, "State should be removed")
				return
			}

			var state schemaregistryaclmodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			assert.Equal(t, tt.initialState.ID, state.ID, "ID should be preserved")
			assert.Equal(t, tt.initialState.ClusterID, state.ClusterID, "ClusterID should be preserved")
			assert.Equal(t, tt.initialState.Principal, state.Principal, "Principal should be preserved")
			assert.Equal(t, tt.initialState.ResourceType, state.ResourceType, "ResourceType should be preserved")
			assert.Equal(t, tt.initialState.ResourceName, state.ResourceName, "ResourceName should be preserved")
			assert.Equal(t, tt.initialState.PatternType, state.PatternType, "PatternType should be preserved")
			assert.Equal(t, tt.initialState.Host, state.Host, "Host should be preserved")
			assert.Equal(t, tt.initialState.Operation, state.Operation, "Operation should be preserved")
			assert.Equal(t, tt.initialState.Permission, state.Permission, "Permission should be preserved")
			assert.Equal(t, tt.initialState.Username, state.Username, "Username should be preserved")
			assert.Equal(t, tt.initialState.Password, state.Password, "Password should be preserved")
			assert.Equal(t, tt.initialState.AllowDeletion, state.AllowDeletion, "AllowDeletion should be preserved")
		})
	}
}

func TestSchemaRegistryACL_Update(t *testing.T) {
	tests := []struct {
		name         string
		initialState schemaregistryaclmodel.ResourceModel
		plan         schemaregistryaclmodel.ResourceModel
		wantErr      bool
	}{
		{
			name: "update username only",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:alice:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:alice"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("olduser"),
				Password:     types.StringValue("password"),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:alice:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:alice"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("newuser"),
				Password:     types.StringValue("password"),
			},
		},
		{
			name: "update password only",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:bob:SUBJECT:test:LITERAL:*:WRITE:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:bob"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("user"),
				Password:     types.StringValue("oldpass"),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:bob:SUBJECT:test:LITERAL:*:WRITE:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:bob"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("user"),
				Password:     types.StringValue("newpass"),
			},
		},
		{
			name: "update allow_deletion only",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:carol:SUBJECT:test:LITERAL:*:DELETE:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:carol"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("DELETE"),
				Permission:    types.StringValue("ALLOW"),
				AllowDeletion: types.BoolValue(false),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:carol:SUBJECT:test:LITERAL:*:DELETE:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:carol"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("DELETE"),
				Permission:    types.StringValue("ALLOW"),
				AllowDeletion: types.BoolValue(true),
			},
		},
		{
			name: "update username and password together",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:dave:REGISTRY:*:LITERAL:*:ALL:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:dave"),
				ResourceType: types.StringValue("REGISTRY"),
				ResourceName: types.StringValue("*"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("ALL"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("olduser"),
				Password:     types.StringValue("oldpass"),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:dave:REGISTRY:*:LITERAL:*:ALL:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:dave"),
				ResourceType: types.StringValue("REGISTRY"),
				ResourceName: types.StringValue("*"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("ALL"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("newuser"),
				Password:     types.StringValue("newpass"),
			},
		},
		{
			name: "update all updatable fields",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:eve:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:eve"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("READ"),
				Permission:    types.StringValue("ALLOW"),
				Username:      types.StringValue("user1"),
				Password:      types.StringValue("pass1"),
				AllowDeletion: types.BoolValue(false),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:eve:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:eve"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("READ"),
				Permission:    types.StringValue("ALLOW"),
				Username:      types.StringValue("user2"),
				Password:      types.StringValue("pass2"),
				AllowDeletion: types.BoolValue(true),
			},
		},
		{
			name: "set username from null to value",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:frank:SUBJECT:test:LITERAL:*:WRITE:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:frank"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringNull(),
				Password:     types.StringValue("pass"),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:frank:SUBJECT:test:LITERAL:*:WRITE:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:frank"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("WRITE"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("newuser"),
				Password:     types.StringValue("pass"),
			},
		},
		{
			name: "set password from value to null",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:grace:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:grace"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("user"),
				Password:     types.StringValue("oldpass"),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:grace:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:grace"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("user"),
				Password:     types.StringNull(),
			},
		},
		{
			name: "no changes - update with same values",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:henry:SUBJECT:test:LITERAL:*:ALL:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:henry"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("ALL"),
				Permission:    types.StringValue("ALLOW"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				AllowDeletion: types.BoolValue(true),
			},
			plan: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:henry:SUBJECT:test:LITERAL:*:ALL:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:henry"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("ALL"),
				Permission:    types.StringValue("ALLOW"),
				Username:      types.StringValue("user"),
				Password:      types.StringValue("pass"),
				AllowDeletion: types.BoolValue(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			sr := &SchemaRegistryACL{}

			schemaResp := resource.SchemaResponse{}
			sr.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

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

			sr.Update(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Update should not error: %v", resp.Diagnostics)

			var state schemaregistryaclmodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")

			assert.Equal(t, tt.plan.Username, state.Username, "Username should match plan")
			assert.Equal(t, tt.plan.Password, state.Password, "Password should match plan")
			assert.Equal(t, tt.plan.AllowDeletion, state.AllowDeletion, "AllowDeletion should match plan")

			assert.Equal(t, tt.initialState.ID, state.ID, "ID should not change")
			assert.Equal(t, tt.initialState.ClusterID, state.ClusterID, "ClusterID should not change")
			assert.Equal(t, tt.initialState.Principal, state.Principal, "Principal should not change")
			assert.Equal(t, tt.initialState.ResourceType, state.ResourceType, "ResourceType should not change")
			assert.Equal(t, tt.initialState.ResourceName, state.ResourceName, "ResourceName should not change")
			assert.Equal(t, tt.initialState.PatternType, state.PatternType, "PatternType should not change")
			assert.Equal(t, tt.initialState.Host, state.Host, "Host should not change")
			assert.Equal(t, tt.initialState.Operation, state.Operation, "Operation should not change")
			assert.Equal(t, tt.initialState.Permission, state.Permission, "Permission should not change")
		})
	}
}

func TestSchemaRegistryACL_Delete(t *testing.T) {
	tests := []struct {
		name         string
		initialState schemaregistryaclmodel.ResourceModel
		mockError    error
		wantErr      bool
	}{
		{
			name: "successful deletion",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:alice:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:alice"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
			},
		},
		{
			name: "deletion with allow_deletion=true",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:bob:SUBJECT:test:LITERAL:*:WRITE:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:bob"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("WRITE"),
				Permission:    types.StringValue("ALLOW"),
				AllowDeletion: types.BoolValue(true),
			},
		},
		{
			name: "deletion with allow_deletion=false",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:            types.StringValue("cluster-1:User:carol:SUBJECT:test:LITERAL:*:DELETE:ALLOW"),
				ClusterID:     types.StringValue("cluster-1"),
				Principal:     types.StringValue("User:carol"),
				ResourceType:  types.StringValue("SUBJECT"),
				ResourceName:  types.StringValue("test"),
				PatternType:   types.StringValue("LITERAL"),
				Host:          types.StringValue("*"),
				Operation:     types.StringValue("DELETE"),
				Permission:    types.StringValue("ALLOW"),
				AllowDeletion: types.BoolValue(false),
			},
		},
		{
			name: "deletion with allow_deletion unset (null)",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:dave:REGISTRY:*:LITERAL:*:ALL:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:dave"),
				ResourceType: types.StringValue("REGISTRY"),
				ResourceName: types.StringValue("*"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("ALL"),
				Permission:   types.StringValue("ALLOW"),
			},
		},
		{
			name: "deletion with username and password",
			initialState: schemaregistryaclmodel.ResourceModel{
				ID:           types.StringValue("cluster-1:User:eve:SUBJECT:test:LITERAL:*:READ:ALLOW"),
				ClusterID:    types.StringValue("cluster-1"),
				Principal:    types.StringValue("User:eve"),
				ResourceType: types.StringValue("SUBJECT"),
				ResourceName: types.StringValue("test"),
				PatternType:  types.StringValue("LITERAL"),
				Host:         types.StringValue("*"),
				Operation:    types.StringValue("READ"),
				Permission:   types.StringValue("ALLOW"),
				Username:     types.StringValue("testuser"),
				Password:     types.StringValue("testpass"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)

			if tt.mockError != nil {
				mockClient.EXPECT().
					DeleteACL(ctx, gomock.Any()).
					Return(tt.mockError)
			} else {
				mockClient.EXPECT().
					DeleteACL(ctx, gomock.Any()).
					Return(nil)
			}

			sr := &SchemaRegistryACL{
				clientFactory: func(_ context.Context, _ *cloud.ControlPlaneClientSet, _, _, _ string) (kclients.SchemaRegistryACLClientInterface, error) {
					return mockClient, nil
				},
			}

			schemaResp := resource.SchemaResponse{}
			sr.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			req := resource.DeleteRequest{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}
			diags := req.State.Set(ctx, &tt.initialState)
			require.False(t, diags.HasError(), "State.Set should not error")

			resp := resource.DeleteResponse{
				State: tfsdk.State{Schema: schemaResp.Schema},
			}

			sr.Delete(ctx, req, &resp)

			if tt.wantErr {
				require.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "Delete should not error: %v", resp.Diagnostics)

			var state *schemaregistryaclmodel.ResourceModel
			diags = resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "State.Get should not error")
			assert.Nil(t, state, "State should be removed after delete")
		})
	}
}

func Test_parseImportID(t *testing.T) {
	tests := []struct {
		name      string
		importID  string
		want      *importIDComponents
		wantErr   bool
		errString string
	}{
		{
			name:     "valid import - basic subject ACL",
			importID: "cluster-1:User:alice:SUBJECT:test-subject:LITERAL:*:READ:ALLOW:myuser:mypass",
			want: &importIDComponents{
				clusterID:    "cluster-1",
				principal:    "User:alice",
				resourceType: "SUBJECT",
				resourceName: "test-subject",
				patternType:  "LITERAL",
				host:         "*",
				operation:    "READ",
				permission:   "ALLOW",
				username:     "myuser",
				password:     "mypass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - registry ACL",
			importID: "cluster-2:User:admin:REGISTRY:*:LITERAL:*:ALL:ALLOW:admin:secret",
			want: &importIDComponents{
				clusterID:    "cluster-2",
				principal:    "User:admin",
				resourceType: "REGISTRY",
				resourceName: "*",
				patternType:  "LITERAL",
				host:         "*",
				operation:    "ALL",
				permission:   "ALLOW",
				username:     "admin",
				password:     "secret",
			},
			wantErr: false,
		},
		{
			name:     "valid import - prefixed pattern",
			importID: "cluster-1:User:bob:SUBJECT:prod-:PREFIXED:192.168.1.1:WRITE:ALLOW:bob:pass123",
			want: &importIDComponents{
				clusterID:    "cluster-1",
				principal:    "User:bob",
				resourceType: "SUBJECT",
				resourceName: "prod-",
				patternType:  "PREFIXED",
				host:         "192.168.1.1",
				operation:    "WRITE",
				permission:   "ALLOW",
				username:     "bob",
				password:     "pass123",
			},
			wantErr: false,
		},
		{
			name:     "valid import - DENY permission",
			importID: "cluster-1:User:eve:SUBJECT:restricted:LITERAL:*:DELETE:DENY:eve:secret",
			want: &importIDComponents{
				clusterID:    "cluster-1",
				principal:    "User:eve",
				resourceType: "SUBJECT",
				resourceName: "restricted",
				patternType:  "LITERAL",
				host:         "*",
				operation:    "DELETE",
				permission:   "DENY",
				username:     "eve",
				password:     "secret",
			},
			wantErr: false,
		},
		{
			name:     "valid import - principal with multiple colons",
			importID: "cluster-1:RedpandaRole:admin:extra:REGISTRY:*:LITERAL:*:ALL:ALLOW:rpuser:rppass",
			want: &importIDComponents{
				clusterID:    "cluster-1",
				principal:    "RedpandaRole:admin:extra",
				resourceType: "REGISTRY",
				resourceName: "*",
				patternType:  "LITERAL",
				host:         "*",
				operation:    "ALL",
				permission:   "ALLOW",
				username:     "rpuser",
				password:     "rppass",
			},
			wantErr: false,
		},
		{
			name:      "invalid - missing parts (only 9 parts)",
			importID:  "cluster-1:User:alice:SUBJECT:test:LITERAL:*:READ:ALLOW",
			want:      nil,
			wantErr:   true,
			errString: "got 9 parts (expected at least 10)",
		},
		{
			name:      "invalid - missing parts (only 8 parts)",
			importID:  "cluster-1:User:alice:SUBJECT:test:LITERAL:*:READ",
			want:      nil,
			wantErr:   true,
			errString: "got 8 parts (expected at least 10)",
		},
		{
			name:      "invalid - empty import ID",
			importID:  "",
			want:      nil,
			wantErr:   true,
			errString: "got 1 parts (expected at least 10)",
		},
		{
			name:     "valid - only colons (10 parts)",
			importID: ":::::::::",
			want: &importIDComponents{
				clusterID:    "",
				principal:    "",
				resourceType: "",
				resourceName: "",
				patternType:  "",
				host:         "",
				operation:    "",
				permission:   "",
				username:     "",
				password:     "",
			},
			wantErr: false,
		},
		{
			name:     "valid - exact 10 parts",
			importID: "c:p:t:n:pt:h:o:perm:u:pass",
			want: &importIDComponents{
				clusterID:    "c",
				principal:    "p",
				resourceType: "t",
				resourceName: "n",
				patternType:  "pt",
				host:         "h",
				operation:    "o",
				permission:   "perm",
				username:     "u",
				password:     "pass",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseImportID(tt.importID)

			if tt.wantErr {
				require.Error(t, err, "parseImportID should return error")
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString, "error message should contain expected string")
				}
				assert.Nil(t, got, "result should be nil on error")
				return
			}

			require.NoError(t, err, "parseImportID should not return error")
			require.NotNil(t, got, "result should not be nil")

			assert.Equal(t, tt.want.clusterID, got.clusterID, "clusterID should match")
			assert.Equal(t, tt.want.principal, got.principal, "principal should match")
			assert.Equal(t, tt.want.resourceType, got.resourceType, "resourceType should match")
			assert.Equal(t, tt.want.resourceName, got.resourceName, "resourceName should match")
			assert.Equal(t, tt.want.patternType, got.patternType, "patternType should match")
			assert.Equal(t, tt.want.host, got.host, "host should match")
			assert.Equal(t, tt.want.operation, got.operation, "operation should match")
			assert.Equal(t, tt.want.permission, got.permission, "permission should match")
		})
	}
}
