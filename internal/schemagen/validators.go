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

package schemagen

import (
	"fmt"
	"strings"
)

const validatorsImport = "github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"

// ValidatorDef describes how to generate Go code for a named validator.
type ValidatorDef struct {
	Expr string

	Imports []string

	AttrType string

	ReturnsSlice bool

	Parameterized bool

	GenFunc func(fieldPath string, params map[string]string) (expr string, imports []string)
}

var validatorRegistry = map[string]ValidatorDef{
	"ClusterTypes": {
		Expr:         "validators.ClusterTypes()",
		Imports:      []string{validatorsImport},
		AttrType:     "String",
		ReturnsSlice: true,
	},
	"RequirePrivateConnectionValidator": {
		Expr:     "validators.RequirePrivateConnectionValidator{}",
		Imports:  []string{validatorsImport},
		AttrType: "String",
	},
	"CloudProviders": {
		Expr:         "validators.CloudProviders()",
		Imports:      []string{validatorsImport},
		AttrType:     "String",
		ReturnsSlice: true,
	},

	"ACLResourceTypes": {
		Expr:         "aclResourceTypeValidator()",
		AttrType:     "String",
		ReturnsSlice: true,
	},
	"ACLPatternTypes": {
		Expr:         "aclResourcePatternTypeValidator()",
		AttrType:     "String",
		ReturnsSlice: true,
	},
	"ACLOperations": {
		Expr:         "aclOperationValidator()",
		AttrType:     "String",
		ReturnsSlice: true,
	},
	"ACLPermissions": {
		Expr:         "aclPermissionTypeValidator()",
		AttrType:     "String",
		ReturnsSlice: true,
	},
	"AWSZoneIDValidator": {
		Expr:     "validators.AWSZoneIDValidator{}",
		Imports:  []string{validatorsImport},
		AttrType: "List",
	},
	"ClusterConfiguration": {
		Expr:     "validators.ClusterConfiguration()",
		Imports:  []string{validatorsImport},
		AttrType: "Object",
	},
	"Password": {
		Parameterized: true,
		GenFunc: func(_ string, params map[string]string) (string, []string) {
			field1 := params["field1"]
			field2 := params["field2"]
			return fmt.Sprintf(`validators.Password(path.MatchRoot(%q), path.MatchRoot(%q))`, field1, field2),
				[]string{validatorsImport, "github.com/hashicorp/terraform-plugin-framework/path"}
		},
	},
	"LengthAtLeast": {
		Parameterized: true,
		GenFunc: func(_ string, params map[string]string) (string, []string) {
			n := params["n"]
			if n == "" {
				n = "1"
			}
			return fmt.Sprintf("listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(%s))", n),
				[]string{
					"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator",
					"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator",
				}
		},
		AttrType: "List",
	},
	"StringLengthAtLeast": {
		Parameterized: true,
		GenFunc: func(_ string, params map[string]string) (string, []string) {
			n := params["n"]
			if n == "" {
				n = "1"
			}
			return fmt.Sprintf("stringvalidator.LengthAtLeast(%s)", n),
				[]string{"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"}
		},
		AttrType: "String",
	},
	"MemorySharesValidator": {
		Expr:     "validators.MemorySharesValidator{}",
		Imports:  []string{validatorsImport},
		AttrType: "String",
	},
	"CPUSharesValidator": {
		Expr:     "validators.CPUSharesValidator{}",
		Imports:  []string{validatorsImport},
		AttrType: "String",
	},
	"ListSizeAtLeast": {
		Parameterized: true,
		GenFunc: func(_ string, params map[string]string) (string, []string) {
			n := params["n"]
			if n == "" {
				n = "1"
			}
			return fmt.Sprintf("listvalidator.SizeAtLeast(%s)", n),
				[]string{"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"}
		},
		AttrType: "List",
	},
	"CIDRBlock": {
		Expr:     "validators.CIDRBlockValidator{}",
		Imports:  []string{validatorsImport},
		AttrType: "String",
	},
	"CIDRBlockRegex": {
		Expr: "stringvalidator.RegexMatches(\n\t\t\t\t\tregexp.MustCompile(`^(\\d{1,3}\\.){3}\\d{1,3}/(\\d{1,2})$`),\n\t\t\t\t\t\"The value must be a valid CIDR block (e.g., 192.168.0.0/16)\",\n\t\t\t\t)",
		Imports: []string{
			"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator",
			"regexp",
		},
		AttrType: "String",
	},
	"LengthAtMost": {
		Parameterized: true,
		GenFunc: func(_ string, params map[string]string) (string, []string) {
			n := params["n"]
			if n == "" {
				n = "1"
			}
			return fmt.Sprintf("stringvalidator.LengthAtMost(%s)", n),
				[]string{"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"}
		},
		AttrType: "String",
	},
	"GCPResourceName": {
		Expr: "stringvalidator.RegexMatches(\n\t\t\t\t\tregexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),\n\t\t\t\t\t\"must start with a lowercase letter and can only contain lowercase letters, numbers, and hyphens, and must end with a letter or number\",\n\t\t\t\t)",
		Imports: []string{
			"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator",
			"regexp",
		},
		AttrType: "String",
	},
	"OneOf": {
		Parameterized: true,
		GenFunc: func(_ string, params map[string]string) (string, []string) {
			vals := params["values"]
			parts := strings.Split(vals, "|")
			quoted := make([]string, len(parts))
			for i, p := range parts {
				quoted[i] = fmt.Sprintf("%q", strings.TrimSpace(p))
			}
			return fmt.Sprintf("stringvalidator.OneOf(%s)", strings.Join(quoted, ", ")),
				[]string{"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"}
		},
		AttrType: "String",
	},
	"RequireTrue": {
		Expr:     "boolvalidator.Equals(true)",
		Imports:  []string{"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"},
		AttrType: "Bool",
	},
}

func resolveValidator(name, fieldPath, _ string) (expr string, imports []string, returnsSlice bool, err error) {
	baseName := name
	params := make(map[string]string)
	if idx := strings.Index(name, "{"); idx > 0 {
		baseName = strings.TrimSpace(name[:idx])
		paramStr := strings.TrimSuffix(strings.TrimSpace(name[idx+1:]), "}")
		for _, pair := range strings.Split(paramStr, ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
			if len(parts) == 2 {
				params[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	def, ok := validatorRegistry[baseName]
	if !ok {
		return "", nil, false, fmt.Errorf("unknown validator %q — add it to validatorRegistry in validators.go", baseName)
	}

	if def.Parameterized {
		if def.GenFunc == nil {
			return "", nil, false, fmt.Errorf("validator %q is parameterized but has no GenFunc", baseName)
		}
		expr, imports = def.GenFunc(fieldPath, params)
		return expr, imports, false, nil
	}

	return def.Expr, def.Imports, def.ReturnsSlice, nil
}

func wrapValidatorSlice(attrType string, exprs []string) string {
	joined := strings.Join(exprs, ", ")
	return fmt.Sprintf("[]validator.%s{%s}", attrType, joined)
}

func validatorAttrType(schemaAttrType string) string {
	if info, ok := attrTypeTable[schemaAttrType]; ok && info.ValidatorSuffix != "" {
		return info.ValidatorSuffix
	}
	return "String"
}
