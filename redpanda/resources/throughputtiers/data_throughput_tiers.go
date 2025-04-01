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

// Package throughputtiers contains the implementation of the ThroughputTiers data
// source following the Terraform framework interfaces.
package throughputtiers

import (
	"context"
	"fmt"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
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
	_ datasource.DataSource = &DataSourceThroughputTiers{}
)

// DataSourceThroughputTiers represents a data source for a list of Redpanda Cloud throughput tiers.
type DataSourceThroughputTiers struct {
	CpCl *cloud.ControlPlaneClientSet
}

// DataSourceThroughputTiersSchema defines the schema for a Throughput Tiers data source.
func DataSourceThroughputTiersSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Optional:    true,
				Description: "Cloud provider where the Throughput Tiers are available",
				Validators:  validators.CloudProviders(),
			},
			"throughput_tiers": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"cloud_provider": schema.StringAttribute{
							Computed:    true,
							Description: "Cloud provider where the Throughput Tier is available",
						},
						"display_name": schema.StringAttribute{
							Computed:    true,
							Description: "Display name of the Throughput Tier",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Unique name of the Throughput Tier",
						},
					},
				},
				Description: "Throughput Tiers",
			},
		},
		Description: "Data source for a list of Redpanda Cloud throughput tiers",
	}
}

// Metadata returns the metadata for the Throughput Tiers data source.
func (*DataSourceThroughputTiers) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_throughput_tiers"
}

// Schema returns the schema for the Throughput Tiers data source.
func (*DataSourceThroughputTiers) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = DataSourceThroughputTiersSchema()
}

// Read reads the Throughput Tiers data source's values and updates the state.
func (r *DataSourceThroughputTiers) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.ThroughputTiers
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	listReq := &controlplanev1beta2.ListThroughputTiersRequest{}
	if !model.CloudProvider.IsNull() {
		cloudProvider, err := utils.StringToCloudProviderBeta(model.CloudProvider.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("unsupported cloud provider", utils.DeserializeGrpcError(err))
			return
		}

		listReq.Filter = &controlplanev1beta2.ListThroughputTiersRequest_Filter{
			CloudProvider: cloudProvider,
		}
	}

	tiers, err := r.CpCl.ThroughputTier.ListThroughputTiers(ctx, listReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to read throughput tiers", utils.DeserializeGrpcError(err))
		return
	}
	if tiers.ThroughputTiers == nil {
		resp.Diagnostics.AddError("failed to read throughput tiers; please report this bug to Redpanda Support", "")
		return
	}

	model.ThroughputTiers = []models.ThroughputTiersItem{}
	for _, v := range tiers.ThroughputTiers {
		item := models.ThroughputTiersItem{
			CloudProvider: utils.CloudProviderToStringBeta(v.CloudProvider),
			DisplayName:   v.DisplayName,
			Name:          v.Name,
		}
		model.ThroughputTiers = append(model.ThroughputTiers, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

// Configure uses provider level data to configure DataSourceThroughputTiers client.
func (r *DataSourceThroughputTiers) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
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
