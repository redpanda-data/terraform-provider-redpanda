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

	bufvalidate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
)

// Schema attribute type constants.
const (
	AttrTypeString       = "StringAttribute"
	AttrTypeBool         = "BoolAttribute"
	AttrTypeInt32        = "Int32Attribute"
	AttrTypeInt64        = "Int64Attribute"
	AttrTypeFloat64      = "Float64Attribute"
	AttrTypeList         = "ListAttribute"
	AttrTypeSet          = "SetAttribute"
	AttrTypeMap          = "MapAttribute"
	AttrTypeListNested   = "ListNestedAttribute"
	AttrTypeSetNested    = "SetNestedAttribute"
	AttrTypeMapNested    = "MapNestedAttribute"
	AttrTypeSingleNested = "SingleNestedAttribute"
	AttrTypeNumber       = "NumberAttribute"
	AttrTypeObject       = "ObjectAttribute"

	SchemaTypeDatasource = "datasource"

	ModelTypeResource = "ResourceModel"

	ModelTypeDatasource = "DataModel"

	DatasourcePrefix = "Data"

	KindString   = "string"
	KindBool     = "bool"
	KindInt32    = "int32"
	KindInt64    = "int64"
	KindFloat    = "float"
	KindDouble   = "double"
	KindMessage  = "message"
	KindMap      = "map"
	KindRepeated = "repeated"
	KindList     = "list"
	KindSet      = "set"

	elemTypeString  = "types.StringType"
	elemTypeBool    = "types.BoolType"
	elemTypeInt32   = "types.Int32Type"
	elemTypeInt64   = "types.Int64Type"
	elemTypeFloat64 = "types.Float64Type"
)

// ProtoField represents a single field extracted from a protobuf message.
type ProtoField struct {
	Name            string
	Kind            string
	Cardinality     string
	IsOptional      bool
	IsScalarWrapper bool
	OneofName       string
	MapKeyKind      string
	MapValKind      string
	Nested          *ProtoMessage
	EnumName        string
	EnumGoName      string
	EnumProtoPkg    string
	EnumValues      []string

	ValidateRules *bufvalidate.FieldRules
}

// ProtoMessage represents a parsed protobuf message.
type ProtoMessage struct {
	Name string

	GoName string

	ExternalPkgAlias string

	ExternalPkgImport string
	Fields            []ProtoField
}

// FindField returns the proto field with the given snake_case name, or nil
// if no such field exists. A nil receiver returns nil — callers that want
// "no proto info = skip" semantics check the result against nil.
func (m *ProtoMessage) FindField(name string) *ProtoField {
	if m == nil {
		return nil
	}
	for i := range m.Fields {
		if m.Fields[i].Name == name {
			return &m.Fields[i]
		}
	}
	return nil
}

// FindPath walks a dot-separated path from this message and returns the
// terminal field, or nil if any segment fails to resolve. Each non-terminal
// segment must be a message field (so its Nested can be walked). A nil
// receiver returns nil.
func (m *ProtoMessage) FindPath(path string) *ProtoField {
	if m == nil || path == "" {
		return nil
	}
	cur := m
	parts := strings.Split(path, ".")
	for i, part := range parts {
		f := cur.FindField(part)
		if f == nil {
			return nil
		}
		if i == len(parts)-1 {
			return f
		}
		cur = f.Nested
		if cur == nil {
			return nil
		}
	}
	return nil
}

// SchemaAttr is the unified representation of a Terraform schema attribute,
// produced by the merger and consumed by the generator.
type SchemaAttr struct {
	Name string

	ProtoName          string
	AttrType           string
	Description        string
	Required           bool
	Optional           bool
	Computed           bool
	Sensitive          bool
	WriteOnly          bool
	DeprecationMessage string
	PlanModifiers      string

	PlanModifierNames []string
	Validators        string
	Default           string

	MinimalDefault string
	ElementType    string
	NestedAttrs    []SchemaAttr

	FlattenSkip bool
	EnumValues  []string
}

type protoKindInfo struct {
	AttrType    string
	ElementType string
}

var protoKindTable = map[string]protoKindInfo{
	"string":      {AttrTypeString, elemTypeString},
	"bool":        {AttrTypeBool, elemTypeBool},
	"int32":       {AttrTypeInt32, elemTypeInt32},
	"int64":       {AttrTypeInt64, elemTypeInt64},
	"uint32":      {AttrTypeInt64, ""},
	"uint64":      {AttrTypeInt64, ""},
	"float":       {AttrTypeFloat64, elemTypeFloat64},
	"double":      {AttrTypeFloat64, elemTypeFloat64},
	"bytes":       {AttrTypeString, ""},
	"enum":        {AttrTypeString, ""},
	"timestamp":   {AttrTypeString, ""},
	"duration":    {AttrTypeString, ""},
	"json_struct": {AttrTypeString, ""},
}

func goTypeForProtoKind(kind string) string {
	if info, ok := protoKindTable[kind]; ok {
		return info.AttrType
	}
	return AttrTypeString
}

func elementTypeForKind(kind string) string {
	if info, ok := protoKindTable[kind]; ok && info.ElementType != "" {
		return info.ElementType
	}
	return elemTypeString
}

type attrTypeInfo struct {
	ModelType       string
	ValidatorSuffix string
	// TypeExpr and NullExpr are set only for scalar AttrTypes. Collection
	// and object AttrTypes leave them empty — their null expressions are
	// element-dependent and built by NullExpr's non-scalar branch.
	TypeExpr string
	NullExpr string
}

