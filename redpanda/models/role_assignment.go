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

import "github.com/hashicorp/terraform-plugin-framework/types"

// RoleAssignment defines the structure for role assignment configuration settings parsed from HCL.
type RoleAssignment struct {
	RoleName      types.String `tfsdk:"role_name"`
	Principal     types.String `tfsdk:"principal"`
	ClusterAPIURL types.String `tfsdk:"cluster_api_url"`
	ID            types.String `tfsdk:"id"`
}
