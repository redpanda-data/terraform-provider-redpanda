package testutil

import (
	"fmt"
	"reflect"
	"testing"

	dstimeouts "github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TypeDefinitionSkipList defines schema attributes that are intentionally not covered by type definitions.
// Key format: "path.to.attribute" (e.g., "aws_private_link.status.vpc_endpoint_connections[].some_field")
// Value: reason for skipping
var TypeDefinitionSkipList = map[string]string{
	// Example: "some_deprecated_field": "Deprecated, will be removed in v2.0"
}

// ValidateSchemaModelAlignment validates that a Terraform resource model and schema are properly aligned.
// Checks that all model fields have corresponding schema attributes and vice versa, and that types match.
func ValidateSchemaModelAlignment(t *testing.T, model any, resourceSchema schema.Schema) {
	t.Helper()

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	modelFields := extractModelFields(modelType)
	schemaAttrs := resourceSchema.Attributes

	for fieldName, tfsdkTag := range modelFields {
		attr, exists := schemaAttrs[tfsdkTag]
		if !exists {
			t.Errorf("Model field %s with tfsdk tag '%s' has no corresponding schema attribute", fieldName, tfsdkTag)
			continue
		}

		field, _ := modelType.FieldByName(fieldName)
		validateTypeCorrespondence(t, field.Type, attr, tfsdkTag)
	}

	for attrName := range schemaAttrs {
		found := false
		for _, tfsdkTag := range modelFields {
			if tfsdkTag == attrName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Schema attribute '%s' has no corresponding model field", attrName)
		}
	}

	t.Logf("Validation complete: %d model fields, %d schema attributes", len(modelFields), len(schemaAttrs))
}

// ValidateDataSourceSchemaModelAlignment validates that a Terraform data source model and schema are properly aligned.
func ValidateDataSourceSchemaModelAlignment(t *testing.T, model any, dataSourceSchema dsschema.Schema) {
	t.Helper()

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	modelFields := extractModelFields(modelType)
	schemaAttrs := dataSourceSchema.Attributes

	for fieldName, tfsdkTag := range modelFields {
		attr, exists := schemaAttrs[tfsdkTag]
		if !exists {
			t.Errorf("Model field %s with tfsdk tag '%s' has no corresponding schema attribute", fieldName, tfsdkTag)
			continue
		}

		field, _ := modelType.FieldByName(fieldName)
		validateDataSourceTypeCorrespondence(t, field.Type, attr, tfsdkTag)
	}

	for attrName := range schemaAttrs {
		found := false
		for _, tfsdkTag := range modelFields {
			if tfsdkTag == attrName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Schema attribute '%s' has no corresponding model field", attrName)
		}
	}

	t.Logf("Validation complete: %d model fields, %d schema attributes", len(modelFields), len(schemaAttrs))
}

// extractModelFields extracts field names and their tfsdk tags from a model struct
func extractModelFields(modelType reflect.Type) map[string]string {
	fields := make(map[string]string)

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		tfsdkTag := field.Tag.Get("tfsdk")
		if tfsdkTag != "" && tfsdkTag != "-" {
			fields[field.Name] = tfsdkTag
		}
	}

	return fields
}

