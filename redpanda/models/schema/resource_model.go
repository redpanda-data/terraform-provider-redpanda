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

// Package schema contains schema resource models.
package schema

import (
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/twmb/franz-go/pkg/sr"
)

// ResourceModel represents the Terraform schema for the schema resource.
type ResourceModel struct {
	Subject       types.String         `tfsdk:"subject"`
	Schema        jsontypes.Normalized `tfsdk:"schema"`
	SchemaType    types.String         `tfsdk:"schema_type"`
	Version       types.Int64          `tfsdk:"version"`
	ID            types.Int64          `tfsdk:"id"`
	ClusterID     types.String         `tfsdk:"cluster_id"`
	References    types.List           `tfsdk:"references"`
	Compatibility types.String         `tfsdk:"compatibility"`
	Username      types.String         `tfsdk:"username"`
	Password      types.String         `tfsdk:"password"`
	AllowDeletion types.Bool           `tfsdk:"allow_deletion"`
}

// GetID returns the schema ID.
func (r *ResourceModel) GetID() string {
	return r.ID.String()
}

// GetSubject returns the subject name
func (r *ResourceModel) GetSubject() string {
	return r.Subject.ValueString()
}

// GetVersion returns the version as an int pointer (nil if not set)
func (r *ResourceModel) GetVersion() *int {
	if r.Version.IsNull() || r.Version.IsUnknown() {
		return nil
	}
	version := int(r.Version.ValueInt64())
	return &version
}

// UpdateFromSchema updates the model from a schema registry response
func (r *ResourceModel) UpdateFromSchema(schemaResp sr.SubjectSchema) {
	r.ID = types.Int64Value(int64(schemaResp.ID))
	r.Version = types.Int64Value(int64(schemaResp.Version))
	// Normalize the JSON to compact format for consistent storage
	r.Schema = jsontypes.NewNormalizedValue(compactJSON(schemaResp.Schema.Schema))
	r.SchemaType = types.StringValue(strings.ToUpper(schemaResp.Type.String()))
	r.References = r.convertReferencesToTerraform(schemaResp.References)
}

// compactJSON compacts a JSON string by removing unnecessary whitespace
func compactJSON(jsonStr string) string {
	var obj any
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		// If unmarshal fails, return original string
		return jsonStr
	}
	compact, err := json.Marshal(obj)
	if err != nil {
		// If marshal fails, return original string
		return jsonStr
	}
	return string(compact)
}

// convertReferencesToTerraform converts sr.SchemaReference slice to Terraform List type
func (*ResourceModel) convertReferencesToTerraform(refs []sr.SchemaReference) types.List {
	if len(refs) == 0 {
		return types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"name":    types.StringType,
				"subject": types.StringType,
				"version": types.Int64Type,
			},
		})
	}

	references := make([]types.Object, 0, len(refs))
	for _, ref := range refs {
		refObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"name":    types.StringType,
				"subject": types.StringType,
				"version": types.Int64Type,
			},
			map[string]attr.Value{
				"name":    types.StringValue(ref.Name),
				"subject": types.StringValue(ref.Subject),
				"version": types.Int64Value(int64(ref.Version)),
			},
		)
		references = append(references, refObj)
	}

	refsList, _ := types.ListValue(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"name":    types.StringType,
				"subject": types.StringType,
				"version": types.Int64Type,
			},
		},
		func() []attr.Value {
			result := make([]attr.Value, len(references))
			for i, ref := range references {
				result[i] = ref
			}
			return result
		}(),
	)

	return refsList
}

// ToSchemaRequest converts the ResourceModel to a sr.Schema for API requests
func (r *ResourceModel) ToSchemaRequest() sr.Schema {
	return sr.Schema{
		Schema:     r.Schema.ValueString(),
		Type:       r.convertSchemaType(),
		References: r.parseSchemaReferences(),
	}
}

// convertSchemaType converts string schema type to sr.SchemaType
func (r *ResourceModel) convertSchemaType() sr.SchemaType {
	schemaType := r.SchemaType.ValueString()
	switch strings.ToUpper(schemaType) {
	case "AVRO":
		return sr.TypeAvro
	case "JSON":
		return sr.TypeJSON
	case "PROTOBUF":
		return sr.TypeProtobuf
	default:
		return sr.TypeAvro
	}
}

// parseSchemaReferences parses the references from Terraform types to sr.SchemaReference
func (r *ResourceModel) parseSchemaReferences() []sr.SchemaReference {
	if r.References.IsNull() || r.References.IsUnknown() {
		return nil
	}

	elements := r.References.Elements()
	references := make([]sr.SchemaReference, 0, len(elements))

	for _, elem := range elements {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		ref := sr.SchemaReference{}

		if name, ok := attrs["name"].(types.String); ok && !name.IsNull() {
			ref.Name = name.ValueString()
		}
		if subject, ok := attrs["subject"].(types.String); ok && !subject.IsNull() {
			ref.Subject = subject.ValueString()
		}
		if version, ok := attrs["version"].(types.Int64); ok && !version.IsNull() {
			ref.Version = int(version.ValueInt64())
		}

		references = append(references, ref)
	}

	return references
}

// GetCompatibility returns the compatibility as a string
func (r *ResourceModel) GetCompatibility() string {
	if r.Compatibility.IsNull() || r.Compatibility.IsUnknown() {
		return ""
	}
	return r.Compatibility.ValueString()
}

// UpdateCompatibility updates the compatibility from API response
func (r *ResourceModel) UpdateCompatibility(compatibility string) {
	if compatibility != "" {
		r.Compatibility = types.StringValue(strings.ToUpper(compatibility))
	}
}
