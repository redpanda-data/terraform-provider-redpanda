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

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &User{}
	_ resource.ResourceWithConfigure   = &User{}
	_ resource.ResourceWithImportState = &User{}
)

// User represents the User Terraform resource.
type User struct {
	UserClient dataplanev1alpha1.UserServiceClient

	resData utils.ResourceData
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
	p, ok := req.ProviderData.(utils.ResourceData)
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
				Computed: true,
			},
		},
	}
}

// Create creates a User resource.
func (u *User) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.User
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	err := u.createUserClient(ctx, model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create user client", err.Error())
		return
	}
	user, err := u.UserClient.CreateUser(ctx, &dataplanev1alpha1.CreateUserRequest{
		User: &dataplanev1alpha1.CreateUserRequest_User{
			Name:      model.Name.ValueString(),
			Password:  model.Password.ValueString(), // This seems wrong and bad. See issue #12.
			Mechanism: utils.StringToUserMechanism(model.Mechanism.ValueString()),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create user", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.User{
		Name:          types.StringValue(user.User.Name),
		Password:      model.Password,
		Mechanism:     model.Mechanism,
		ClusterAPIURL: model.ClusterAPIURL,
		ID:            types.StringValue(user.User.Name),
	})...)
}

// Read reads the state of the User resource.
func (u *User) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.User
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	err := u.createUserClient(ctx, model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create user client", err.Error())
		return
	}
	user, err := utils.FindUserByName(ctx, model.Name.ValueString(), u.UserClient)
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to find user %s", model.Name), err.Error())
		return
	}
	mechanism := model.Mechanism
	if user.Mechanism != nil {
		mechanism = types.StringValue(utils.UserMechanismToString(user.Mechanism))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.User{
		Name:          types.StringValue(user.Name),
		Password:      model.Password,
		Mechanism:     mechanism,
		ClusterAPIURL: model.ClusterAPIURL,
		ID:            types.StringValue(user.Name),
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

	err := u.createUserClient(ctx, model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create user client", err.Error())
		return
	}
	_, err = u.UserClient.DeleteUser(ctx, &dataplanev1alpha1.DeleteUserRequest{
		Name: model.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete user %s", model.Name), err.Error())
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
	client, err := clients.NewClusterServiceClient(ctx, u.resData.CloudEnv, clients.ClientRequest{
		ClientID:     u.resData.ClientID,
		ClientSecret: u.resData.ClientSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("unable to start a cluster client", "unable to start a cluster client; make sure ADDR ID format is <user_name>:<cluster_id>")
		return
	}
	cluster, err := client.GetCluster(ctx, &cloudv1beta1.GetClusterRequest{
		Id: clusterID,
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to find cluster with ID %q", clusterID), err.Error())
		return
	}
	clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
	if err != nil {
		resp.Diagnostics.AddError("unable to parse Cluster API URL", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), clusterURL)...)
}

func (u *User) createUserClient(ctx context.Context, clusterURL string) error {
	if u.UserClient != nil { // Client already started, no need to create another one.
		return nil
	}
	if clusterURL == "" {
		return errors.New("unable to create client with empty target cluster API URL")
	}
	client, err := clients.NewUserServiceClient(ctx, u.resData.CloudEnv, clusterURL, clients.ClientRequest{
		ClientID:     u.resData.ClientID,
		ClientSecret: u.resData.ClientSecret,
	})
	if err != nil {
		return fmt.Errorf("unable to create user client: %v", err)
	}
	u.UserClient = client
	return nil
}
