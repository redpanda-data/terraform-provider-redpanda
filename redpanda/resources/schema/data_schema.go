package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	schemamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schema"
	"github.com/twmb/franz-go/pkg/sr"
	"golang.org/x/oauth2"
)

var (
	_ datasource.DataSource              = &SchemaDataSource{}
	_ datasource.DataSourceWithConfigure = &SchemaDataSource{}
)

// SchemaDataSource reads schema information from the Schema Registry.
//
//nolint:revive // SchemaDataSource stutters (schema.SchemaDataSource) but matches the resource naming convention (schema.Schema).
type SchemaDataSource struct {
	base.DataSourceBase
	dsData        config.Datasource
	clientFactory func(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID string, ts oauth2.TokenSource, username, password string) (SRClienter, error)
}

// NewSchemaDataSource constructs a Schema datasource.
func NewSchemaDataSource() *SchemaDataSource {
	d := &SchemaDataSource{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_schema", DatasourceSchemaSchema, func(p config.Datasource) {
		d.dsData = p
	})
	return d
}

// DatasourceSchemaSchema returns the schema for the Schema datasource.
func DatasourceSchemaSchema(_ context.Context) schema.Schema {
	return schema.Schema{
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
			"username": schema.StringAttribute{
				Description: "SASL username for Schema Registry HTTP Basic authentication. Optional: when omitted (together with password) the provider authenticates using its cloud Bearer token.",
				Optional:    true,
				Sensitive:   true,
			},
			"password": schema.StringAttribute{
				Description: "SASL password for Schema Registry HTTP Basic authentication. Pair with username when Basic auth is required instead of the cloud Bearer token.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

// getClient returns an SR client using the factory when set, else the default implementation.
func (d *SchemaDataSource) getClient(ctx context.Context, clusterID, username, password string) (SRClienter, error) {
	if d.clientFactory != nil {
		return d.clientFactory(ctx, d.CpCl, clusterID, d.dsData.TokenSource, username, password)
	}
	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, d.CpCl, clusterID, d.dsData.TokenSource, username, password)
	if err != nil {
		return nil, err
	}
	return newSchemaRegistryClientWrapper(client), nil
}

// Read fetches schema data from the Schema Registry.
func (d *SchemaDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var cfg schemamodel.DataModel
	response.Diagnostics.Append(request.Config.Get(ctx, &cfg)...)
	if response.Diagnostics.HasError() {
		return
	}

	clusterID := cfg.ClusterID.ValueString()
	subject := cfg.Subject.ValueString()

	client, err := d.getClient(ctx, clusterID, cfg.Username.ValueString(), cfg.Password.ValueString())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", clusterID, err),
		)
		return
	}

	var version *int
	if !cfg.Version.IsNull() && !cfg.Version.IsUnknown() {
		v := int(cfg.Version.ValueInt64())
		version = &v
	}

	sub, err := fetchSchema(ctx, client, subject, version)
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to read schema",
			fmt.Sprintf("Unable to read schema for subject %s: %v", subject, err),
		)
		return
	}

	cfg.ID = types.Int64Value(int64(sub.ID))
	cfg.Version = types.Int64Value(int64(sub.Version))
	cfg.Schema = types.StringValue(sub.Schema.Schema)
	cfg.SchemaType = types.StringValue(strings.ToUpper(sub.Type.String()))
	refsList, refDiags := convertRefsToList(sub.References)
	response.Diagnostics.Append(refDiags...)
	cfg.References = refsList

	response.Diagnostics.Append(response.State.Set(ctx, &cfg)...)
}

// convertRefsToList converts sr.SchemaReference slice to a Terraform types.List.
func convertRefsToList(refs []sr.SchemaReference) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := map[string]attr.Type{
		"name":    types.StringType,
		"subject": types.StringType,
		"version": types.Int64Type,
	}
	if len(refs) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: attrTypes}), diags
	}
	elems := make([]attr.Value, 0, len(refs))
	for _, ref := range refs {
		obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
			"name":    types.StringValue(ref.Name),
			"subject": types.StringValue(ref.Subject),
			"version": types.Int64Value(int64(ref.Version)),
		})
		diags.Append(d...)
		elems = append(elems, obj)
	}
	list, d := types.ListValue(types.ObjectType{AttrTypes: attrTypes}, elems)
	diags.Append(d...)
	return list, diags
}
