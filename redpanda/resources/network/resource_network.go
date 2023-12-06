package network

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"regexp"
	"time"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Network{}
var _ resource.ResourceWithConfigure = &Network{}
var _ resource.ResourceWithImportState = &Network{}

type Network struct {
	NetClient cloudv1beta1.NetworkServiceClient
	OpsClient cloudv1beta1.OperationServiceClient
}

func (n *Network) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_network"
}

func (n *Network) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		// we can't add a diagnostic for an unset providerdata here because during the early part of the terraform
		// lifecycle, the provider data is not set and this is valid
		// but we also can't do anything until it is set
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at network.Configure")
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
	client, err := clients.NewNetworkServiceClient(ctx, p.Version, clients.ClientRequest{
		AuthToken:    p.AuthToken,
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create network client", err.Error())
		return
	}

	opsClient, err := clients.NewOperationServiceClient(ctx, p.Version, clients.ClientRequest{
		AuthToken:    p.AuthToken,
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

func (n *Network) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = ResourceNetworkSchema()
}

func ResourceNetworkSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the network",
			},
			"cidr_block": schema.StringAttribute{
				Required:    true,
				Description: "The cidr_block to create the network in",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}\/(\d{1,2})$`),
						"The value must be a valid CIDR block (e.g., 192.168.0.0/16)",
					),
				},
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "The region to create the network in. Can also be set at the provider level",
				// TODO add appropriate validators
			},
			"cloud_provider": schema.StringAttribute{
				Optional:    true,
				Description: "The cloud provider to create the network in. Can also be set at the provider level",
				Validators: []validator.String{
					stringvalidator.OneOf("gcp", "aws"),
				},
			},
			"namespace_id": schema.StringAttribute{
				Required:    true,
				Description: "The id of the namespace in which to create the network",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the namespace",
			},
			"cluster_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of cluster this network is associated with, can be one of dedicated or cloud",
				Validators: []validator.String{
					stringvalidator.OneOf("dedicated", "cloud"),
				},
			},
		},
	}
}

func (n *Network) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.Network
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	cloudProvider := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	// TODO add a check to the provider data here to see if region and cloud provider are set
	// prefer the local value, but accept the provider value if local is unavailable
	// if neither are set, fail

	op, err := n.NetClient.CreateNetwork(ctx, &cloudv1beta1.CreateNetworkRequest{
		Network: &cloudv1beta1.Network{
			Name:          model.Name.ValueString(),
			CidrBlock:     model.CidrBlock.ValueString(),
			Region:        model.Region.ValueString(),
			CloudProvider: cloudProvider,
			NamespaceId:   model.NamespaceId.ValueString(),
			ClusterType:   utils.StringToClusterType(model.ClusterType.ValueString()),
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

	// TODO accept user configuration for timeout
	if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, n.OpsClient); err != nil {
		response.Diagnostics.AddError("failed waiting for network creation", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, models.Network{
		Name:          model.Name,
		Id:            utils.TrimmedStringValue(metadata.GetNetworkId()),
		CidrBlock:     model.CidrBlock,
		Region:        model.Region,
		NamespaceId:   model.NamespaceId,
		ClusterType:   model.ClusterType,
		CloudProvider: model.CloudProvider,
	})...)
}

func (n *Network) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Network
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	nw, err := n.NetClient.GetNetwork(ctx, &cloudv1beta1.GetNetworkRequest{
		Id: model.Id.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		} else {
			response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.Id.ValueString()), err.Error())
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(ctx, models.Network{
		Name:          types.StringValue(nw.Name),
		Id:            types.StringValue(nw.Id),
		CidrBlock:     types.StringValue(nw.CidrBlock),
		Region:        types.StringValue(nw.Region),
		NamespaceId:   types.StringValue(nw.NamespaceId),
		CloudProvider: types.StringValue(utils.CloudProviderToString(nw.CloudProvider)),
		ClusterType:   types.StringValue(utils.ClusterTypeToString(nw.ClusterType)),
	})...)
}

func (n *Network) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	// TODO no update AFAIK, need to delete then create every time
}

func (n *Network) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Network
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	op, err := n.NetClient.DeleteNetwork(ctx, &cloudv1beta1.DeleteNetworkRequest{
		Id: model.Id.ValueString(),
	})

	if err != nil {
		response.Diagnostics.AddError("failed to delete network", err.Error())
		return
	}
	// TODO allow configurable timeout
	if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, n.OpsClient); err != nil {
		response.Diagnostics.AddError("failed waiting for network deletion", err.Error())
	}
}

// ImportState refreshes the state with the correct ID for the namespace, allowing TF to use Read to get the correct Namespace name into state
// see https://developer.hashicorp.com/terraform/plugin/framework/resources/import for more details
func (n *Network) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(ctx, models.Network{
		Id: types.StringValue(request.ID),
	})...)
}
