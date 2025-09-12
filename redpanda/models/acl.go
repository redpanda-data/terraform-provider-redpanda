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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ACL defines the structure for configuration settings parsed from HCL.
type ACL struct {
	ResourceType        types.String `tfsdk:"resource_type"`
	ResourceName        types.String `tfsdk:"resource_name"`
	ResourcePatternType types.String `tfsdk:"resource_pattern_type"`
	Principal           types.String `tfsdk:"principal"`
	Host                types.String `tfsdk:"host"`
	Operation           types.String `tfsdk:"operation"`
	PermissionType      types.String `tfsdk:"permission_type"`
	ClusterAPIURL       types.String `tfsdk:"cluster_api_url"`
	AllowDeletion       types.Bool   `tfsdk:"allow_deletion"`
	ID                  types.String `tfsdk:"id"`
}

// GenerateID generates a unique ID for the ACL
func (a *ACL) GenerateID() string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		a.ResourceType.ValueString(),
		a.ResourceName.ValueString(),
		a.ResourcePatternType.ValueString(),
		a.Principal.ValueString(),
		a.Host.ValueString(),
		a.Operation.ValueString(),
		a.PermissionType.ValueString())
}
