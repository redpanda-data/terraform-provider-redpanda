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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceCluster{}
)

// DataSourceCluster represents a cluster data source.
type DataSourceCluster struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the metadata for the Cluster data source.
func (*DataSourceCluster) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_cluster"
}

// Configure uses provider level data to configure DataSourceCluster's client.
func (d *DataSourceCluster) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read reads the Cluster data source's values and updates the state.
func (d *DataSourceCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	cluster, err := d.CpCl.ClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.Diagnostics.AddError(fmt.Sprintf("unable to find cluster %s", model.ID), err.Error())
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), err.Error())
		return
	}
	clusterZones, dg := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if dg.HasError() {
		resp.Diagnostics.Append(dg...)
		return
	}
	clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
	if err != nil {
		resp.Diagnostics.AddError("unable to parse Cluster API URL", err.Error())
		return
	}
	tags := make(map[string]attr.Value)
	for k, v := range cluster.CloudProviderTags {
		tags[k] = types.StringValue(v)
	}
	tagsValue, diags := types.MapValue(types.StringType, tags)
	if diags.HasError() {
		resp.Diagnostics.AddError("unable to parse Cloud tags", err.Error())
		return
	}
	rr, derr := types.ListValueFrom(ctx, types.StringType, cluster.ReadReplicaClusterIds)
	if derr.HasError() {
		resp.Diagnostics.Append(derr...)
		return
	}

	// Mapping the fields from the cluster to the Terraform state
	persist := &models.Cluster{
		Name:                  types.StringValue(cluster.Name),
		ConnectionType:        types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:         types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:           types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion:       types.StringValue(cluster.RedpandaVersion),
		ThroughputTier:        types.StringValue(cluster.ThroughputTier),
		Region:                types.StringValue(cluster.Region),
		Zones:                 clusterZones,
		Tags:                  tagsValue,
		ResourceGroupID:       types.StringValue(cluster.ResourceGroupId),
		NetworkID:             types.StringValue(cluster.NetworkId),
		ID:                    types.StringValue(cluster.Id),
		ClusterAPIURL:         types.StringValue(clusterURL),
		ReadReplicaClusterIds: rr,
	}

	if !isAwsPrivateLinkSpecNil(cluster.AwsPrivateLink) {
		if len(cluster.AwsPrivateLink.AllowedPrincipals) > 0 {
			pl, dg := awsPrivateLinkStructToModel(ctx, cluster.GetAwsPrivateLink())
			if dg.HasError() {
				resp.Diagnostics.Append(dg...)
			}
			persist.AwsPrivateLink = pl
		}
	}
	if !isGcpPrivateServiceConnectSpecNil(cluster.GcpPrivateServiceConnect) {
		if len(cluster.GcpPrivateServiceConnect.ConsumerAcceptList) > 0 {
			persist.GcpPrivateServiceConnect = &models.GcpPrivateServiceConnect{
				Enabled:             types.BoolValue(cluster.GcpPrivateServiceConnect.Enabled),
				GlobalAccessEnabled: types.BoolValue(cluster.GcpPrivateServiceConnect.GlobalAccessEnabled),
				ConsumerAcceptList:  gcpConnectConsumerStructToModel(cluster.GcpPrivateServiceConnect.ConsumerAcceptList),
			}
		}
	}

	kAPI, dg := toMtlsModel(ctx, cluster.GetKafkaApi().GetMtls())
	if dg != nil {
		resp.Diagnostics.Append(dg...)
		return
	}
	persist.KafkaAPI = &models.KafkaAPI{
		Mtls: kAPI,
	}
	hp, dg := toMtlsModel(ctx, cluster.GetHttpProxy().GetMtls())
	if dg != nil {
		resp.Diagnostics.Append(dg...)
		return
	}
	persist.HTTPProxy = &models.HTTPProxy{
		Mtls: hp,
	}
	sr, dg := toMtlsModel(ctx, cluster.GetSchemaRegistry().GetMtls())
	if dg != nil {
		resp.Diagnostics.Append(dg...)
		return
	}
	persist.SchemaRegistry = &models.SchemaRegistry{
		Mtls: sr,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Schema returns the schema for the Cluster data source.
func (*DataSourceCluster) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceClusterSchema() // Reuse the schema from the resource
}

func datasourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the cluster",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the cluster",
			},
			"cluster_type": schema.StringAttribute{
				Computed:    true,
				Description: "Type of the cluster",
			},
			"connection_type": schema.StringAttribute{
				Computed:    true,
				Description: "Connection type of the cluster",
			},
			"cloud_provider": schema.StringAttribute{
				Computed:    true,
				Description: "Must be one of aws or gcp",
			},
			"redpanda_version": schema.StringAttribute{
				Computed:    true,
				Description: "Version of Redpanda to deploy",
			},
			"throughput_tier": schema.StringAttribute{
				Computed:    true,
				Description: "Throughput tier of the cluster",
			},
			"region": schema.StringAttribute{
				Computed:    true,
				Description: "Cloud provider specific region of the cluster",
			},
			"zones": schema.ListAttribute{
				Computed:    true,
				Description: "Cloud provider specific zones of the cluster",
				ElementType: types.StringType,
			},
			"allow_deletion": schema.BoolAttribute{
				Computed:    true,
				Description: "allows deletion of the cluster. defaults to true. Not recommended for production use",
			},
			"tags": schema.MapAttribute{
				Computed:    true,
				Description: "Tags to apply to the cluster",
				ElementType: types.StringType,
			},
			"resource_group_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the resource group in which to create the cluster",
			},
			"network_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the network in which to create the cluster",
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:    true,
				Description: "The URL of the cluster API",
			},
			"aws_private_link": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required: true,
					},
					"allowed_principals": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
					},
				},
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required: true,
					},
					"global_access_enabled": schema.BoolAttribute{
						Required: true,
					},
					"consumer_accept_list": schema.ListNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									Required: true,
								},
							},
						},
					},
				},
			},
			"kafka_api": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
						},
					},
				},
			},
			"http_proxy": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
						},
					},
				},
			},
			"schema_registry": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
						},
					},
				},
			},
			"read_replica_cluster_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
		},
		Description: "Data source for a Redpanda Cloud cluster",
	}
}
