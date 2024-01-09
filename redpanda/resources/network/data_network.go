package network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceNetwork{}
)

// DataSourceNetwork represents a data source for a Redpanda Cloud network.
type DataSourceNetwork struct {
	NetClient cloudv1beta1.NetworkServiceClient
}

// Metadata returns the metadata for the Network data source.
func (*DataSourceNetwork) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_network"
}

// Schema returns the schema for the Network data source.
func (*DataSourceNetwork) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceNetworkSchema()
}

// datasourceNetworkSchema defines the schema for a network data source.
func datasourceNetworkSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the network",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the network",
			},
			// Add other attributes here as needed, marking them as 'Computed'
		},
		Description: "Data source for a Redpanda Cloud network",
	}
}

// Read reads the Network data source's values and updates the state.
func (n *DataSourceNetwork) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Network
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	nw, err := n.NetClient.GetNetwork(ctx, &cloudv1beta1.GetNetworkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to read network", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.Network{
		Name: types.StringValue(nw.Name),
		ID:   types.StringValue(nw.Id),
		// Map other network attributes here as needed
	})...)
}

// Configure uses provider level data to configure DataSourceNetwork's client.
func (n *DataSourceNetwork) Configure(ctx context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at DataSourceNetwork.Configure")
		return
	}

	p, ok := request.ProviderData.(utils.ResourceData)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	client, err := clients.NewNetworkServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create network client", err.Error())
		return
	}
	n.NetClient = client
}
