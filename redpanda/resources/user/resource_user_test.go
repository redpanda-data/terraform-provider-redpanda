package user

import (
	"context"
	"testing"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	usermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setConfig populates a tfsdk.Config using a temporary State (since Config has no Set method).
func setConfig(ctx context.Context, s schema.Schema, val any) (tfsdk.Config, diag.Diagnostics) {
	tmp := tfsdk.State{Schema: s}
	diags := tmp.Set(ctx, val)
	return tfsdk.Config{Schema: s, Raw: tmp.Raw}, diags
}

func TestUser_Create_WriteOnlyPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockUserServiceClient(ctrl)

	mockClient.EXPECT().
		CreateUser(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *dataplanev1.CreateUserRequest, _ ...any) (*dataplanev1.CreateUserResponse, error) {
			assert.Equal(t, "secret", req.User.Password, "password_wo value should be read from Config, not Plan")
			return &dataplanev1.CreateUserResponse{
				User: &dataplanev1.CreateUserResponse_User{
					Name: req.User.Name,
				},
			}, nil
		})

	conn, err := grpc.NewClient("passthrough:///localhost:9644", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	u := &User{
		UserClient:    mockClient,
		dataplaneConn: conn,
	}

	schemaResp := resource.SchemaResponse{}
	u.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	// Plan has password_wo as null (simulates real Terraform behavior for write-only attrs)
	plan := usermodel.ResourceModel{
		Name:              types.StringValue("testuser"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Mechanism:         types.StringValue("scram-sha-256"),
		PasswordWO:        types.StringNull(),
		PasswordWOVersion: types.Int64Value(1),
	}

	// Config has the actual password_wo value
	cfg := usermodel.ResourceModel{
		Name:              types.StringValue("testuser"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Mechanism:         types.StringValue("scram-sha-256"),
		PasswordWO:        types.StringValue("secret"),
		PasswordWOVersion: types.Int64Value(1),
	}

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := req.Plan.Set(ctx, &plan)
	require.False(t, diags.HasError(), "Plan.Set should not error")
	req.Config, diags = setConfig(ctx, schemaResp.Schema, &cfg)
	require.False(t, diags.HasError(), "Config set should not error")

	resp := resource.CreateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	u.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), "Create should not error: %v", resp.Diagnostics)

	var state usermodel.ResourceModel
	diags = resp.State.Get(ctx, &state)
	require.False(t, diags.HasError(), "State.Get should not error")
	assert.Equal(t, "testuser", state.Name.ValueString())
}

func TestUser_Update_WriteOnlyPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockUserServiceClient(ctrl)

	mockClient.EXPECT().
		UpdateUser(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *dataplanev1.UpdateUserRequest, _ ...any) (*dataplanev1.UpdateUserResponse, error) {
			assert.Equal(t, "new-secret", req.User.Password, "password_wo value should be read from Config, not Plan")
			return &dataplanev1.UpdateUserResponse{
				User: &dataplanev1.UpdateUserResponse_User{
					Name: req.User.Name,
				},
			}, nil
		})

	conn, err := grpc.NewClient("passthrough:///localhost:9644", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	u := &User{
		UserClient:    mockClient,
		dataplaneConn: conn,
	}

	schemaResp := resource.SchemaResponse{}
	u.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	// Plan has password_wo as null (simulates real Terraform behavior)
	planModel := usermodel.ResourceModel{
		Name:              types.StringValue("testuser"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Mechanism:         types.StringValue("scram-sha-256"),
		PasswordWO:        types.StringNull(),
		PasswordWOVersion: types.Int64Value(2),
	}

	// State has the previous version
	stateModel := usermodel.ResourceModel{
		Name:              types.StringValue("testuser"),
		ID:                types.StringValue("testuser"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Mechanism:         types.StringValue("scram-sha-256"),
		PasswordWOVersion: types.Int64Value(1),
	}

	// Config has the actual password_wo value
	cfgModel := usermodel.ResourceModel{
		Name:              types.StringValue("testuser"),
		ClusterAPIURL:     types.StringValue("http://localhost:9644"),
		Mechanism:         types.StringValue("scram-sha-256"),
		PasswordWO:        types.StringValue("new-secret"),
		PasswordWOVersion: types.Int64Value(2),
	}

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	diags := req.Plan.Set(ctx, &planModel)
	require.False(t, diags.HasError(), "Plan.Set should not error")
	diags = req.State.Set(ctx, &stateModel)
	require.False(t, diags.HasError(), "State.Set should not error")
	req.Config, diags = setConfig(ctx, schemaResp.Schema, &cfgModel)
	require.False(t, diags.HasError(), "Config set should not error")

	resp := resource.UpdateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	u.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), "Update should not error: %v", resp.Diagnostics)

	var resultState usermodel.ResourceModel
	diags = resp.State.Get(ctx, &resultState)
	require.False(t, diags.HasError(), "State.Get should not error")
	assert.Equal(t, "testuser", resultState.Name.ValueString())
}
