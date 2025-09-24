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

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
)

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

// ToSchemaRegistryACLRequest converts the model to a SchemaRegistryACLRequest for API calls
func (s *SchemaRegistryACL) ToSchemaRegistryACLRequest() kclients.SchemaRegistryACLRequest {
	return kclients.SchemaRegistryACLRequest{
		Principal:    s.Principal.ValueString(),
		Resource:     s.ResourceName.ValueString(),
		ResourceType: s.ResourceType.ValueString(),
		PatternType:  s.PatternType.ValueString(),
		Host:         s.Host.ValueString(),
		Operation:    s.Operation.ValueString(),
		Permission:   s.Permission.ValueString(),
	}
}

// ToSchemaRegistryACLFilter converts the model to a SchemaRegistryACLFilter for API calls
func (s *SchemaRegistryACL) ToSchemaRegistryACLFilter() kclients.SchemaRegistryACLFilter {
	return kclients.SchemaRegistryACLFilter{
		Principal:    s.Principal.ValueString(),
		Resource:     s.ResourceName.ValueString(),
		ResourceType: s.ResourceType.ValueString(),
		PatternType:  s.PatternType.ValueString(),
		Host:         s.Host.ValueString(),
		Operation:    s.Operation.ValueString(),
		Permission:   s.Permission.ValueString(),
	}
}

// GenerateID generates the unique ID for the Schema Registry ACL
func (s *SchemaRegistryACL) GenerateID() string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s",
		s.ClusterID.ValueString(),
		s.Principal.ValueString(),
		s.ResourceType.ValueString(),
		s.ResourceName.ValueString(),
		s.PatternType.ValueString(),
		s.Host.ValueString(),
		s.Operation.ValueString(),
		s.Permission.ValueString())
}

// MatchesACLResponse checks if the model matches the given ACL response
func (s *SchemaRegistryACL) MatchesACLResponse(acl *kclients.SchemaRegistryACLResponse) bool {
	return acl.Principal == s.Principal.ValueString() &&
		acl.Resource == s.ResourceName.ValueString() &&
		acl.ResourceType == s.ResourceType.ValueString() &&
		acl.PatternType == s.PatternType.ValueString() &&
		acl.Host == s.Host.ValueString() &&
		acl.Operation == s.Operation.ValueString() &&
		acl.Permission == s.Permission.ValueString()
}
