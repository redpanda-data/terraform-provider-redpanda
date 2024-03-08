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

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Network{}
	_ resource.ResourceWithConfigure   = &Network{}
	_ resource.ResourceWithImportState = &Network{}
)

// Network represents a network managed resource.
type Network struct {
	NetClient cloudv1beta1.NetworkServiceClient
	OpsClient cloudv1beta1.OperationServiceClient
}

// Metadata returns the full name of the Network resource.
func (*Network) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_network"
}

// Configure uses provider level data to configure Network's clients.
func (n *Network) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		// we can't add a diagnostic for an unset ProviderData here because
		// during the early part of the terraform lifecycle, the provider data
		// is not set and this is valid, but we also can't do anything until it
		// is set
		return
	}

	p, ok := request.ProviderData.(utils.ResourceData)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	client, err := clients.NewNetworkServiceClient(ctx, p.CloudEnv, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create network client", err.Error())
		return
	}

	opsClient, err := clients.NewOperationServiceClient(ctx, p.CloudEnv, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create ops client", err.Error())
		return
	}

	n.NetClient = client
	n.OpsClient = opsClient
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
				Required:      true,
				Description:   "The cidr_block to create the network in",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}/(\d{1,2})$`),
						"The value must be a valid CIDR block (e.g., 192.168.0.0/16)",
					),
				},
			},
			"region": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "The region to create the network in. Can also be set at the provider level",
			},
			"cloud_provider": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "The cloud provider to create the network in. Can also be set at the provider level",
				Validators: []validator.String{
					stringvalidator.OneOf("gcp", "aws"),
				},
			},
			"namespace_id": schema.StringAttribute{
				Required:      true,
				Description:   "The id of the namespace in which to create the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cluster_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of cluster this network is associated with, can be one of dedicated or cloud",
				Validators: []validator.String{
					stringvalidator.OneOf("dedicated", "cloud"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
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
		response.Diagnostics.AddError("unsupported cloud provider", err.Error())
		return
	}
	clusterType, err := utils.StringToClusterType(model.ClusterType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("unsupported cluster type", err.Error())
		return
	}
	// TODO add a check to the provider data here to see if region and cloud provider are set
	// prefer the local value, but accept the provider value if local is
	// unavailable if neither are set, fail

	op, err := n.NetClient.CreateNetwork(ctx, &cloudv1beta1.CreateNetworkRequest{
		Network: &cloudv1beta1.Network{
			Name:          model.Name.ValueString(),
			CidrBlock:     model.CidrBlock.ValueString(),
			Region:        model.Region.ValueString(),
			CloudProvider: cloudProvider,
			NamespaceId:   model.NamespaceID.ValueString(),
			ClusterType:   clusterType,
		},
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create network", err.Error())
		return
	}
	var metadata cloudv1beta1.CreateNetworkMetadata
	if err := op.Metadata.UnmarshalTo(&metadata); err != nil {
		response.Diagnostics.AddError("failed to unmarshal network metadata", err.Error())
		return
	}

	if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, time.Minute, n.OpsClient); err != nil {
		response.Diagnostics.AddError("failed waiting for network creation", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, models.Network{
		Name:          model.Name,
		ID:            utils.TrimmedStringValue(metadata.GetNetworkId()),
		CidrBlock:     model.CidrBlock,
		Region:        model.Region,
		NamespaceID:   model.NamespaceID,
		ClusterType:   model.ClusterType,
		CloudProvider: model.CloudProvider,
	})...)
}

// Read reads Network resource's values and updates the state.
func (n *Network) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Network
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	nw, err := n.NetClient.GetNetwork(ctx, &cloudv1beta1.GetNetworkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.ID.ValueString()), err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, models.Network{
		Name:          types.StringValue(nw.Name),
		ID:            types.StringValue(nw.Id),
		CidrBlock:     types.StringValue(nw.CidrBlock),
		Region:        types.StringValue(nw.Region),
		NamespaceID:   types.StringValue(nw.NamespaceId),
		CloudProvider: types.StringValue(utils.CloudProviderToString(nw.CloudProvider)),
		ClusterType:   types.StringValue(utils.ClusterTypeToString(nw.ClusterType)),
	})...)
}

// Update is not supported for network. As a result all configurable schema
// elements have been marked as RequiresReplace.
func (*Network) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the Network resource.
func (n *Network) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Network
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	op, err := n.NetClient.DeleteNetwork(ctx, &cloudv1beta1.DeleteNetworkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		response.Diagnostics.AddError("failed to delete network", err.Error())
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, time.Minute, n.OpsClient); err != nil {
		response.Diagnostics.AddError("failed waiting for network deletion", err.Error())
	}
}

// ImportState refreshes the state with the correct ID for the network, allowing
// TF to use Read to get the correct Network name into state see
// https://developer.hashicorp.com/terraform/plugin/framework/resources/import
// for more details.
func (*Network) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
