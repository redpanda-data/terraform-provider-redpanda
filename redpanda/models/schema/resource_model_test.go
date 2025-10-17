package schema

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/twmb/franz-go/pkg/sr"
)

func TestResourceModel_GetID(t *testing.T) {
	model := &ResourceModel{
		ID: types.Int64Value(54321),
	}

	assert.Equal(t, "54321", model.GetID())
}

func TestResourceModel_GetID_NullValue(t *testing.T) {
	model := &ResourceModel{
		ID: types.Int64Null(),
	}

	assert.Equal(t, "<null>", model.GetID())
}

func TestResourceModel_GetID_UnknownValue(t *testing.T) {
	model := &ResourceModel{
		ID: types.Int64Unknown(),
	}

	assert.Equal(t, "<unknown>", model.GetID())
}

func TestResourceModel_AllFields(t *testing.T) {
	model := &ResourceModel{
		Subject:    types.StringValue("test-resource-subject"),
		Schema:     jsontypes.NewNormalizedValue(`{"type": "string"}`),
		SchemaType: types.StringValue("JSON"),
		Version:    types.Int64Value(2),
		ID:         types.Int64Value(200),
		ClusterID:  types.StringValue("cluster-456"),
		References: types.ListNull(types.ObjectType{}),
	}

	assert.Equal(t, "test-resource-subject", model.Subject.ValueString())
	assert.Equal(t, `{"type": "string"}`, model.Schema.ValueString())
	assert.Equal(t, "JSON", model.SchemaType.ValueString())
	assert.Equal(t, int64(2), model.Version.ValueInt64())
	assert.Equal(t, int64(200), model.ID.ValueInt64())
	assert.Equal(t, "cluster-456", model.ClusterID.ValueString())
	assert.True(t, model.References.IsNull())
}

func TestResourceModel_EmptyFields(t *testing.T) {
	model := &ResourceModel{
		Subject:    types.StringValue(""),
		Schema:     jsontypes.NewNormalizedValue(""),
		SchemaType: types.StringValue(""),
		Version:    types.Int64Value(0),
		ID:         types.Int64Value(0),
		ClusterID:  types.StringValue(""),
		References: types.ListUnknown(types.ObjectType{}),
	}

	assert.Equal(t, "", model.Subject.ValueString())
	assert.Equal(t, "", model.Schema.ValueString())
	assert.Equal(t, "", model.SchemaType.ValueString())
	assert.Equal(t, int64(0), model.Version.ValueInt64())
	assert.Equal(t, int64(0), model.ID.ValueInt64())
	assert.Equal(t, "", model.ClusterID.ValueString())
	assert.True(t, model.References.IsUnknown())
}

func TestResourceModel_ConvertSchemaType(t *testing.T) {
	tests := []struct {
		name       string
		schemaType string
		expected   sr.SchemaType
	}{
		{"AVRO uppercase", "AVRO", sr.TypeAvro},
		{"avro lowercase", "avro", sr.TypeAvro},
		{"Avro mixed case", "Avro", sr.TypeAvro},
		{"JSON uppercase", "JSON", sr.TypeJSON},
		{"json lowercase", "json", sr.TypeJSON},
		{"Json mixed case", "Json", sr.TypeJSON},
		{"PROTOBUF uppercase", "PROTOBUF", sr.TypeProtobuf},
		{"protobuf lowercase", "protobuf", sr.TypeProtobuf},
		{"ProtoBuf mixed case", "ProtoBuf", sr.TypeProtobuf},
		{"invalid type defaults to AVRO", "invalid", sr.TypeAvro},
		{"empty string defaults to AVRO", "", sr.TypeAvro},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &ResourceModel{
				SchemaType: types.StringValue(tt.schemaType),
			}
			assert.Equal(t, tt.expected, model.convertSchemaType())
		})
	}
}

func TestResourceModel_PlanApplyComparison(t *testing.T) {
	tests := []struct {
		name                string
		currentSchema       string
		registrySchema      string
		expectNormalization bool
		description         string
	}{
		{
			name:                "formatted vs minified - same content",
			currentSchema:       `{"type": "record", "name": "User", "fields": [{"name": "id", "type": "int"}]}`,
			registrySchema:      `{"type":"record","name":"User","fields":[{"name":"id","type":"int"}]}`,
			expectNormalization: true,
			description:         "Should preserve original formatting when content is semantically identical",
		},
		{
			name: "pretty printed vs minified - same content",
			currentSchema: `{
  "type": "record",
  "name": "User", 
  "fields": [
    {
      "name": "id",
      "type": "int"
    },
    {
      "name": "name",
      "type": "string"
    }
  ]
}`,
			registrySchema:      `{"type":"record","name":"User","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}`,
			expectNormalization: true,
			description:         "Should preserve pretty printing when content matches",
		},
		{
			name:                "different content - field added",
			currentSchema:       `{"type": "record", "name": "User", "fields": [{"name": "id", "type": "int"}]}`,
			registrySchema:      `{"type":"record","name":"User","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}`,
			expectNormalization: false,
			description:         "Should not normalize when content actually differs",
		},
		{
			name:                "different field order - semantically same",
			currentSchema:       `{"name": "User", "type": "record", "fields": [{"name": "id", "type": "int"}]}`,
			registrySchema:      `{"type":"record","name":"User","fields":[{"name":"id","type":"int"}]}`,
			expectNormalization: true,
			description:         "Should normalize when field order differs but content is same",
		},
		{
			name:                "invalid JSON in current",
			currentSchema:       `{invalid json}`,
			registrySchema:      `{"type":"record","name":"User","fields":[]}`,
			expectNormalization: false,
			description:         "Should handle invalid JSON gracefully",
		},
		{
			name:                "invalid JSON in registry",
			currentSchema:       `{"type": "record", "name": "User", "fields": []}`,
			registrySchema:      `{invalid json}`,
			expectNormalization: false,
			description:         "Should handle invalid registry JSON gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test compactJSON directly to verify normalization behavior
			compactCurrent := compactJSON(tt.currentSchema)
			compactRegistry := compactJSON(tt.registrySchema)

			if tt.expectNormalization {
				assert.Equal(t, compactCurrent, compactRegistry, tt.description)
			} else {
				assert.NotEqual(t, compactCurrent, compactRegistry, tt.description)
			}
		})
	}
}

