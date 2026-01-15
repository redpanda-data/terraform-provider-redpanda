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

// ServerlessPrivateLink represents the Terraform schema for the serverless private link resource.
type ServerlessPrivateLink struct {
	// Required fields
	Name             types.String `tfsdk:"name"`
	ResourceGroupID  types.String `tfsdk:"resource_group_id"`
	CloudProvider    types.String `tfsdk:"cloud_provider"`
	ServerlessRegion types.String `tfsdk:"serverless_region"`

	// Cloud provider config (oneof - contains aws)
	CloudProviderConfig types.Object `tfsdk:"cloud_provider_config"`

	// Optional fields
	AllowDeletion types.Bool `tfsdk:"allow_deletion"`

	// Computed fields
	ID        types.String `tfsdk:"id"`
	State     types.String `tfsdk:"state"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`

	// Status (cloud-specific)
	Status types.Object `tfsdk:"status"`
}
