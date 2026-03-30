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

// Package serviceaccount contains the implementation of the ServiceAccount resource
// following the Terraform framework interfaces.
package serviceaccount

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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ServiceAccount{}
	_ resource.ResourceWithConfigure   = &ServiceAccount{}
	_ resource.ResourceWithImportState = &ServiceAccount{}
)

// ServiceAccount represents a Redpanda Cloud service account managed resource.
type ServiceAccount struct {
	IAMCl *cloud.IAMClientSet
}

// Metadata returns the full name of the ServiceAccount resource.
func (*ServiceAccount) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_service_account"
}

// Configure uses provider level data to configure ServiceAccount client.
func (s *ServiceAccount) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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
	s.IAMCl = cloud.NewIAMClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the ServiceAccount resource.
func (*ServiceAccount) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "UUID of the service account",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the service account",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the service account",
			},
		},
		Description: "A Redpanda Cloud service account for programmatic access.",
		Version:     1,
	}
}

// Create creates a new ServiceAccount resource.
func (s *ServiceAccount) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.ServiceAccount
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := s.IAMCl.CreateServiceAccount(ctx, model.Name.ValueString(), model.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create service account", utils.DeserializeGrpcError(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.ServiceAccount{
		ID:          types.StringValue(sa.GetId()),
		Name:        types.StringValue(sa.GetName()),
		Description: types.StringValue(sa.GetDescription()),
	})...)
}

// Read reads ServiceAccount resource's values and updates the state.
func (s *ServiceAccount) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.ServiceAccount
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := s.IAMCl.ServiceAccountForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read service account", utils.DeserializeGrpcError(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.ServiceAccount{
		ID:          types.StringValue(sa.GetId()),
		Name:        types.StringValue(sa.GetName()),
		Description: types.StringValue(sa.GetDescription()),
	})...)
}

// Update updates the ServiceAccount resource.
func (s *ServiceAccount) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.ServiceAccount
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state models.ServiceAccount
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := s.IAMCl.UpdateServiceAccount(ctx, state.ID.ValueString(), plan.Name.ValueString(), plan.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to update service account", utils.DeserializeGrpcError(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.ServiceAccount{
		ID:          types.StringValue(sa.GetId()),
		Name:        types.StringValue(sa.GetName()),
		Description: types.StringValue(sa.GetDescription()),
	})...)
}

// Delete deletes the ServiceAccount resource.
func (s *ServiceAccount) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.ServiceAccount
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := s.IAMCl.DeleteServiceAccount(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete service account", utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState allows importing an existing service account by ID.
func (*ServiceAccount) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
