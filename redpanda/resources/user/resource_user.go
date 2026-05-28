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
	"os"
	"strings"
	"time"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	usermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/user"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Per-RPC retry budget for dataplane calls (e.g., the freshly-provisioned-cluster
// DNS-propagation window). Sized to fit ~5 attempts under Retry's 1s→60s cap.
const dataplaneRetryTimeout = 2 * time.Minute

var (
	_ resource.Resource                = &User{}
	_ resource.ResourceWithConfigure   = &User{}
	_ resource.ResourceWithImportState = &User{}
)

// User represents the User Terraform resource.
type User struct {
	base.ResourceBase

	UserClient dataplanev1grpc.UserServiceClient

	resData config.Resource
}

// NewUser constructs a User resource.
func NewUser() *User {
	u := &User{}
	u.ResourceBase = base.NewResourceBase(
		"redpanda_user",
		ResourceUserSchema,
		func(p config.Resource) { u.resData = p },
	)
	return u
}

// Create creates a User resource.
func (u *User) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model usermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	var cfg usermodel.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	model.PasswordWO = cfg.PasswordWO

	if err := u.createUserClient(model.ClusterAPIURL.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to create user client", utils.DeserializeGrpcError(err))
		return
	}

	pbReq, diags := usermodel.ExpandCreate(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var createdUser usermodel.UserResponse
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		created, rpcErr := u.UserClient.CreateUser(ctx, pbReq)
		if rpcErr == nil {
			createdUser = created.GetUser()
			return nil
		}
		// Adopt the existing user on AlreadyExists from a prior retry's lost response.
		if utils.IsAlreadyExists(rpcErr) {
			if existing, findErr := utils.FindUserByName(ctx, model.Name.ValueString(), u.UserClient); findErr == nil {
				createdUser = existing
				return nil
			}
			return utils.NonRetryableError(rpcErr)
		}
		// Probe before retrying so the next attempt doesn't trip AlreadyExists.
		if utils.IsUnavailable(rpcErr) {
			if existing, findErr := utils.FindUserByName(ctx, model.Name.ValueString(), u.UserClient); findErr == nil {
				createdUser = existing
				return nil
			}
			return utils.RetryableError(rpcErr)
		}
		return utils.NonRetryableError(rpcErr)
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create user", utils.DeserializeGrpcError(err))
		return
	}

	persist, fdiags := usermodel.Flatten(ctx, createdUser, &model)
	resp.Diagnostics.Append(fdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads the state of the User resource.
func (u *User) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model usermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	userName := model.Name.ValueString()

	if err := u.createUserClient(model.ClusterAPIURL.ValueString()); err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "create user client")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	var user *dataplanev1.ListUsersResponse_User
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		var rpcErr error
		user, rpcErr = utils.FindUserByName(ctx, userName, u.UserClient)
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "find user")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	persist, diags := usermodel.Flatten(ctx, user, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update updates the state of the User resource.
func (u *User) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state usermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	var cfg usermodel.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.PasswordWO = cfg.PasswordWO

	passwordChanged := !plan.Password.Equal(state.Password)
	passwordWOVersionChanged := !plan.PasswordWOVersion.Equal(state.PasswordWOVersion)
	mechanismChanged := !plan.Mechanism.Equal(state.Mechanism)

	if !passwordChanged && !passwordWOVersionChanged && !mechanismChanged {
		state.AllowDeletion = plan.AllowDeletion
		state.PasswordWOVersion = plan.PasswordWOVersion
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}

	if err := u.createUserClient(plan.ClusterAPIURL.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to create user client", utils.DeserializeGrpcError(err))
		return
	}

	pbReq, diags := usermodel.ExpandUpdate(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updateResp *dataplanev1.UpdateUserResponse
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		var rpcErr error
		updateResp, rpcErr = u.UserClient.UpdateUser(ctx, pbReq)
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update user", utils.DeserializeGrpcError(err))
		return
	}

	persist, fdiags := usermodel.Flatten(ctx, updateResp.User, &plan)
	resp.Diagnostics.Append(fdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Delete deletes the User resource.
func (u *User) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model usermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	userName := model.Name.ValueString()

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("user deletion not allowed", "allow_deletion is set to false")
		return
	}

	if err := u.createUserClient(model.ClusterAPIURL.ValueString()); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "create user client")
		resp.Diagnostics.Append(diags...)
		return
	}

	pbReq, diags := usermodel.ExpandDelete(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		_, rpcErr := u.UserClient.DeleteUser(ctx, pbReq)
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		_, ddiags := utils.HandleGracefulRemoval(ctx, "user", userName, model.AllowDeletion, err, "delete user")
		resp.Diagnostics.Append(ddiags...)
		return
	}
}

// ImportState imports the state of the User resource.
// Format: <user_name>,<cluster_id>[,<password>[,<mechanism>]]
// Password can also be set via REDPANDA_IMPORT_PASSWORD env var.
func (u *User) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	split := strings.Split(req.ID, ",")
	if len(split) < 2 || len(split) > 4 {
		resp.Diagnostics.AddError(
			fmt.Sprintf("wrong import ID format: %v", req.ID),
			"Import ID format is <user_name>,<cluster_id>[,<password>[,<mechanism>]]. Password can also be set via REDPANDA_IMPORT_PASSWORD env var.",
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

	if envPassword := os.Getenv("REDPANDA_IMPORT_PASSWORD"); envPassword != "" {
		password = envPassword
	}

	dataplaneURL, err := u.CpCl.DataplaneURLForCluster(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to resolve dataplane URL for cluster %q; make sure import ID format is <user_name>,<cluster_id>[,<password>[,<mechanism>]]", clusterID),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(user))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), dataplaneURL)...)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceUserSchema(ctx), &resp.State, "allow_deletion")...)
	if password != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("password"), types.StringValue(password))...)
	}
	if mechanism != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mechanism"), types.StringValue(mechanism))...)
	}
}

func (u *User) createUserClient(clusterURL string) error {
	if u.UserClient != nil {
		return nil
	}
	if clusterURL == "" {
		return errors.New("unable to create client with empty target cluster API URL")
	}
	if u.resData.DataplaneConnPool == nil {
		return errors.New("provider not configured: dataplane connection pool is nil")
	}
	conn, err := u.resData.DataplaneConnPool.GetConnection(clusterURL)
	if err != nil {
		return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
	}
	u.UserClient = dataplanev1grpc.NewUserServiceClient(conn)
	return nil
}
