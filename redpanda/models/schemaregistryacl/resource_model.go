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

package schemaregistryacl

import (
	"fmt"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
)

// ToSchemaRegistryACLRequest converts the model to a SchemaRegistryACLRequest for API calls
func (s *ResourceModel) ToSchemaRegistryACLRequest() kclients.SchemaRegistryACLRequest {
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
func (s *ResourceModel) ToSchemaRegistryACLFilter() kclients.SchemaRegistryACLFilter {
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
func (s *ResourceModel) GenerateID() string {
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
func (s *ResourceModel) MatchesACLResponse(acl *kclients.SchemaRegistryACLResponse) bool {
	return acl.Principal == s.Principal.ValueString() &&
		acl.Resource == s.ResourceName.ValueString() &&
		acl.ResourceType == s.ResourceType.ValueString() &&
		acl.PatternType == s.PatternType.ValueString() &&
		acl.Host == s.Host.ValueString() &&
		acl.Operation == s.Operation.ValueString() &&
		acl.Permission == s.Permission.ValueString()
}
