// Copyright 2026 Redpanda Data, Inc.
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

// Package cloudprovideraccess contains the implementation of the
// CloudProviderAccess resource and datasources following the Terraform
// framework interfaces.
package cloudprovideraccess

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	cpamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cloudprovideraccess"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_ resource.Resource                = &CloudProviderAccess{}
	_ resource.ResourceWithConfigure   = &CloudProviderAccess{}
	_ resource.ResourceWithImportState = &CloudProviderAccess{}
)

// rotationHint tells a user how to replace an access a network still uses.
// Terraform deletes a removed or replaced dependency before updating its
// dependents unless the dependency is create_before_destroy, which it records
// in state; the control plane refuses the delete while a network uses it.
const rotationHint = "If a network still uses this access, set lifecycle { create_before_destroy = true } on it and give the new access a different name: Terraform then creates the new access, re-points the networks in place, and deletes this one last."

// CloudProviderAccess represents a cloud provider access managed resource.
type CloudProviderAccess struct {
	base.ResourceBase
}

// NewCloudProviderAccess constructs a CloudProviderAccess resource.
func NewCloudProviderAccess() *CloudProviderAccess {
	r := &CloudProviderAccess{}
	r.ResourceBase = base.NewResourceBase("redpanda_cloud_provider_access", ResourceCloudProviderAccessSchema, nil)
	return r
}

// Create creates a new CloudProviderAccess resource.
func (r *CloudProviderAccess) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cpamodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "creating cloud provider access", map[string]any{"name": plan.Name.ValueString()})

	pbReq, diags := cpamodel.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.CpCl.CloudProviderAccess.CreateCloudProviderAccess(ctx, pbReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create cloud provider access", utils.DeserializeGrpcError(err))
		return
	}
	cpa := apiResp.GetCloudProviderAccess()
	if cpa.GetId() == "" {
		resp.Diagnostics.AddError("failed to create cloud provider access", "the API returned no cloud provider access; please report this issue to the provider developers")
		return
	}

	persist, diags := cpamodel.Flatten(ctx, cpa, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
	tflog.Info(ctx, "cloud provider access created", map[string]any{"id": cpa.GetId()})
}

// Read reads the CloudProviderAccess resource's values and updates the state.
func (r *CloudProviderAccess) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cpamodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cpa, err := r.CpCl.CloudProviderAccessForID(ctx, state.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cloud provider access %s", state.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	persist, diags := cpamodel.Flatten(ctx, cpa, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update refuses the change. The API has no update RPC and every attribute
// forces replacement, so a plan never reaches Update; saving one here would
// record state the API never saw.
func (*CloudProviderAccess) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("cloud provider access cannot be updated",
		"A cloud provider access has no update API; change it by replacing it.")
}

// Delete deletes the CloudProviderAccess resource. The control plane refuses
// to delete an access a network still references.
func (r *CloudProviderAccess) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cpamodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "deleting cloud provider access", map[string]any{"id": state.ID.ValueString()})

	pbReq, diags := cpamodel.ExpandDelete(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.CpCl.CloudProviderAccess.DeleteCloudProviderAccess(ctx, pbReq); err != nil {
		if utils.IsNotFound(err) {
			return
		}
		detail := utils.DeserializeGrpcError(err)
		if isReferencedByNetwork(err) {
			if detail = strings.TrimRight(detail, " :."); detail != "" {
				detail += ". "
			}
			detail += rotationHint
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete cloud provider access %s", state.ID.ValueString()), detail)
		return
	}
	tflog.Info(ctx, "cloud provider access deleted", map[string]any{"id": state.ID.ValueString()})
}

// isReferencedByNetwork reports whether err is the control plane refusing to
// delete an access a network still references. That refusal is the delete
// path's only FailedPrecondition, and the public API strips its message
// (cloudv2 pkg/grpc NewSafePublicError keeps a message only from an
// ExternalError detail), so the code alone identifies it.
func isReferencedByNetwork(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.FailedPrecondition
}

// ImportState imports a cloud provider access by ID; the following Read
// populates the remaining attributes.
func (*CloudProviderAccess) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
