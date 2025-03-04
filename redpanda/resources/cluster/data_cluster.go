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
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
			resp.Diagnostics.AddError(fmt.Sprintf("unable to find cluster %s", model.ID), utils.DeserializeGrpcError(err))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	// Convert cloud provider tags to Terraform map
	tags := make(map[string]attr.Value)
	for k, v := range cluster.CloudProviderTags {
		tags[k] = types.StringValue(v)
	}
	tagsValue, diags := types.MapValue(types.StringType, tags)
	if diags.HasError() {
		resp.Diagnostics.AddError("unable to parse Cloud tags", utils.DeserializeGrpcError(err))
		return
	}

	// Create persistence model
	persist := &models.Cluster{
		Name:                  types.StringValue(cluster.Name),
		ConnectionType:        types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:         types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:           types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion:       types.StringValue(cluster.RedpandaVersion),
		ThroughputTier:        types.StringValue(cluster.ThroughputTier),
		Region:                types.StringValue(cluster.Region),
		ResourceGroupID:       types.StringValue(cluster.ResourceGroupId),
		NetworkID:             types.StringValue(cluster.NetworkId),
		ID:                    types.StringValue(cluster.Id),
		Tags:                  tagsValue,
		Zones:                 utils.StringSliceToTypeList(cluster.Zones),
		ReadReplicaClusterIDs: utils.StringSliceToTypeList(cluster.ReadReplicaClusterIds),
		AllowDeletion:         types.BoolValue(true), // Default to true for data source
		State:                 types.StringValue(cluster.State.String()),
	}

	if cluster.HasCreatedAt() {
		persist.CreatedAt = types.StringValue(cluster.CreatedAt.AsTime().Format(time.RFC3339))
	}

	if cluster.HasStateDescription() {
		stateDescription, d := generateStateDescription(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.StateDescription = stateDescription
	}

	if cluster.HasDataplaneApi() {
		persist.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}

	// Kafka API
	if cluster.HasKafkaApi() {
		kafkaAPI, d := generateKafkaAPI(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.KafkaAPI = kafkaAPI
	}

	// HTTP Proxy
	if cluster.HasHttpProxy() {
		httpProxy, d := generateHTTPProxy(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.HTTPProxy = httpProxy
	}

	// Schema Registry
	if cluster.HasSchemaRegistry() {
		schemaRegistry, d := generateSchemaRegistry(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.SchemaRegistry = schemaRegistry
	}

	// Redpanda Console
	if cluster.HasRedpandaConsole() {
		console, d := generateRedpandaConsole(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.RedpandaConsole = console
	}

	// Prometheus
	if cluster.HasPrometheus() {
		prometheus, d := generatePrometheus(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.Prometheus = prometheus
	}

	// Maintenance Window
	if cluster.HasMaintenanceWindowConfig() {
		window, d := generateMaintenanceWindow(cluster)
		if d.HasError() {
			resp.Diagnostics.Append(d...)
			return
		}
		persist.MaintenanceWindowConfig = window
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Helper functions to generate nested objects

func generateStateDescription(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasStateDescription() {
		return types.ObjectNull(stateDescriptionType), diags
	}
	sd := cluster.GetStateDescription()
	obj, d := types.ObjectValue(stateDescriptionType, map[string]attr.Value{
		"message": types.StringValue(sd.GetMessage()),
		"code":    types.Int32Value(sd.GetCode()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate state description object", "could not create state description object")
		return types.ObjectNull(stateDescriptionType), diags
	}
	return obj, diags
}

func generateKafkaAPI(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasKafkaApi() {
		return types.ObjectNull(kafkaAPIType), diags
	}

	kafkaAPI := cluster.GetKafkaApi()
	mtls, d := generateMTLS(kafkaAPI.GetMtls())
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}

	obj, d := types.ObjectValue(kafkaAPIType, map[string]attr.Value{
		"mtls":         mtls,
		"seed_brokers": utils.StringSliceToTypeList(kafkaAPI.GetSeedBrokers()),
	})
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}
	return obj, diags
}

func generateHTTPProxy(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasHttpProxy() {
		return types.ObjectNull(httpProxyType), diags
	}

	httpProxy := cluster.GetHttpProxy()
	mtls, d := generateMTLS(httpProxy.GetMtls())
	if d.HasError() {
		return types.ObjectNull(httpProxyType), d
	}

	obj, d := types.ObjectValue(httpProxyType, map[string]attr.Value{
		"mtls": mtls,
		"url":  types.StringValue(httpProxy.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(httpProxyType), diags
	}
	return obj, diags
}

func generateSchemaRegistry(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasSchemaRegistry() {
		return types.ObjectNull(schemaRegistryType), diags
	}

	registry := cluster.GetSchemaRegistry()
	mtls, d := generateMTLS(registry.GetMtls())
	if d.HasError() {
		return types.ObjectNull(schemaRegistryType), d
	}

	obj, d := types.ObjectValue(schemaRegistryType, map[string]attr.Value{
		"mtls": mtls,
		"url":  types.StringValue(registry.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(schemaRegistryType), diags
	}
	return obj, diags
}

func generateMTLS(mtls *controlplanev1beta2.MTLSSpec) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	if mtls == nil {
		return types.ObjectNull(mtlsType), diags
	}

	obj, d := types.ObjectValue(mtlsType, map[string]attr.Value{
		"enabled":                 types.BoolValue(mtls.GetEnabled()),
		"ca_certificates_pem":     utils.StringSliceToTypeList(mtls.GetCaCertificatesPem()),
		"principal_mapping_rules": utils.StringSliceToTypeList(mtls.GetPrincipalMappingRules()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(mtlsType), diags
	}
	return obj, diags
}

func generateRedpandaConsole(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasRedpandaConsole() {
		return types.ObjectNull(redpandaConsoleType), diags
	}

	console := cluster.GetRedpandaConsole()
	obj, d := types.ObjectValue(redpandaConsoleType, map[string]attr.Value{
		"url": types.StringValue(console.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(redpandaConsoleType), diags
	}
	return obj, diags
}

func generatePrometheus(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasPrometheus() {
		return types.ObjectNull(prometheusType), diags
	}

	prometheus := cluster.GetPrometheus()
	obj, d := types.ObjectValue(prometheusType, map[string]attr.Value{
		"url": types.StringValue(prometheus.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(prometheusType), diags
	}
	return obj, diags
}

func generateMaintenanceWindow(cluster *controlplanev1beta2.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasMaintenanceWindowConfig() {
		return types.ObjectNull(maintenanceWindowConfigType), diags
	}

	maintenance := cluster.GetMaintenanceWindowConfig()
	window := make(map[string]attr.Value)

	// Handle each possible window type
	if maintenance.HasDayHour() {
		dayHour := maintenance.GetDayHour()
		dayHourObj, d := types.ObjectValue(dayHourType, map[string]attr.Value{
			"hour_of_day": types.Int32Value(dayHour.GetHourOfDay()),
			"day_of_week": types.StringValue(dayHour.GetDayOfWeek().String()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(maintenanceWindowConfigType), diags
		}
		window["day_hour"] = dayHourObj
	}

	window["anytime"] = types.BoolValue(maintenance.HasAnytime())
	window["unspecified"] = types.BoolValue(maintenance.HasUnspecified())

	obj, d := types.ObjectValue(maintenanceWindowConfigType, window)
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(maintenanceWindowConfigType), diags
	}
	return obj, diags
}

// Schema returns the schema for the Cluster data source.
func (*DataSourceCluster) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceClusterSchema() // Reuse the schema from the resource
}

func datasourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required field to look up cluster
			"id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the cluster. ID is an output from the Create Cluster endpoint and cannot be set by the caller.",
			},

			// Computed fields returned by the cluster API
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
				Description: "Cloud provider region.",
			},
			"zones": schema.ListAttribute{
				Computed:    true,
				Description: "Zones of the cluster. Must be valid zones within the selected region. If multiple zones are used, the cluster is a multi-AZ cluster.",
				ElementType: types.StringType,
			},
			"tags": schema.MapAttribute{
				Computed:    true,
				Description: "Tags placed on cloud resources.",
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
			"allow_deletion": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether cluster deletion is allowed.",
			},

			// Kafka API configuration
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
								Computed:    true,
								ElementType: types.StringType,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
								Description: "Principal mapping rules for mTLS authentication.",
							},
						},
					},
					"seed_brokers": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "List of Kafka broker addresses.",
					},
				},
			},

			// HTTP Proxy configuration
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
								Computed:    true,
								ElementType: types.StringType,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
								Description: "Principal mapping rules for mTLS authentication.",
							},
						},
					},
					"url": schema.StringAttribute{
						Computed:    true,
						Description: "The HTTP Proxy URL.",
					},
				},
			},

			// Schema Registry configuration
			"schema_registry": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Schema Registry properties.",
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
								Computed:    true,
								ElementType: types.StringType,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
								Description: "Principal mapping rules for mTLS authentication.",
							},
						},
					},
					"url": schema.StringAttribute{
						Computed:    true,
						Description: "The Schema Registry URL.",
					},
				},
			},

			// Read Replica Cluster IDs
			"read_replica_cluster_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IDs of clusters that can create read-only topics from this cluster.",
			},

			// Service endpoints
			"redpanda_console": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Redpanda Console properties.",
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Computed:    true,
						Description: "The Redpanda Console URL.",
					},
				},
			},

			"prometheus": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Prometheus metrics endpoint properties.",
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Computed:    true,
						Description: "The Prometheus metrics endpoint URL.",
					},
				},
			},

			// Status fields
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "Current state of the cluster.",
			},
			"state_description": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Detailed state description when cluster is in a non-ready state.",
				Attributes: map[string]schema.Attribute{
					"code": schema.Int32Attribute{
						Computed:    true,
						Description: "Error code if cluster is in error state.",
					},
					"message": schema.StringAttribute{
						Computed:    true,
						Description: "Detailed error message if cluster is in error state.",
					},
				},
			},

			// Maintenance window configuration
			"maintenance_window_config": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Maintenance window configuration for the cluster.",
				Attributes: map[string]schema.Attribute{
					"day_hour": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"hour_of_day": schema.Int32Attribute{
								Computed:    true,
								Description: "Hour of day.",
							},
							"day_of_week": schema.StringAttribute{
								Computed:    true,
								Description: "Day of week.",
							},
						},
					},
					"anytime": schema.BoolAttribute{
						Computed:    true,
						Description: "If true, maintenance can occur at any time.",
					},
					"unspecified": schema.BoolAttribute{
						Computed:    true,
						Description: "If true, maintenance window is unspecified.",
					},
				},
			},
		},
		Description: "Data source for a Redpanda Cloud cluster",
	}
}