func TestResourceModel_UpdateFromSchema_JSONNormalization(t *testing.T) {
	// Test that UpdateFromSchema preserves JSON formatting when content is equivalent
	currentFormattedSchema := `{
  "type": "record",
  "name": "User",
  "fields": [
    {
      "name": "id", 
      "type": "int"
    },
    {
      "name": "name",
      "type": "string"
    }
  ]
}`

	minifiedFromRegistry := `{"type":"record","name":"User","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}`

	model := &ResourceModel{
		Subject:    types.StringValue("test-subject"),
		Schema:     jsontypes.NewNormalizedValue(currentFormattedSchema),
		SchemaType: types.StringValue("AVRO"),
		Version:    types.Int64Value(1),
		ID:         types.Int64Value(100),
		ClusterID:  types.StringValue("test-cluster"),
	}

	schemaResp := sr.SubjectSchema{
		Subject: "test-subject",
		ID:      200,
		Version: 2,
		Schema: sr.Schema{
			Schema: minifiedFromRegistry,
			Type:   sr.TypeAvro,
		},
	}

	model.UpdateFromSchema(schemaResp)

	// Schema gets compacted by UpdateFromSchema
	expectedCompact := compactJSON(currentFormattedSchema)
	assert.Equal(t, expectedCompact, model.Schema.ValueString(),
		"Schema should be compacted by UpdateFromSchema")
	assert.Equal(t, int64(200), model.ID.ValueInt64())
	assert.Equal(t, int64(2), model.Version.ValueInt64())
	assert.Equal(t, "AVRO", model.SchemaType.ValueString())
}

func TestResourceModel_UpdateFromSchema_DifferentContent(t *testing.T) {
	// Test that UpdateFromSchema uses registry response when content actually differs
	currentSchema := `{
  "type": "record",
  "name": "User", 
  "fields": [
    {
      "name": "id",
      "type": "int"
    }
  ]
}`

	updatedSchemaFromRegistry := `{"type":"record","name":"User","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}`

	model := &ResourceModel{
		Subject:    types.StringValue("test-subject"),
		Schema:     jsontypes.NewNormalizedValue(currentSchema),
		SchemaType: types.StringValue("AVRO"),
		Version:    types.Int64Value(1),
		ID:         types.Int64Value(100),
		ClusterID:  types.StringValue("test-cluster"),
	}

	schemaResp := sr.SubjectSchema{
		Subject: "test-subject",
		ID:      200,
		Version: 2,
		Schema: sr.Schema{
			Schema: updatedSchemaFromRegistry,
			Type:   sr.TypeAvro,
		},
	}

	model.UpdateFromSchema(schemaResp)

	// Should use registry response since content actually differs
	// Note: compactJSON may reorder keys, so we compare compacted versions
	expectedCompact := compactJSON(updatedSchemaFromRegistry)
	assert.Equal(t, expectedCompact, model.Schema.ValueString(),
		"Should use registry response when content actually differs")
	assert.Equal(t, int64(200), model.ID.ValueInt64())
	assert.Equal(t, int64(2), model.Version.ValueInt64())
}

func TestResourceModel_ToSchemaRequest_Equivalence(t *testing.T) {
	// Test that ToSchemaRequest produces equivalent results for semantically identical schemas
	formattedSchema := `{
  "type": "record",
  "name": "User",
  "fields": [
    {
      "name": "id",
      "type": "int"
    }
  ]
}`

	minifiedSchema := `{"type":"record","name":"User","fields":[{"name":"id","type":"int"}]}`

	model1 := &ResourceModel{
		Schema:     jsontypes.NewNormalizedValue(formattedSchema),
		SchemaType: types.StringValue("AVRO"),
		References: types.ListNull(types.ObjectType{}),
	}

	model2 := &ResourceModel{
		Schema:     jsontypes.NewNormalizedValue(minifiedSchema),
		SchemaType: types.StringValue("avro"), // different case
		References: types.ListNull(types.ObjectType{}),
	}

	req1 := model1.ToSchemaRequest()
	req2 := model2.ToSchemaRequest()

	// Schema content should be identical (both use their original formatting)
	assert.Equal(t, formattedSchema, req1.Schema)
	assert.Equal(t, minifiedSchema, req2.Schema)

	// But schema type should be normalized to same value
	assert.Equal(t, req1.Type, req2.Type)
	assert.Equal(t, sr.TypeAvro, req1.Type)
}
