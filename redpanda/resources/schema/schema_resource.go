package schema

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchemaSchema() schema.Schema {
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
				Description: "The SASL username for Schema Registry authentication.",
				Required:    true,
			},
			"password": schema.StringAttribute{
				Description: "The SASL password for Schema Registry authentication.",
				Required:    true,
				Sensitive:   true,
			},
		},
	}
}
