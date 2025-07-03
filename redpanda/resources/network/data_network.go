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

package network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceNetwork{}

// DataSourceNetwork represents a data source for a Redpanda Cloud network.
type DataSourceNetwork struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the metadata for the Network data source.
func (*DataSourceNetwork) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_network"
}

// Schema returns the schema for the Network data source.
func (*DataSourceNetwork) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceNetworkSchema()
}

// Read reads the Network data source's values and updates the state.
func (n *DataSourceNetwork) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model network.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	nw, err := n.CpCl.NetworkForID(ctx, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	m, d := model.GetUpdatedModel(ctx, nw)
	if d.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, d...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// Configure uses provider level data to configure DataSourceNetwork's client.
func (n *DataSourceNetwork) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
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
	n.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}
