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
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlessprivatelink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ServerlessPrivateLink{}
	_ resource.ResourceWithConfigure   = &ServerlessPrivateLink{}
	_ resource.ResourceWithImportState = &ServerlessPrivateLink{}
)

// ServerlessPrivateLink represents a serverless private link managed resource.
type ServerlessPrivateLink struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the full name of the ServerlessPrivateLink resource.
func (*ServerlessPrivateLink) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_serverless_private_link"
}

// Configure uses provider level data to configure ServerlessPrivateLink's clients.
func (s *ServerlessPrivateLink) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	s.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the ServerlessPrivateLink resource.
func (*ServerlessPrivateLink) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceServerlessPrivateLinkSchema()
}

// ResourceServerlessPrivateLinkSchema returns the schema for the ServerlessPrivateLink resource.
func ResourceServerlessPrivateLinkSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a Redpanda Serverless Private Link",
		Attributes: map[string]schema.Attribute{
			// Required fields
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the serverless private link",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the Resource Group in which to create the serverless private link",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_provider": schema.StringAttribute{
				Required:      true,
				Description:   "Cloud provider (aws)",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    validators.CloudProviders(),
			},
			"serverless_region": schema.StringAttribute{
				Required:      true,
				Description:   "Redpanda serverless region",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},

			// Cloud provider config (oneof)
			"cloud_provider_config": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Cloud provider specific configuration",
				Attributes: map[string]schema.Attribute{
					"aws": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "AWS-specific configuration. Required when cloud_provider is 'aws'.",
						Attributes: map[string]schema.Attribute{
							"allowed_principals": schema.ListAttribute{
								Required:    true,
								ElementType: types.StringType,
								Description: "AWS principals (ARNs) allowed to connect to the private link endpoint",
								Validators: []validator.List{
									listvalidator.SizeAtLeast(1),
								},
							},
						},
					},
				},
			},

			// Optional fields
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Allows deletion of the serverless private link. Defaults to false.",
			},

			// Computed fields
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the serverless private link",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				Computed:      true,
				Description:   "Current state of the serverless private link (STATE_CREATING, STATE_READY, STATE_DELETING, STATE_FAILED, STATE_UPDATING)",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Description:   "Timestamp when the serverless private link was created",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
				Description: "Timestamp when the serverless private link was last updated. " +
					"This value changes on every update operation.",
			},

			// Status (cloud-specific, read-only)
			"status": schema.SingleNestedAttribute{
				Computed:      true,
				Description:   "Cloud provider specific status information",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"aws": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "AWS-specific status information",
						Attributes: map[string]schema.Attribute{
							"vpc_endpoint_service_name": schema.StringAttribute{
								Computed:      true,
								Description:   "VPC endpoint service name for connecting to the private link",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"availability_zones": schema.ListAttribute{
								Computed:      true,
								ElementType:   types.StringType,
								Description:   "Availability zones where the private link endpoint service is available",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
						},
					},
				},
			},
		},
	}
}

// Create creates a new ServerlessPrivateLink resource. It updates the state if the resource
// is successfully created.
func (s *ServerlessPrivateLink) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model serverlessprivatelink.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	privateLinkReq, err := GenerateServerlessPrivateLinkRequest(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError("unable to parse CreateServerlessPrivateLink request", err.Error())
		return
	}

	plResp, err := s.CpCl.ServerlessPrivateLink.CreateServerlessPrivateLink(ctx, &controlplanev1.CreateServerlessPrivateLinkRequest{
		ServerlessPrivateLink: privateLinkReq,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	op := plResp.Operation
	// Write initial state so that if creation fails, we can still track and delete it
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), op.GetResourceId())...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := utils.AreWeDoneYet(ctx, op, 30*time.Minute, s.CpCl.Operation); err != nil {
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

	persist, err := generateModel(ctx, privateLink, model.AllowDeletion)
	if err != nil {
		resp.Diagnostics.AddError("failed to generate serverless private link model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads ServerlessPrivateLink resource's values and updates the state.
func (s *ServerlessPrivateLink) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model serverlessprivatelink.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	privateLink, err := s.CpCl.ServerlessPrivateLinkForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			// Treat HTTP 404 Not Found status as a signal to recreate resource and return early
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read serverless private link %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	if privateLink.GetState() == controlplanev1.ServerlessPrivateLink_STATE_DELETING {
		// Remove from state to force recreation if needed
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("serverless private link %s is in state %s", privateLink.Id, privateLink.GetState()),
			"The resource will be removed from state")
		return
	}

	persist, err := generateModel(ctx, privateLink, model.AllowDeletion)
	if err != nil {
		resp.Diagnostics.AddError("failed to generate serverless private link model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update updates the ServerlessPrivateLink resource. Currently supports updating allowed_principals for AWS.
func (s *ServerlessPrivateLink) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model serverlessprivatelink.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, err := GenerateServerlessPrivateLinkUpdateRequest(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError("unable to parse UpdateServerlessPrivateLink request", err.Error())
		return
	}

	plResp, err := s.CpCl.ServerlessPrivateLink.UpdateServerlessPrivateLink(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to update serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	op := plResp.Operation
	if err := utils.AreWeDoneYet(ctx, op, 30*time.Minute, s.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("operation error while updating serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	privateLink, err := s.CpCl.ServerlessPrivateLinkForID(ctx, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("successfully updated the serverless private link with ID %q, but failed to read the configuration: %v", model.ID.ValueString(), err),
			utils.DeserializeGrpcError(err))
		return
	}

	persist, err := generateModel(ctx, privateLink, model.AllowDeletion)
	if err != nil {
		resp.Diagnostics.AddError("failed to generate serverless private link model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Delete deletes the ServerlessPrivateLink resource.
func (s *ServerlessPrivateLink) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model serverlessprivatelink.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("serverless private link deletion not allowed", "allow_deletion is set to false")
		return
	}

	plResp, err := s.CpCl.ServerlessPrivateLink.DeleteServerlessPrivateLink(ctx, &controlplanev1.DeleteServerlessPrivateLinkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			// Resource already deleted, treat as success
			return
		}
		resp.Diagnostics.AddError("failed to delete serverless private link", utils.DeserializeGrpcError(err))
		return
	}

	if err := utils.AreWeDoneYet(ctx, plResp.Operation, 30*time.Minute, s.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed to delete serverless private link", utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState imports and updates the state of the serverless private link resource.
func (*ServerlessPrivateLink) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)
}
