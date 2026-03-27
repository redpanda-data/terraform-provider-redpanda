// Copyright 2024 Redpanda Data, Inc.
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

// Package group contains the implementation of the Group resource
// following the Terraform framework interfaces.
package group

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Group{}
	_ resource.ResourceWithConfigure   = &Group{}
	_ resource.ResourceWithImportState = &Group{}
)

// Group represents a Redpanda Cloud group managed resource.
type Group struct {
	GroupCl *cloud.GroupClient
}

// Metadata returns the full name of the Group resource.
func (*Group) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_group"
}

// Configure uses provider level data to configure Group client.
func (g *Group) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	p, ok := request.ProviderData.(config.Resource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.Resource, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	g.GroupCl = cloud.NewGroupClient(p.CloudAPIURL, p.AuthToken)
}

// Schema returns the schema for the Group resource.
func (*Group) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = groupSchema()
}

func groupSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "UUID of the group",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the group. Changing the name requires replacing the group.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the group",
			},
		},
		Description: "A Redpanda Cloud group for group-based access control (GBAC).",
		Version:     1,
	}
}

// Create creates a new Group resource.
func (g *Group) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Group
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grp, err := g.GroupCl.CreateGroup(ctx, model.Name.ValueString(), model.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create group", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.Group{
		ID:          types.StringValue(grp.ID),
		Name:        types.StringValue(grp.Name),
		Description: types.StringValue(grp.Description),
	})...)
}

// Read reads Group resource's values and updates the state.
func (g *Group) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.Group
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grp, err := g.GroupCl.GetGroup(ctx, model.ID.ValueString())
	if err != nil {
		if cloud.IsHTTPNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read group", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.Group{
		ID:          types.StringValue(grp.ID),
		Name:        types.StringValue(grp.Name),
		Description: types.StringValue(grp.Description),
	})...)
}

// Update updates the Group resource. Only description can be updated.
func (*Group) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("update not supported", "group name changes require replacement; description updates are not yet supported by the API")
}

// Delete deletes the Group resource.
func (g *Group) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.Group
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := g.GroupCl.DeleteGroup(ctx, model.ID.ValueString())
	if err != nil {
		if cloud.IsHTTPNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete group", err.Error())
		return
	}
}

// ImportState allows importing an existing group by ID.
func (*Group) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
