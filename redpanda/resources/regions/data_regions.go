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

// Package regions contains the implementation of the Regions data
// source following the Terraform framework interfaces.
package regions

import (
	"context"
	"fmt"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceRegions{}
)

// DataSourceRegions represents a data source for a list of Redpanda
// Cloud regions.
type DataSourceRegions struct {
	CpCl *cloud.ControlPlaneClientSet
}

// DataSourceRegionsSchema defines the schema for a Regions data
// source.
func DataSourceRegionsSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Required:    true,
				Description: "Cloud provider where the regions exist",
				Validators:  validators.CloudProviders(),
			},
			"regions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Name of the region",
						},
						"zones": schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Zones available in the region",
						},
					},
				},
				Description: "Regions available for the cloud provider",
			},
		},
		Description: "Data source for a list of Redpanda Cloud regions",
	}
}

// Metadata returns the metadata for the Regions data source.
func (*DataSourceRegions) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_regions"
}

// Schema returns the schema for the Regions data source.
func (*DataSourceRegions) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = DataSourceRegionsSchema()
}

// Read reads the Regions data source's values and updates the state.
func (r *DataSourceRegions) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Regions
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudProvider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("unsupported cloud provider", err.Error())
		return
	}

	regions, err := r.CpCl.Region.ListRegions(ctx, &controlplanev1beta2.ListRegionsRequest{
		CloudProvider: cloudProvider,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to read regions", err.Error())
		return
	}
	if regions.Regions == nil {
		resp.Diagnostics.AddError("failed to read regions; please report this bug to Redpanda Support", "")
		return
	}

	model.Regions = []models.RegionsItem{}
	for _, v := range regions.Regions {
		item := models.RegionsItem{
			Name:  v.Name,
			Zones: v.Zones,
		}
		model.Regions = append(model.Regions, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

// Configure uses provider level data to configure DataSourceRegions client.
func (r *DataSourceRegions) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
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
