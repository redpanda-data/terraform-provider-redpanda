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

// Package serverlessregions contains the implementation of the ServerlessRegions data
// source following the Terraform framework interfaces.
package serverlessregions

import (
	"context"
	"fmt"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceServerlessRegions{}
)

// DataSourceServerlessRegions represents a data source for a list of Redpanda Cloud serverless regions.
type DataSourceServerlessRegions struct {
	CpCl *cloud.ControlPlaneClientSet
}

// DataSourceServerlessRegionsSchema defines the schema for a ServerlessServerlessRegions data source.
func DataSourceServerlessRegionsSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Required:    true,
				Description: "Cloud provider where the serverless regions exist",
				Validators:  validators.CloudProviders(),
			},
			"serverless_regions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"available": schema.BoolAttribute{
							Computed:    true,
							Description: "Region available",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Name of the serverless region",
						},
						"time_zone": schema.StringAttribute{
							Computed:    true,
							Description: "Time zone of the serverless region",
						},
					},
				},
				Description: "Serverless regions available for the cloud provider",
			},
		},
		Description: "Data source for a list of Redpanda Cloud serverless regions",
	}
}

// Metadata returns the metadata for the ServerlessRegions data source.
func (*DataSourceServerlessRegions) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_serverless_regions"
}

// Schema returns the schema for the ServerlessRegions data source.
func (*DataSourceServerlessRegions) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = DataSourceServerlessRegionsSchema()
}

// Read reads the ServerlessRegions data source's values and updates the state.
func (r *DataSourceServerlessRegions) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.ServerlessRegions
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudProvider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("unsupported cloud provider", utils.DeserializeGrpcError(err))
		return
	}
	regions, err := r.CpCl.ServerlessRegion.ListServerlessRegions(ctx, &controlplanev1.ListServerlessRegionsRequest{
		CloudProvider: cloudProvider,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to read regions", utils.DeserializeGrpcError(err))
		return
	}
	if regions.ServerlessRegions == nil {
		resp.Diagnostics.AddError("failed to read regions; please report this bug to Redpanda Support", "")
		return
	}

	model.ServerlessRegions = []models.ServerlessRegionsItem{}
	for _, v := range regions.ServerlessRegions {
		item := models.ServerlessRegionsItem{
			CloudProvider: utils.CloudProviderToString(v.GetCloudProvider()),
			Name:          v.GetName(),
			TimeZone:      v.GetDefaultTimezone().String(),
		}
		model.ServerlessRegions = append(model.ServerlessRegions, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

// Configure uses provider level data to configure DataSourceServerlessRegions client.
func (r *DataSourceServerlessRegions) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
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
	r.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}
