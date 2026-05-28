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

// Package role contains the implementation of the Role resource following the Terraform framework interfaces.
package role

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	rolemodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/role"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &Role{}
	_ resource.ResourceWithConfigure   = &Role{}
	_ resource.ResourceWithImportState = &Role{}
)

// SecurityServiceClientFactory is a function type for creating security service clients.
// This allows dependency injection for testing.
type SecurityServiceClientFactory func(ctx context.Context, clusterURL, authToken, providerVersion, terraformVersion string) (consolev1alpha1grpc.SecurityServiceClient, error)

// Role represents the Role Terraform resource.
type Role struct {
	base.ResourceBase

	SecurityClient consolev1alpha1grpc.SecurityServiceClient
	resData        config.Resource
	clientFactory  SecurityServiceClientFactory
}

// NewRole constructs a Role resource.
func NewRole() *Role {
	r := &Role{}
	r.ResourceBase = base.NewResourceBase(
		"redpanda_role",
		ResourceRoleSchema,
		func(p config.Resource) {
			r.resData = p
			if r.clientFactory == nil {
				r.clientFactory = func(_ context.Context, clusterURL, _, _, _ string) (consolev1alpha1grpc.SecurityServiceClient, error) {
					if r.resData.DataplaneConnPool == nil {
						return nil, errors.New("provider not configured: dataplane connection pool is nil")
					}
					consoleURL := utils.ConvertToConsoleURL(clusterURL)
					conn, err := r.resData.DataplaneConnPool.GetConnection(consoleURL)
					if err != nil {
						return nil, fmt.Errorf("unable to open a connection with the console API at %s: %v", consoleURL, err)
					}
					return consolev1alpha1grpc.NewSecurityServiceClient(conn), nil
				}
			}
		},
	)
	return r
}

// Create creates a Role resource.
func (r *Role) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model rolemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.createSecurityClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}

	innerReq, diags := rolemodel.ExpandCreate(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.SecurityClient.CreateRole(ctx, &consolev1alpha1.CreateRoleRequest{Request: innerReq}); err != nil {
		resp.Diagnostics.AddError("Failed to create role", utils.DeserializeGrpcError(err))
		return
	}

	model.ID = types.StringValue(model.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Read reads the state of the Role resource.
func (r *Role) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model rolemodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := model.Name.ValueString()
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if roleName == "" {
		resp.Diagnostics.AddError(
			"Invalid state data",
			fmt.Sprintf("Missing required field - name: '%s'", roleName),
		)
		return
	}

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "role", roleName, model.AllowDeletion, err, "create security client")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	exists, err := r.roleExists(ctx, roleName)
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "role", roleName, model.AllowDeletion, err, "verify role exists")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	if !exists {
		action, diags := utils.HandleGracefulRemoval(ctx, "role", roleName, model.AllowDeletion, utils.NotFoundError{Message: fmt.Sprintf("role %s not found", roleName)}, "find role")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Update is a no-op for the proto-derived attrs (roles are immutable;
// every mutable schema attr is RequiresReplace). The TF-only extras
// allow_deletion and delete_acls can flip without recreation, so the
// plan is written to state directly — without this, the framework
// raises "provider produced inconsistent result after apply".
func (*Role) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan rolemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the Role resource.
func (r *Role) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model rolemodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("role deletion not allowed", "allow_deletion is set to false")
		return
	}

	roleName := model.Name.ValueString()
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "role", roleName, model.AllowDeletion, err, "create security client")
		resp.Diagnostics.Append(diags...)
		return
	}

	innerReq, diags := rolemodel.ExpandDelete(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.SecurityClient.DeleteRole(ctx, &consolev1alpha1.DeleteRoleRequest{Request: innerReq}); err != nil {
		_, ddiags := utils.HandleGracefulRemoval(ctx, "role", roleName, model.AllowDeletion, err, "delete role")
		resp.Diagnostics.Append(ddiags...)
		return
	}
}

// ImportState imports the state of the Role resource.
func (r *Role) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: role_name,cluster_id
	split := strings.SplitN(req.ID, ",", 2)
	if len(split) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: role_name,cluster_id",
		)
		return
	}

	roleName := split[0]
	clusterID := split[1]

	dataplaneURL, err := r.CpCl.DataplaneURLForCluster(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to resolve dataplane URL for cluster %q; make sure import ID format is role_name,cluster_id", clusterID),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(roleName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(roleName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), dataplaneURL)...)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceRoleSchema(ctx), &resp.State, "allow_deletion")...)
}

// roleExists checks if a role exists
func (r *Role) roleExists(ctx context.Context, roleName string) (bool, error) {
	dataplaneReq := &dataplanev1.GetRoleRequest{
		RoleName: roleName,
	}
	consoleReq := &consolev1alpha1.GetRoleRequest{
		Request: dataplaneReq,
	}

	_, err := r.SecurityClient.GetRole(ctx, consoleReq)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "notfound") ||
			strings.Contains(errStr, "does not exist") ||
			strings.Contains(errStr, "unknown role") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// createSecurityClient creates a SecurityService client using the configured factory
func (r *Role) createSecurityClient(ctx context.Context, clusterURL string) error {
	if r.SecurityClient != nil {
		return nil
	}

	client, err := r.clientFactory(ctx, clusterURL, r.resData.AuthToken, r.resData.ProviderVersion, r.resData.TerraformVersion)
	if err != nil {
		return err
	}

	r.SecurityClient = client
	return nil
}
