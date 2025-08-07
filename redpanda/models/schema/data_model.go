// Copyright 2025 Redpanda Data, Inc.
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

// Package schema contains schema datasource models.
package schema

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DataModel represents schema datasource schema.
type DataModel struct {
	Subject    types.String `tfsdk:"subject"`
	Schema     types.String `tfsdk:"schema"`
	SchemaType types.String `tfsdk:"schema_type"`
	Version    types.Int64  `tfsdk:"version"`
	ID         types.Int64  `tfsdk:"id"`
	ClusterID  types.String `tfsdk:"cluster_id"`
	References types.List   `tfsdk:"references"`
}

// GetID returns the schema ID.
func (d *DataModel) GetID() string {
	return d.ID.String()
}

// Reference represents a reference to another schema.
type Reference struct {
	Name    types.String `tfsdk:"name"`
	Subject types.String `tfsdk:"subject"`
	Version types.Int64  `tfsdk:"version"`
}
