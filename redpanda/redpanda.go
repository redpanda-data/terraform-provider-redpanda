package redpanda

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ tfprovider.Provider = &Redpanda{}

type Redpanda struct {
	version string
}

// New spawns a basic provider struct, no client. Configure must be called for a working client
func New(ctx context.Context, version string) func() tfprovider.Provider {
	// TODO consider whether the below should support a mock flow with a ci switch? since configure is primary TF entrypoint anyway
	return func() tfprovider.Provider {
		return &Redpanda{
			version: version,
		}
	}
}

func Provider() *schema.Schema {
	return &schema.Schema{
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
func (r *Redpanda) Configure(ctx context.Context, request tfprovider.ConfigureRequest, response *tfprovider.ConfigureResponse) {
	var conf models.Redpanda
	response.Diagnostics.Append(request.Config.Get(ctx, &conf)...)
	if response.Diagnostics.HasError() {
		return
	}

	cv2c := clients.NewCloudV2Client(ctx, r.version, conf)

	// Clients are passed through to downstream resources through the response struct
	response.ResourceData = &utils.ResourceData{CloudV2Client: cv2c}
	response.DataSourceData = &utils.DatasourceData{CloudV2Client: cv2c}
}

func (r *Redpanda) Metadata(ctx context.Context, request tfprovider.MetadataRequest, response *tfprovider.MetadataResponse) {
	response.TypeName = "redpanda"
	response.Version = r.version
}

func (r *Redpanda) Schema(ctx context.Context, request tfprovider.SchemaRequest, response *tfprovider.SchemaResponse) {
	response.Schema = *Provider()
}

func (r *Redpanda) DataSources(ctx context.Context) []func() datasource.DataSource {
	// TODO implement
	return []func() datasource.DataSource{}
}

func (r *Redpanda) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			return resources.Namespace{}
		},
		// TODO implement remaining resources
	}
}
