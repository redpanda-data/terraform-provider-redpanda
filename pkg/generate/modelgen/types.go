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

package main

// SchemaInfo holds the extracted schema information
type SchemaInfo struct {
	Description string          `json:"description"`
	Attributes  []AttributeInfo `json:"attributes"`
	HasTimeouts bool            `json:"has_timeouts"`
}

// AttributeInfo holds information about a single attribute
type AttributeInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Required   bool   `json:"required"`
	Optional   bool   `json:"optional"`
	Computed   bool   `json:"computed"`
	Sensitive  bool   `json:"sensitive"`
	Deprecated bool   `json:"deprecated"`
}

// typeMap maps Terraform schema attribute types to Go model types
var typeMap = map[string]string{
	"StringAttribute":       "types.String",
	"BoolAttribute":         "types.Bool",
	"Int32Attribute":        "types.Int32",
	"Int64Attribute":        "types.Int64",
	"NumberAttribute":       "types.Number",
	"Float64Attribute":      "types.Float64",
	"ListAttribute":         "types.List",
	"SetAttribute":          "types.Set",
	"MapAttribute":          "types.Map",
	"ObjectAttribute":       "types.Object",
	"SingleNestedAttribute": "types.Object",
	"ListNestedAttribute":   "types.List",
	"SetNestedAttribute":    "types.Set",
	"MapNestedAttribute":    "types.Map",
	"DynamicAttribute":      "types.Dynamic",
}

// goTypeForAttribute returns the Go type for a given schema attribute type
func goTypeForAttribute(attrType string) string {
	if t, ok := typeMap[attrType]; ok {
		return t
	}
	// Default to types.String for unknown types
	return "types.String"
}