var attrTypeTable = map[string]attrTypeInfo{
	AttrTypeString:  {ModelType: "types.String", ValidatorSuffix: "String", TypeExpr: elemTypeString, NullExpr: "types.StringNull()"},
	AttrTypeBool:    {ModelType: "types.Bool", ValidatorSuffix: "Bool", TypeExpr: elemTypeBool, NullExpr: "types.BoolNull()"},
	AttrTypeInt32:   {ModelType: "types.Int32", ValidatorSuffix: "Int32", TypeExpr: elemTypeInt32, NullExpr: "types.Int32Null()"},
	AttrTypeInt64:   {ModelType: "types.Int64", ValidatorSuffix: "Int64", TypeExpr: elemTypeInt64, NullExpr: "types.Int64Null()"},
	AttrTypeFloat64: {ModelType: "types.Float64", ValidatorSuffix: "Float64", TypeExpr: elemTypeFloat64, NullExpr: "types.Float64Null()"},
	AttrTypeNumber:  {ModelType: "types.Number", TypeExpr: "types.NumberType", NullExpr: "types.NumberNull()"},

	AttrTypeList:         {ModelType: "types.List", ValidatorSuffix: "List"},
	AttrTypeListNested:   {ModelType: "types.List", ValidatorSuffix: "List"},
	AttrTypeSet:          {ModelType: "types.Set", ValidatorSuffix: "Set"},
	AttrTypeSetNested:    {ModelType: "types.Set", ValidatorSuffix: "Set"},
	AttrTypeMap:          {ModelType: "types.Map", ValidatorSuffix: "Map"},
	AttrTypeMapNested:    {ModelType: "types.Map", ValidatorSuffix: "Map"},
	AttrTypeObject:       {ModelType: "types.Object", ValidatorSuffix: "Object"},
	AttrTypeSingleNested: {ModelType: "types.Object", ValidatorSuffix: "Object"},
}

func modelGoTypeForAttr(attrType string) string {
	if info, ok := attrTypeTable[attrType]; ok {
		return info.ModelType
	}
	return "types.String"
}

// scalarNullExprForAttr returns the Terraform null expression for a scalar
// AttrType. Known non-scalar AttrTypes return ("", nil) — callers check the
// empty result to fall through to non-scalar handling. Unknown AttrTypes
// return a non-nil error so schemagen drift surfaces at codegen time.
func scalarNullExprForAttr(attrType string) (string, error) {
	info, ok := attrTypeTable[attrType]
	if !ok {
		return "", fmt.Errorf("schemagen: unknown AttrType %q in scalarNullExprForAttr", attrType)
	}
	return info.NullExpr, nil
}

// NullExprOptions parameterises NullExpr for the three call sites that need
// slightly different output shapes.
type NullExprOptions struct {
	HelperPrefix string
	HelperPkg    string
	SkipScalars  bool
}

// NullExpr returns the framework null expression for an attribute.
// Exhaustive over every defined AttrType — adding a new AttrType constant
// without extending this switch produces a clear schemagen error at codegen
// time, before any *_gen.go file is written.
//
// Returns ("", nil) only when SkipScalars is set and the AttrType is scalar.
// All other "I don't know" cases return a non-nil error.
func NullExpr(a *SchemaAttr, opts NullExprOptions) (string, error) {
	e, err := scalarNullExprForAttr(a.AttrType)
	if err != nil {
		return "", err
	}
	if e != "" {
		if opts.SkipScalars {
			return "", nil
		}
		return e, nil
	}
	helper := opts.HelperPrefix + pathToPascal(a.Name) + "AttrTypes"
	if opts.HelperPkg != "" {
		helper = opts.HelperPkg + "." + helper
	}
	switch a.AttrType {
	case AttrTypeList:
		if a.ElementType == "" {
			return "", fmt.Errorf("schemagen: AttrTypeList field %q missing ElementType (use ListNested for object lists, or set element_type in yaml)", a.Name)
		}
		return fmt.Sprintf("types.ListNull(%s)", a.ElementType), nil
	case AttrTypeSet:
		if a.ElementType == "" {
			return "", fmt.Errorf("schemagen: AttrTypeSet field %q missing ElementType (use SetNested for object sets, or set element_type in yaml)", a.Name)
		}
		return fmt.Sprintf("types.SetNull(%s)", a.ElementType), nil
	case AttrTypeMap:
		if a.ElementType == "" {
			return "", fmt.Errorf("schemagen: AttrTypeMap field %q missing ElementType (use MapNested for object maps, or set element_type in yaml)", a.Name)
		}
		return fmt.Sprintf("types.MapNull(%s)", a.ElementType), nil
	case AttrTypeListNested:
		return fmt.Sprintf("types.ListNull(types.ObjectType{AttrTypes: %s()})", helper), nil
	case AttrTypeSetNested:
		return fmt.Sprintf("types.SetNull(types.ObjectType{AttrTypes: %s()})", helper), nil
	case AttrTypeMapNested:
		return fmt.Sprintf("types.MapNull(types.ObjectType{AttrTypes: %s()})", helper), nil
	case AttrTypeSingleNested, AttrTypeObject:
		return fmt.Sprintf("types.ObjectNull(%s())", helper), nil
	default:
		return "", fmt.Errorf("schemagen: unsupported AttrType %q for NullExpr (field %q)", a.AttrType, a.Name)
	}
}
