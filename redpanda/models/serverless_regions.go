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

// ServerlessRegions represents the Terraform model for the ServerlessRegions data source.
type ServerlessRegions struct {
	CloudProvider     types.String            `tfsdk:"cloud_provider"`
	ServerlessRegions []ServerlessRegionsItem `tfsdk:"serverless_regions"`
}

// Placement represents the placement configuration for a serverless region.
type Placement struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

// ServerlessRegionsItem represents a single region in a ServerlessRegions data source.
type ServerlessRegionsItem struct {
	CloudProvider string    `tfsdk:"cloud_provider"`
	TimeZone      string    `tfsdk:"time_zone"`
	Name          string    `tfsdk:"name"`
	Placement     Placement `tfsdk:"placement"`
}
