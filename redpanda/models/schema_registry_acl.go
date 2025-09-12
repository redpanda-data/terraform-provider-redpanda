// Copyright 2024 Redpanda Data, Inc.
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

// SchemaRegistryACL represents the Terraform resource model for a Schema Registry ACL
type SchemaRegistryACL struct {
	ID            types.String `tfsdk:"id"`
	ClusterID     types.String `tfsdk:"cluster_id"`
	Principal     types.String `tfsdk:"principal"`
	ResourceType  types.String `tfsdk:"resource_type"`
	ResourceName  types.String `tfsdk:"resource_name"`
	PatternType   types.String `tfsdk:"pattern_type"`
	Host          types.String `tfsdk:"host"`
	Operation     types.String `tfsdk:"operation"`
	Permission    types.String `tfsdk:"permission"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	AllowDeletion types.Bool   `tfsdk:"allow_deletion"`
}
