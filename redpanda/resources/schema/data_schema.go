package schema

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	schemamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schema"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &SchemaDataSource{}
	_ datasource.DataSourceWithConfigure = &SchemaDataSource{}
)

//nolint:revive // SchemaDataSource is the correct name for this type
type SchemaDataSource struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Configure configures the schema data source.
func (d *SchemaDataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	p, ok := request.ProviderData.(config.Datasource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected config.Datasource, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}

	d.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Metadata returns the data source metadata.
func (*SchemaDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_schema"
}

// Schema returns the data source schema.
func (*SchemaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Schema data source allows you to retrieve information about a Schema Registry schema",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Description: "The ID of the cluster where the schema is stored.",
				Required:    true,
			},
			"subject": schema.StringAttribute{
				Description: "The subject name for the schema.",
				Required:    true,
			},
			"version": schema.Int64Attribute{
				Description: "The version of the schema. If not specified, the latest version is used.",
				Optional:    true,
				Computed:    true,
			},
			"schema": schema.StringAttribute{
				Description: "The schema definition in JSON format.",
				Computed:    true,
			},
			"schema_type": schema.StringAttribute{
				Description: "The type of schema (AVRO, JSON, PROTOBUF).",
				Computed:    true,
			},
			"id": schema.Int64Attribute{
				Description: "The unique identifier for the schema.",
				Computed:    true,
			},
			"references": schema.ListNestedAttribute{
				Description: "List of schema references.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the referenced schema.",
							Computed:    true,
						},
						"subject": schema.StringAttribute{
							Description: "The subject of the referenced schema.",
							Computed:    true,
						},
						"version": schema.Int64Attribute{
							Description: "The version of the referenced schema.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

// Read reads the schema data.
func (*SchemaDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var config schemamodel.DataModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	// TODO: Implement schema reading via Schema Registry API
	response.Diagnostics.AddError(
		"Schema data source not yet implemented",
		"The schema data source requires integration with Schema Registry which is not yet implemented.",
	)
}
