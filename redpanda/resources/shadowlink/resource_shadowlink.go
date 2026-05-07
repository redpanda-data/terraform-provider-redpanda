// Copyright 2025 Redpanda Data, Inc.
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

// Package shadowlink implements the redpanda_shadow_link resource.
package shadowlink

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	shadowlinkmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/shadowlink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var (
	_ resource.Resource                     = &ShadowLink{}
	_ resource.ResourceWithConfigure        = &ShadowLink{}
	_ resource.ResourceWithImportState      = &ShadowLink{}
	_ resource.ResourceWithConfigValidators = &ShadowLink{}
)

// ShadowLink represents the ShadowLink Terraform resource.
type ShadowLink struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the type name for the ShadowLink resource.
func (*ShadowLink) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_shadow_link"
}

// Configure stores provider-supplied data on the resource.
func (s *ShadowLink) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	p, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.Resource, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	s.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the ShadowLink resource.
func (*ShadowLink) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceShadowLinkSchema(ctx)
}

// Create creates a ShadowLink resource.
func (s *ShadowLink) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := model.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, d := model.BuildCreateRequest(ctx)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := s.CpCl.ShadowLink.CreateShadowLink(ctx, &controlplanev1.CreateShadowLinkRequest{
		ShadowLink: createReq,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create shadow link", utils.DeserializeGrpcError(err))
		return
	}

	op := createResp.GetOperation()

	if err := utils.AreWeDoneYet(ctx, op, createTimeout, s.CpCl.Operation); err != nil {
		// Persist a known minimal state so destroy can find the resource after a mid-create failure.
		resp.Diagnostics.Append(resp.State.Set(ctx, shadowlinkmodel.GenerateMinimalResourceModel(op.GetResourceId(), model.Timeouts))...)
		resp.Diagnostics.AddError("failed waiting for shadow link creation", utils.DeserializeGrpcError(err))
		return
	}

	sl, err := s.CpCl.ShadowLinkForID(ctx, op.GetResourceId())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read shadow link %s", op.GetResourceId()), utils.DeserializeGrpcError(err))
		return
	}
	persist, d := model.GetUpdatedModel(ctx, sl)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	persist.Timeouts = model.Timeouts
	persist.AllowDeletion = model.AllowDeletion
	persist.SourceRedpandaID = model.SourceRedpandaID

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads the ShadowLink resource.
func (s *ShadowLink) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sl, err := s.CpCl.ShadowLinkForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read shadow link %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	if sl.GetState() == controlplanev1.ShadowLink_STATE_DELETING {
		// Null out state so Terraform forces destroy-and-recreate on the next plan.
		resp.Diagnostics.Append(resp.State.Set(ctx, shadowlinkmodel.GenerateMinimalResourceModel(sl.GetId(), model.Timeouts))...)
		resp.Diagnostics.AddWarning(fmt.Sprintf("shadow link %s is in state %s", sl.GetId(), sl.GetState()), "")
		return
	}

	persist, d := model.GetUpdatedModel(ctx, sl)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	persist.Timeouts = model.Timeouts
	persist.AllowDeletion = model.AllowDeletion
	persist.SourceRedpandaID = model.SourceRedpandaID

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update updates the ShadowLink resource.
func (s *ShadowLink) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, paths, d := plan.BuildUpdateRequest(ctx, &state)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(paths) > 0 {
		updateTimeout, diags := plan.Timeouts.Update(ctx, 30*time.Minute)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		updateResp, err := s.CpCl.ShadowLink.UpdateShadowLink(ctx, &controlplanev1.UpdateShadowLinkRequest{
			ShadowLink: updateReq,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to update shadow link", utils.DeserializeGrpcError(err))
			return
		}

		op := updateResp.GetOperation()
		if err := utils.AreWeDoneYet(ctx, op, updateTimeout, s.CpCl.Operation); err != nil {
			resp.Diagnostics.AddError("failed waiting for shadow link update", utils.DeserializeGrpcError(err))
			return
		}
	}

	sl, err := s.CpCl.ShadowLinkForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read shadow link %s", plan.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	persist, d := plan.GetUpdatedModel(ctx, sl)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	persist.Timeouts = plan.Timeouts
	persist.AllowDeletion = plan.AllowDeletion
	persist.SourceRedpandaID = plan.SourceRedpandaID

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Delete deletes the ShadowLink resource.
func (s *ShadowLink) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("shadow link deletion not allowed", "allow_deletion is set to false")
		return
	}

	deleteTimeout, diags := model.Timeouts.Delete(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	delResp, err := s.CpCl.ShadowLink.DeleteShadowLink(ctx, &controlplanev1.DeleteShadowLinkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("failed to delete shadow link", utils.DeserializeGrpcError(err))
		return
	}
	if err := utils.AreWeDoneYet(ctx, delResp.GetOperation(), deleteTimeout, s.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed waiting for shadow link deletion", utils.DeserializeGrpcError(err))
	}
}

// ImportState passes the ID through. Read repopulates everything else.
func (*ShadowLink) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// Seed allow_deletion=false on import so the user must opt in.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)
}
