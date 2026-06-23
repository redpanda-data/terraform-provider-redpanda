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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/twmb/franz-go/pkg/sr"
)

// ResourceModel represents the Terraform schema for the schema resource.
type ResourceModel struct {
	AllowDeletion     types.Bool   `tfsdk:"allow_deletion"`
	ClusterID         types.String `tfsdk:"cluster_id"`
	Compatibility     types.String `tfsdk:"compatibility"`
	ID                types.Int64  `tfsdk:"id"`
	Password          types.String `tfsdk:"password"`
	PasswordWO        types.String `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64  `tfsdk:"password_wo_version"`
	References        types.List   `tfsdk:"references"`
	Schema            types.String `tfsdk:"schema"`
	SchemaType        types.String `tfsdk:"schema_type"`
	Subject           types.String `tfsdk:"subject"`
	Username          types.String `tfsdk:"username"`
	Version           types.Int64  `tfsdk:"version"`
}

// GetEffectivePassword returns the password to use, preferring password_wo over password.
func (r *ResourceModel) GetEffectivePassword() string {
	return utils.GetEffectivePassword(r.Password, r.PasswordWO)
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
func (r *ResourceModel) UpdateFromSchema(schemaResp sr.SubjectSchema) diag.Diagnostics {
	r.ID = types.Int64Value(int64(schemaResp.ID))
	r.Version = types.Int64Value(int64(schemaResp.Version))

	if preserved := r.preserveUserSchemaBody(schemaResp.Schema.Schema); preserved != "" {
		r.Schema = types.StringValue(preserved)
	} else {
		r.Schema = types.StringValue(schemaResp.Schema.Schema)
	}

	r.SchemaType = types.StringValue(strings.ToUpper(schemaResp.Type.String()))
	refsList, diags := r.convertReferencesToTerraform(schemaResp.References, r.References)
	r.References = refsList
	return diags
}

// convertReferencesToTerraform converts sr.SchemaReference slice to Terraform
// List type. When the registry returns no references, the prior list's shape
// is preserved: a configured empty list stays empty, an omitted (null) block
// stays null. Coercing []->null trips Terraform's post-apply consistency check
// (references is Optional, not Computed).
func (*ResourceModel) convertReferencesToTerraform(refs []sr.SchemaReference, prior types.List) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	objType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":    types.StringType,
			"subject": types.StringType,
			"version": types.Int64Type,
		},
	}
	if len(refs) == 0 {
		if !prior.IsNull() && !prior.IsUnknown() {
			return types.ListValueMust(objType, []attr.Value{}), diags
		}
		return types.ListNull(objType), diags
	}

	elems := make([]attr.Value, 0, len(refs))
	for _, ref := range refs {
		refObj, d := types.ObjectValue(
			objType.AttrTypes,
			map[string]attr.Value{
				"name":    types.StringValue(ref.Name),
				"subject": types.StringValue(ref.Subject),
				"version": types.Int64Value(int64(ref.Version)),
			},
		)
		diags.Append(d...)
		elems = append(elems, refObj)
	}

	refsList, d := types.ListValue(objType, elems)
	diags.Append(d...)
	return refsList, diags
}

// ToSchemaRequest converts the ResourceModel to a sr.Schema for API requests
func (r *ResourceModel) ToSchemaRequest() sr.Schema {
	return sr.Schema{
		Schema:     r.Schema.ValueString(),
		Type:       r.convertSchemaType(),
		References: r.parseSchemaReferences(),
	}
}

// convertSchemaType converts a string schema type to sr.SchemaType. Accepts
// both the friendly form ("AVRO"/"JSON"/"PROTOBUF") and the proto-style
// form ("SCHEMA_TYPE_AVRO"/...) — state written by earlier provider
// versions, or configs that pasted the proto-form value, must round-trip
// cleanly. Unknown input falls back to Avro for backward compatibility.
func (r *ResourceModel) convertSchemaType() sr.SchemaType {
	switch strings.ToUpper(r.SchemaType.ValueString()) {
	case "AVRO", "SCHEMA_TYPE_AVRO":
		return sr.TypeAvro
	case "JSON", "SCHEMA_TYPE_JSON":
		return sr.TypeJSON
	case "PROTOBUF", "SCHEMA_TYPE_PROTOBUF":
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

// preserveUserSchemaBody returns the user's current schema body when it is
// semantically equivalent to the registry's stored form, so the user's input
// (formatting, declaration order, FQN-vs-relative type refs) is preserved in
// state rather than churning to the registry's canonicalized form. Layers:
//
//  1. PROTOBUF: equal under the protobuf canonicalizer (Schema Registry
//     reorders definitions and fully-qualifies in-package type refs on write).
//  2. JSON-level: same parsed-then-canonical-encoding (whitespace + key-order
//     tolerant) — applies to JSON and Avro bodies.
//  3. Avro-level (only when r.SchemaType is AVRO): same Avro schema modulo
//     namespace-relative vs FQN type references and non-essential metadata.
//
// Returns empty string when no layer matches, signaling the caller to use the
// registry response as-is.
func (r *ResourceModel) preserveUserSchemaBody(registrySchema string) string {
	if r.Schema.IsNull() || r.Schema.IsUnknown() {
		return ""
	}

	currentSchema := r.Schema.ValueString()

	// PROTOBUF bodies are not JSON; compare under the protobuf canonicalizer.
	if strings.EqualFold(r.SchemaType.ValueString(), "PROTOBUF") {
		if ProtobufBodiesEquivalent(currentSchema, registrySchema) {
			return currentSchema
		}
		return ""
	}

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

	// JSON-level not equal — for Avro, the registry may have canonicalized
	// FQN type references to their namespace-relative form. Compare under
	// our Avro canonicalizer.
	if strings.EqualFold(r.SchemaType.ValueString(), "AVRO") &&
		AvroBodiesEquivalent(currentSchema, registrySchema) {
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
