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

// NestedType captures enough information to emit a typed Go struct for a
// nested attribute, the attr.Type table that matches it, and the
// As/To converter helpers used by callers.
type NestedType struct {
	TypeName string

	AttrTypesFunc string

	Fields []ModelField

	AttrTypes []AttrTypeEntry

	Path string
}

// AttrTypeEntry is one row in an attr.Type map.
type AttrTypeEntry struct {
	Tag  string
	Expr string
}

// Converter captures the data needed to emit one As<X> / <X>ToObject /
// <X>ToList / <X>ToMap trio at the root-model level for a nested attribute.
type Converter struct {
	RootFieldName string

	ConverterName string

	FieldTag string

	NestedType string

	AttrTypesFunc string

	Kind string
}

func collectNestedTypes(root []SchemaAttr, prefix string) ([]NestedType, error) {
	var out []NestedType
	seen := make(map[string]bool)
	var walk func(path string, attrs []SchemaAttr) error
	walk = func(path string, attrs []SchemaAttr) error {
		for i := range attrs {
			a := &attrs[i]
			if !isNestedMessage(a.AttrType) {
				continue
			}
			childPath := a.Name
			if path != "" {
				childPath = path + "." + a.Name
			}
			typeName := prefix + pathToPascal(childPath) + "Model"
			if seen[typeName] {
				continue
			}
			seen[typeName] = true
			nt := NestedType{
				TypeName:      typeName,
				AttrTypesFunc: prefix + pathToPascal(childPath) + "AttrTypes",
				Path:          childPath,
			}
			for j := range a.NestedAttrs {
				child := &a.NestedAttrs[j]
				expr, err := attrTypeExpr(child, childPath, prefix)
				if err != nil {
					return fmt.Errorf("attr type for %q: %w", childPath+"."+child.Name, err)
				}
				nt.Fields = append(nt.Fields, ModelField{
					Name: toGoFieldName(child.Name),
					Type: modelGoTypeForAttr(child.AttrType),
					Tag:  child.Name,
				})
				nt.AttrTypes = append(nt.AttrTypes, AttrTypeEntry{
					Tag:  child.Name,
					Expr: expr,
				})
			}
			out = append(out, nt)
			if err := walk(childPath, a.NestedAttrs); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk("", root); err != nil {
		return nil, err
	}
	return out, nil
}

func collectConverters(root []SchemaAttr, prefix string) []Converter {
	var out []Converter
	for i := range root {
		a := &root[i]
		var kind string
		switch a.AttrType {
		case AttrTypeSingleNested:
			kind = "single"
		case AttrTypeListNested:
			kind = "list"
		case AttrTypeMapNested, AttrTypeSetNested:
			kind = "map"
		default:
			continue
		}
		typeName := prefix + pathToPascal(a.Name) + "Model"
		out = append(out, Converter{
			RootFieldName: toGoFieldName(a.Name),
			ConverterName: prefix + pathToPascal(a.Name),
			FieldTag:      a.Name,
			NestedType:    typeName,
			AttrTypesFunc: prefix + pathToPascal(a.Name) + "AttrTypes",
			Kind:          kind,
		})
	}
	return out
}

// collectSubConverters gathers Decode*/ToObject helpers for SingleNested types
// that are NOT at the root level (i.e., nested inside other SingleNested
// containers). These are used by NestedPreserveSubBlock in conv_gen templates
// to restore sensitive leaves at arbitrary depth.
func collectSubConverters(root []SchemaAttr, prefix string) []SubConverter {
	var out []SubConverter
	seen := make(map[string]bool)
	var walk func(path string, parentGoType string, children []SchemaAttr)
	walk = func(path string, parentGoType string, children []SchemaAttr) {
		for i := range children {
			a := &children[i]
			if !isNestedMessage(a.AttrType) {
				continue
			}
			childPath := a.Name
			if path != "" {
				childPath = path + "." + a.Name
			}
			fullPascal := prefix + pathToPascal(childPath)
			nestedType := fullPascal + "Model"
			// Decode helpers exist only for SingleNested fields — their
			// parent struct exposes a typed types.Object that can be unpacked
			// in a single helper call. List/Set/Map containers use ranged
			// element iteration instead, so they don't get a Decode emitted
			// here — but we still walk INTO their element types so SingleNested
			// fields nested inside list/set/map elements get Decode helpers.
			if a.AttrType == AttrTypeSingleNested {
				decodeName := "Decode" + prefix + pathToPascal(childPath)
				toObjName := fullPascal + "ToObject"
				if !seen[decodeName] {
					seen[decodeName] = true
					out = append(out, SubConverter{
						DecodeFuncName:   decodeName,
						ToObjectFuncName: toObjName,
						ParentType:       parentGoType,
						FieldName:        toGoFieldName(a.Name),
						NestedType:       nestedType,
						AttrTypesFunc:    fullPascal + "AttrTypes",
					})
				}
			}
			walk(childPath, nestedType, a.NestedAttrs)
		}
	}
	for i := range root {
		a := &root[i]
		if !isNestedMessage(a.AttrType) {
			continue
		}
		rootType := prefix + pathToPascal(a.Name) + "Model"
		walk(a.Name, rootType, a.NestedAttrs)
	}
	return out
}

func attrTypeExpr(a *SchemaAttr, parentPath, prefix string) (string, error) {
	switch a.AttrType {
	case AttrTypeString, AttrTypeBool, AttrTypeInt32, AttrTypeInt64, AttrTypeFloat64, AttrTypeNumber:
		return scalarAttrType(a.AttrType)
	case AttrTypeList:
		return fmt.Sprintf("types.ListType{ElemType: %s}", elementAttrType(a.ElementType)), nil
	case AttrTypeSet:
		return fmt.Sprintf("types.SetType{ElemType: %s}", elementAttrType(a.ElementType)), nil
	case AttrTypeMap:
		return fmt.Sprintf("types.MapType{ElemType: %s}", elementAttrType(a.ElementType)), nil
	case AttrTypeSingleNested, AttrTypeObject:
		path := a.Name
		if parentPath != "" {
			path = parentPath + "." + a.Name
		}
		return fmt.Sprintf("types.ObjectType{AttrTypes: %s()}", prefix+pathToPascal(path)+"AttrTypes"), nil
	case AttrTypeListNested:
		path := a.Name
		if parentPath != "" {
			path = parentPath + "." + a.Name
		}
		return fmt.Sprintf("types.ListType{ElemType: types.ObjectType{AttrTypes: %s()}}",
			prefix+pathToPascal(path)+"AttrTypes"), nil
	case AttrTypeSetNested:
		path := a.Name
		if parentPath != "" {
			path = parentPath + "." + a.Name
		}
		return fmt.Sprintf("types.SetType{ElemType: types.ObjectType{AttrTypes: %s()}}",
			prefix+pathToPascal(path)+"AttrTypes"), nil
	case AttrTypeMapNested:
		path := a.Name
		if parentPath != "" {
			path = parentPath + "." + a.Name
		}
		return fmt.Sprintf("types.MapType{ElemType: types.ObjectType{AttrTypes: %s()}}",
			prefix+pathToPascal(path)+"AttrTypes"), nil
	default:
		return "", fmt.Errorf("schemagen: unsupported AttrType %q for attrTypeExpr", a.AttrType)
	}
}

func scalarAttrType(attrType string) (string, error) {
	switch attrType {
	case AttrTypeString:
		return elemTypeString, nil
	case AttrTypeBool:
		return "types.BoolType", nil
	case AttrTypeInt32:
		return "types.Int32Type", nil
	case AttrTypeInt64:
		return "types.Int64Type", nil
	case AttrTypeFloat64:
		return "types.Float64Type", nil
	case AttrTypeNumber:
		return "types.NumberType", nil
	default:
		return "", fmt.Errorf("schemagen: unsupported scalar AttrType %q", attrType)
	}
}

func elementAttrType(elementType string) string {
	if elementType != "" {
		return elementType
	}
	return elemTypeString
}

func isNestedMessage(attrType string) bool {
	switch attrType {
	case AttrTypeSingleNested, AttrTypeListNested, AttrTypeMapNested, AttrTypeSetNested:
		return true
	default:
		return false
	}
}

func pathToPascal(path string) string {
	parts := strings.Split(path, ".")
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(toGoFieldName(p))
	}
	return b.String()
}
