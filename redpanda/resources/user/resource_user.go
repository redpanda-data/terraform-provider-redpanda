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
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
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
	resp.Schema = resourceUserSchema()
}

// ResourceUserSchema returns the schema for the User resource.
func resourceUserSchema() schema.Schema {
	return schema.Schema{
		Description: "User is a user that can be created in Redpanda",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Name of the user, must be unique",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				Description:   "Password of the user",
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mechanism": schema.StringAttribute{
				Description:   "Which authentication method to use, see https://docs.redpanda.com/current/manage/security/authentication/ for more information",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.OneOf("", "scram-sha-256", "scram-sha-512"),
				},
			},
			"cluster_api_url": schema.StringAttribute{
				Required: true,
				Description: "The cluster API URL. Changing this will prevent deletion of the resource on the existing " +
					"cluster. It is generally a better idea to delete an existing resource and create a new one than to " +
					"change this value unless you are planning to do state imports",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Allows deletion of the user. If false, the user cannot be deleted and the resource will be removed from the state on destruction. Defaults to false.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

// Create creates a User resource.
func (u *User) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.User
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

	allowDeletion := model.AllowDeletion
	if allowDeletion.IsNull() || allowDeletion.IsUnknown() {
		allowDeletion = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.User{
		Name:          types.StringValue(user.User.Name),
		Password:      model.Password,
		Mechanism:     model.Mechanism,
		ClusterAPIURL: model.ClusterAPIURL,
		ID:            types.StringValue(user.User.Name),
		AllowDeletion: allowDeletion,
	})...)
}

// Read reads the state of the User resource.
func (u *User) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.User
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	err := u.createUserClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		// Check if cluster is unreachable (DNS resolution failed)
		if utils.IsClusterUnreachable(err) {
			// Only remove from state if deletion is allowed
			if model.AllowDeletion.IsNull() || model.AllowDeletion.ValueBool() {
				tflog.Info(ctx, fmt.Sprintf("cluster unreachable for user %s, removing from state since allow_deletion is true", model.Name.ValueString()))
				resp.State.RemoveResource(ctx)
				return
			}
			// If deletion is not allowed, keep the resource in state but report the error
			tflog.Warn(ctx, fmt.Sprintf("cluster unreachable for user %s, keeping in state since allow_deletion is false", model.Name.ValueString()))
			resp.Diagnostics.AddWarning(
				"Cluster Unreachable",
				fmt.Sprintf("Unable to reach cluster for user %s. Resource will remain in state because allow_deletion is false. Error: %s", model.Name.ValueString(), utils.DeserializeGrpcError(err)),
			)
			return
		}
		resp.Diagnostics.AddError("failed to create user client", utils.DeserializeGrpcError(err))
		return
	}
	defer u.dataplaneConn.Close()
	user, err := utils.FindUserByName(ctx, model.Name.ValueString(), u.UserClient)
	if err != nil {
		// Check if this is a not found error or cluster unreachable error
		if utils.IsNotFound(err) || utils.IsClusterUnreachable(err) {
			// Only remove from state if deletion is allowed
			if model.AllowDeletion.IsNull() || model.AllowDeletion.ValueBool() {
				tflog.Info(ctx, fmt.Sprintf("user %s not found or cluster unreachable, removing from state since allow_deletion is true", model.Name.ValueString()))
				resp.State.RemoveResource(ctx)
				return
			}
			// If deletion is not allowed, keep the resource in state but report the error
			tflog.Warn(ctx, fmt.Sprintf("user %s not found or cluster unreachable, keeping in state since allow_deletion is false", model.Name.ValueString()))
			if utils.IsNotFound(err) {
				resp.Diagnostics.AddWarning(
					"User Not Found",
					fmt.Sprintf("User %s not found but will remain in state because allow_deletion is false", model.Name.ValueString()),
				)
			} else {
				resp.Diagnostics.AddWarning(
					"Cluster Unreachable",
					fmt.Sprintf("Unable to reach cluster for user %s. Resource will remain in state because allow_deletion is false. Error: %s", model.Name.ValueString(), utils.DeserializeGrpcError(err)),
				)
			}
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to find user %s", model.Name), utils.DeserializeGrpcError(err))
		return
	}
	mechanism := model.Mechanism
	if user.Mechanism != nil {
		mechanism = types.StringValue(utils.UserMechanismToString(user.Mechanism))
	}
	allowDeletion := model.AllowDeletion
	if allowDeletion.IsNull() || allowDeletion.IsUnknown() {
		allowDeletion = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.User{
		Name:          types.StringValue(user.Name),
		Password:      model.Password,
		Mechanism:     mechanism,
		ClusterAPIURL: model.ClusterAPIURL,
		ID:            types.StringValue(user.Name),
		AllowDeletion: allowDeletion,
	})...)
}

// Update updates the state of the User resource.
func (*User) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// TODO implement me
}

// Delete deletes the User resource.
func (u *User) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.User
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	err := u.createUserClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create user client", utils.DeserializeGrpcError(err))
		return
	}
	defer u.dataplaneConn.Close()
	_, err = u.UserClient.DeleteUser(ctx, &dataplanev1.DeleteUserRequest{
		Name: model.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete user %s", model.Name), utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState imports the state of the User resource.
func (u *User) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// We need multiple attributes here: Name and the cluster URL. But asking
	// for the URL is a bad UX, so we get the cluster ID and get the URL from
	// there.
	split := strings.SplitN(req.ID, ",", 2)
	if len(split) != 2 {
		resp.Diagnostics.AddError(fmt.Sprintf("wrong ADDR ID format: %v", req.ID), "ADDR ID format is <user_name>,<cluster_id>")
		return
	}
	user, clusterID := split[0], split[1]

	client := cloud.NewControlPlaneClientSet(u.resData.ControlPlaneConnection)
	cluster, err := client.ClusterForID(ctx, clusterID)
	var dataplaneURL string

	if err == nil && cluster != nil {
		dataplaneURL = cluster.DataplaneApi.Url
	} else {
		serverlessCluster, serr := client.ServerlessClusterForID(ctx, clusterID)
		if serr != nil || serverlessCluster == nil {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to find cluster with ID %q; make sure ADDR ID format is <user_name>,<cluster_id>", clusterID), utils.DeserializeGrpcError(err)+utils.DeserializeGrpcError(serr))
			return
		}
		dataplaneURL = serverlessCluster.DataplaneApi.Url
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), dataplaneURL)...)
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
