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

// Package user contains the implementation of the User resource following the Terraform framework interfaces.
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	usermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/user"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &User{}
	_ resource.ResourceWithConfigure   = &User{}
	_ resource.ResourceWithImportState = &User{}
)

// User represents the User Terraform resource.
type User struct {
	UserClient dataplanev1grpc.UserServiceClient

	resData       config.Resource
	dataplaneConn *grpc.ClientConn
}

// Metadata returns the metadata for the User resource.
func (*User) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_user"
}

// Configure configures the User resource.
func (u *User) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	p, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	u.resData = p
}

// Schema returns the schema for the User resource.
func (*User) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceUserSchema()
}

// Create creates a User resource.
func (u *User) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model usermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	err := u.createUserClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create user client", utils.DeserializeGrpcError(err))
		return
	}
	defer u.dataplaneConn.Close()
	user, err := u.UserClient.CreateUser(ctx, &dataplanev1.CreateUserRequest{
		User: &dataplanev1.CreateUserRequest_User{
			Name:      model.Name.ValueString(),
			Password:  model.Password.ValueString(), // This seems wrong and bad. See issue #12.
			Mechanism: utils.StringToUserMechanism(model.Mechanism.ValueString()),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create user", utils.DeserializeGrpcError(err))
		return
	}

	persist := model.GetUpdatedModel(user.User, usermodel.ContingentFields{
		AllowDeletion: model.AllowDeletion,
	})
	// Preserve password and cluster URL from plan (not returned by API)
	persist.Password = model.Password
	persist.ClusterAPIURL = model.ClusterAPIURL

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads the state of the User resource.
func (u *User) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model usermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	userName := model.Name.ValueString()

	err := u.createUserClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "create user client")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	defer u.dataplaneConn.Close()
	user, err := utils.FindUserByName(ctx, userName, u.UserClient)
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "find user")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	persist := &usermodel.ResourceModel{}
	persist.GetUpdatedModel(user, usermodel.ContingentFields{})

	// Preserve fields not returned by API from existing state
	persist.Password = model.Password
	persist.ClusterAPIURL = model.ClusterAPIURL
	persist.AllowDeletion = model.AllowDeletion

	// Preserve mechanism if not returned by API
	if persist.Mechanism.IsNull() || persist.Mechanism.IsUnknown() {
		persist.Mechanism = model.Mechanism
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update updates the state of the User resource.
func (u *User) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan usermodel.ResourceModel
	var state usermodel.ResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	passwordChanged := !plan.Password.Equal(state.Password)
	mechanismChanged := !plan.Mechanism.Equal(state.Mechanism)

	if passwordChanged || mechanismChanged {
		err := u.createUserClient(plan.ClusterAPIURL.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("failed to create user client", utils.DeserializeGrpcError(err))
			return
		}
		defer u.dataplaneConn.Close()

		updateResp, err := u.UserClient.UpdateUser(ctx, &dataplanev1.UpdateUserRequest{
			User: &dataplanev1.UpdateUserRequest_User{
				Name:      plan.Name.ValueString(),
				Password:  plan.Password.ValueString(),
				Mechanism: utils.StringToUserMechanism(plan.Mechanism.ValueString()),
			},
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to update user", utils.DeserializeGrpcError(err))
			return
		}

		persist := plan.GetUpdatedModel(updateResp.User, usermodel.ContingentFields{
			AllowDeletion: plan.AllowDeletion,
		})
		persist.Password = plan.Password
		persist.ClusterAPIURL = plan.ClusterAPIURL

		resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
		return
	}

	// Only allow_deletion changed - just update state directly
	state.AllowDeletion = plan.AllowDeletion
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete deletes the User resource.
func (u *User) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model usermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	userName := model.Name.ValueString()

	// Block deletion only if allow_deletion is explicitly set to false
	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("user deletion not allowed", "allow_deletion is set to false")
		return
	}

	err := u.createUserClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "create user client")
		resp.Diagnostics.Append(diags...)
		return
	}
	defer u.dataplaneConn.Close()
	_, err = u.UserClient.DeleteUser(ctx, &dataplanev1.DeleteUserRequest{
		Name: userName,
	})
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "delete user")
		resp.Diagnostics.Append(diags...)
		return
	}
}

// ImportState imports the state of the User resource.
func (u *User) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// We need multiple attributes here: Name and the cluster URL. But asking
	// for the URL is a bad UX, so we get the cluster ID and get the URL from
	// there.
	// Import ID format supports:
	// - <user_name>,<cluster_id>
	// - <user_name>,<cluster_id>,<password>
	// - <user_name>,<cluster_id>,<password>,<mechanism>
	split := strings.Split(req.ID, ",")
	if len(split) < 2 || len(split) > 4 {
		resp.Diagnostics.AddError(
			fmt.Sprintf("wrong import ID format: %v", req.ID),
			"Import ID format is <user_name>,<cluster_id>[,<password>[,<mechanism>]]",
		)
		return
	}

	user := split[0]
	clusterID := split[1]
	var password, mechanism string
	if len(split) >= 3 {
		password = split[2]
	}
	if len(split) == 4 {
		mechanism = split[3]
	}

	client := cloud.NewControlPlaneClientSet(u.resData.ControlPlaneConnection)
	cluster, err := client.ClusterForID(ctx, clusterID)
	var dataplaneURL string

	if err == nil && cluster != nil {
		dataplaneURL = cluster.DataplaneApi.Url
	} else {
		serverlessCluster, serr := client.ServerlessClusterForID(ctx, clusterID)
		if serr != nil || serverlessCluster == nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("failed to find cluster with ID %q; make sure import ID format is <user_name>,<cluster_id>[,<password>[,<mechanism>]]", clusterID),
				utils.DeserializeGrpcError(err)+utils.DeserializeGrpcError(serr),
			)
			return
		}
		dataplaneURL = serverlessCluster.DataplaneApi.Url
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), dataplaneURL)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)

	// Set optional fields if provided during import
	if password != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("password"), types.StringValue(password))...)
	}
	if mechanism != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mechanism"), types.StringValue(mechanism))...)
	}
}

func (u *User) createUserClient(clusterURL string) error {
	if u.UserClient != nil { // Client already started, no need to create another one.
		return nil
	}
	if clusterURL == "" {
		return errors.New("unable to create client with empty target cluster API URL")
	}
	if u.dataplaneConn == nil {
		conn, err := cloud.SpawnConn(clusterURL, u.resData.AuthToken, u.resData.ProviderVersion, u.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
		}
		u.dataplaneConn = conn
	}
	u.UserClient = dataplanev1grpc.NewUserServiceClient(u.dataplaneConn)
	return nil
}
