// Copyright 2023 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Package redpanda contains the implementation of the Terraform Provider
// framework interface for Redpanda.
package redpanda

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/acl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/resourcegroup"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlesscluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/topic"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/user"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"google.golang.org/grpc"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ provider.Provider = &Redpanda{}

// Redpanda represents the Redpanda Terraform provider.
type Redpanda struct {
	// cloudEnv is the cloud environment which the Terraform provider points to;
	// one of 'ign' or 'dev'.
	cloudEnv string
	// version is the Redpanda terraform provider.
	version string
	// conn is the connection to the control plane API.
	conn *grpc.ClientConn
}

const (
	// AccessTokenEnv is the access token used to authenticate to Redpanda cloud.
	AccessTokenEnv = "REDPANDA_ACCESS_TOKEN"
	// ClientIDEnv is the client_id used to authenticate to Redpanda cloud.
	ClientIDEnv = "REDPANDA_CLIENT_ID"
	// ClientSecretEnv is the client_secret used to authenticate to Redpanda cloud.
	ClientSecretEnv = "REDPANDA_CLIENT_SECRET"
)

// New spawns a basic provider struct, no client. Configure must be called for a
// working client.
func New(_ context.Context, cloudEnv, version string) func() provider.Provider {
	return func() provider.Provider {
		return &Redpanda{
			cloudEnv: cloudEnv,
			version:  version,
		}
	}
}

func providerSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"access_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: fmt.Sprintf("Redpanda client token. You need either `access_token`, or both `client_id` "+
					"and `client_secret` to use this provider. Can also be set with the `%v` environment variable.", AccessTokenEnv),
				Validators: []validator.String{
					validators.NotUnknown(),
				},
			},
			"client_id": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: fmt.Sprintf("The ID for the client. You need either `client_id` AND `client_secret`, "+
					"or `access_token`, to use this provider. Can also be set with the `%v` environment variable.", ClientIDEnv),
				Validators: []validator.String{
					validators.AlsoRequiresOneOf(
						path.MatchRoot("client_secret"),
						path.MatchRoot("access_token"),
					),
					validators.NotUnknown(),
				},
			},
			"client_secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: fmt.Sprintf("Redpanda client secret. You need either `client_id` AND `client_secret`, "+
					"or `access_token`, to use this provider. Can also be set with the `%v` environment variable.", ClientSecretEnv),
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("client_id")),
					validators.NotUnknown(),
				},
			},
		},
		Description:         "Redpanda Data terraform provider",
		MarkdownDescription: "Provider configuration",
	}
}

type credentials struct {
	ClientID       string
	ClientSecret   string
	EndpointAPIURL string
	Token          string
}

