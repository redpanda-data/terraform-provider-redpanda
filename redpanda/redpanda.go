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
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/acl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/namespace"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/topic"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/user"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ provider.Provider = &Redpanda{}

// Redpanda represents the Redpanda Terraform provider.
type Redpanda struct {
	version string
}

// New spawns a basic provider struct, no client. Configure must be called for a working client.
func New(_ context.Context, version string) func() provider.Provider {
	return func() provider.Provider {
		return &Redpanda{
			version: version,
		}
	}
}

func providerSchema() schema.Schema {
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

// Configure is the primary entrypoint for terraform and properly initializes the client.
func (r *Redpanda) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var conf models.Redpanda
	response.Diagnostics.Append(request.Config.Get(ctx, &conf)...)
	if response.Diagnostics.HasError() {
		return
	}

	var id string
	cfgID := conf.ClientID.ValueString()
	envID := os.Getenv("CLIENT_ID")
	switch {
	case cfgID == "" && envID != "":
		id = envID
	case cfgID != "" && envID == "":
		id = cfgID
	case cfgID != "" && envID != "":
		id = cfgID
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

	// Clients are passed through to downstream resources through the response
	// struct.
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
		func() datasource.DataSource {
			return &cluster.DataSourceCluster{}
		},
		func() datasource.DataSource {
			return &namespace.DataSourceNamespace{}
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
			return &namespace.Namespace{}
		},
		func() resource.Resource {
			return &network.Network{}
		},
		func() resource.Resource {
			return &cluster.Cluster{}
		},
		func() resource.Resource { return &acl.ACL{} },
		func() resource.Resource { return &user.User{} },
		func() resource.Resource { return &topic.Topic{} },
	}
}
