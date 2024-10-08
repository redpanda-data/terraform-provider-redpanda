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

// Package serverlesscluster contains the implementation of the ServerlessCluster resource
// following the Terraform framework interfaces.
package serverlesscluster

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceServerlessCluster{}
)

// DataSourceServerlessCluster represents a serverless cluster data source.
type DataSourceServerlessCluster struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the metadata for the ServerlessCluster data source.
func (*DataSourceServerlessCluster) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_serverless_cluster"
}

// Configure uses provider level data to configure DataSourceServerlessCluster's client.
func (d *DataSourceServerlessCluster) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(config.Datasource)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	d.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Read reads the ServerlessCluster data source's values and updates the state.
func (d *DataSourceServerlessCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.ServerlessCluster
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	serverlessCluster, err := d.CpCl.ServerlessClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.Diagnostics.AddError(fmt.Sprintf("unable to find serverless cluster %s", model.ID), err.Error())
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read serverless cluster %s", model.ID), err.Error())
		return
	}
	// Mapping the fields from the serverless cluster to the Terraform state
	persist := generateModel(serverlessCluster)
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Schema returns the schema for the ServerlessCluster data source.
func (*DataSourceServerlessCluster) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceServerlessClusterSchema() // Reuse the schema from the resource
}

func datasourceServerlessClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the serverless cluster",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the serverless cluster",
			},
			"serverless_region": schema.StringAttribute{
				Computed:    true,
				Description: "Redpanda specific region for the serverless cluster",
			},
			"resource_group_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the resource group in which to create the serverless cluster",
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:    true,
				Description: "The URL of the cluster API",
			},
		},
		Description: "Data source for a Redpanda Cloud serverless cluster",
	}
}
