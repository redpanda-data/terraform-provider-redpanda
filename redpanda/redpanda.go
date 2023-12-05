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
	"os"
)

var _ provider.Provider = &Redpanda{}

type Redpanda struct {
	version string
}

// New spawns a basic provider struct, no client. Configure must be called for a working client
func New(ctx context.Context, version string) func() provider.Provider {
	return func() provider.Provider {
		return &Redpanda{
			version: version,
		}
	}
}

func ProviderSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "The id for the client. You need client_id AND client_secret to use this provider",
			},
			"client_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Redpanda client secret. You need client_id AND client_secret to use this provider",
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

	var id string
	cfgId := conf.ClientID.ValueString()
	envId := os.Getenv("CLIENT_ID")
	switch {
	case cfgId == "" && envId != "":
		id = envId
	case cfgId != "" && envId == "":
		id = cfgId
	case cfgId != "" && envId != "":
		id = cfgId
	default:
		response.Diagnostics.AddError("no client id", "no client id found")
	}
	var sec string
	cfgSec := conf.ClientSecret.ValueString()
	envSec := os.Getenv("CLIENT_SECRET")
	switch {
	case cfgSec == "" && envSec != "":
		sec = envSec
	case cfgSec != "" && envSec == "":
		sec = cfgSec
	case cfgSec != "" && envSec != "":
		sec = cfgSec
	default:
		response.Diagnostics.AddError("no client secret", "no client secret found")
	}

	// Clients are passed through to downstream resources through the response struct
	response.ResourceData = utils.ResourceData{
		ClientID:     id,
		ClientSecret: sec,
		Version:      r.version,
	}
	response.DataSourceData = utils.DatasourceData{
		ClientID:     conf.ClientID.ValueString(),
		ClientSecret: conf.ClientSecret.ValueString(),
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
	}
}
