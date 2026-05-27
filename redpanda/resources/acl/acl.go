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

// Package acl contains the implementation of the ACL resource following the
// Terraform framework interfaces.
package acl

import (
	"strings"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Schema-validator wiring: OneOf validators built from each enum's
// auto-generated _name map. Referenced from schema_resource_gen.go via
// the ACL{ResourceTypes,PatternTypes,Operations,Permissions} entries in
// the schemagen validator registry, which emit the raw expression
// `aclXValidator()` rather than an import path — so the helpers must
// stay package-local.

const (
	aclResourceTypePrefix        = "RESOURCE_TYPE_"
	aclResourcePatternTypePrefix = "RESOURCE_PATTERN_TYPE_"
	aclOperationPrefix           = "OPERATION_"
	aclPermissionTypePrefix      = "PERMISSION_TYPE_"
)

// mapValueToValidator builds a stringvalidator.OneOf from a proto enum's
// auto-generated _name map, stripping the supplied prefix from each value.
func mapValueToValidator(cutset string, m map[int32]string) []validator.String {
	types := make([]string, 0, len(m))
	for _, v := range m {
		types = append(types, strings.TrimPrefix(v, cutset))
	}
	return []validator.String{stringvalidator.OneOf(types...)}
}

func aclResourceTypeValidator() []validator.String {
	return mapValueToValidator(aclResourceTypePrefix, dataplanev1.ACL_ResourceType_name)
}

func aclResourcePatternTypeValidator() []validator.String {
	return mapValueToValidator(aclResourcePatternTypePrefix, dataplanev1.ACL_ResourcePatternType_name)
}

func aclOperationValidator() []validator.String {
	return mapValueToValidator(aclOperationPrefix, dataplanev1.ACL_Operation_name)
}

func aclPermissionTypeValidator() []validator.String {
	return mapValueToValidator(aclPermissionTypePrefix, dataplanev1.ACL_PermissionType_name)
}