// validateTypeCorrespondence validates that a model field type corresponds to the correct schema attribute type
func validateTypeCorrespondence(t *testing.T, modelFieldType reflect.Type, schemaAttr schema.Attribute, attrName string) {
	t.Helper()

	modelTypeName := getTypeName(modelFieldType)

	switch attr := schemaAttr.(type) {
	case schema.StringAttribute:
		if !isStringType(modelFieldType) {
			t.Errorf("Attribute '%s': schema is StringAttribute but model field is %s", attrName, modelTypeName)
		}

	case schema.BoolAttribute:
		if !isBoolType(modelFieldType) {
			t.Errorf("Attribute '%s': schema is BoolAttribute but model field is %s", attrName, modelTypeName)
		}

	case schema.Int32Attribute:
		if !isInt32Type(modelFieldType) {
			t.Errorf("Attribute '%s': schema is Int32Attribute but model field is %s", attrName, modelTypeName)
		}

	case schema.Int64Attribute:
		if !isInt64Type(modelFieldType) {
			t.Errorf("Attribute '%s': schema is Int64Attribute but model field is %s", attrName, modelTypeName)
		}

	case schema.Float64Attribute:
		if !isFloat64Type(modelFieldType) {
			t.Errorf("Attribute '%s': schema is Float64Attribute but model field is %s", attrName, modelTypeName)
		}

	case schema.SingleNestedAttribute:
		if !isObjectType(modelFieldType) && !isTimeoutsType(modelFieldType) {
			// Allow Go native struct - recursively validate
			if modelFieldType.Kind() == reflect.Struct {
				validateNestedStruct(t, modelFieldType, attr.Attributes, attrName)
			} else {
				t.Errorf("Attribute '%s': schema is SingleNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		} else if isObjectType(modelFieldType) {
			// For types.Object fields, we can't extract the structure from the model
			// but we can validate the schema itself for internal consistency
			validateObjectTypeAgainstSchema(t, attr.Attributes, attrName)
		}

	case schema.ListAttribute:
		// Allow both types.List and Go native slices of primitive types
		if !isListType(modelFieldType) {
			if modelFieldType.Kind() == reflect.Slice {
				elemType := modelFieldType.Elem()
				// Allow []string, []int, etc. for ListAttribute with primitive element types
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrName, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the slice element type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrName, modelTypeName)
			}
		} else {
			// Model field is types.List - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.List, schema has ListAttribute with ElementType %T (element type validation limited)", attrName, attr.ElementType)
		}

	case schema.ListNestedAttribute:
		if !isListType(modelFieldType) {
			// Allow Go native []StructType - recursively validate element type
			if modelFieldType.Kind() == reflect.Slice {
				elemType := modelFieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStruct(t, elemType, attr.NestedObject.Attributes, attrName+"[]")
				} else {
					t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s (expected types.List or []struct)", attrName, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	case schema.MapAttribute:
		if !isMapType(modelFieldType) {
			if modelFieldType.Kind() == reflect.Map {
				// For native Go maps, check the value type (key is always string)
				elemType := modelFieldType.Elem()
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrName, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the map value type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrName, modelTypeName)
			}
		} else {
			// Model field is types.Map - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Map, schema has MapAttribute with ElementType %T (element type validation limited)", attrName, attr.ElementType)
		}

	case schema.MapNestedAttribute:
		if !isMapType(modelFieldType) {
			// Allow Go native map[string]StructType - recursively validate value type
			if modelFieldType.Kind() == reflect.Map {
				elemType := modelFieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStruct(t, elemType, attr.NestedObject.Attributes, attrName+"[*]")
				} else {
					t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s (expected types.Map or map[string]struct)", attrName, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	case schema.SetAttribute:
		if !isSetType(modelFieldType) {
			t.Errorf("Attribute '%s': schema is SetAttribute but model field is %s", attrName, modelTypeName)
		} else {
			// Model field is types.Set - we can't check the element type at runtime
			// Go doesn't have native sets, so this is the only option
			t.Logf("Attribute '%s': model is types.Set, schema has SetAttribute with ElementType %T (element type validation limited)", attrName, attr.ElementType)
		}

	case schema.SetNestedAttribute:
		if !isSetType(modelFieldType) {
			// For sets, we typically need types.Set since Go doesn't have native sets
			// But check if it's a slice and validate the element type anyway
			if modelFieldType.Kind() == reflect.Slice {
				elemType := modelFieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStruct(t, elemType, attr.NestedObject.Attributes, attrName+"[]")
				} else {
					t.Errorf("Attribute '%s': schema is SetNestedAttribute but model field is %s (expected types.Set)", attrName, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is SetNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	default:
		t.Logf("Attribute '%s': unknown schema attribute type %T, skipping type validation", attrName, schemaAttr)
	}
}

// validateDataSourceTypeCorrespondence validates that a model field type corresponds to the correct datasource schema attribute type
func validateDataSourceTypeCorrespondence(t *testing.T, modelFieldType reflect.Type, schemaAttr dsschema.Attribute, attrName string) {
	t.Helper()

	// Get the type name for better error messages
	modelTypeName := getTypeName(modelFieldType)

	switch attr := schemaAttr.(type) {
	case dsschema.StringAttribute:
		if !isStringType(modelFieldType) {
			t.Errorf("Attribute '%s': schema is StringAttribute but model field is %s", attrName, modelTypeName)
		}

	case dsschema.BoolAttribute:
		if !isBoolType(modelFieldType) {
			t.Errorf("Attribute '%s': schema is BoolAttribute but model field is %s", attrName, modelTypeName)
		}

	case dsschema.Int32Attribute:
		if !isInt32Type(modelFieldType) {
			t.Errorf("Attribute '%s': schema is Int32Attribute but model field is %s", attrName, modelTypeName)
		}

	case dsschema.Int64Attribute:
		if !isInt64Type(modelFieldType) {
			t.Errorf("Attribute '%s': schema is Int64Attribute but model field is %s", attrName, modelTypeName)
		}

	case dsschema.Float64Attribute:
		if !isFloat64Type(modelFieldType) {
			t.Errorf("Attribute '%s': schema is Float64Attribute but model field is %s", attrName, modelTypeName)
		}

	case dsschema.SingleNestedAttribute:
		if !isObjectType(modelFieldType) && !isTimeoutsType(modelFieldType) {
			// Allow Go native struct - recursively validate
			if modelFieldType.Kind() == reflect.Struct {
				validateNestedStructDataSource(t, modelFieldType, attr.Attributes, attrName)
			} else {
				t.Errorf("Attribute '%s': schema is SingleNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	case dsschema.ListAttribute:
		// Allow both types.List and Go native slices of primitive types
		if !isListType(modelFieldType) {
			if modelFieldType.Kind() == reflect.Slice {
				elemType := modelFieldType.Elem()
				// Allow []string, []int, etc. for ListAttribute with primitive element types
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrName, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the slice element type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrName, modelTypeName)
			}
		} else {
			// Model field is types.List - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.List, schema has ListAttribute with ElementType %T (element type validation limited)", attrName, attr.ElementType)
		}

	case dsschema.ListNestedAttribute:
		if !isListType(modelFieldType) {
			// Allow Go native []StructType - recursively validate element type
			if modelFieldType.Kind() == reflect.Slice {
				elemType := modelFieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStructDataSource(t, elemType, attr.NestedObject.Attributes, attrName+"[]")
				} else {
					t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s (expected types.List or []struct)", attrName, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	case dsschema.MapAttribute:
		if !isMapType(modelFieldType) {
			if modelFieldType.Kind() == reflect.Map {
				// For native Go maps, check the value type
				elemType := modelFieldType.Elem()
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrName, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the map value type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrName, modelTypeName)
			}
		} else {
			// Model field is types.Map - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Map, schema has MapAttribute with ElementType %T (element type validation limited)", attrName, attr.ElementType)
		}

	case dsschema.MapNestedAttribute:
		if !isMapType(modelFieldType) {
			// Allow Go native map[string]StructType - recursively validate value type
			if modelFieldType.Kind() == reflect.Map {
				elemType := modelFieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStructDataSource(t, elemType, attr.NestedObject.Attributes, attrName+"[*]")
				} else {
					t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s (expected types.Map or map[string]struct)", attrName, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	case dsschema.SetAttribute:
		if !isSetType(modelFieldType) {
			t.Errorf("Attribute '%s': schema is SetAttribute but model field is %s", attrName, modelTypeName)
		} else {
			// Model field is types.Set - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Set, schema has SetAttribute with ElementType %T (element type validation limited)", attrName, attr.ElementType)
		}

	case dsschema.SetNestedAttribute:
		if !isSetType(modelFieldType) {
			// For sets, we typically need types.Set since Go doesn't have native sets
			// But check if it's a slice and validate the element type anyway
			if modelFieldType.Kind() == reflect.Slice {
				elemType := modelFieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStructDataSource(t, elemType, attr.NestedObject.Attributes, attrName+"[]")
				} else {
					t.Errorf("Attribute '%s': schema is SetNestedAttribute but model field is %s (expected types.Set)", attrName, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is SetNestedAttribute but model field is %s", attrName, modelTypeName)
			}
		}

	default:
		t.Logf("Attribute '%s': unknown schema attribute type %T, skipping type validation", attrName, schemaAttr)
	}
}

// Type checking helper functions

func getTypeName(t reflect.Type) string {
	// Use t.String() for unnamed types (slices, maps, etc.). Resolves issue with blank names for go types on the models
	if t.Name() == "" {
		return t.String()
	}

	if t.PkgPath() != "" {
		return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
	}
	return t.Name()
}

func isStringType(t reflect.Type) bool {
	return t == reflect.TypeOf(types.String{}) || t == reflect.TypeOf(basetypes.StringValue{})
}

func isBoolType(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Bool{}) || t == reflect.TypeOf(basetypes.BoolValue{})
}

func isInt32Type(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Int32{}) || t == reflect.TypeOf(basetypes.Int32Value{})
}

func isInt64Type(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Int64{}) || t == reflect.TypeOf(basetypes.Int64Value{})
}

func isFloat64Type(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Float64{}) || t == reflect.TypeOf(basetypes.Float64Value{})
}

func isObjectType(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Object{}) || t == reflect.TypeOf(basetypes.ObjectValue{})
}

func isListType(t reflect.Type) bool {
	return t == reflect.TypeOf(types.List{}) || t == reflect.TypeOf(basetypes.ListValue{})
}

func isMapType(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Map{}) || t == reflect.TypeOf(basetypes.MapValue{})
}

func isSetType(t reflect.Type) bool {
	return t == reflect.TypeOf(types.Set{}) || t == reflect.TypeOf(basetypes.SetValue{})
}

func isTimeoutsType(t reflect.Type) bool {
	return t == reflect.TypeOf(timeouts.Value{}) || t == reflect.TypeOf(dstimeouts.Value{})
}

func validateNestedStruct(t *testing.T, structType reflect.Type, schemaAttrs map[string]schema.Attribute, path string) {
	t.Helper()

	nestedFields := extractModelFields(structType)

	for fieldName, tfsdkTag := range nestedFields {
		attr, exists := schemaAttrs[tfsdkTag]
		if !exists {
			t.Errorf("Nested field %s.%s with tfsdk '%s' has no corresponding schema attribute", path, fieldName, tfsdkTag)
			continue
		}

		field, _ := structType.FieldByName(fieldName)
		validateNestedFieldType(t, field.Type, attr, fmt.Sprintf("%s.%s", path, tfsdkTag))
	}

	for attrName := range schemaAttrs {
		found := false
		for _, tfsdkTag := range nestedFields {
			if tfsdkTag == attrName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Nested schema attribute '%s.%s' has no corresponding model field", path, attrName)
		}
	}
}

func validateNestedStructDataSource(t *testing.T, structType reflect.Type, schemaAttrs map[string]dsschema.Attribute, path string) {
	t.Helper()

	nestedFields := extractModelFields(structType)

	for fieldName, tfsdkTag := range nestedFields {
		attr, exists := schemaAttrs[tfsdkTag]
		if !exists {
			t.Errorf("Nested field %s.%s with tfsdk '%s' has no corresponding schema attribute", path, fieldName, tfsdkTag)
			continue
		}

		field, _ := structType.FieldByName(fieldName)
		validateNestedFieldTypeDataSource(t, field.Type, attr, fmt.Sprintf("%s.%s", path, tfsdkTag))
	}

	for attrName := range schemaAttrs {
		found := false
		for _, tfsdkTag := range nestedFields {
			if tfsdkTag == attrName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Nested schema attribute '%s.%s' has no corresponding model field", path, attrName)
		}
	}
}

func validateElementType(t *testing.T, schemaElementType attr.Type, expectedKind reflect.Kind, attrPath string) {
	t.Helper()

	switch expectedKind {
	case reflect.String:
		if schemaElementType != types.StringType {
			t.Errorf("Attribute '%s': schema ElementType is %T but expected types.StringType (model is []string)", attrPath, schemaElementType)
		}
	case reflect.Bool:
		if schemaElementType != types.BoolType {
			t.Errorf("Attribute '%s': schema ElementType is %T but expected types.BoolType (model is []bool)", attrPath, schemaElementType)
		}
	case reflect.Int32:
		if schemaElementType != types.Int32Type {
			t.Errorf("Attribute '%s': schema ElementType is %T but expected types.Int32Type (model is []int32)", attrPath, schemaElementType)
		}
	case reflect.Int64, reflect.Int:
		if schemaElementType != types.Int64Type {
			t.Errorf("Attribute '%s': schema ElementType is %T but expected types.Int64Type (model is []int64 or []int)", attrPath, schemaElementType)
		}
	case reflect.Float64:
		if schemaElementType != types.Float64Type {
			t.Errorf("Attribute '%s': schema ElementType is %T but expected types.Float64Type (model is []float64)", attrPath, schemaElementType)
		}
	default:
		t.Errorf("Attribute '%s': unsupported element kind %v for ElementType validation", attrPath, expectedKind)
	}
}

// validateObjectTypeAgainstSchema recursively validates nested schema attributes for types.Object fields
// Since we can't extract type information from types.Object at runtime, this validates the schema structure itself
func validateObjectTypeAgainstSchema(t *testing.T, nestedAttrs map[string]schema.Attribute, path string) {
	t.Helper()

	for attrName, schemaAttr := range nestedAttrs {
		attrPath := path + "." + attrName

		switch attr := schemaAttr.(type) {
		case schema.ListAttribute:
			validateSchemaElementType(t, attr.ElementType, attrPath)
		case schema.MapAttribute:
			validateSchemaElementType(t, attr.ElementType, attrPath)
		case schema.SetAttribute:
			validateSchemaElementType(t, attr.ElementType, attrPath)
		case schema.SingleNestedAttribute:
			validateObjectTypeAgainstSchema(t, attr.Attributes, attrPath)
		case schema.ListNestedAttribute:
			validateObjectTypeAgainstSchema(t, attr.NestedObject.Attributes, attrPath+"[]")
		case schema.MapNestedAttribute:
			validateObjectTypeAgainstSchema(t, attr.NestedObject.Attributes, attrPath+"[*]")
		case schema.SetNestedAttribute:
			validateObjectTypeAgainstSchema(t, attr.NestedObject.Attributes, attrPath+"[]")
		}
	}
}

func validateSchemaElementType(t *testing.T, elemType attr.Type, attrPath string) {
	t.Helper()
	if elemType == nil {
		t.Errorf("Attribute '%s': schema ElementType is nil", attrPath)
		return
	}
	t.Logf("Attribute '%s': schema has ElementType %T", attrPath, elemType)
}

// ValidateSchemaAgainstObjectType validates that a schema's nested attributes match the expected object type definition
// This function should be called with the type definition map (e.g., getMtlsType()) to validate against the schema
func ValidateSchemaAgainstObjectType(t *testing.T, schemaAttrs map[string]schema.Attribute, attrTypes map[string]attr.Type, path string) {
	t.Helper()

	for attrName, expectedType := range attrTypes {
		schemaAttr, exists := schemaAttrs[attrName]
		if !exists {
			t.Errorf("Type definition has attribute '%s.%s' but schema does not", path, attrName)
			continue
		}

		attrPath := path + "." + attrName

		switch expectedType {
		case types.StringType:
			if _, ok := schemaAttr.(schema.StringAttribute); !ok {
				t.Errorf("Attribute '%s': type definition expects StringType but schema is %T", attrPath, schemaAttr)
			}
		case types.BoolType:
			if _, ok := schemaAttr.(schema.BoolAttribute); !ok {
				t.Errorf("Attribute '%s': type definition expects BoolType but schema is %T", attrPath, schemaAttr)
			}
		case types.Int64Type:
			if _, ok := schemaAttr.(schema.Int64Attribute); !ok {
				t.Errorf("Attribute '%s': type definition expects Int64Type but schema is %T", attrPath, schemaAttr)
			}
		case types.Int32Type:
			if _, ok := schemaAttr.(schema.Int32Attribute); !ok {
				t.Errorf("Attribute '%s': type definition expects Int32Type but schema is %T", attrPath, schemaAttr)
			}
		case types.Float64Type:
			if _, ok := schemaAttr.(schema.Float64Attribute); !ok {
				t.Errorf("Attribute '%s': type definition expects Float64Type but schema is %T", attrPath, schemaAttr)
			}
		case types.NumberType:
			if _, ok := schemaAttr.(schema.NumberAttribute); !ok {
				t.Errorf("Attribute '%s': type definition expects NumberType but schema is %T", attrPath, schemaAttr)
			}
		default:
			switch expectedAttr := expectedType.(type) {
			case types.ListType:
				switch listSchemaAttr := schemaAttr.(type) {
				case schema.ListAttribute:
					if listSchemaAttr.ElementType != expectedAttr.ElemType {
						t.Errorf("Attribute '%s': schema ElementType is %T but type definition expects %T", attrPath, listSchemaAttr.ElementType, expectedAttr.ElemType)
					}
				case schema.ListNestedAttribute:
					if objType, ok := expectedAttr.ElemType.(types.ObjectType); ok {
						ValidateSchemaAgainstObjectType(t, listSchemaAttr.NestedObject.Attributes, objType.AttrTypes, attrPath+"[]")
					} else {
						t.Errorf("Attribute '%s': schema is ListNestedAttribute but type definition expects ListType with %T (should be ObjectType)", attrPath, expectedAttr.ElemType)
					}
				default:
					t.Errorf("Attribute '%s': type definition expects ListType but schema is %T", attrPath, schemaAttr)
				}
			case types.MapType:
				if mapAttr, ok := schemaAttr.(schema.MapAttribute); ok {
					if mapAttr.ElementType != expectedAttr.ElemType {
						t.Errorf("Attribute '%s': schema ElementType is %T but type definition expects %T", attrPath, mapAttr.ElementType, expectedAttr.ElemType)
					}
				} else {
					t.Errorf("Attribute '%s': type definition expects MapType but schema is %T", attrPath, schemaAttr)
				}
			case types.SetType:
				if setAttr, ok := schemaAttr.(schema.SetAttribute); ok {
					if setAttr.ElementType != expectedAttr.ElemType {
						t.Errorf("Attribute '%s': schema ElementType is %T but type definition expects %T", attrPath, setAttr.ElementType, expectedAttr.ElemType)
					}
				} else {
					t.Errorf("Attribute '%s': type definition expects SetType but schema is %T", attrPath, schemaAttr)
				}
			case types.ObjectType:
				if singleNested, ok := schemaAttr.(schema.SingleNestedAttribute); ok {
					ValidateSchemaAgainstObjectType(t, singleNested.Attributes, expectedAttr.AttrTypes, attrPath)
				} else {
					t.Errorf("Attribute '%s': type definition expects ObjectType but schema is %T", attrPath, schemaAttr)
				}
			default:
				t.Logf("Attribute '%s': unknown expected type %T, skipping validation", attrPath, expectedType)
			}
		}
	}

	// Reverse check: verify all schema attributes have corresponding type definitions
	for schemaAttrName := range schemaAttrs {
		if _, exists := attrTypes[schemaAttrName]; exists {
			continue
		}
		attrPath := path + "." + schemaAttrName
		// Check if this attribute is in the skip list
		if reason, skipped := TypeDefinitionSkipList[attrPath]; skipped {
			t.Logf("Skipping coverage check for '%s': %s", attrPath, reason)
			continue
		}
		t.Errorf("Schema attribute '%s' has no corresponding type definition (add to TypeDefinitionSkipList if intentional)", attrPath)
	}
}

// validateNestedFieldType validates a field within a nested struct, supporting both framework and Go native types
func validateNestedFieldType(t *testing.T, fieldType reflect.Type, schemaAttr schema.Attribute, attrPath string) {
	t.Helper()

	modelTypeName := getTypeName(fieldType)

	switch attr := schemaAttr.(type) {
	case schema.StringAttribute:
		// Allow both types.String and Go native string in nested structs
		if !isStringType(fieldType) && fieldType.Kind() != reflect.String {
			t.Errorf("Attribute '%s': schema is StringAttribute but model field is %s", attrPath, modelTypeName)
		}

	case schema.BoolAttribute:
		if !isBoolType(fieldType) && fieldType.Kind() != reflect.Bool {
			t.Errorf("Attribute '%s': schema is BoolAttribute but model field is %s", attrPath, modelTypeName)
		}

	case schema.Int32Attribute:
		if !isInt32Type(fieldType) && fieldType.Kind() != reflect.Int32 {
			t.Errorf("Attribute '%s': schema is Int32Attribute but model field is %s", attrPath, modelTypeName)
		}

	case schema.Int64Attribute:
		if !isInt64Type(fieldType) && fieldType.Kind() != reflect.Int64 {
			t.Errorf("Attribute '%s': schema is Int64Attribute but model field is %s", attrPath, modelTypeName)
		}

	case schema.Float64Attribute:
		if !isFloat64Type(fieldType) && fieldType.Kind() != reflect.Float64 {
			t.Errorf("Attribute '%s': schema is Float64Attribute but model field is %s", attrPath, modelTypeName)
		}

	case schema.SingleNestedAttribute:
		// Allow both types.Object and Go native struct
		if !isObjectType(fieldType) && !isTimeoutsType(fieldType) {
			if fieldType.Kind() == reflect.Struct {
				// Recursively validate the nested struct
				validateNestedStruct(t, fieldType, attr.Attributes, attrPath)
			} else {
				t.Errorf("Attribute '%s': schema is SingleNestedAttribute but model field is %s", attrPath, modelTypeName)
			}
		}

	case schema.ListAttribute:
		// Allow both types.List and Go native []T for primitive types
		if !isListType(fieldType) {
			if fieldType.Kind() == reflect.Slice {
				elemType := fieldType.Elem()
				// Allow []string, []int, etc. for ListAttribute with primitive element types
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrPath, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the slice element type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrPath)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrPath, modelTypeName)
			}
		} else {
			// Model field is types.List - we can't check the element type at runtime,
			// but we should at least log a warning if ElementType seems wrong
			// Note: types.List doesn't expose its element type via reflection, so we can only do basic checks
			t.Logf("Attribute '%s': model is types.List, schema has ListAttribute with ElementType %T (element type validation limited)", attrPath, attr.ElementType)
		}

	case schema.ListNestedAttribute:
		// Allow both types.List and Go native []StructType
		if !isListType(fieldType) {
			if fieldType.Kind() == reflect.Slice {
				elemType := fieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					// Recursively validate the slice element struct
					validateNestedStruct(t, elemType, attr.NestedObject.Attributes, attrPath+"[]")
				} else {
					t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s (expected slice of structs)", attrPath, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s", attrPath, modelTypeName)
			}
		}

	case schema.MapAttribute:
		if !isMapType(fieldType) {
			if fieldType.Kind() == reflect.Map {
				// For native Go maps, check the value type
				elemType := fieldType.Elem()
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrPath, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the map value type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrPath)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrPath, modelTypeName)
			}
		} else {
			// Model field is types.Map - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Map, schema has MapAttribute with ElementType %T (element type validation limited)", attrPath, attr.ElementType)
		}

	case schema.MapNestedAttribute:
		// Allow both types.Map and Go native map[string]StructType
		if !isMapType(fieldType) {
			if fieldType.Kind() == reflect.Map {
				elemType := fieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					// Recursively validate the map value struct
					validateNestedStruct(t, elemType, attr.NestedObject.Attributes, attrPath+"[*]")
				} else {
					t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s (expected map of structs)", attrPath, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s", attrPath, modelTypeName)
			}
		}

	case schema.SetAttribute:
		if !isSetType(fieldType) {
			t.Errorf("Attribute '%s': schema is SetAttribute but model field is %s", attrPath, modelTypeName)
		} else {
			// Model field is types.Set - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Set, schema has SetAttribute with ElementType %T (element type validation limited)", attrPath, attr.ElementType)
		}

	case schema.SetNestedAttribute:
		// For SetNestedAttribute, we expect types.Set (Go doesn't have native set types)
		if !isSetType(fieldType) {
			t.Errorf("Attribute '%s': schema is SetNestedAttribute but model field is %s", attrPath, modelTypeName)
		}

	default:
		t.Logf("Attribute '%s': unknown schema attribute type %T, skipping type validation", attrPath, schemaAttr)
	}
}

// validateNestedFieldTypeDataSource validates a field within a nested struct for data sources
func validateNestedFieldTypeDataSource(t *testing.T, fieldType reflect.Type, schemaAttr dsschema.Attribute, attrPath string) {
	t.Helper()

	modelTypeName := getTypeName(fieldType)

	switch attr := schemaAttr.(type) {
	case dsschema.StringAttribute:
		if !isStringType(fieldType) && fieldType.Kind() != reflect.String {
			t.Errorf("Attribute '%s': schema is StringAttribute but model field is %s", attrPath, modelTypeName)
		}

	case dsschema.BoolAttribute:
		if !isBoolType(fieldType) && fieldType.Kind() != reflect.Bool {
			t.Errorf("Attribute '%s': schema is BoolAttribute but model field is %s", attrPath, modelTypeName)
		}

	case dsschema.Int32Attribute:
		if !isInt32Type(fieldType) && fieldType.Kind() != reflect.Int32 {
			t.Errorf("Attribute '%s': schema is Int32Attribute but model field is %s", attrPath, modelTypeName)
		}

	case dsschema.Int64Attribute:
		if !isInt64Type(fieldType) && fieldType.Kind() != reflect.Int64 {
			t.Errorf("Attribute '%s': schema is Int64Attribute but model field is %s", attrPath, modelTypeName)
		}

	case dsschema.Float64Attribute:
		if !isFloat64Type(fieldType) && fieldType.Kind() != reflect.Float64 {
			t.Errorf("Attribute '%s': schema is Float64Attribute but model field is %s", attrPath, modelTypeName)
		}

	case dsschema.SingleNestedAttribute:
		if !isObjectType(fieldType) && !isTimeoutsType(fieldType) {
			if fieldType.Kind() == reflect.Struct {
				validateNestedStructDataSource(t, fieldType, attr.Attributes, attrPath)
			} else {
				t.Errorf("Attribute '%s': schema is SingleNestedAttribute but model field is %s", attrPath, modelTypeName)
			}
		}

	case dsschema.ListAttribute:
		if !isListType(fieldType) {
			if fieldType.Kind() == reflect.Slice {
				elemType := fieldType.Elem()
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrPath, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the slice element type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrPath)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListAttribute but model field is %s", attrPath, modelTypeName)
			}
		} else {
			// Model field is types.List - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.List, schema has ListAttribute with ElementType %T (element type validation limited)", attrPath, attr.ElementType)
		}

	case dsschema.ListNestedAttribute:
		if !isListType(fieldType) {
			if fieldType.Kind() == reflect.Slice {
				elemType := fieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStructDataSource(t, elemType, attr.NestedObject.Attributes, attrPath+"[]")
				} else {
					t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s (expected slice of structs)", attrPath, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is ListNestedAttribute but model field is %s", attrPath, modelTypeName)
			}
		}

	case dsschema.MapAttribute:
		if !isMapType(fieldType) {
			if fieldType.Kind() == reflect.Map {
				// For native Go maps, check the value type
				elemType := fieldType.Elem()
				if elemType.Kind() != reflect.String && elemType.Kind() != reflect.Int &&
					elemType.Kind() != reflect.Int32 && elemType.Kind() != reflect.Int64 &&
					elemType.Kind() != reflect.Bool && elemType.Kind() != reflect.Float64 {
					t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrPath, modelTypeName)
				} else {
					// Validate that the schema's ElementType matches the map value type
					validateElementType(t, attr.ElementType, elemType.Kind(), attrPath)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapAttribute but model field is %s", attrPath, modelTypeName)
			}
		} else {
			// Model field is types.Map - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Map, schema has MapAttribute with ElementType %T (element type validation limited)", attrPath, attr.ElementType)
		}

	case dsschema.MapNestedAttribute:
		if !isMapType(fieldType) {
			if fieldType.Kind() == reflect.Map {
				elemType := fieldType.Elem()
				if elemType.Kind() == reflect.Struct {
					validateNestedStructDataSource(t, elemType, attr.NestedObject.Attributes, attrPath+"[*]")
				} else {
					t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s (expected map of structs)", attrPath, modelTypeName)
				}
			} else {
				t.Errorf("Attribute '%s': schema is MapNestedAttribute but model field is %s", attrPath, modelTypeName)
			}
		}

	case dsschema.SetAttribute:
		if !isSetType(fieldType) {
			t.Errorf("Attribute '%s': schema is SetAttribute but model field is %s", attrPath, modelTypeName)
		} else {
			// Model field is types.Set - we can't check the element type at runtime
			t.Logf("Attribute '%s': model is types.Set, schema has SetAttribute with ElementType %T (element type validation limited)", attrPath, attr.ElementType)
		}

	case dsschema.SetNestedAttribute:
		if !isSetType(fieldType) {
			t.Errorf("Attribute '%s': schema is SetNestedAttribute but model field is %s", attrPath, modelTypeName)
		}

	default:
		t.Logf("Attribute '%s': unknown schema attribute type %T, skipping type validation", attrPath, schemaAttr)
	}
}
