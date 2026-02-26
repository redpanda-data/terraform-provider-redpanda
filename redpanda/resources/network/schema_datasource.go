package network

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DatasourceNetworkSchema returns the schema for the network data source.
func DatasourceNetworkSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the network",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the network",
			},
			"cidr_block": schema.StringAttribute{
				Computed:    true,
				Description: "The cidr_block to create the network in",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}/(\d{1,2})$`),
						"The value must be a valid CIDR block (e.g., 192.168.0.0/16)",
					),
				},
			},
			"region": schema.StringAttribute{
				Computed:    true,
				Description: "The region to create the network in. Can also be set at the provider level",
			},
			"cloud_provider": schema.StringAttribute{
				Computed:    true,
				Description: "The cloud provider to create the network in. Can also be set at the provider level",
				Validators: []validator.String{
					stringvalidator.OneOf("gcp", "aws"),
				},
			},
			"resource_group_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the resource group in which to create the network",
			},
			"cluster_type": schema.StringAttribute{
				Computed:    true,
				Description: "The type of cluster this network is associated with, can be one of dedicated or cloud",
				Validators: []validator.String{
					stringvalidator.OneOf("dedicated", "cloud"),
				},
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "Current state of the network.",
			},
			"zones": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Network availability zones.",
			},
			"customer_managed_resources": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"gcp": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"network_name": schema.StringAttribute{
								Computed:    true,
								Description: "Name of user-created network where the Redpanda cluster is deployed",
							},
							"network_project_id": schema.StringAttribute{
								Computed:    true,
								Description: "GCP project ID where the network is created",
							},
							"management_bucket": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:    true,
										Description: "GCP storage bucket name for storing the state of Redpanda cluster deployment",
									},
								},
							},
						},
					},
					"aws": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"management_bucket": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS storage bucket identifier",
									},
								},
							},
							"dynamodb_table": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS DynamoDB table identifier",
									},
								},
							},
							"vpc": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS VPC identifier",
									},
								},
							},
							"private_subnets": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"arns": schema.ListAttribute{
										Required:    true,
										ElementType: types.StringType,
										Description: "AWS private subnet identifiers",
									},
								},
							},
						},
					},
				},
			},
		},
		Description: "Data source for a Redpanda Cloud network",
	}
}
