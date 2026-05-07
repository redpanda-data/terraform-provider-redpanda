// Copyright 2025 Redpanda Data, Inc.
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

package secret

import (
	"context"
	"errors"
	"testing"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	secretmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setConfig(ctx context.Context, s schema.Schema, val any) (tfsdk.Config, diag.Diagnostics) {
	tmp := tfsdk.State{Schema: s}
	diags := tmp.Set(ctx, val)
	return tfsdk.Config{Schema: s, Raw: tmp.Raw}, diags
}

func scopesList(ctx context.Context, t *testing.T, vals ...string) types.List {
	t.Helper()
	l, diags := types.ListValueFrom(ctx, types.StringType, vals)
	require.False(t, diags.HasError(), "scopesList: %v", diags)
	return l
}

func TestSecret_Create_WriteOnlySecretData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	mockClient := mocks.NewMockSecretServiceClient(ctrl)
	mockClient.EXPECT().
		CreateSecret(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *dataplanev1.CreateSecretRequest, _ ...any) (*dataplanev1.CreateSecretResponse, error) {
			assert.Equal(t, "MY_SECRET", req.GetId())
			assert.Equal(t, []byte("super-sensitive"), req.GetSecretData(), "secret_data must come from Config, not Plan")
			assert.Equal(t, []dataplanev1.Scope{dataplanev1.Scope_SCOPE_REDPANDA_CLUSTER}, req.GetScopes())
			return &dataplanev1.CreateSecretResponse{Secret: &dataplanev1.Secret{
				Id:     req.GetId(),
				Scopes: req.GetScopes(),
			}}, nil
		})

	s := &Secret{SecretClient: mockClient}
	schemaResp := resource.SchemaResponse{}
	s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	plan := secretmodel.ResourceModel{
		Name:              types.StringValue("MY_SECRET"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Scopes:            scopesList(ctx, t, "SCOPE_REDPANDA_CLUSTER"),
		SecretData:        types.StringNull(), // write-only attrs are null in plan
		SecretDataVersion: types.Int64Value(1),
		Labels:            types.MapNull(types.StringType),
	}
	cfg := plan
	cfg.SecretData = types.StringValue("super-sensitive")

	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	diags := req.Plan.Set(ctx, &plan)
	require.False(t, diags.HasError(), "Plan.Set: %v", diags)
	var cfgDiags diag.Diagnostics
	req.Config, cfgDiags = setConfig(ctx, schemaResp.Schema, &cfg)
	require.False(t, cfgDiags.HasError(), "Config set: %v", cfgDiags)

	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	s.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), "Create: %v", resp.Diagnostics)

	var state secretmodel.ResourceModel
	diags = resp.State.Get(ctx, &state)
	require.False(t, diags.HasError(), "State.Get: %v", diags)
	assert.Equal(t, "MY_SECRET", state.Name.ValueString())
	assert.Equal(t, "MY_SECRET", state.ID.ValueString())
	assert.Equal(t, int64(1), state.SecretDataVersion.ValueInt64())
}

func TestSecret_Create_APIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	mockClient := mocks.NewMockSecretServiceClient(ctrl)
	mockClient.EXPECT().
		CreateSecret(ctx, gomock.Any()).
		Return(nil, errors.New("boom"))

	s := &Secret{SecretClient: mockClient}
	schemaResp := resource.SchemaResponse{}
	s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	plan := secretmodel.ResourceModel{
		Name:          types.StringValue("MY_SECRET"),
		ClusterAPIURL: types.StringValue("http://localhost:9644"),
		Scopes:        scopesList(ctx, t, "SCOPE_REDPANDA_CLUSTER"),
		SecretData:    types.StringNull(),
		Labels:        types.MapNull(types.StringType),
	}
	cfg := plan
	cfg.SecretData = types.StringValue("v")

	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.False(t, req.Plan.Set(ctx, &plan).HasError())
	req.Config, _ = setConfig(ctx, schemaResp.Schema, &cfg)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	s.Create(ctx, req, &resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), "failed to create secret")
}

