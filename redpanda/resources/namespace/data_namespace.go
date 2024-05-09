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

package namespace

import (
	"context"
	"fmt"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta1/controlplanev1beta1grpc"
	controlplanev1beta1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceNamespace{}
)

// DataSourceNamespace represents a data source for a Redpanda Cloud namespace.
type DataSourceNamespace struct {
	Client controlplanev1beta1grpc.NamespaceServiceClient
}

// Metadata returns the metadata for the Namespace data source.
func (*DataSourceNamespace) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_namespace"
}

// Schema returns the schema for the Namespace data source.
func (*DataSourceNamespace) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
func (n *DataSourceNamespace) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Namespace
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	ns, err := n.Client.GetNamespace(ctx, &controlplanev1beta1.GetNamespaceRequest{
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

// Configure uses provider level data to configure DataSourceNamespace client.
func (n *DataSourceNamespace) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	p, ok := request.ProviderData.(config.Datasource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	n.Client = controlplanev1beta1grpc.NewNamespaceServiceClient(p.ControlPlaneConnection)
}
