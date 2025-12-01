// Copyright 2023 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package models

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Pipeline defines the structure for pipeline configuration settings parsed from HCL.
type Pipeline struct {
	ID               types.String `tfsdk:"id"`
	ClusterAPIURL    types.String `tfsdk:"cluster_api_url"`
	DisplayName      types.String `tfsdk:"display_name"`
	Description      types.String `tfsdk:"description"`
	ConfigYaml       types.String `tfsdk:"config_yaml"`
	StartAfterCreate types.Bool   `tfsdk:"start_after_create"`
	State            types.String `tfsdk:"state"`
	URL              types.String `tfsdk:"url"`
	Resources        types.Object `tfsdk:"resources"`
	Tags             types.Map    `tfsdk:"tags"`
}

// PipelineResources defines the structure for pipeline resource allocation.
type PipelineResources struct {
	MemoryShares types.String `tfsdk:"memory_shares"`
	CpuShares    types.String `tfsdk:"cpu_shares"`
}

// GetPipelineResourcesType returns the attribute types for the resources nested object.
func GetPipelineResourcesType() map[string]attr.Type {
	return map[string]attr.Type{
		"memory_shares": types.StringType,
		"cpu_shares":    types.StringType,
	}
}
