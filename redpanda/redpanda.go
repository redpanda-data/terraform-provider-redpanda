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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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
			"client_id": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: fmt.Sprintf("The ID for the client. You need `client_id` AND `client_secret`, "+
					"to use this provider. Can also be set with the `%v` environment variable.", ClientIDEnv),
			},
			"client_secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: fmt.Sprintf("Redpanda client secret. You need `client_id` AND `client_secret`, "+
					"to use this provider. Can also be set with the `%v` environment variable.", ClientSecretEnv),
			},
		},
		Description:         "Redpanda Data terraform provider",
		MarkdownDescription: "Provider configuration",
	}
}

// Configure is the primary entrypoint for terraform and properly initializes
// the client.
func (r *Redpanda) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var conf models.Redpanda
	response.Diagnostics.Append(request.Config.Get(ctx, &conf)...)
	if response.Diagnostics.HasError() {
		return
	}
	// Environment variables overrides.
	id, sec := conf.ClientID.ValueString(), conf.ClientSecret.ValueString()
	for _, override := range []struct {
		name string
		src  string
		dst  *string
	}{
		{"Client ID", os.Getenv(ClientIDEnv), &id},
		{"Client Secret", os.Getenv(ClientSecretEnv), &sec},
	} {
		if *override.dst == "" {
			*override.dst = override.src
		}
		if override.src == "" && *override.dst == "" {
			response.Diagnostics.AddError(
				fmt.Sprintf("%v missing", override.name),
				fmt.Sprintf("no %v found, please set the corresponding variable in the configuration file", override.name),
			)
		}
	}
	// Clients are passed through to downstream resources through the response
	// struct.
	token, endpoint, err := cloud.RequestTokenAndEnv(ctx, r.cloudEnv, id, sec)
	if err != nil {
		response.Diagnostics.AddError("failed to authenticate with Redpanda API", err.Error())
		return
	}
	if r.conn == nil {
		conn, err := cloud.SpawnConn(endpoint.APIURL, token)
		if err != nil {
			response.Diagnostics.AddError("failed to open a connection with the Redpanda Cloud API", err.Error())
			return
		}
		r.conn = conn
	}

	response.ResourceData = config.Resource{
		AuthToken:              token,
		ClientID:               id,
		ClientSecret:           sec,
		CloudEnv:               r.cloudEnv,
		ControlPlaneConnection: r.conn,
	}
	response.DataSourceData = config.Datasource{
		AuthToken:              token,
		ClientID:               id,
		ClientSecret:           sec,
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
