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

// Package resourcegroup contains the implementation of the ResourceGroup resource
// following the Terraform framework interfaces.
package resourcegroup

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	resourcegroupmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/resourcegroup"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &ResourceGroup{}
	_ resource.ResourceWithConfigure   = &ResourceGroup{}
	_ resource.ResourceWithImportState = &ResourceGroup{}
)

// ResourceGroup represents a cluster managed resource.
type ResourceGroup struct {
	base.ResourceBase
}

// NewResourceGroup constructs a ResourceGroup resource.
func NewResourceGroup() *ResourceGroup {
	r := &ResourceGroup{}
	r.ResourceBase = base.NewResourceBase("redpanda_resource_group", ResourceGroupSchema, nil)
	return r
}

// Create creates a new ResourceGroup resource. It updates the state if the
// resource is successfully created.
func (n *ResourceGroup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model resourcegroupmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pbReq, diags := resourcegroupmodel.ExpandCreate(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := n.CpCl.ResourceGroup.CreateResourceGroup(ctx, pbReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create resource group", utils.DeserializeGrpcError(err))
		return
	}

	persist, diags := resourcegroupmodel.Flatten(ctx, apiResp.ResourceGroup, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads ResourceGroup resource's values and updates the state.
func (n *ResourceGroup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model resourcegroupmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	rg, err := n.CpCl.ResourceGroupForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read resource group", utils.DeserializeGrpcError(err))
		return
	}

	persist, diags := resourcegroupmodel.Flatten(ctx, rg, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update updates the state of the ResourceGroup resource.
func (n *ResourceGroup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resourcegroupmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pbReq, diags := resourcegroupmodel.ExpandUpdate(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := n.CpCl.ResourceGroup.UpdateResourceGroup(ctx, pbReq); err != nil {
		resp.Diagnostics.AddError("failed to update resource group", utils.DeserializeGrpcError(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the ResourceGroup resource.
func (n *ResourceGroup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model resourcegroupmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pbReq, diags := resourcegroupmodel.ExpandDelete(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := n.CpCl.ResourceGroup.DeleteResourceGroup(ctx, pbReq); err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete resource group", utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState refreshes the state with the correct ID for the ResourceGroup,
// allowing TF to use Read to get the correct ResourceGroup name into state see
// https://developer.hashicorp.com/terraform/plugin/framework/resources/import
// for more details.
func (*ResourceGroup) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
