package schema

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestDataModel_GetID(t *testing.T) {
	model := &DataModel{
		ID: types.Int64Value(12345),
	}

	assert.Equal(t, "12345", model.GetID())
}

func TestDataModel_GetID_NullValue(t *testing.T) {
	model := &DataModel{
		ID: types.Int64Null(),
	}

	assert.Equal(t, "<null>", model.GetID())
}

func TestDataModel_GetID_UnknownValue(t *testing.T) {
	model := &DataModel{
		ID: types.Int64Unknown(),
	}

	assert.Equal(t, "<unknown>", model.GetID())
}

func TestDataModel_SchemaReference(t *testing.T) {
	ref := Reference{
		Name:    types.StringValue("test-reference"),
		Subject: types.StringValue("test-subject"),
		Version: types.Int64Value(1),
	}

	assert.Equal(t, "test-reference", ref.Name.ValueString())
	assert.Equal(t, "test-subject", ref.Subject.ValueString())
	assert.Equal(t, int64(1), ref.Version.ValueInt64())
}

func TestDataModel_SchemaReference_NullValues(t *testing.T) {
	ref := Reference{
		Name:    types.StringNull(),
		Subject: types.StringNull(),
		Version: types.Int64Null(),
	}

	assert.True(t, ref.Name.IsNull())
	assert.True(t, ref.Subject.IsNull())
	assert.True(t, ref.Version.IsNull())
}

func TestDataModel_SchemaReference_UnknownValues(t *testing.T) {
	ref := Reference{
		Name:    types.StringUnknown(),
		Subject: types.StringUnknown(),
		Version: types.Int64Unknown(),
	}

	assert.True(t, ref.Name.IsUnknown())
	assert.True(t, ref.Subject.IsUnknown())
	assert.True(t, ref.Version.IsUnknown())
}

func TestDataModel_AllFields(t *testing.T) {
	model := &DataModel{
		Subject:    types.StringValue("test-subject"),
		Schema:     types.StringValue(`{"type": "record", "name": "Test"}`),
		SchemaType: types.StringValue("AVRO"),
		Version:    types.Int64Value(1),
		ID:         types.Int64Value(100),
		ClusterID:  types.StringValue("cluster-123"),
		References: types.ListNull(types.ObjectType{}),
	}

	assert.Equal(t, "test-subject", model.Subject.ValueString())
	assert.Equal(t, `{"type": "record", "name": "Test"}`, model.Schema.ValueString())
	assert.Equal(t, "AVRO", model.SchemaType.ValueString())
	assert.Equal(t, int64(1), model.Version.ValueInt64())
	assert.Equal(t, int64(100), model.ID.ValueInt64())
	assert.Equal(t, "cluster-123", model.ClusterID.ValueString())
	assert.True(t, model.References.IsNull())
}
