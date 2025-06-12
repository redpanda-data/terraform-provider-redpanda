package cluster

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "Current state of the cluster.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the cluster was created.",
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

			// Cloud provider specific configurations
			"aws_private_link": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "AWS PrivateLink configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether AWS PrivateLink is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Console is connected via PrivateLink.",
					},
					"allowed_principals": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "The ARN of the principals that can access the Redpanda AWS PrivateLink Endpoint Service.",
					},
					"status": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "Current status of the PrivateLink configuration.",
						Attributes: map[string]schema.Attribute{
							"service_id": schema.StringAttribute{
								Computed:    true,
								Description: "The PrivateLink service ID.",
							},
							"service_name": schema.StringAttribute{
								Computed:    true,
								Description: "The PrivateLink service name.",
							},
							"service_state": schema.StringAttribute{
								Computed:    true,
								Description: "Current state of the PrivateLink service.",
							},
							"created_at": schema.StringAttribute{
								Computed:    true,
								Description: "When the PrivateLink service was created.",
							},
							"deleted_at": schema.StringAttribute{
								Computed:    true,
								Description: "When the PrivateLink service was deleted.",
							},
							"vpc_endpoint_connections": schema.ListNestedAttribute{
								Computed:    true,
								Description: "List of VPC endpoint connections.",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"id": schema.StringAttribute{
											Computed:    true,
											Description: "The endpoint connection ID.",
										},
										"owner": schema.StringAttribute{
											Computed:    true,
											Description: "Owner of the endpoint connection.",
										},
										"state": schema.StringAttribute{
											Computed:    true,
											Description: "State of the endpoint connection.",
										},
										"created_at": schema.StringAttribute{
											Computed:    true,
											Description: "When the endpoint connection was created.",
										},
										"connection_id": schema.StringAttribute{
											Computed:    true,
											Description: "The connection ID.",
										},
										"load_balancer_arns": schema.ListAttribute{
											Computed:    true,
											ElementType: types.StringType,
											Description: "ARNs of associated load balancers.",
										},
										"dns_entries": schema.ListNestedAttribute{
											Computed:    true,
											Description: "DNS entries for the endpoint.",
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"dns_name": schema.StringAttribute{
														Computed:    true,
														Description: "The DNS name.",
													},
													"hosted_zone_id": schema.StringAttribute{
														Computed:    true,
														Description: "The hosted zone ID.",
													},
												},
											},
										},
									},
								},
							},
							"kafka_api_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Kafka API seed brokers.",
							},
							"schema_registry_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Schema Registry.",
							},
							"redpanda_proxy_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for HTTP proxy.",
							},
							"kafka_api_node_base_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Base port for Kafka API nodes.",
							},
							"redpanda_proxy_node_base_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Base port for HTTP proxy nodes.",
							},
							"console_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Redpanda Console.",
							},
						},
					},
				},
			},
			"gcp_global_access_enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "If true, GCP global access is enabled.",
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "GCP Private Service Connect configuration.",
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
						Description: "List of consumers that are allowed to connect to Redpanda GCP PSC service attachment.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									Computed:    true,
									Description: "Either the GCP project number or its alphanumeric ID.",
								},
							},
						},
					},
					"status": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "Current status of the Private Service Connect configuration.",
						Attributes: map[string]schema.Attribute{
							"service_attachment": schema.StringAttribute{
								Computed:    true,
								Description: "The service attachment identifier.",
							},
							"created_at": schema.StringAttribute{
								Computed:    true,
								Description: "When the Private Service Connect service was created.",
							},
							"deleted_at": schema.StringAttribute{
								Computed:    true,
								Description: "When the Private Service Connect service was deleted.",
							},
							"kafka_api_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Kafka API seed brokers.",
							},
							"schema_registry_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Schema Registry.",
							},
							"redpanda_proxy_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for HTTP proxy.",
							},
							"kafka_api_node_base_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Base port for Kafka API nodes.",
							},
							"redpanda_proxy_node_base_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Base port for HTTP proxy nodes.",
							},
							"connected_endpoints": schema.ListNestedAttribute{
								Computed:    true,
								Description: "List of connected endpoints.",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"connection_id": schema.StringAttribute{
											Computed:    true,
											Description: "The connection ID.",
										},
										"consumer_network": schema.StringAttribute{
											Computed:    true,
											Description: "The consumer network.",
										},
										"endpoint": schema.StringAttribute{
											Computed:    true,
											Description: "The endpoint address.",
										},
										"status": schema.StringAttribute{
											Computed:    true,
											Description: "Status of the endpoint connection.",
										},
									},
								},
							},
							"dns_a_records": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
								Description: "DNS A records for the service.",
							},
							"seed_hostname": schema.StringAttribute{
								Computed:    true,
								Description: "Hostname for the seed brokers.",
							},
						},
					},
				},
			},

			"azure_private_link": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Azure Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Redpanda Azure Private Link Endpoint Service is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Console is connected in Redpanda Azure Private Link Service.",
					},
					"allowed_subscriptions": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "The subscriptions that can access the Redpanda Azure PrivateLink Endpoint Service.",
					},
					"status": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "Current status of the Private Link configuration.",
						Attributes: map[string]schema.Attribute{
							"service_id": schema.StringAttribute{
								Computed:    true,
								Description: "The Private Link service ID.",
							},
							"service_name": schema.StringAttribute{
								Computed:    true,
								Description: "The Private Link service name.",
							},
							"created_at": schema.StringAttribute{
								Computed:    true,
								Description: "When the Private Link service was created.",
							},
							"deleted_at": schema.StringAttribute{
								Computed:    true,
								Description: "When the Private Link service was deleted.",
							},
							"private_endpoint_connections": schema.ListNestedAttribute{
								Computed:    true,
								Description: "List of private endpoint connections.",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"private_endpoint_name": schema.StringAttribute{
											Computed:    true,
											Description: "Name of the private endpoint.",
										},
										"private_endpoint_id": schema.StringAttribute{
											Computed:    true,
											Description: "ID of the private endpoint.",
										},
										"connection_name": schema.StringAttribute{
											Computed:    true,
											Description: "Name of the connection.",
										},
										"connection_id": schema.StringAttribute{
											Computed:    true,
											Description: "ID of the connection.",
										},
										"status": schema.StringAttribute{
											Computed:    true,
											Description: "Status of the endpoint connection.",
										},
										"created_at": schema.StringAttribute{
											Computed:    true,
											Description: "When the endpoint connection was created.",
										},
									},
								},
							},
							"dns_a_record": schema.StringAttribute{
								Computed:    true,
								Description: "DNS A record for the service.",
							},
							"approved_subscriptions": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
								Description: "List of approved Azure subscription IDs.",
							},
							"kafka_api_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Kafka API seed brokers.",
							},
							"schema_registry_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Schema Registry.",
							},
							"redpanda_proxy_seed_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for HTTP proxy.",
							},
							"kafka_api_node_base_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Base port for Kafka API nodes.",
							},
							"redpanda_proxy_node_base_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Base port for HTTP proxy nodes.",
							},
							"console_port": schema.Int32Attribute{
								Computed:    true,
								Description: "Port for Redpanda Console.",
							},
						},
					},
				},
			},
			// Kafka Connect configuration
			"kafka_connect": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Kafka Connect configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether Kafka Connect is enabled.",
					},
				},
			},

			// Customer managed resources
			"customer_managed_resources": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Customer managed resources configuration for the cluster.",
				Attributes: map[string]schema.Attribute{
					"aws": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"agent_instance_profile": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the agent instance profile",
									},
								},
							},
							"connectors_node_group_instance_profile": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the connectors node group instance profile",
									},
								},
							},
							"utility_node_group_instance_profile": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the utility node group instance profile",
									},
								},
							},
							"redpanda_node_group_instance_profile": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the redpanda node group instance profile",
									},
								},
							},
							"k8s_cluster_role": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the Kubernetes cluster role",
									},
								},
							},
							"redpanda_agent_security_group": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the redpanda agent security group",
									},
								},
							},
							"connectors_security_group": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the connectors security group",
									},
								},
							},
							"redpanda_node_group_security_group": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the redpanda node group security group",
									},
								},
							},
							"utility_security_group": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the utility security group",
									},
								},
							},
							"cluster_security_group": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the cluster security group",
									},
								},
							},
							"node_security_group": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the node security group",
									},
								},
							},
							"cloud_storage_bucket": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the cloud storage bucket",
									},
								},
							},
							"permissions_boundary_policy": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Computed:    true,
										Description: "ARN for the permissions boundary policy",
									},
								},
							},
						},
					},
					"gcp": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"subnet": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP subnet where Redpanda cluster is deployed.",
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Computed:    true,
										Description: "Subnet name.",
									},
									"secondary_ipv4_range_pods": schema.SingleNestedAttribute{
										Computed:    true,
										Description: "Secondary IPv4 range for pods.",
										Attributes: map[string]schema.Attribute{
											"name": schema.StringAttribute{
												Computed:    true,
												Description: "Secondary IPv4 range name for pods.",
											},
										},
									},
									"secondary_ipv4_range_services": schema.SingleNestedAttribute{
										Computed:    true,
										Description: "Secondary IPv4 range for services.",
										Attributes: map[string]schema.Attribute{
											"name": schema.StringAttribute{
												Computed:    true,
												Description: "Secondary IPv4 range name for services.",
											},
										},
									},
									"k8s_master_ipv4_range": schema.StringAttribute{
										Computed:    true,
										Description: "Kubernetes Master IPv4 range, e.g. 10.0.0.0/24.",
									},
								},
							},
							"agent_service_account": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP service account for the agent.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Computed:    true,
										Description: "GCP service account email.",
									},
								},
							},
							"console_service_account": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP service account for Redpanda Console.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Computed:    true,
										Description: "GCP service account email.",
									},
								},
							},
							"connector_service_account": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP service account for managed connectors.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Computed:    true,
										Description: "GCP service account email.",
									},
								},
							},
							"redpanda_cluster_service_account": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP service account for the Redpanda cluster.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Computed:    true,
										Description: "GCP service account email.",
									},
								},
							},
							"gke_service_account": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP service account for GCP Kubernetes Engine (GKE).",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Computed:    true,
										Description: "GCP service account email.",
									},
								},
							},
							"tiered_storage_bucket": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "GCP storage bucket for Tiered storage.",
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Computed:    true,
										Description: "GCP storage bucket name.",
									},
								},
							},
							"psc_nat_subnet_name": schema.StringAttribute{
								Computed:    true,
								Description: "NAT subnet name if GCP Private Service Connect is enabled.",
							},
						},
					},
				},
			},

			"cluster_configuration": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Configuration for the cluster.",
				Attributes: map[string]schema.Attribute{
					"custom_properties_json": schema.StringAttribute{
						Computed:    true,
						Description: "Custom properties for the cluster in JSON format.",
					},
					"computed_properties_json": schema.StringAttribute{
						Computed:    true,
						Description: "Computed properties for the cluster in JSON format.",
					},
				},
			},
		},
		Description: "Data source for a Redpanda Cloud cluster",
	}
}
