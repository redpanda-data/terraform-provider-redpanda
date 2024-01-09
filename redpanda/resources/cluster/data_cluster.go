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

// Package cluster contains the implementation of the Cluster resource
// following the Terraform framework interfaces.
package cluster

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
	_ datasource.DataSource = &DataSourceCluster{}
)

// DataSourceCluster represents a cluster data source.
type DataSourceCluster struct {
	CluClient cloudv1beta1.ClusterServiceClient
}

// Metadata returns the metadata for the Cluster data source.
func (*DataSourceCluster) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_cluster"
}

// Configure uses provider level data to configure DataSourceCluster's client.
func (d *DataSourceCluster) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		resp.Diagnostics.AddWarning("provider data not set", "provider data not set at clusterdatasource.Configure")
		return
	}

	p, ok := req.ProviderData.(utils.DatasourceData)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	client, err := clients.NewClusterServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster client", err.Error())
		return
	}
	d.CluClient = client
}

// Read reads the Cluster data source's values and updates the state.
func (d *DataSourceCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	cluster, err := d.CluClient.GetCluster(ctx, &cloudv1beta1.GetClusterRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), err.Error())
		return
	}

	lv, dg := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if dg.HasError() {
		resp.Diagnostics.Append(dg...)
		return
	}

	// Mapping the fields from the cluster to the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &models.Cluster{
		Name:            types.StringValue(cluster.Name),
		ConnectionType:  types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion: types.StringValue(cluster.RedpandaVersion),
		ThroughputTier:  types.StringValue(cluster.ThroughputTier),
		Region:          types.StringValue(cluster.Region),
		Zones:           lv,
		AllowDeletion:   model.AllowDeletion,
		Tags:            model.Tags,
		NamespaceID:     types.StringValue(cluster.NamespaceId),
		NetworkID:       types.StringValue(cluster.NetworkId),
		ID:              types.StringValue(cluster.Id),
	})...)
}

// Schema returns the schema for the Cluster data source.
func (*DataSourceCluster) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceClusterSchema() // Reuse the schema from the resource
}

func datasourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the cluster",
			},
			"cluster_type": schema.StringAttribute{
				Required:    true,
				Description: "Type of the cluster",
			},
			"connection_type": schema.StringAttribute{
				Required:    true,
				Description: "Connection type of the cluster",
			},
			"cloud_provider": schema.StringAttribute{
				Optional:    true,
				Description: "Must be one of aws or gcp",
			},
			"redpanda_version": schema.StringAttribute{
				Optional:    true,
				Description: "Version of redpanda to deploy",
			},
			"throughput_tier": schema.StringAttribute{
				Required:    true,
				Description: "Throughput tier of the cluster",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "Cloud provider specific region of the cluster",
			},
			"zones": schema.ListAttribute{
				Optional:    true,
				Description: "Cloud provider specific zones of the cluster",
				ElementType: types.StringType,
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Description: "allows deletion of the cluster. defaults to true. should probably be set to false for production use",
			},
			"tags": schema.MapAttribute{
				Optional:    true,
				Description: "Tags to apply to the cluster",
				ElementType: types.StringType,
			},
			"namespace_id": schema.StringAttribute{
				Required:    true,
				Description: "The id of the namespace in which to create the cluster",
			},
			"network_id": schema.StringAttribute{
				Required:    true,
				Description: "The id of the network in which to create the cluster",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The id of the cluster",
			},
		},
	}
}
