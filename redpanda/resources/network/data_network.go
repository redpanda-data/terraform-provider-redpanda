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
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceNetwork{}
)

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
			"cidr_block": schema.StringAttribute{
				Computed:    true,
				Description: "The cidr_block to create the network in",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}/(\d{1,2})$`),
						"The value must be a valid CIDR block (e.g., 192.168.0.0/16)",
					),
				},
			},
			"region": schema.StringAttribute{
				Computed:    true,
				Description: "The region to create the network in. Can also be set at the provider level",
			},
			"cloud_provider": schema.StringAttribute{
				Computed:    true,
				Description: "The cloud provider to create the network in. Can also be set at the provider level",
				Validators: []validator.String{
					stringvalidator.OneOf("gcp", "aws"),
				},
			},
			"resource_group_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the resource group in which to create the network",
			},
			"cluster_type": schema.StringAttribute{
				Computed:    true,
				Description: "The type of cluster this network is associated with, can be one of dedicated or cloud",
				Validators: []validator.String{
					stringvalidator.OneOf("dedicated", "cloud"),
				},
			},
		},
		Description: "Data source for a Redpanda Cloud network",
	}
}

// Read reads the Network data source's values and updates the state.
func (n *DataSourceNetwork) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Network
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	nw, err := n.CpCl.NetworkForID(ctx, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, generateModel(nw))...)
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
