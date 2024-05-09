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
	"fmt"
	"strings"

	dataplanev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1alpha1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// stringToEnum converts a string to an enum given a certain map. It prepends
// 's' with 'prepend'.
func stringToEnum(s string, prepend string, m map[string]int32) (int32, error) {
	if e, ok := m[prepend+s]; ok {
		return e, nil
	}
	return -1, fmt.Errorf("unknown parameter: %v", s)
}

// enumToString converts an enum to a string using a given map. It cuts the
// given cutset from the response.
func enumToString(e int32, cutset string, m map[int32]string) string {
	if s, ok := m[e]; ok {
		return strings.TrimPrefix(s, cutset)
	}
	return "UNKNOWN"
}

// mapValueToValidator creates strings OneOf validators for the values of the
// given map 'm'. It removes the 'cutset' from the value of the map.
func mapValueToValidator(cutset string, m map[int32]string) []validator.String {
	var types []string
	for _, v := range m {
		types = append(types, strings.TrimPrefix(v, cutset))
	}
	return []validator.String{
		stringvalidator.OneOf(types...),
	}
}

// ACL_RESOURCE_TYPE

const aclResourceTypePrefix = "RESOURCE_TYPE_"

func stringToACLResourceType(s string) (dataplanev1alpha1.ACL_ResourceType, error) {
	enum, err := stringToEnum(s, aclResourceTypePrefix, dataplanev1alpha1.ACL_ResourceType_value)
	if err != nil {
		return -1, fmt.Errorf("failed to parse ACL resource type: %v", err)
	}
	return dataplanev1alpha1.ACL_ResourceType(enum), nil
}

func aclResourceTypeToString(e dataplanev1alpha1.ACL_ResourceType) string {
	return enumToString(int32(e), aclResourceTypePrefix, dataplanev1alpha1.ACL_ResourceType_name)
}

func aclResourceTypeValidator() []validator.String {
	return mapValueToValidator(aclResourceTypePrefix, dataplanev1alpha1.ACL_ResourceType_name)
}

// ACL_RESOURCE_PATTERN_TYPE

const aclResourcePatternTypePrefix = "RESOURCE_PATTERN_TYPE_"

func stringToACLResourcePatternType(s string) (dataplanev1alpha1.ACL_ResourcePatternType, error) {
	enum, err := stringToEnum(s, aclResourcePatternTypePrefix, dataplanev1alpha1.ACL_ResourcePatternType_value)
	if err != nil {
		return -1, fmt.Errorf("failed to parse ACL resource pattern type: %v", err)
	}
	return dataplanev1alpha1.ACL_ResourcePatternType(enum), nil
}

func aclResourcePatternTypeToString(e dataplanev1alpha1.ACL_ResourcePatternType) string {
	return enumToString(int32(e), aclResourcePatternTypePrefix, dataplanev1alpha1.ACL_ResourcePatternType_name)
}

func aclResourcePatternTypeValidator() []validator.String {
	return mapValueToValidator(aclResourcePatternTypePrefix, dataplanev1alpha1.ACL_ResourcePatternType_name)
}

// ACL_OPERATION

const aclOperationPrefix = "OPERATION_"

func stringToACLOperation(s string) (dataplanev1alpha1.ACL_Operation, error) {
	enum, err := stringToEnum(s, aclOperationPrefix, dataplanev1alpha1.ACL_Operation_value)
	if err != nil {
		return -1, fmt.Errorf("failed to parse operation: %v", err)
	}
	return dataplanev1alpha1.ACL_Operation(enum), nil
}

func aclOperationValidator() []validator.String {
	return mapValueToValidator(aclOperationPrefix, dataplanev1alpha1.ACL_Operation_name)
}

// ACL_PERMISSION_TYPE

const aclPermissionTypePrefix = "PERMISSION_TYPE_"

func stringToACLPermissionType(s string) (dataplanev1alpha1.ACL_PermissionType, error) {
	enum, err := stringToEnum(s, aclPermissionTypePrefix, dataplanev1alpha1.ACL_PermissionType_value)
	if err != nil {
		return -1, fmt.Errorf("failed to parse operation: %v", err)
	}
	return dataplanev1alpha1.ACL_PermissionType(enum), nil
}

func aclPermissionTypeValidator() []validator.String {
	return mapValueToValidator(aclPermissionTypePrefix, dataplanev1alpha1.ACL_PermissionType_name)
}
