package schema

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// ResourceSchemaSchema returns the schema for the schema resource.
func ResourceSchemaSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Description: "Schema represents a Schema Registry schema",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Description:   "The ID of the cluster where the schema is stored.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"subject": schema.StringAttribute{
				Description:   "The subject name for the schema.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"schema": schema.StringAttribute{
				Description: "The schema definition in JSON format.",
				Required:    true,
			},
			"schema_type": schema.StringAttribute{
				Description: "The type of schema (AVRO, JSON, PROTOBUF).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.Int64Attribute{
				Description: "The version of the schema.",
				Computed:    true,
			},
			"id": schema.Int64Attribute{
				Description: "The unique identifier for the schema.",
				Computed:    true,
			},
			"references": schema.ListNestedAttribute{
				Description: "List of schema references.",
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the referenced schema.",
							Required:    true,
						},
						"subject": schema.StringAttribute{
							Description: "The subject of the referenced schema.",
							Required:    true,
						},
						"version": schema.Int64Attribute{
							Description: "The version of the referenced schema.",
							Required:    true,
						},
					},
				},
			},
			"username": schema.StringAttribute{
				Description: "SASL username for Schema Registry HTTP Basic authentication. Optional: when omitted (together with password) the provider authenticates to Schema Registry using its cloud Bearer token. Supply username + password only when you need writes to be attributed to a specific SASL identity (e.g., audit / least-privilege).",
				Optional:    true,
				Sensitive:   true,
			},
			"password": schema.StringAttribute{
				Description: "SASL password for Schema Registry HTTP Basic authentication. Pair with username when you need writes attributed to a specific SASL identity instead of the provider's cloud Bearer token. Stored in Terraform state.",
				Optional:    true,
				Sensitive:   true,
			},
			"password_wo": schema.StringAttribute{
				Description:        "Deprecated. The Terraform Plugin Framework does not persist write-only attributes to state, leaving the provider unable to authenticate to Schema Registry during refresh — this attribute cannot reliably manage schemas. Use the default cloud Bearer authentication (omit username and password) or the regular `password` attribute.",
				Optional:           true,
				WriteOnly:          true,
				DeprecationMessage: "password_wo cannot be used reliably with redpanda_schema: write-only attributes are not available at refresh time, so the provider cannot authenticate to Schema Registry during plan. Use cloud Bearer authentication (omit username + password) or the `password` attribute instead.",
			},
			"password_wo_version": schema.Int64Attribute{
				Description:        "Deprecated. Version counter for password_wo, which is itself deprecated for this resource.",
				Optional:           true,
				PlanModifiers:      []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				DeprecationMessage: "password_wo_version is paired with password_wo, which is deprecated. See the password_wo deprecation message for migration guidance.",
			},
			"compatibility": schema.StringAttribute{
				Description: "The compatibility level for schema evolution (BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE, NONE). Defaults to BACKWARD.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(
						"BACKWARD", "BACKWARD_TRANSITIVE",
						"FORWARD", "FORWARD_TRANSITIVE",
						"FULL", "FULL_TRANSITIVE",
						"NONE",
					),
				},
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Whether terraform may destroy this schema subject. Defaults to `false` — `terraform destroy` will refuse until you set this to `true`. After `terraform import`, defaults to `false` regardless of what was previously in state; set to `true` in your config before destroy.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}
