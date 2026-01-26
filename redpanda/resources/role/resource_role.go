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
	"fmt"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	rolemodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/role"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Role{}
	_ resource.ResourceWithConfigure   = &Role{}
	_ resource.ResourceWithImportState = &Role{}
)

// SecurityServiceClientFactory is a function type for creating security service clients.
// This allows dependency injection for testing.
type SecurityServiceClientFactory func(ctx context.Context, clusterURL, authToken, providerVersion, terraformVersion string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error)

// Role represents the Role Terraform resource.
type Role struct {
	SecurityClient consolev1alpha1grpc.SecurityServiceClient
	resData        config.Resource
	dataplaneConn  *grpc.ClientConn
	clientFactory  SecurityServiceClientFactory
}

// Metadata returns the metadata for the Role resource.
func (*Role) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

// Configure configures the Role resource.
func (r *Role) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	resData, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.Resource, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.resData = resData

	if r.clientFactory == nil {
		r.clientFactory = func(_ context.Context, clusterURL, authToken, providerVersion, terraformVersion string) (consolev1alpha1grpc.SecurityServiceClient, *grpc.ClientConn, error) {
			consoleURL := utils.ConvertToConsoleURL(clusterURL)
			conn, err := cloud.SpawnConn(consoleURL, authToken, providerVersion, terraformVersion)
			if err != nil {
				return nil, nil, fmt.Errorf("unable to open a connection with the console API at %s: %v", consoleURL, err)
			}
			client := consolev1alpha1grpc.NewSecurityServiceClient(conn)
			return client, conn, nil
		}
	}
}

// Schema returns the schema for the Role resource.
func (*Role) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceRoleSchema()
}

// Create creates a Role resource.
func (r *Role) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model rolemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := model.Name.ValueString()
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}
	defer func() {
		if r.dataplaneConn != nil {
			_ = r.dataplaneConn.Close()
		}
	}()

	dataplaneReq := &dataplanev1.CreateRoleRequest{
		Role: &dataplanev1.Role{
			Name: roleName,
		},
	}
	consoleReq := &consolev1alpha1.CreateRoleRequest{
		Request: dataplaneReq,
	}

	_, err := r.SecurityClient.CreateRole(ctx, consoleReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create role", utils.DeserializeGrpcError(err))
		return
	}

	model.ID = types.StringValue(roleName)
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
	defer func() {
		if r.dataplaneConn != nil {
			_ = r.dataplaneConn.Close()
		}
	}()

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

// Update updates the state of the Role resource (not supported - requires replace).
func (*Role) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// Roles are immutable - updates require replacement
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
	defer func() {
		if r.dataplaneConn != nil {
			_ = r.dataplaneConn.Close()
		}
	}()

	dataplaneReq := &dataplanev1.DeleteRoleRequest{
		RoleName: roleName,
	}
	consoleReq := &consolev1alpha1.DeleteRoleRequest{
		Request: dataplaneReq,
	}

	_, err := r.SecurityClient.DeleteRole(ctx, consoleReq)
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "role", roleName, model.AllowDeletion, err, "delete role")
		resp.Diagnostics.Append(diags...)
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

	client := cloud.NewControlPlaneClientSet(r.resData.ControlPlaneConnection)
	cluster, err := client.ClusterForID(ctx, clusterID)
	var dataplaneURL string

	if err == nil && cluster != nil {
		dataplaneURL = cluster.DataplaneApi.Url
	} else {
		serverlessCluster, serr := client.ServerlessClusterForID(ctx, clusterID)
		if serr != nil || serverlessCluster == nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("failed to find cluster with ID %q; make sure import ID format is role_name,cluster_id", clusterID),
				utils.DeserializeGrpcError(err)+utils.DeserializeGrpcError(serr),
			)
			return
		}
		dataplaneURL = serverlessCluster.DataplaneApi.Url
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(roleName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(roleName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), dataplaneURL)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)
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
		return nil // Client already exists
	}

	client, conn, err := r.clientFactory(ctx, clusterURL, r.resData.AuthToken, r.resData.ProviderVersion, r.resData.TerraformVersion)
	if err != nil {
		return err
	}

	r.SecurityClient = client
	r.dataplaneConn = conn
	return nil
}
