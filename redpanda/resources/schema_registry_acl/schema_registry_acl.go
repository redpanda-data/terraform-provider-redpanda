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

// Package schema_registry_acl contains the implementation of the Schema Registry ACL resource
// following the Terraform framework interfaces.
package schema_registry_acl

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// schemaRegistryResourceTypeValidator returns validators for Schema Registry resource types
func schemaRegistryResourceTypeValidator() []validator.String {
	return []validator.String{
		stringvalidator.OneOf("SUBJECT", "REGISTRY"),
	}
}

// schemaRegistryPatternTypeValidator returns validators for pattern types
func schemaRegistryPatternTypeValidator() []validator.String {
	return []validator.String{
		stringvalidator.OneOf("LITERAL", "PREFIXED"),
	}
}

// schemaRegistryOperationValidator returns validators for Schema Registry operations
func schemaRegistryOperationValidator() []validator.String {
	return []validator.String{
		stringvalidator.OneOf(
			"ALL",
			"READ",
			"WRITE",
			"DELETE",
			"DESCRIBE",
			"DESCRIBE_CONFIGS",
			"ALTER",
			"ALTER_CONFIGS",
		),
	}
}

// schemaRegistryPermissionValidator returns validators for permission types
func schemaRegistryPermissionValidator() []validator.String {
	return []validator.String{
		stringvalidator.OneOf("ALLOW", "DENY"),
	}
}
