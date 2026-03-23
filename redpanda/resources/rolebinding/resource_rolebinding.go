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

// Package rolebinding contains the implementation of the RoleBinding resource
// following the Terraform framework interfaces.
package rolebinding

import (
	"context"
	"fmt"
	"strings"

	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &RoleBinding{}
	_ resource.ResourceWithConfigure   = &RoleBinding{}
	_ resource.ResourceWithImportState = &RoleBinding{}
)

// scopeResourceTypeMap maps user-friendly string names to proto enum values.
var scopeResourceTypeMap = map[string]iamv1.RoleBinding_ScopeResourceType{
	"resource_group":     iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_RESOURCE_GROUP,
	"network":            iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_NETWORK,
	"cluster":            iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_CLUSTER,
	"serverless_cluster": iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_SERVERLESS_CLUSTER,
	"network_peering":    iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_NETWORK_PEERING,
	"organization":       iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_ORGANIZATION,
}

// scopeResourceTypeReverseMap maps proto enum values to user-friendly string names.
var scopeResourceTypeReverseMap = map[iamv1.RoleBinding_ScopeResourceType]string{
	iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_RESOURCE_GROUP:     "resource_group",
	iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_NETWORK:            "network",
	iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_CLUSTER:            "cluster",
	iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_SERVERLESS_CLUSTER: "serverless_cluster",
	iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_NETWORK_PEERING:    "network_peering",
	iamv1.RoleBinding_SCOPE_RESOURCE_TYPE_ORGANIZATION:       "organization",
}

// RoleBinding represents a role binding managed resource.
type RoleBinding struct {
	IAMCl *cloud.IAMClientSet
}

// Metadata returns the full name of the RoleBinding resource.
func (*RoleBinding) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_role_binding"
}

// Configure uses provider level data to configure RoleBinding client.
func (r *RoleBinding) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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
	r.IAMCl = cloud.NewIAMClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the RoleBinding resource.
func (*RoleBinding) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = roleBindingSchema()
}

func validScopeResourceTypes() []string {
	keys := make([]string, 0, len(scopeResourceTypeMap))
	for k := range scopeResourceTypeMap {
		keys = append(keys, k)
	}
	return keys
}

func roleBindingSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "UUID of the role binding",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role_name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the role to bind",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"account_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the account (user or service account) to bind the role to",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scope_resource_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The type of resource to scope the role binding to. Valid values: resource_group, network, cluster, serverless_cluster, network_peering, organization. If not set, the role binding is at the organization level.",
				Validators: []validator.String{
					stringvalidator.OneOf(validScopeResourceTypes()...),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scope_resource_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "The ID of the resource to scope the role binding to. Required when scope_resource_type is set.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
		Description: "Binds a role to an account (user or service account), optionally scoped to a specific resource.",
		Version:     1,
	}
}

// Create creates a new RoleBinding resource.
func (r *RoleBinding) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.RoleBinding
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope := buildScope(model)

	rb, err := r.IAMCl.CreateRoleBinding(ctx, model.RoleName.ValueString(), model.AccountID.ValueString(), scope)
	if err != nil {
		resp.Diagnostics.AddError("failed to create role binding", utils.DeserializeGrpcError(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, roleBindingToModel(rb))...)
}

// Read reads RoleBinding resource's values and updates the state.
func (r *RoleBinding) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.RoleBinding
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rb, err := r.IAMCl.RoleBindingForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read role binding", utils.DeserializeGrpcError(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, roleBindingToModel(rb))...)
}

// Update is a no-op since all attributes require replacement.
func (*RoleBinding) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("update not supported", "role bindings are immutable; changes require replacement")
}

// Delete deletes the RoleBinding resource.
func (r *RoleBinding) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.RoleBinding
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.IAMCl.DeleteRoleBinding(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete role binding", utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState allows importing an existing role binding by ID.
func (*RoleBinding) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildScope constructs the proto Scope from the Terraform model.
func buildScope(model models.RoleBinding) *iamv1.RoleBinding_Scope {
	if model.ScopeResourceType.IsNull() || model.ScopeResourceType.IsUnknown() || model.ScopeResourceType.ValueString() == "" {
		return nil
	}

	resourceType, ok := scopeResourceTypeMap[strings.ToLower(model.ScopeResourceType.ValueString())]
	if !ok {
		return nil
	}

	return &iamv1.RoleBinding_Scope{
		ResourceType: resourceType,
		ResourceId:   model.ScopeResourceID.ValueString(),
	}
}

// roleBindingToModel converts a proto RoleBinding to a Terraform model.
func roleBindingToModel(rb *iamv1.RoleBinding) models.RoleBinding {
	model := models.RoleBinding{
		ID:        types.StringValue(rb.GetId()),
		RoleName:  types.StringValue(rb.GetRoleName()),
		AccountID: types.StringValue(rb.GetAccountId()),
	}

	if rb.GetScope() != nil {
		if name, ok := scopeResourceTypeReverseMap[rb.GetScope().GetResourceType()]; ok {
			model.ScopeResourceType = types.StringValue(name)
		} else {
			model.ScopeResourceType = types.StringValue("")
		}
		model.ScopeResourceID = types.StringValue(rb.GetScope().GetResourceId())
	} else {
		model.ScopeResourceType = types.StringValue("")
		model.ScopeResourceID = types.StringValue("")
	}

	return model
}
