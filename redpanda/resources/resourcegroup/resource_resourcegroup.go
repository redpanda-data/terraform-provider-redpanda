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
	"fmt"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/resourcegroup"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ResourceGroup{}
	_ resource.ResourceWithConfigure   = &ResourceGroup{}
	_ resource.ResourceWithImportState = &ResourceGroup{}
)

// ResourceGroup represents a cluster managed resource.
type ResourceGroup struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the full name of the ResourceGroup resource.
func (*ResourceGroup) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_resource_group"
}

// Configure uses provider level data to configure ResourceGroup client.
func (n *ResourceGroup) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		// We can't add a diagnostic for an unset ProviderData here because
		// during the early part of the terraform lifecycle, the provider data
		// is not set and this is valid, but we also can't do anything until it
		// is set.
		return
	}

	p, ok := request.ProviderData.(config.Resource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	n.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the ResourceGroup resource.
func (*ResourceGroup) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceGroupSchema()
}

// ResourceGroupSchema defines the schema for a resource group.
func ResourceGroupSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the resource group. Changing the name of a resource group will result in a new resource group being created and the old one being destroyed",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "UUID of the resource group",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
		Description: "A Redpanda Cloud resource group",
		Version:     1,
	}
}

// Create creates a new ResourceGroup resource. It updates the state if the
// resource is successfully created.
func (n *ResourceGroup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model resourcegroup.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	rg, err := n.CpCl.CreateResourceGroup(ctx, model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create resource group", utils.DeserializeGrpcError(err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, resourcegroup.ResourceModel{
		Name: types.StringValue(rg.Name),
		ID:   types.StringValue(rg.Id),
	})...)
}

// Read reads ResourceGroup resource's values and updates the state.
func (n *ResourceGroup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model resourcegroup.ResourceModel
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

	resp.Diagnostics.Append(resp.State.Set(ctx, resourcegroup.ResourceModel{
		Name: types.StringValue(rg.Name),
		ID:   types.StringValue(rg.Id),
	})...)
}

// Update updates the state of the ResourceGroup resource.
func (n *ResourceGroup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model resourcegroup.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	name, id := model.Name.ValueString(), model.ID.ValueString()
	_, err := n.CpCl.ResourceGroup.UpdateResourceGroup(ctx, &controlplanev1.UpdateResourceGroupRequest{
		ResourceGroup: &controlplanev1.ResourceGroupUpdate{
			Name: name,
			Id:   id,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update resource group", utils.DeserializeGrpcError(err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, resourcegroup.ResourceModel{
		Name: types.StringValue(name),
		ID:   types.StringValue(id),
	})...)
}

// Delete deletes the ResourceGroup resource.
func (n *ResourceGroup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model resourcegroup.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	_, err := n.CpCl.ResourceGroup.DeleteResourceGroup(ctx, &controlplanev1.DeleteResourceGroupRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
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
