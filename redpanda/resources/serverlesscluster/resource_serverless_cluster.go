// Copyright 2023 Redpanda Data, Inc.
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

// Package serverlesscluster contains the implementation of the ServerlessCluster resource
// following the Terraform framework interfaces.
package serverlesscluster

import (
	"context"
	"fmt"
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ServerlessCluster{}
	_ resource.ResourceWithConfigure   = &ServerlessCluster{}
	_ resource.ResourceWithImportState = &ServerlessCluster{}
)

// ServerlessCluster represents a cluster managed resource.
type ServerlessCluster struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the full name of the ServerlessCluster resource.
func (*ServerlessCluster) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_serverless_cluster"
}

// Configure uses provider level data to configure ServerlessCluster's clients.
func (c *ServerlessCluster) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(config.Resource)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	c.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the ServerlessCluster resource.
func (*ServerlessCluster) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceServerlessClusterSchema()
}

func resourceServerlessClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the serverless cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"serverless_region": schema.StringAttribute{
				Optional:      true,
				Description:   "Redpanda specific region of the serverless cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the Resource Group in which to create the serverless cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the serverless cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:      true,
				Description:   "The URL of the dataplane API for the serverless cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// Create creates a new ServerlessCluster resource. It updates the state if the resource
// is successfully created.
func (c *ServerlessCluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.ServerlessCluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	clusterReq, err := GenerateServerlessClusterRequest(model)
	if err != nil {
		resp.Diagnostics.AddError("unable to parse CreateServerlessCluster request", err.Error())
		return
	}
	clResp, err := c.CpCl.ServerlessCluster.CreateServerlessCluster(ctx, &controlplanev1beta2.CreateServerlessClusterRequest{ServerlessCluster: clusterReq})
	if err != nil {
		resp.Diagnostics.AddError("failed to create serverless cluster", err.Error())
		return
	}
	if _, err := utils.GetServerlessClusterUntilRunningState(ctx, 0, 80, clusterReq.Name, c.CpCl); err != nil {
		resp.Diagnostics.AddError("failed at getting ready state while creating serverless cluster", err.Error())
		return
	}
	operation, err := c.CpCl.Operation.GetOperation(ctx, &controlplanev1beta2.GetOperationRequest{Id: clResp.Operation.Id})
	if err != nil {
		resp.Diagnostics.AddError("failed to retrieve create serverless cluster operation", err.Error())
		return
	}
	op := operation.Operation
	var metadata controlplanev1beta2.CreateServerlessClusterMetadata
	if err := op.Metadata.UnmarshalTo(&metadata); err != nil {
		resp.Diagnostics.AddError("failed to unmarshal serverless cluster metadata", err.Error())
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, time.Minute, 3*time.Second, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("operation error while creating serverless cluster", err.Error())
		return
	}
	cluster, err := c.CpCl.ServerlessClusterForID(ctx, metadata.GetServerlessClusterId())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("successfully created the serverless cluster with ID %q, but failed to read the serverless cluster configuration: %v", model.ID.ValueString(), err), err.Error())
		return
	}
	clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
	if err != nil {
		resp.Diagnostics.AddError("unable to parse Cluster API URL", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.ServerlessCluster{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
		ClusterAPIURL:    types.StringValue(clusterURL),
	})...)
}

// Read reads ServerlessCluster resource's values and updates the state.
func (c *ServerlessCluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.ServerlessCluster
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	cluster, err := c.CpCl.ServerlessClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			// Treat HTTP 404 Not Found status as a signal to recreate resource and return early
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read serverless cluster %s", model.ID), err.Error())
		return
	}
	clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
	if err != nil {
		resp.Diagnostics.AddError("unable to parse Cluster API URL", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.ServerlessCluster{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
		ClusterAPIURL:    types.StringValue(clusterURL),
	})...)
}

// Update all serverless cluster updates are currently delete and recreate.
func (*ServerlessCluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.ServerlessCluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// We pass through the plan to state. Currently, every serverless cluster change needs
	// a resource replacement.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the ServerlessCluster resource.
func (c *ServerlessCluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.ServerlessCluster
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	// We need to wait for the serverless cluster to be in a running state before we can delete it
	_, err := utils.GetServerlessClusterUntilRunningState(ctx, 0, 30, model.Name.ValueString(), c.CpCl)
	if err != nil {
		return
	}

	clResp, err := c.CpCl.ServerlessCluster.DeleteServerlessCluster(ctx, &controlplanev1beta2.DeleteServerlessClusterRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to delete serverless cluster", err.Error())
		return
	}

	if err := utils.AreWeDoneYet(ctx, clResp.Operation, time.Minute, 3*time.Second, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed to delete serverless cluster", err.Error())
		return
	}
}

// ImportState imports and update the state of the serverless cluster resource.
func (*ServerlessCluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// GenerateServerlessClusterRequest was pulled out to enable unit testing
func GenerateServerlessClusterRequest(model models.ServerlessCluster) (*controlplanev1beta2.ServerlessClusterCreate, error) {
	return &controlplanev1beta2.ServerlessClusterCreate{
		Name:             model.Name.ValueString(),
		ServerlessRegion: model.ServerlessRegion.ValueString(),
		ResourceGroupId:  model.ResourceGroupID.ValueString(),
	}, nil
}
