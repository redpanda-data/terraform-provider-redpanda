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

package resourcegroup

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/resourcegroup"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceResourceGroup{}

// DataSourceResourceGroup represents a data source for a Redpanda Cloud resource group.
type DataSourceResourceGroup struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the metadata for the ResourceGroup data source.
func (*DataSourceResourceGroup) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_resource_group"
}

// Schema returns the schema for the ResourceGroup data source.
func (*DataSourceResourceGroup) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceResourceGroupSchema()
}

// datasourceResourceGroupSchema defines the schema for a resource group data source.
func datasourceResourceGroupSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "UUID of the resource group",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the resource group",
			},
		},
		Description: "Data source for a Redpanda Cloud resource group",
	}
}

// Read reads the ResourceGroup data source's values and updates the state.
func (n *DataSourceResourceGroup) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model resourcegroup.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	rg, err := n.CpCl.ResourceGroupForIDOrName(ctx, model.ID.ValueString(), model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read resource group", utils.DeserializeGrpcError(err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, resourcegroup.DataModel{
		Name: types.StringValue(rg.Name),
		ID:   types.StringValue(rg.Id),
	})...)
}

// Configure uses provider level data to configure DataSourceResourceGroup client.
func (n *DataSourceResourceGroup) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
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
