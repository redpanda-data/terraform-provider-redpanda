package namespace

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
	_ datasource.DataSource = &NamespaceDataSource{}
)

// NamespaceDataSource represents a data source for a Redpanda Cloud namespace.
type NamespaceDataSource struct {
	Client cloudv1beta1.NamespaceServiceClient
}

func (*NamespaceDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_namespace"
}

// Schema returns the schema for the Namespace data source.
func (*NamespaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceNamespaceSchema()
}

// datasourceNamespaceSchema defines the schema for a namespace data source.
func datasourceNamespaceSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the namespace",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the namespace",
			},
		},
		Description: "Data source for a Redpanda Cloud namespace",
	}
}

// Read reads the Namespace data source's values and updates the state.
func (n *NamespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Namespace
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	ns, err := n.Client.GetNamespace(ctx, &cloudv1beta1.GetNamespaceRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to read namespace", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.Namespace{
		Name: types.StringValue(ns.Name),
		ID:   types.StringValue(ns.Id),
	})...)
}

// Configure uses provider level data to configure NamespaceDataSource client.
func (n *NamespaceDataSource) Configure(ctx context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at NamespaceDataSource.Configure")
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
	client, err := clients.NewNamespaceServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create namespace client", err.Error())
		return
	}
	n.Client = client
}
