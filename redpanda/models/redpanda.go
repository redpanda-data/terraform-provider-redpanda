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

package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Redpanda represents the Terraform schema for the Redpanda TF provider.
type Redpanda struct {
	AccessToken             types.String `tfsdk:"access_token"`
	ClientID                types.String `tfsdk:"client_id"`
	ClientSecret            types.String `tfsdk:"client_secret"`
	AzureSubscriptionID     types.String `tfsdk:"azure_subscription_id"`
	GcpProjectID            types.String `tfsdk:"gcp_project_id"`
	AzureClientID           types.String `tfsdk:"azure_client_id"`
	AzureClientSecret       types.String `tfsdk:"azure_client_secret"`
	AzureTenantID           types.String `tfsdk:"azure_tenant_id"`
	GoogleCredentials       types.String `tfsdk:"google_credentials"`
	GoogleCredentialsBase64 types.String `tfsdk:"google_credentials_base64"`
}
