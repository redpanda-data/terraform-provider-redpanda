package redpanda

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/namespace"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ provider.Provider = &Redpanda{}

type Redpanda struct {
	version string
}

// New spawns a basic provider struct, no client. Configure must be called for a working client
func New(ctx context.Context, version string) func() provider.Provider {
	// TODO consider whether the below should support a mock flow with a ci switch? since configure is primary TF entrypoint anyway
	return func() provider.Provider {
		return &Redpanda{
			version: version,
		}
	}
}

func ProviderSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"auth_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Redpanda Auth token. You need either (client_id AND client_secret) OR auth_token to use this provider",
				Validators:  nil, // TODO consider writing validator for bearer token. while the api can always just handle it, it might be smart to preempt it for the simple stuff
			},
			"client_id": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "The id for the client. You need either (client_id AND client_secret) OR auth_token to use this provider",
			},
			"client_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Redpanda client secret. You need either (client_id AND client_secret) OR auth_token to use this provider",
			},
			"cloud_provider": schema.StringAttribute{
				Optional:    true,
				Description: "Which supported cloud provider you are using (GCP, AWS). Can also be specified per resource",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "Cloud provider regions for the clusters you wish to build. Can also be specified per resource",
			},
			"zones": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Cloud provider zones for the clusters you wish to build. Can also be specified per resource",
			},
		},
		Description: "Redpanda Data terraform provider",
	}
}

// Configure is the primary entrypoint for terraform and properly initializes the client
func (r *Redpanda) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var conf models.Redpanda
	response.Diagnostics.Append(request.Config.Get(ctx, &conf)...)
	if response.Diagnostics.HasError() {
		return
	}

	// Clients are passed through to downstream resources through the response struct
	response.ResourceData = utils.ResourceData{
		ClientID:     conf.ClientID.ValueString(),
		ClientSecret: conf.ClientSecret.ValueString(),
		AuthToken:    conf.AuthToken.ValueString(),
		Version:      r.version,
	}
	response.DataSourceData = utils.DatasourceData{
		ClientID:     conf.ClientID.ValueString(),
		ClientSecret: conf.ClientSecret.ValueString(),
		AuthToken:    conf.AuthToken.ValueString(),
		Version:      r.version,
	}
}

func (r *Redpanda) Metadata(ctx context.Context, request provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "redpanda"
	response.Version = r.version
}

func (r *Redpanda) Schema(ctx context.Context, request provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = ProviderSchema()
}

func (r *Redpanda) DataSources(ctx context.Context) []func() datasource.DataSource {
	// TODO implement
	return []func() datasource.DataSource{}
}

func (r *Redpanda) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			return &namespace.Namespace{}
		},
		func() resource.Resource {
			return &network.Network{}
		},
		func() resource.Resource {
			return &cluster.Cluster{}
		},
		// TODO implement remaining resources
	}
}
