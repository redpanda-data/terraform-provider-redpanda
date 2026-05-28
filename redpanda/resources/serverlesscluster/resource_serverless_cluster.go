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

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	serverlessclustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlesscluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &ServerlessCluster{}
	_ resource.ResourceWithConfigure   = &ServerlessCluster{}
	_ resource.ResourceWithImportState = &ServerlessCluster{}
)

// ServerlessCluster represents a cluster managed resource.
type ServerlessCluster struct {
	base.ResourceBase
}

// NewServerlessCluster constructs a ServerlessCluster resource.
func NewServerlessCluster() *ServerlessCluster {
	c := &ServerlessCluster{}
	c.ResourceBase = base.NewResourceBase("redpanda_serverless_cluster", ResourceServerlessClusterSchema, nil)
	return c
}

// Create creates a new ServerlessCluster resource.
func (c *ServerlessCluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverlessclustermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, expandDiags := serverlessclustermodel.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clResp, err := c.CpCl.ServerlessCluster.CreateServerlessCluster(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create serverless cluster", utils.DeserializeGrpcError(err))
		return
	}
	op := clResp.Operation
	// Write initial state so a failed create is still trackable / deletable.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), op.GetResourceId())...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, createTimeout, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("operation error while creating serverless cluster", utils.DeserializeGrpcError(err))
		return
	}
	cluster, err := c.CpCl.ServerlessClusterForID(ctx, op.GetResourceId())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("successfully created the serverless cluster with ID %q, but failed to read the serverless cluster configuration: %v", op.GetResourceId(), err), utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := serverlessclustermodel.Flatten(ctx, cluster, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read reads ServerlessCluster resource's values and updates the state.
func (c *ServerlessCluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model serverlessclustermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	cluster, err := c.CpCl.ServerlessClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read serverless cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}
	if cluster.GetState() == controlplanev1.ServerlessCluster_STATE_DELETING {
		// Null out state to force destroy + recreate.
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), cluster.Id)...)
		resp.Diagnostics.AddWarning(fmt.Sprintf("serverless cluster %s is in state %s", cluster.Id, cluster.GetState()), "")
		return
	}
	state, flatDiags := serverlessclustermodel.Flatten(ctx, cluster, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the ServerlessCluster. Supports private_link_id, networking_config, tags.
func (c *ServerlessCluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverlessclustermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, expandDiags := serverlessclustermodel.ExpandUpdate(ctx, &plan)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clResp, err := c.CpCl.ServerlessCluster.UpdateServerlessCluster(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to update serverless cluster", utils.DeserializeGrpcError(err))
		return
	}
	op := clResp.Operation
	updateTimeout, tdiags := plan.Timeouts.Update(ctx, 30*time.Minute)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, updateTimeout, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("operation error while updating serverless cluster", utils.DeserializeGrpcError(err))
		return
	}
	cluster, err := c.CpCl.ServerlessClusterForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("successfully updated the serverless cluster with ID %q, but failed to read the configuration: %v", plan.ID.ValueString(), err),
			utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := serverlessclustermodel.Flatten(ctx, cluster, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete deletes the ServerlessCluster resource.
func (c *ServerlessCluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model serverlessclustermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("serverless cluster deletion not allowed", "allow_deletion is set to false")
		return
	}

	delReq, expandDiags := serverlessclustermodel.ExpandDelete(ctx, &model)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clResp, err := c.CpCl.ServerlessCluster.DeleteServerlessCluster(ctx, delReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to delete serverless cluster", utils.DeserializeGrpcError(err))
		return
	}
	deleteTimeout, tdiags := model.Timeouts.Delete(ctx, 30*time.Minute)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := utils.AreWeDoneYet(ctx, clResp.Operation, deleteTimeout, c.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed to delete serverless cluster", utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState imports the serverless cluster resource by ID.
func (*ServerlessCluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceServerlessClusterSchema(ctx), &resp.State, "allow_deletion")...)
}
