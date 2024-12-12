package cluster

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

func resourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
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
				Required:      true,
				Description:   "Cluster connection type. Private clusters are not exposed to the internet. For BYOC clusters, Private is best-practice.",
				Validators:    validators.ConnectionTypes(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
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
			"cluster_api_url": schema.StringAttribute{
				Computed:      true,
				Description:   "The URL of the cluster API.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"aws_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The AWS Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda AWS Private Link Endpoint Service is enabled.",
					},
					"connect_console": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Console is connected in Redpanda AWS Private Link Service.",
					},
					"allowed_principals": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The ARN of the principals that can access the Redpanda AWS PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
					},
				},
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "aws_private_link",
						CloudProvider: "aws",
					},
				},
			},
			"azure_private_link": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The Azure Private Link configuration.",
				Attributes: map[string]schema.Attribute{
					"allowed_subscriptions": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
						Description: "The subscriptions that can access the Redpanda Azure PrivateLink Endpoint Service. To grant permissions to all principals, use an asterisk (*).",
					},
					"connect_console": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Console is connected in Redpanda Azure Private Link Service.",
					},
					"enabled": schema.BoolAttribute{
						Required:    true,
						Description: "Whether Redpanda Azure Private Link Endpoint Service is enabled.",
					},
				},
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "azure_private_link",
						CloudProvider: "azure",
					},
				},
			},
			"gcp_private_service_connect": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The GCP Private Service Connect configuration.",
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
				},
				Validators: []validator.Object{
					validators.CloudProviderDependentValidator{
						AttributeName: "gcp_private_service_connect",
						CloudProvider: "gcp",
					},
				},
			},
			"kafka_api": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Cluster's Kafka API properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"http_proxy": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "HTTP Proxy properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"schema_registry": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Cluster's Schema Registry properties.",
				Attributes: map[string]schema.Attribute{
					"mtls": schema.SingleNestedAttribute{
						Required:    true,
						Description: "mTLS configuration.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:    true,
								Description: "Whether mTLS is enabled.",
							},
							"ca_certificates_pem": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "CA certificate in PEM format.",
							},
							"principal_mapping_rules": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Description: "Principal mapping rules for mTLS authentication. See the Redpanda documentation on configuring authentication.",
							},
						},
					},
				},
			},
			"read_replica_cluster_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "IDs of clusters which may create read-only topics from this cluster.",
			},
		},
	}
}
