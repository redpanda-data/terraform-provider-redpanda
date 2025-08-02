package schema

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
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
		Schema:     types.StringValue(`{"type": "string"}`),
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
		Schema:     types.StringValue(""),
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

func TestResourceModel_SchemaTypes(t *testing.T) {
	tests := []struct {
		name       string
		schemaType string
	}{
		{"AVRO schema type", "AVRO"},
		{"JSON schema type", "JSON"},
		{"PROTOBUF schema type", "PROTOBUF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &ResourceModel{
				SchemaType: types.StringValue(tt.schemaType),
			}
			assert.Equal(t, tt.schemaType, model.SchemaType.ValueString())
		})
	}
}
