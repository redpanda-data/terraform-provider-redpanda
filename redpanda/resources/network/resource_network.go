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

// Package network contains the implementation of the Network resource
// following the Terraform framework interfaces.
package network

import (
	"context"
	"fmt"
	"regexp"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Network{}
	_ resource.ResourceWithConfigure   = &Network{}
	_ resource.ResourceWithImportState = &Network{}
)

// Network represents a network managed resource.
type Network struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the full name of the Network resource.
func (*Network) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_network"
}

// Configure uses provider level data to configure Network's clients.
func (n *Network) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		// we can't add a diagnostic for an unset ProviderData here because
		// during the early part of the terraform lifecycle, the provider data
		// is not set and this is valid, but we also can't do anything until it
		// is set
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

// Schema returns the schema for the Network resource.
func (*Network) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceNetworkSchema()
}

func resourceNetworkSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cidr_block": schema.StringAttribute{
				Optional:      true,
				Description:   "The cidr_block to create the network in",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					validators.CIDRBlockValidator{},
				},
			},
			"region": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "The region to create the network in.",
			},
			"cloud_provider": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "The cloud provider to create the network in.",
				Validators:    validators.CloudProviders(),
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the resource group in which to create the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cluster_type": schema.StringAttribute{
				Required:      true,
				Description:   "The type of cluster this network is associated with, can be one of dedicated or cloud",
				Validators:    validators.ClusterTypes(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"customer_managed_resources": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"gcp": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
						Attributes: map[string]schema.Attribute{
							"network_name": schema.StringAttribute{
								Required:    true,
								Description: "Name of user-created network where the Redpanda cluster is deployed",
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
										"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
									),
									stringvalidator.LengthAtMost(62),
								},
							},
							"network_project_id": schema.StringAttribute{
								Required:    true,
								Description: "GCP project ID where the network is created",
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
										"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
									),
									stringvalidator.LengthAtMost(30),
								},
							},
							"management_bucket": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:    true,
										Description: "GCP storage bucket name for storing the state of Redpanda cluster deployment",
									},
								},
							},
						},
					},
					"aws": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
						Attributes: map[string]schema.Attribute{
							"management_bucket": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS storage bucket identifier",
									},
								},
							},
							"dynamodb_table": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS DynamoDB table identifier",
									},
								},
							},
							"vpc": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS VPC identifier",
									},
								},
							},
							"private_subnets": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arns": schema.ListAttribute{
										Required:    true,
										ElementType: types.StringType,
										Description: "AWS private subnet identifiers",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Create creates a new Network resource. It updates the state if the resource
// is successfully created.
func (n *Network) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.Network
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	cloudProvider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		response.Diagnostics.AddError("unsupported cloud provider", utils.DeserializeGrpcError(err))
		return
	}
	clusterType, err := utils.StringToClusterType(model.ClusterType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("unsupported cluster type", utils.DeserializeGrpcError(err))
		return
	}

	cmr, dgs := generateNetworkCMR(ctx, model, response.Diagnostics)
	if dgs.HasError() {
		response.Diagnostics = dgs
		return
	}

	netResp, err := n.CpCl.Network.CreateNetwork(ctx, &controlplanev1.CreateNetworkRequest{
		Network: &controlplanev1.NetworkCreate{
			Name:                     model.Name.ValueString(),
			CidrBlock:                model.CidrBlock.ValueString(),
			Region:                   model.Region.ValueString(),
			CloudProvider:            cloudProvider,
			ResourceGroupId:          model.ResourceGroupID.ValueString(),
			ClusterType:              clusterType,
			CustomerManagedResources: cmr,
		},
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create network", utils.DeserializeGrpcError(err))
		return
	}
	op := netResp.Operation
	// write initial state so that if network creation fails, we can still track and delete it
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), utils.TrimmedStringValue(op.GetResourceId()))...)

	if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, n.CpCl.Operation); err != nil {
		response.Diagnostics.AddError("failed waiting for network creation", utils.DeserializeGrpcError(err))
		return
	}

	nw, err := n.CpCl.NetworkForID(ctx, op.GetResourceId())
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", op.GetResourceId()), utils.DeserializeGrpcError(err))
		return
	}
	m, d := generateModel(model.CloudProvider.ValueString(), nw, response.Diagnostics)
	if d.HasError() {
		response.Diagnostics = append(response.Diagnostics, d...)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, m)...)
}

// Read reads Network resource's values and updates the state.
func (n *Network) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Network
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	nw, err := n.CpCl.NetworkForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	if nw.GetState() == controlplanev1.Network_STATE_DELETING {
		// null out the state, force it to be destroyed and recreated
		response.State.RemoveResource(ctx)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), nw.Id)...)
		response.Diagnostics.AddWarning(fmt.Sprintf("network %s is in state %s", nw.Id, nw.GetState()), "")
		return
	}
	m, d := generateModel(model.CloudProvider.ValueString(), nw, response.Diagnostics)
	if d.HasError() {
		response.Diagnostics = append(response.Diagnostics, d...)
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, m)...)
}

// Update is not supported for network. As a result all configurable schema
// elements have been marked as RequiresReplace.
func (*Network) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the Network resource.
func (n *Network) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Network
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	netResp, err := n.CpCl.Network.DeleteNetwork(ctx, &controlplanev1.DeleteNetworkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		response.Diagnostics.AddError("failed to delete network", utils.DeserializeGrpcError(err))
		return
	}
	if err := utils.AreWeDoneYet(ctx, netResp.Operation, 15*time.Minute, n.CpCl.Operation); err != nil {
		response.Diagnostics.AddError("failed waiting for network deletion", utils.DeserializeGrpcError(err))
	}
}

// ImportState refreshes the state with the correct ID for the network, allowing
// TF to use Read to get the correct Network name into state see
// https://developer.hashicorp.com/terraform/plugin/framework/resources/import
// for more details.
func (*Network) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