func TestSecret_Update_VersionTriggersWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	mockClient := mocks.NewMockSecretServiceClient(ctrl)
	mockClient.EXPECT().
		UpdateSecret(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *dataplanev1.UpdateSecretRequest, _ ...any) (*dataplanev1.UpdateSecretResponse, error) {
			assert.Equal(t, "MY_SECRET", req.GetId())
			assert.Equal(t, []byte("rotated"), req.GetSecretData(), "secret_data should be sent from Config when version changes")
			assert.Empty(t, req.GetLabels(), "labels are immutable (RequiresReplace) — Update must not send them")
			return &dataplanev1.UpdateSecretResponse{Secret: &dataplanev1.Secret{
				Id:     req.GetId(),
				Scopes: req.GetScopes(),
			}}, nil
		})

	s := &Secret{SecretClient: mockClient}
	schemaResp := resource.SchemaResponse{}
	s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	plan := secretmodel.ResourceModel{
		Name:              types.StringValue("MY_SECRET"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Scopes:            scopesList(ctx, t, "SCOPE_REDPANDA_CLUSTER"),
		SecretData:        types.StringNull(),
		SecretDataVersion: types.Int64Value(2),
		Labels:            types.MapNull(types.StringType),
	}
	state := plan
	state.SecretDataVersion = types.Int64Value(1)
	state.ID = types.StringValue("MY_SECRET")
	cfg := plan
	cfg.SecretData = types.StringValue("rotated")

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	require.False(t, req.Plan.Set(ctx, &plan).HasError())
	require.False(t, req.State.Set(ctx, &state).HasError())
	req.Config, _ = setConfig(ctx, schemaResp.Schema, &cfg)
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	s.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), "Update: %v", resp.Diagnostics)

	var got secretmodel.ResourceModel
	require.False(t, resp.State.Get(ctx, &got).HasError())
	assert.Equal(t, int64(2), got.SecretDataVersion.ValueInt64())
}

func TestSecret_Update_NoChange_Skips_API(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	// Asserting no calls - mockClient has no EXPECT.
	mockClient := mocks.NewMockSecretServiceClient(ctrl)
	s := &Secret{SecretClient: mockClient}
	schemaResp := resource.SchemaResponse{}
	s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	plan := secretmodel.ResourceModel{
		Name:              types.StringValue("MY_SECRET"),
		ID:                types.StringValue("MY_SECRET"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Scopes:            scopesList(ctx, t, "SCOPE_REDPANDA_CLUSTER"),
		SecretData:        types.StringNull(),
		SecretDataVersion: types.Int64Value(1),
		Labels:            types.MapNull(types.StringType),
		AllowDeletion:     types.BoolValue(true),
	}
	state := plan
	cfg := plan

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	require.False(t, req.Plan.Set(ctx, &plan).HasError())
	require.False(t, req.State.Set(ctx, &state).HasError())
	req.Config, _ = setConfig(ctx, schemaResp.Schema, &cfg)
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	s.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError())
}

func TestSecret_Read_RoundTripsScopesAndLabels(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	mockClient := mocks.NewMockSecretServiceClient(ctrl)
	mockClient.EXPECT().
		GetSecret(ctx, gomock.Any()).
		Return(&dataplanev1.GetSecretResponse{Secret: &dataplanev1.Secret{
			Id:     "MY_SECRET",
			Scopes: []dataplanev1.Scope{dataplanev1.Scope_SCOPE_REDPANDA_CLUSTER, dataplanev1.Scope_SCOPE_MCP_SERVER},
			Labels: map[string]string{"team": "data"},
		}}, nil)

	s := &Secret{SecretClient: mockClient}
	schemaResp := resource.SchemaResponse{}
	s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := secretmodel.ResourceModel{
		Name:              types.StringValue("MY_SECRET"),
		ID:                types.StringValue("MY_SECRET"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Scopes:            scopesList(ctx, t, "SCOPE_REDPANDA_CLUSTER", "SCOPE_MCP_SERVER"),
		SecretDataVersion: types.Int64Value(1),
		Labels:            types.MapNull(types.StringType),
	}

	req := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.False(t, req.State.Set(ctx, &state).HasError())
	resp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	s.Read(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), "Read: %v", resp.Diagnostics)

	var got secretmodel.ResourceModel
	require.False(t, resp.State.Get(ctx, &got).HasError())
	assert.Equal(t, "MY_SECRET", got.ID.ValueString())
	assert.Equal(t, 2, len(got.Scopes.Elements()))
	assert.False(t, got.Labels.IsNull())
}

func TestSecret_Delete_RespectsAllowDeletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	mockClient := mocks.NewMockSecretServiceClient(ctrl)
	// Expect no Delete call when allow_deletion=false.
	s := &Secret{SecretClient: mockClient}
	schemaResp := resource.SchemaResponse{}
	s.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := secretmodel.ResourceModel{
		Name:          types.StringValue("MY_SECRET"),
		ClusterAPIURL: types.StringValue("http://localhost:9644"),
		Scopes:        scopesList(ctx, t, "SCOPE_REDPANDA_CLUSTER"),
		Labels:        types.MapNull(types.StringType),
		AllowDeletion: types.BoolValue(false),
	}

	req := resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.False(t, req.State.Set(ctx, &state).HasError())
	resp := resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	s.Delete(ctx, req, &resp)
	require.True(t, resp.Diagnostics.HasError(), "Delete should refuse when allow_deletion=false")
}
