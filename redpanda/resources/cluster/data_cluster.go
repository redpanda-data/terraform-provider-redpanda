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
		ClusterAPIURL:         types.StringValue(cluster.DataplaneApi.Url),
		ReadReplicaClusterIDs: rr,
	}

	if !isAwsPrivateLinkSpecNil(cluster.AwsPrivateLink) {
		ap, dig := types.ListValueFrom(ctx, types.StringType, cluster.AwsPrivateLink.AllowedPrincipals)
		if dig.HasError() {
			resp.Diagnostics.Append(dig...)
			return
		}
		persist.AwsPrivateLink = &models.AwsPrivateLink{
			Enabled:           types.BoolValue(cluster.AwsPrivateLink.Enabled),
			ConnectConsole:    types.BoolValue(cluster.AwsPrivateLink.ConnectConsole),
			AllowedPrincipals: ap,
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

	if !isAzurePrivateLinkSpecNil(cluster.AzurePrivateLink) {
		as, dig := types.ListValueFrom(ctx, types.StringType, cluster.AzurePrivateLink.AllowedSubscriptions)
		if dig.HasError() {
			resp.Diagnostics.Append(dig...)
			return
		}
		persist.AzurePrivateLink = &models.AzurePrivateLink{
			Enabled:              types.BoolValue(cluster.AzurePrivateLink.Enabled),
			ConnectConsole:       types.BoolValue(cluster.AzurePrivateLink.ConnectConsole),
			AllowedSubscriptions: as,
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
				Description: "ID of the cluster. ID is an output from the Create Cluster endpoint and cannot be set by the caller.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Unique name of the cluster.",
			},
			"cluster_type": schema.StringAttribute{
				Computed:    true,
				Description: "Cluster type. Type is immutable and can only be set on cluster creation.",
			},
			"connection_type": schema.StringAttribute{
				Computed:    true,
				Description: "Cluster connection type. Private clusters are not exposed to the internet. For BYOC clusters, Private is best-practice.",
			},
			"cloud_provider": schema.StringAttribute{
				Computed:    true,
				Description: "Cloud provider where resources are created.",
			},
			"redpanda_version": schema.StringAttribute{
				Computed:    true,
				Description: "Current Redpanda version of the cluster.",
			},
			"throughput_tier": schema.StringAttribute{
				Computed:    true,
				Description: "Throughput tier of the cluster.",
			},
			"region": schema.StringAttribute{
				Computed:    true,
				Description: "Cloud provider region. Region represents the name of the region where the cluster will be provisioned.",
			},
			"zones": schema.ListAttribute{
				Computed:    true,
				Description: "Zones of the cluster. Must be valid zones within the selected region. If multiple zones are used, the cluster is a multi-AZ cluster.",
				ElementType: types.StringType,
			},
			"allow_deletion": schema.BoolAttribute{
				Computed:    true,
				Description: "Allows deletion of the cluster. Defaults to true. Not recommended for production use.",
			},
			"tags": schema.MapAttribute{
				Computed:    true,
				Description: "Tags placed on cloud resources. If the cloud provider is GCP and the name of a tag has the prefix \"gcp.network-tag.\", the tag is a network tag that will be added to the Redpanda cluster GKE nodes. Otherwise, the tag is a normal tag. For example, if the name of a tag is \"gcp.network-tag.network-tag-foo\", the network tag named \"network-tag-foo\" will be added to the Redpanda cluster GKE nodes. Note: The value of a network tag will be ignored. See the details on network tags at https://cloud.google.com/vpc/docs/add-remove-network-tags.",
				ElementType: types.StringType,
			},
			"resource_group_id": schema.StringAttribute{
				Computed:    true,
				Description: "Resource group ID of the cluster.",
			},
			"network_id": schema.StringAttribute{
				Computed:    true,
				Description: "Network ID where cluster is placed.",
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:    true,
				Description: "The URL of the cluster API.",
			},
			"aws_private_link": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The AWS Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Redpanda AWS Private Link Endpoint Service is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Console is connected in Redpanda AWS Private Link Service.",
					},
					"allowed_principals": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
						Description: "The ARN of the principals that can access the Redpanda AWS PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
					},
				},
			},
			"azure_private_link": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The Azure Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"allowed_subscriptions": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
						Description: "The subscriptions that can access the Redpanda Azure PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
					},
					"connect_console": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Console is connected in Redpanda Azure Private Link Service.",
					},
					"enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Redpanda Azure Private Link Endpoint Service is enabled.",
					},
				},
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The GCP Private Service Connect configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Redpanda GCP Private Service Connect is enabled.",
					},
					"global_access_enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether global access is enabled.",
					},
					"consumer_accept_list": schema.ListNestedAttribute{
						Computed:    true,
						Description: "List of consumers that are allowed to connect to Redpanda GCP PSC (Private Service Connect) service attachment.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									Computed:    true,
									Description: "Either the GCP project number or its alphanumeric ID.",
								},
							},
						},
					},
				},
			},
			"kafka_api": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Cluster's Kafka API properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"http_proxy": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "HTTP Proxy properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"schema_registry": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Cluster's Schema Registry properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"read_replica_cluster_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of clusters which may create read-only topics from this cluster.",
			},
		},
		Description: "Data source for a Redpanda Cloud cluster",
	}
}
