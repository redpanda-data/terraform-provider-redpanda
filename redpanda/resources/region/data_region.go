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

// Package region contains the implementation of the Region data
// source following the Terraform framework interfaces.
package region

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
	_ datasource.DataSource = &DataSourceRegion{}
)

// DataSourceRegion represents a data source for a Redpanda Cloud region.
type DataSourceRegion struct {
	CpCl *cloud.ControlPlaneClientSet
}

// DataSourceRegionSchema defines the schema for a region data source.
func DataSourceRegionSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Required:    true,
				Description: "Cloud provider where the region exists",
				Validators:  validators.CloudProviders(),
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the region",
			},
			"zones": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Zones available in the region",
			},
		},
		Description: "Data source for a Redpanda Cloud region",
	}
}

// Metadata returns the metadata for the Region data source.
func (*DataSourceRegion) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_region"
}

// Schema returns the schema for the Region data source.
func (*DataSourceRegion) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = DataSourceRegionSchema()
}

// Read reads the Region data source's values and updates the state.
func (r *DataSourceRegion) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Region
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudProvider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("unsupported cloud provider", utils.DeserializeGrpcError(err))
		return
	}

	region, err := r.CpCl.Region.GetRegion(ctx, &controlplanev1beta2.GetRegionRequest{
		Name:          model.Name.ValueString(),
		CloudProvider: cloudProvider,
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read region %v", model.Name), utils.DeserializeGrpcError(err))
		return
	}
	if region.Region == nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read region %v; please report this bug to Redpanda Support", model.Name), "")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.Region{
		CloudProvider: types.StringValue(utils.CloudProviderToString(region.Region.CloudProvider)),
		Name:          types.StringValue(region.Region.Name),
		Zones:         region.Region.Zones,
	})...)
}

// Configure uses provider level data to configure DataSourceRegion client.
func (r *DataSourceRegion) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
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
