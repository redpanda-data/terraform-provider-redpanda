package schemaregistryacl

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// ResourceSchemaRegistryACLSchema returns the schema for the SchemaRegistryACL resource.
func ResourceSchemaRegistryACLSchema(_ context.Context) schema.Schema {
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
				Description: "SASL username for Schema Registry HTTP Basic authentication. Optional: when omitted (together with password) the provider authenticates to Schema Registry using its cloud Bearer token. Supply username + password only when you need writes to be attributed to a specific SASL identity (e.g., audit / least-privilege).",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "SASL password for Schema Registry HTTP Basic authentication. Pair with username when you need writes attributed to a specific SASL identity instead of the provider's cloud Bearer token. Stored in Terraform state.",
			},
			"password_wo": schema.StringAttribute{
				Optional:           true,
				WriteOnly:          true,
				Description:        "Deprecated. The Terraform Plugin Framework does not persist write-only attributes to state, leaving the provider unable to authenticate to Schema Registry during refresh — this attribute cannot reliably manage ACLs. Use the default cloud Bearer authentication (omit username and password) or the regular `password` attribute.",
				DeprecationMessage: "password_wo cannot be used reliably with redpanda_schema_registry_acl: write-only attributes are not available at refresh time, so the provider cannot authenticate to Schema Registry during plan. Use cloud Bearer authentication (omit username + password) or the `password` attribute instead.",
			},
			"password_wo_version": schema.Int64Attribute{
				Optional:           true,
				Description:        "Deprecated. Version counter for password_wo, which is itself deprecated for this resource.",
				PlanModifiers:      []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				DeprecationMessage: "password_wo_version is paired with password_wo, which is deprecated. See the password_wo deprecation message for migration guidance.",
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
