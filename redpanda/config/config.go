// Copyright 2024 Redpanda Data, Inc.
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

// Package config contains the configuration structs to initialize our clients
// and provider.
package config

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

// Resource is the config used to pass data and dependencies to resource
// implementations.
type Resource struct {
	TokenSource            oauth2.TokenSource
	ByocClient             utils.ByocRunner
	ControlPlaneConnection *grpc.ClientConn
	DataplaneConnPool      *cloud.ConnPool
	TerraformVersion       string
	ProviderVersion        string
	// NoCredentials marks a provider configuration that resolved no credentials
	// under skip_credentials_validation or ignore_environment_credentials.
	NoCredentials bool
}

// Datasource is the config used to pass data and dependencies to data source
// implementations.
type Datasource struct {
	TokenSource            oauth2.TokenSource
	ControlPlaneConnection *grpc.ClientConn
	DataplaneConnPool      *cloud.ConnPool
	TerraformVersion       string
	ProviderVersion        string
	// NoCredentials marks a provider configuration that resolved no credentials
	// under skip_credentials_validation or ignore_environment_credentials.
	NoCredentials bool
}

// TODO add cloud provider and region as values to persist

// RequireCredentials reports whether the provider configuration resolved
// credentials, appending the no-credentials error when it did not. Every
// consumer of a Resource calls it before wiring clients, because a
// NoCredentials configuration carries no connections.
func (r Resource) RequireCredentials(diags *diag.Diagnostics) bool {
	return requireCredentials(r.NoCredentials, diags)
}

// RequireCredentials is the Datasource counterpart of Resource.RequireCredentials.
func (d Datasource) RequireCredentials(diags *diag.Diagnostics) bool {
	return requireCredentials(d.NoCredentials, diags)
}

func requireCredentials(noCredentials bool, diags *diag.Diagnostics) bool {
	if noCredentials {
		diags.AddError(NoCredentialsSummary, NoCredentialsDetail)
	}
	return !noCredentials
}

// NoCredentialsSummary is the summary of the error a resource, data source, or
// action raises when its provider configuration has no credentials.
const NoCredentialsSummary = "Redpanda provider has no credentials"

// NoCredentialsDetail tells the user how to resolve NoCredentialsSummary.
const NoCredentialsDetail = "This uses a redpanda provider configuration that has no credentials: it sets " +
	"ignore_environment_credentials = true with no client_id and client_secret or access_token in its block, or " +
	"skip_credentials_validation = true with none in its block or the environment (an empty string counts as " +
	"unset). Point it at a configured " +
	"provider with the provider meta-argument (or the module's providers map), or give this provider configuration " +
	"credentials."
