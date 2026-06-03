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

// Package serverlessprivatelink contains the implementation of the ServerlessPrivateLink resource
// following the Terraform framework interfaces.
package serverlessprivatelink

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	splmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlessprivatelink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &ServerlessPrivateLink{}
	_ resource.ResourceWithConfigure   = &ServerlessPrivateLink{}
	_ resource.ResourceWithImportState = &ServerlessPrivateLink{}
)

// ServerlessPrivateLink represents a serverless private link managed resource.
type ServerlessPrivateLink struct {
	base.ResourceBase
}

// NewServerlessPrivateLink constructs a ServerlessPrivateLink resource.
func NewServerlessPrivateLink() *ServerlessPrivateLink {
	s := &ServerlessPrivateLink{}
	s.ResourceBase = base.NewResourceBase("redpanda_serverless_private_link", ResourceServerlessPrivateLinkSchema, nil)
	return s
}

// Create creates a new ServerlessPrivateLink resource.
func (s *ServerlessPrivateLink) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan splmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, expandDiags := splmodel.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plResp, err := s.CpCl.ServerlessPrivateLink.CreateServerlessPrivateLink(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	op := plResp.Operation
	tflog.Info(ctx, "creating serverless private link", map[string]any{"id": op.GetResourceId()})
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), op.GetResourceId())...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, tdiags := plan.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, createTimeout, s.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("operation error while creating serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	privateLink, err := s.CpCl.ServerlessPrivateLinkForID(ctx, op.GetResourceId())
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("successfully created the serverless private link with ID %q, but failed to read the configuration: %v", op.GetResourceId(), err),
			utils.DeserializeGrpcError(err))
		return
	}

	state, flatDiags := splmodel.Flatten(ctx, privateLink, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "serverless private link created", map[string]any{"id": privateLink.GetId()})
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read reads ServerlessPrivateLink resource's values and updates the state.
func (s *ServerlessPrivateLink) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model splmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	privateLink, err := s.CpCl.ServerlessPrivateLinkForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read serverless private link %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	if privateLink.GetState() == controlplanev1.ServerlessPrivateLink_STATE_DELETING {
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("serverless private link %s is in state %s", privateLink.Id, privateLink.GetState()),
			"The resource will be removed from state")
		return
	}

	tflog.Debug(ctx, "read serverless private link", map[string]any{"id": privateLink.GetId(), "state": privateLink.GetState().String()})
	state, flatDiags := splmodel.Flatten(ctx, privateLink, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the ServerlessPrivateLink resource. Currently supports updating allowed_principals for AWS.
func (s *ServerlessPrivateLink) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan splmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state splmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "updating serverless private link", map[string]any{"id": plan.ID.ValueString()})

	updateReq, mask, expandDiags := splmodel.ExpandUpdateWithMask(ctx, &plan, &state)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The update request carries no FieldMask; an empty mask means no
	// backend-relevant field changed (e.g. an allow_deletion-only flip), so
	// skip the RPC and just re-read to settle computed state.
	if len(mask.Paths) != 0 {
		plResp, err := s.CpCl.ServerlessPrivateLink.UpdateServerlessPrivateLink(ctx, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("failed to update serverless private link", utils.DeserializeGrpcError(err))
			return
		}
		updateTimeout, tdiags := plan.Timeouts.Update(ctx, 30*time.Minute)
		resp.Diagnostics.Append(tdiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := utils.AreWeDoneYet(ctx, plResp.Operation, updateTimeout, s.CpCl.Operation); err != nil {
			resp.Diagnostics.AddError("operation error while updating serverless private link", utils.DeserializeGrpcError(err))
			return
		}
	}

	privateLink, err := s.CpCl.ServerlessPrivateLinkForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("successfully updated the serverless private link with ID %q, but failed to read the configuration: %v", plan.ID.ValueString(), err),
			utils.DeserializeGrpcError(err))
		return
	}

	tflog.Info(ctx, "serverless private link updated", map[string]any{"id": plan.ID.ValueString()})
	newState, flatDiags := splmodel.Flatten(ctx, privateLink, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Delete deletes the ServerlessPrivateLink resource.
func (s *ServerlessPrivateLink) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model splmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("serverless private link deletion not allowed", "allow_deletion is set to false")
		return
	}
	tflog.Info(ctx, "deleting serverless private link", map[string]any{"id": model.ID.ValueString()})

	delReq, expandDiags := splmodel.ExpandDelete(ctx, &model)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plResp, err := s.CpCl.ServerlessPrivateLink.DeleteServerlessPrivateLink(ctx, delReq)
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("failed to delete serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	deleteTimeout, tdiags := model.Timeouts.Delete(ctx, 30*time.Minute)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := utils.AreWeDoneYet(ctx, plResp.Operation, deleteTimeout, s.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed to delete serverless private link", utils.DeserializeGrpcError(err))
		return
	}
	tflog.Info(ctx, "serverless private link deleted", map[string]any{"id": model.ID.ValueString()})
}

// ImportState imports and updates the state of the serverless private link resource.
func (*ServerlessPrivateLink) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceServerlessPrivateLinkSchema(ctx), &resp.State, "allow_deletion")...)
}