// getCredentials reads authentication configuration from multiple sources and returns an access token for Redpanda Cloud
func getCredentials(ctx context.Context, cloudEnv string, conf models.Redpanda) (credentials, diag.Diagnostics) {
	// Similar to other Terraform providers, this provider (1) can pick up authentication configuration
	// from multiple sources, and (2) gives precedence to direct configuration over environment variables.
	// The first source which provides a client_id, client_secret, or token will be chosen to validate and
	// try to use. The sources are checked in the following order:
	// 1. Parameters in the provider configuration
	// 2. Environment variables

	// TODO: There are some scenarios here that could be handled more smoothly.
	// - If a source has both a token and a client id/secret pair, should we validate the token 'sub'/'azp'
	//   claim against the client id?
	// - If the token is expired, should we try to renew it using a client id/secret pair? Or bail?
	// - If the provider configuration has a client id/secret pair, but a token is provided in an environment
	//   variable, should we use the token if it validates against the client id?
	// - Should we keep the client id/secret around as well in case we need to re-auth with the API?
	// - If the user passes in a token, should we validate its audience field against the configured
	//   Redpanda cloud environment/endpoint? or, should we pull the endpoint from the audience field? or
	//   do nothing like we do now?

	creds := credentials{}
	diags := diag.Diagnostics{}

	endpoint, err := cloud.EndpointForEnv(cloudEnv)
	if err != nil {
		diags.AddError("error retrieving correct endpoint", err.Error())
		return creds, diags
	}
	creds.EndpointAPIURL = endpoint.APIURL

	// Check provider configuration
	if !conf.ClientID.IsNull() || !conf.ClientSecret.IsNull() || !conf.AccessToken.IsNull() {
		tflog.Info(ctx, "using authentication configuration found in provider configuration")
		// client_id, client_secret, and token are validated in the schema to make sure a valid
		// combination is always provided. the validators have better error messages than checking
		// here would, as they can point directly to the attribute in the user's HCL code.
		creds.ClientID = conf.ClientID.ValueString()
		creds.ClientSecret = conf.ClientSecret.ValueString()
		creds.Token = conf.AccessToken.ValueString()

		if creds.Token == "" {
			creds.Token, err = cloud.RequestToken(ctx, endpoint, creds.ClientID, creds.ClientSecret)
			if err != nil {
				diags.AddError("failed to authenticate with Redpanda API", err.Error())
			}
		}
		return creds, diags
	}

	// Check environment variable configuration
	id, sec, token := os.Getenv(ClientIDEnv), os.Getenv(ClientSecretEnv), os.Getenv(AccessTokenEnv)
	if id != "" || sec != "" || token != "" {
		tflog.Info(ctx, "using authentication configuration found in environment variables")
		if id != "" && sec == "" && token == "" {
			diags.AddError("Client Secret or Token missing",
				fmt.Sprintf("One of the environment variables %v or %v must be set when %v is also set", ClientSecretEnv, AccessTokenEnv, ClientIDEnv))
			return creds, diags
		}
		if sec != "" && id == "" {
			diags.AddError("Client ID missing",
				fmt.Sprintf("Environment variable %v must be set when %v is also set", ClientIDEnv, ClientSecretEnv))
			return creds, diags
		}

		creds.ClientID = id
		creds.ClientSecret = sec
		if creds.Token == "" {
			creds.Token, err = cloud.RequestToken(ctx, endpoint, creds.ClientID, creds.ClientSecret)
			if err != nil {
				diags.AddError("failed to authenticate with Redpanda API", err.Error())
			}
		}
		return creds, diags
	}

	// No authentication configuration found
	diags.AddError("Client configuration missing",
		"no Client ID, Client Secret, or Token found, please set the corresponding variables in the "+
			"configuration file or as environment variables",
	)
	return creds, diags
}

// Configure is the primary entrypoint for terraform and properly initializes
// the client.
func (r *Redpanda) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var conf models.Redpanda
	response.Diagnostics.Append(request.Config.Get(ctx, &conf)...)
	if response.Diagnostics.HasError() {
		return
	}
	// Clients are passed through to downstream resources through the response
	// struct.
	creds, diags := getCredentials(ctx, r.cloudEnv, conf)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	if r.conn == nil {
		conn, err := cloud.SpawnConn(creds.EndpointAPIURL, creds.Token)
		if err != nil {
			response.Diagnostics.AddError("failed to open a connection with the Redpanda Cloud API", err.Error())
			return
		}
		r.conn = conn
	}

	response.ResourceData = config.Resource{
		AuthToken:              creds.Token,
		ClientID:               creds.ClientID,
		ClientSecret:           creds.ClientSecret,
		CloudEnv:               r.cloudEnv,
		ControlPlaneConnection: r.conn,
	}
	response.DataSourceData = config.Datasource{
		AuthToken:              creds.Token,
		ClientID:               creds.ClientID,
		ClientSecret:           creds.ClientSecret,
		CloudEnv:               r.cloudEnv,
		ControlPlaneConnection: r.conn,
	}
}

// Metadata returns the provider metadata.
func (r *Redpanda) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "redpanda"
	response.Version = r.version
}

// Schema returns the Redpanda provider schema.
func (*Redpanda) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = providerSchema()
}

// DataSources returns a slice of functions to instantiate each Redpanda
// DataSource.
func (*Redpanda) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return &serverlesscluster.DataSourceServerlessCluster{} },
		func() datasource.DataSource {
			return &cluster.DataSourceCluster{}
		},
		func() datasource.DataSource {
			return &resourcegroup.DataSourceResourceGroup{}
		},
		func() datasource.DataSource {
			return &network.DataSourceNetwork{}
		},
	}
}

// Resources returns a slice of functions to instantiate each Redpanda resource.
func (*Redpanda) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			return &resourcegroup.ResourceGroup{}
		},
		func() resource.Resource {
			return &network.Network{}
		},
		func() resource.Resource { return &serverlesscluster.ServerlessCluster{} },
		func() resource.Resource {
			return &cluster.Cluster{}
		},
		func() resource.Resource { return &acl.ACL{} },
		func() resource.Resource { return &user.User{} },
		func() resource.Resource { return &topic.Topic{} },
	}
}
