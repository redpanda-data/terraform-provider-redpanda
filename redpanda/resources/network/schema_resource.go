package network

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

func resourceNetworkSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cidr_block": schema.StringAttribute{
				Optional:      true,
				Description:   "The cidr_block to create the network in",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					validators.CIDRBlockValidator{},
				},
			},
			"region": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "The region to create the network in.",
			},
			"cloud_provider": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "The cloud provider to create the network in.",
				Validators:    validators.CloudProviders(),
			},
			"resource_group_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the resource group in which to create the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the network",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cluster_type": schema.StringAttribute{
				Required:      true,
				Description:   "The type of cluster this network is associated with, can be one of dedicated or byoc",
				Validators:    validators.ClusterTypes(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"customer_managed_resources": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"gcp": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
						Attributes: map[string]schema.Attribute{
							"network_name": schema.StringAttribute{
								Required:    true,
								Description: "Name of user-created network where the Redpanda cluster is deployed",
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
										"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
									),
									stringvalidator.LengthAtMost(62),
								},
							},
							"network_project_id": schema.StringAttribute{
								Required:    true,
								Description: "GCP project ID where the network is created",
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
										"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number",
									),
									stringvalidator.LengthAtMost(30),
								},
							},
							"management_bucket": schema.SingleNestedAttribute{
								Required: true,
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
						Optional:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
						Attributes: map[string]schema.Attribute{
							"management_bucket": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS storage bucket identifier",
									},
								},
							},
							"dynamodb_table": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS DynamoDB table identifier",
									},
								},
							},
							"vpc": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"arn": schema.StringAttribute{
										Required:    true,
										Description: "AWS VPC identifier",
									},
								},
							},
							"private_subnets": schema.SingleNestedAttribute{
								Required: true,
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
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}
