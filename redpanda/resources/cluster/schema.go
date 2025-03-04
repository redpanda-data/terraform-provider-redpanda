package cluster

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

func resourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Base cluster attributes
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Unique name of the cluster.",
			},
			"cluster_type": schema.StringAttribute{
				Required:      true,
				Description:   "Cluster type. Type is immutable and can only be set on cluster creation.",
				Validators:    validators.ClusterTypes(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connection_type": schema.StringAttribute{
				Required:    true,
				Description: "Cluster connection type. Private clusters are not exposed to the internet. For BYOC clusters, Private is best-practice.",
				Validators: []validator.String{
					validators.ConnectionTypes(),
					validators.RequirePrivateConnectionValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud_provider": schema.StringAttribute{
				Optional:      true,
				Description:   "Cloud provider where resources are created.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    validators.CloudProviders(),
			},
			"redpanda_version": schema.StringAttribute{
				Optional:      true,
				Description:   "Current Redpanda version of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"throughput_tier": schema.StringAttribute{
				Required:      true,
				Description:   "Throughput tier of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				Optional:      true,
				Description:   "Cloud provider region. Region represents the name of the region where the cluster will be provisioned.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"zones": schema.ListAttribute{
				Optional:      true,
				Description:   "Zones of the cluster. Must be valid zones within the selected region. If multiple zones are used, the cluster is a multi-AZ cluster.",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Description: "Allows deletion of the cluster. Defaults to true. Should probably be set to false for production use.",
			},
			"tags": schema.MapAttribute{
				Optional:      true,
				Description:   "Tags placed on cloud resources. If the cloud provider is GCP and the name of a tag has the prefix \"gcp.network-tag.\", the tag is a network tag that will be added to the Redpanda cluster GKE nodes. Otherwise, the tag is a normal tag. For example, if the name of a tag is \"gcp.network-tag.network-tag-foo\", the network tag named \"network-tag-foo\" will be added to the Redpanda cluster GKE nodes. Note: The value of a network tag will be ignored. See the details on network tags at https://cloud.google.com/vpc/docs/add-remove-network-tags.",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "Resource group ID of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"network_id": schema.StringAttribute{
				Required:      true,
				Description:   "Network ID where cluster is placed.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "ID of the cluster. ID is an output from the Create Cluster endpoint and cannot be set by the caller.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				Computed:      true,
				Description:   "Current state of the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Description:   "Timestamp when the cluster was created.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:      true,
				Description:   "The URL of the cluster API.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"kafka_api": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Cluster's Kafka API properties.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "mTLS configuration.",
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:      true,
								Computed:      true,
								Default:       booldefault.StaticBool(false),
								Description:   "Whether mTLS is enabled.",
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType:   types.StringType,
								Optional:      true,
								Description:   "CA certificate in PEM format.",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType:   types.StringType,
								Optional:      true,
								Description:   "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
						},
					},
					"seed_brokers": schema.ListAttribute{
						ElementType:   types.StringType,
						Computed:      true,
						Description:   "List of Kafka broker addresses.",
						PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
					},
				},
			},
			"http_proxy": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "HTTP Proxy properties.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "mTLS configuration.",
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:      true,
								Computed:      true,
								Default:       booldefault.StaticBool(false),
								Description:   "Whether mTLS is enabled.",
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType:   types.StringType,
								Optional:      true,
								Description:   "CA certificate in PEM format.",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType:   types.StringType,
								Optional:      true,
								Description:   "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
						},
					},
					"url": schema.StringAttribute{
						Computed:      true,
						Description:   "The HTTP Proxy URL.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},
			"schema_registry": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Schema Registry properties.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Description:   "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:      true,
								Computed:      true,
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
								Default:       booldefault.StaticBool(false),
								Description:   "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType:   types.StringType,
								Optional:      true,
								Description:   "CA certificate in PEM format.",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType:   types.StringType,
								Optional:      true,
								Description:   "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
								PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
							},
						},
					},
					"url": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						Description:   "The Schema Registry URL.",
					},
				},
			},

			"redpanda_console": schema.SingleNestedAttribute{
				Computed:      true,
				Description:   "Redpanda Console properties.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						Description:   "The Redpanda Console URL.",
					},
				},
			},

			"prometheus": schema.SingleNestedAttribute{
				Computed:      true,
				Description:   "Prometheus metrics endpoint properties.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						Description:   "The Prometheus metrics endpoint URL.",
					},
				},
			},
			"state_description": schema.SingleNestedAttribute{
				Computed:      true,
				Description:   "Detailed state description when cluster is in a non-ready state.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"code": schema.Int32Attribute{
						Computed:      true,
						Description:   "Error code if cluster is in error state.",
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
					},
					"message": schema.StringAttribute{
						Computed:      true,
						Description:   "Detailed error message if cluster is in error state.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},

			"read_replica_cluster_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "IDs of clusters that can create read-only topics from this cluster.",
			},

			"maintenance_window_config": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maintenance window configuration for the cluster.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"day_hour": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"hour_of_day": schema.Int32Attribute{
								Optional:      true,
								Description:   "Hour of day.",
								PlanModifiers: []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
							},
							"day_of_week": schema.StringAttribute{
								Optional:      true,
								Description:   "Day of week.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
						},
					},
					"anytime": schema.BoolAttribute{
						Optional:      true,
						Description:   "If true, maintenance can occur at any time.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"unspecified": schema.BoolAttribute{
						Computed:      true,
						Description:   "If true, maintenance window is unspecified.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
				},
			},

			"connectivity": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Cloud provider-specific connectivity configuration.",
				Attributes: map[string]schema.Attribute{
					"gcp": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "GCP-specific connectivity settings.",
						Attributes: map[string]schema.Attribute{
							"enable_global_access": schema.BoolAttribute{
								Required:    true,
								Description: "Whether global access is enabled.",
							},
						},
					},
				},
			},
			"kafka_connect": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "Kafka Connect configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
						Default:       booldefault.StaticBool(false),
						Description:   "Whether Kafka Connect is enabled.",
					},
				},
			},
			// Cloud provider specific configurations
			"aws_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "AWS PrivateLink configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether AWS PrivateLink is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Console is connected via PrivateLink.",
					},
					"allowed_principals": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The ARN of the principals that can access the Redpanda AWS PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
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
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "aws_private_link",
						CloudProvider: "aws",
					},
				},
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "GCP Private Service Connect configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda GCP Private Service Connect is enabled.",
					},
					"global_access_enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether global access is enabled.",
					},
					"consumer_accept_list": schema.ListNestedAttribute{
						Required:    true,
						Description: "List of consumers that are allowed to connect to Redpanda GCP PSC (Private Service Connect) service attachment.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									Required:    true,
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
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "gcp_private_service_connect",
						CloudProvider: "gcp",
					},
				},
			},
			"azure_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Azure Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda Azure Private Link Endpoint Service is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Console is connected in Redpanda Azure Private Link Service.",
					},
					"allowed_subscriptions": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The subscriptions that can access the Redpanda Azure PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
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
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "azure_private_link",
						CloudProvider: "azure",
					},
				},
			},
			"customer_managed_resources": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Customer managed resources configuration for the cluster.",
				Attributes: map[string]schema.Attribute{
					"aws": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
						Attributes: map[string]schema.Attribute{
							"agent_instance_profile": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the agent instance profile",
									},
								},
							},
							"connectors_node_group_instance_profile": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the connectors node group instance profile",
									},
								},
							},
							"utility_node_group_instance_profile": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the utility node group instance profile",
									},
								},
							},
							"redpanda_node_group_instance_profile": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the redpanda node group instance profile",
									},
								},
							},
							"k8s_cluster_role": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the Kubernetes cluster role",
									},
								},
							},
							"redpanda_agent_security_group": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the redpanda agent security group",
									},
								},
							},
							"connectors_security_group": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the connectors security group",
									},
								},
							},
							"redpanda_node_group_security_group": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the redpanda node group security group",
									},
								},
							},
							"utility_security_group": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the utility security group",
									},
								},
							},
							"cluster_security_group": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the cluster security group",
									},
								},
							},
							"node_security_group": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the node security group",
									},
								},
							},
							"cloud_storage_bucket": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the cloud storage bucket",
									},
								},
							},
							"permissions_boundary_policy": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "ARN for the permissions boundary policy",
									},
								},
							},
						},
					},
					"gcp": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
						Attributes: map[string]schema.Attribute{
							"subnet": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP subnet where Redpanda cluster is deployed.",
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:      true,
										Description:   "Subnet name.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
										Validators: []validator.String{
											stringvalidator.RegexMatches(
												// this regex is directly from the proto spec for these fields
												regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
												"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
											),
										},
									},
									"secondary_ipv4_range_pods": schema.SingleNestedAttribute{
										Required:    true,
										Description: "Secondary IPv4 range for pods.",
										Attributes: map[string]schema.Attribute{
											"name": schema.StringAttribute{
												Required:      true,
												Description:   "Secondary IPv4 range name for pods.",
												PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
												Validators: []validator.String{
													stringvalidator.RegexMatches(
														regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
														"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
													),
												},
											},
										},
									},
									"secondary_ipv4_range_services": schema.SingleNestedAttribute{
										Required:    true,
										Description: "Secondary IPv4 range for services.",
										Attributes: map[string]schema.Attribute{
											"name": schema.StringAttribute{
												Required:      true,
												Description:   "Secondary IPv4 range name for services.",
												PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
												Validators: []validator.String{
													stringvalidator.RegexMatches(
														regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
														"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
													),
												},
											},
										},
									},
									"k8s_master_ipv4_range": schema.StringAttribute{
										Required:      true,
										Description:   "Kubernetes Master IPv4 range, e.g. 10.0.0.0/24.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
									},
								},
							},
							"agent_service_account": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP service account for the agent.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Required:      true,
										Description:   "GCP service account email.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
										Validators: []validator.String{
											validators.EmailValidator{},
										},
									},
								},
							},
							"console_service_account": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP service account for Redpanda Console.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Required:      true,
										Description:   "GCP service account email.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
										Validators: []validator.String{
											validators.EmailValidator{},
										},
									},
								},
							},
							"connector_service_account": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP service account for managed connectors.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Required:      true,
										Description:   "GCP service account email.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
										Validators: []validator.String{
											validators.EmailValidator{},
										},
									},
								},
							},
							"redpanda_cluster_service_account": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP service account for the Redpanda cluster.",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Required:      true,
										Description:   "GCP service account email.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
										Validators: []validator.String{
											validators.EmailValidator{},
										},
									},
								},
							},
							"gke_service_account": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP service account for GCP Kubernetes Engine (GKE).",
								Attributes: map[string]schema.Attribute{
									"email": schema.StringAttribute{
										Required:      true,
										Description:   "GCP service account email.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
										Validators: []validator.String{
											validators.EmailValidator{},
										},
									},
								},
							},
							"tiered_storage_bucket": schema.SingleNestedAttribute{
								Required:    true,
								Description: "GCP storage bucket for Tiered storage.",
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:      true,
										Description:   "GCP storage bucket name.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
									},
								},
							},
							"psc_nat_subnet_name": schema.StringAttribute{
								Optional:      true,
								Description:   "NAT subnet name if GCP Private Service Connect is enabled.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
							},
						},
					},
				},
			},
		},
	}
}
