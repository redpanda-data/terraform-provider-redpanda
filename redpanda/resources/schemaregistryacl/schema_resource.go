package schemaregistryacl

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// ResourceSchemaRegistryACLSchema returns the schema for the SchemaRegistryACL resource.
func ResourceSchemaRegistryACLSchema() schema.Schema {
	return schema.Schema{
		Description: "Resource for managing Redpanda Schema Registry ACLs (Access Control Lists). " +
			"This resource allows you to configure fine-grained access control for Schema Registry resources.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the cluster where the Schema Registry ACL will be created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal": schema.StringAttribute{
				Required:    true,
				Description: "The principal to apply this ACL for (e.g., User:alice or RedpandaRole:admin)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the resource: SUBJECT or REGISTRY",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.SchemaRegistryResourceType(),
				},
			},
			"resource_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the resource this ACL entry will be on. Use '*' for wildcard",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pattern_type": schema.StringAttribute{
				Required:    true,
				Description: "The pattern type of the resource: LITERAL or PREFIXED",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.SchemaRegistryPatternType(),
				},
			},
			"host": schema.StringAttribute{
				Required:    true,
				Description: "The host address to use for this ACL. Use '*' for wildcard",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"operation": schema.StringAttribute{
				Required:    true,
				Description: "The operation type that shall be allowed or denied: ALL, READ, WRITE, DELETE, DESCRIBE, DESCRIBE_CONFIGS, ALTER, ALTER_CONFIGS",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.SchemaRegistryOperation(),
				},
			},
			"permission": schema.StringAttribute{
				Required:    true,
				Description: "The permission type: ALLOW or DENY",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.SchemaRegistryPermission(),
				},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Username for authentication. Can be set via REDPANDA_SR_USERNAME environment variable",
			},
			"password": schema.StringAttribute{
				Optional:           true,
				Sensitive:          true,
				Description:        "Password for authentication. Deprecated: use password_wo instead. Can be set via REDPANDA_SR_PASSWORD environment variable",
				DeprecationMessage: "Use password_wo instead to avoid storing password in Terraform state",
				Validators: []validator.String{
					validators.Password(
						path.MatchRoot("password"),
						path.MatchRoot("password_wo"),
					),
				},
			},
			"password_wo": schema.StringAttribute{
				Optional:    true,
				WriteOnly:   true,
				Description: "Password for authentication (write-only, not stored in state). Requires Terraform 1.11+. Can be set via REDPANDA_SR_PASSWORD environment variable",
			},
			"password_wo_version": schema.Int64Attribute{
				Optional:      true,
				Description:   "Version number for password_wo. Increment this value to trigger a password update when using password_wo.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "When set to true, allows the resource to be removed from state even if deletion fails due to permission errors",
			},
		},
	}
}
