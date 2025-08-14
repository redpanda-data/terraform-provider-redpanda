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
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/twmb/franz-go/pkg/sr"
)

// ResourceModel represents the Terraform schema for the schema resource.
type ResourceModel struct {
	Subject       types.String `tfsdk:"subject"`
	Schema        types.String `tfsdk:"schema"`
	SchemaType    types.String `tfsdk:"schema_type"`
	Version       types.Int64  `tfsdk:"version"`
	ID            types.Int64  `tfsdk:"id"`
	ClusterID     types.String `tfsdk:"cluster_id"`
	References    types.List   `tfsdk:"references"`
	Compatibility types.String `tfsdk:"compatibility"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	AllowDeletion types.Bool   `tfsdk:"allow_deletion"`
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

	normalizedSchema := r.normalizeJSON(schemaResp.Schema.Schema)
	if normalizedSchema != "" {
		r.Schema = types.StringValue(normalizedSchema)
	} else {
		r.Schema = types.StringValue(schemaResp.Schema.Schema)
	}

	r.SchemaType = types.StringValue(strings.ToUpper(schemaResp.Type.String()))
	r.References = r.convertReferencesToTerraform(schemaResp.References)
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

// normalizeJSON attempts to preserve the original JSON formatting when the content is semantically equivalent.
// If the current schema and registry schema are equivalent, returns the current schema formatting.
// If they differ or normalization fails, returns empty string to use registry response as-is.
func (r *ResourceModel) normalizeJSON(registrySchema string) string {
	if r.Schema.IsNull() || r.Schema.IsUnknown() {
		return ""
	}

	currentSchema := r.Schema.ValueString()

	var currentJSON, registryJSON any

	if err := json.Unmarshal([]byte(currentSchema), &currentJSON); err != nil {
		return ""
	}

	if err := json.Unmarshal([]byte(registrySchema), &registryJSON); err != nil {
		return ""
	}

	currentBytes, err := json.Marshal(currentJSON)
	if err != nil {
		return ""
	}

	registryBytes, err := json.Marshal(registryJSON)
	if err != nil {
		return ""
	}

	if bytes.Equal(currentBytes, registryBytes) {
		return currentSchema
	}

	return ""
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
