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

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FieldInfo represents a parsed struct field
type FieldInfo struct {
	Name       string // Go field name (e.g., "KafkaAPI")
	TFSDKTag   string // tfsdk tag value (e.g., "kafka_api")
	TypeName   string // Type name (e.g., "types.Object", "types.String")
	IsObject   bool   // true if types.Object
	IsList     bool   // true if types.List
	IsMap      bool   // true if types.Map
	IsBool     bool   // true if types.Bool or basetypes.BoolValue
	IsString   bool   // true if types.String
	IsInt64    bool   // true if types.Int64
	IsInt32    bool   // true if types.Int32
	IsFloat64  bool   // true if types.Float64
	IsTimeouts bool   // true if timeouts.Value (skip comparison)
}

// NestedTypeInfo represents a nested object type from Get*Type() functions
type NestedTypeInfo struct {
	FuncName   string           // e.g., "GetKafkaAPIType"
	AttrName   string           // e.g., "kafka_api"
	Attributes []NestedAttrInfo // Attributes in this nested object
	NestedRefs []string         // References to other nested types
}

// NestedAttrInfo represents an attribute in a nested object
type NestedAttrInfo struct {
	Name        string // Attribute name (e.g., "url", "enabled")
	TypeName    string // Type (e.g., "types.StringType", "types.BoolType")
	IsObject    bool
	IsList      bool
	NestedType  string // For nested objects/lists, the Get*Type function name
	ElementType string // For lists, the element type
}

// ModelFields holds the parsed fields for a model type
type ModelFields struct {
	ModelType string
	Fields    []FieldInfo
}

// GenerateCompareFile generates the compare function for resource/data models
// modelTypes specifies which model types to generate for (e.g., "ResourceModel", "DataModel")
// If no modelTypes are specified, defaults to "ResourceModel"
// All models are generated into a single file to avoid duplicate function declarations
func GenerateCompareFile(modelsDir, packageName string, modelTypes ...string) error {
	if len(modelTypes) == 0 {
		modelTypes = []string{"ResourceModel"}
	}

	packagePath := filepath.Join(modelsDir, packageName)
	objectDefsPath := filepath.Join(packagePath, "object_definitions.go")
	outputFile := filepath.Join(packagePath, "models_compare_generated.go")

	// Parse nested type definitions (may not exist for simple packages)
	nestedTypes, err := parseNestedTypeDefinitions(objectDefsPath)
	if err != nil {
		// object_definitions.go doesn't exist or couldn't be parsed - use empty map
		fmt.Printf("Note: No nested type definitions found for %s (this is OK for simple models)\n", packageName)
		nestedTypes = make(map[string]NestedTypeInfo)
	}

	// Parse all model types
	var allModels []ModelFields
	for _, modelType := range modelTypes {
		// Determine source file based on model type
		var sourceFile string
		switch modelType {
		case "ResourceModel":
			sourceFile = filepath.Join(packagePath, "resource_model.go")
		case "DataModel":
			sourceFile = filepath.Join(packagePath, "data_model.go")
		default:
			return fmt.Errorf("unknown model type: %s", modelType)
		}

		// Check if source file exists
		if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
			fmt.Printf("Skipping %s/%s: source file does not exist\n", packageName, modelType)
			continue
		}

		// Parse the model struct
		fields, err := parseModelStruct(sourceFile, modelType)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", modelType, err)
		}

		if len(fields) == 0 {
			fmt.Printf("Skipping %s/%s: no fields found\n", packageName, modelType)
			continue
		}

		allModels = append(allModels, ModelFields{
			ModelType: modelType,
			Fields:    fields,
		})
	}

	if len(allModels) == 0 {
		fmt.Printf("No models found for %s, skipping\n", packageName)
		return nil
	}

	// Generate the combined compare file
	code, err := generateCombinedCompareCode(packageName, allModels, nestedTypes)
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	// Format the code
	formatted, err := format.Source([]byte(code))
	if err != nil {
		fmt.Printf("Warning: failed to format generated code: %v\n", err)
		fmt.Printf("Generated code:\n%s\n", code)
		formatted = []byte(code)
	}

	// Write the file
	if err := os.WriteFile(outputFile, formatted, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Generated compare file: %s\n", outputFile)
	return nil
}

// generateCombinedCompareCode generates compare code for multiple models in one file
func generateCombinedCompareCode(packageName string, models []ModelFields, nestedTypes map[string]NestedTypeInfo) (string, error) {
	var buf bytes.Buffer

	// Collect all object fields to determine if we need the types import
	var allObjectFields []FieldInfo
	for _, m := range models {
		for _, f := range m.Fields {
			if f.IsObject {
				allObjectFields = append(allObjectFields, f)
			}
		}
	}

	// Write header
	buf.WriteString(`// Code generated by go generate; DO NOT EDIT.
// Generated by: redpanda/utils/testutil/generator

package ` + packageName + `

import (
`)
	// Only include types import if we have object fields that need deep comparison
	if len(allObjectFields) > 0 {
		buf.WriteString(`	"github.com/hashicorp/terraform-plugin-framework/types"
`)
	}
	buf.WriteString(`	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/compare"
)

`)

	// Generate Compare function for each model
	for _, m := range models {
		code, err := generateCompareMethod(m.ModelType, m.Fields, nestedTypes)
		if err != nil {
			return "", fmt.Errorf("failed to generate compare method for %s: %w", m.ModelType, err)
		}
		buf.WriteString(code)
	}

	// Generate shared nested compare functions (only once)
	generatedFuncs := make(map[string]bool)
	for _, m := range models {
		for _, f := range m.Fields {
			if f.IsObject {
				typeFuncName := findTypeFuncName(f.TFSDKTag, nestedTypes)
				if err := generateNestedCompareFunc(&buf, typeFuncName, nestedTypes, generatedFuncs); err != nil {
					return "", fmt.Errorf("failed to generate nested compare for %s: %w", f.TFSDKTag, err)
				}
			}
		}
	}

	return buf.String(), nil
}

// generateCompareMethod generates just the Compare method for a model (without nested functions)
func generateCompareMethod(modelType string, fields []FieldInfo, nestedTypes map[string]NestedTypeInfo) (string, error) {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf(`// Compare performs comprehensive field-by-field comparison between two %s instances
func (m *%s) Compare(other *%s) []compare.FieldDiff {
	var diffs []compare.FieldDiff

`, modelType, modelType, modelType))

	// Group fields by type for cleaner output
	var stringFields, boolFields, intFields, listFields, mapFields, objectFields []FieldInfo

	for _, f := range fields {
		if f.IsTimeouts {
			continue // Skip timeouts
		}
		switch {
		case f.IsString:
			stringFields = append(stringFields, f)
		case f.IsBool:
			boolFields = append(boolFields, f)
		case f.IsInt32 || f.IsInt64:
			intFields = append(intFields, f)
		case f.IsList:
			listFields = append(listFields, f)
		case f.IsMap:
			mapFields = append(mapFields, f)
		case f.IsObject:
			objectFields = append(objectFields, f)
		}
	}

	// Generate string comparisons
	if len(stringFields) > 0 {
		buf.WriteString("\t// String fields\n")
		for _, f := range stringFields {
			buf.WriteString(fmt.Sprintf("\tcompare.Collect(&diffs, compare.String(%q, m.%s, other.%s))\n", f.TFSDKTag, f.Name, f.Name))
		}
		buf.WriteString("\n")
	}

	// Generate bool comparisons
	if len(boolFields) > 0 {
		buf.WriteString("\t// Bool fields\n")
		for _, f := range boolFields {
			buf.WriteString(fmt.Sprintf("\tcompare.Collect(&diffs, compare.Bool(%q, m.%s, other.%s))\n", f.TFSDKTag, f.Name, f.Name))
		}
		buf.WriteString("\n")
	}

	// Generate int comparisons
	if len(intFields) > 0 {
		buf.WriteString("\t// Int fields\n")
		for _, f := range intFields {
			if f.IsInt32 {
				buf.WriteString(fmt.Sprintf("\tcompare.Collect(&diffs, compare.Int32(%q, m.%s, other.%s))\n", f.TFSDKTag, f.Name, f.Name))
			} else {
				buf.WriteString(fmt.Sprintf("\tcompare.Collect(&diffs, compare.Int64(%q, m.%s, other.%s))\n", f.TFSDKTag, f.Name, f.Name))
			}
		}
		buf.WriteString("\n")
	}

	// Generate list comparisons
	if len(listFields) > 0 {
		buf.WriteString("\t// List fields\n")
		for _, f := range listFields {
			buf.WriteString(fmt.Sprintf("\tcompare.Collect(&diffs, compare.List(%q, m.%s, other.%s))\n", f.TFSDKTag, f.Name, f.Name))
		}
		buf.WriteString("\n")
	}

	// Generate map comparisons
	if len(mapFields) > 0 {
		buf.WriteString("\t// Map fields\n")
		for _, f := range mapFields {
			buf.WriteString(fmt.Sprintf("\tcompare.Collect(&diffs, compare.Map(%q, m.%s, other.%s))\n", f.TFSDKTag, f.Name, f.Name))
		}
		buf.WriteString("\n")
	}

	// Generate object comparisons (deep comparison)
	if len(objectFields) > 0 {
		buf.WriteString("\t// Object fields (deep comparison)\n")
		for _, f := range objectFields {
			typeFuncName := findTypeFuncName(f.TFSDKTag, nestedTypes)
			compareFuncName := typeFuncToCompareFuncName(typeFuncName)
			buf.WriteString(fmt.Sprintf("\tcompare.CollectAll(&diffs, %s(%q, m.%s, other.%s))\n", compareFuncName, f.TFSDKTag, f.Name, f.Name))
		}
		buf.WriteString("\n")
	}

	buf.WriteString("\treturn diffs\n}\n\n")

	return buf.String(), nil
}

// parseModelStruct parses a model file and extracts field information for the specified model type
func parseModelStruct(filePath, modelTypeName string) ([]FieldInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var fields []FieldInfo

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != modelTypeName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				fieldName := field.Names[0].Name
				tfsdkTag := extractTFSDKTag(field.Tag)
				if tfsdkTag == "" {
					continue
				}

				fieldInfo := FieldInfo{
					Name:     fieldName,
					TFSDKTag: tfsdkTag,
					TypeName: typeExprToString(field.Type),
				}

				// Classify the field type
				classifyFieldType(&fieldInfo)

				fields = append(fields, fieldInfo)
			}
		}
	}

	return fields, nil
}

// extractTFSDKTag extracts the tfsdk tag value from a struct tag
func extractTFSDKTag(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}

	tagValue := strings.Trim(tag.Value, "`")
	for _, part := range strings.Split(tagValue, " ") {
		if strings.HasPrefix(part, "tfsdk:") {
			value := strings.TrimPrefix(part, "tfsdk:")
			return strings.Trim(value, `"`)
		}
	}
	return ""
}

// typeExprToString converts a type expression to string
func typeExprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	}
	return ""
}

// classifyFieldType sets the type flags on FieldInfo
func classifyFieldType(f *FieldInfo) {
	switch f.TypeName {
	case "types.String":
		f.IsString = true
	case "types.Bool", "basetypes.BoolValue":
		f.IsBool = true
	case "types.Int64":
		f.IsInt64 = true
	case "types.Int32":
		f.IsInt32 = true
	case "types.Float64":
		f.IsFloat64 = true
	case "types.Object":
		f.IsObject = true
	case "types.List":
		f.IsList = true
	case "types.Map":
		f.IsMap = true
	case "timeouts.Value":
		f.IsTimeouts = true
	}
}

// parseNestedTypeDefinitions parses object_definitions.go to find Get*Type() or get*Type() functions
func parseNestedTypeDefinitions(filePath string) (map[string]NestedTypeInfo, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	nestedTypes := make(map[string]NestedTypeInfo)

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcName := funcDecl.Name.Name

		// Match both exported (Get*Type) and unexported (get*Type) patterns
		isExported := strings.HasPrefix(funcName, "Get") && strings.HasSuffix(funcName, "Type")
		isUnexported := strings.HasPrefix(funcName, "get") && strings.HasSuffix(funcName, "Type")

		if !isExported && !isUnexported {
			continue
		}

		// Extract attribute name from function name (e.g., GetKafkaAPIType -> kafka_api, getCMRType -> cmr)
		attrName := funcNameToAttrName(funcName)

		info := NestedTypeInfo{
			FuncName: funcName,
			AttrName: attrName,
		}

		// Parse the function body to extract attributes
		if funcDecl.Body != nil {
			info.Attributes, info.NestedRefs = extractNestedAttributes(funcDecl.Body)
		}

		nestedTypes[funcName] = info
	}

	return nestedTypes, nil
}

// funcNameToAttrName converts a function name to an attribute name
// e.g., GetKafkaAPIType -> kafka_api, getCMRType -> cmr
func funcNameToAttrName(funcName string) string {
	// Remove Get/get prefix and Type suffix
	name := strings.TrimPrefix(funcName, "Get")
	name = strings.TrimPrefix(name, "get")
	name = strings.TrimSuffix(name, "Type")

	// Convert PascalCase to snake_case
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Check for acronyms (consecutive uppercase)
			if i+1 < len(name) && name[i+1] >= 'a' && name[i+1] <= 'z' {
				result.WriteRune('_')
			} else if i > 0 && name[i-1] >= 'a' && name[i-1] <= 'z' {
				result.WriteRune('_')
			}
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// extractNestedAttributes extracts attribute information from a function body
func extractNestedAttributes(body *ast.BlockStmt) (attrs []NestedAttrInfo, nestedRefs []string) {
	ast.Inspect(body, func(n ast.Node) bool {
		kvExpr, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}

		keyLit, ok := kvExpr.Key.(*ast.BasicLit)
		if !ok || keyLit.Kind != token.STRING {
			return true
		}

		attrName := strings.Trim(keyLit.Value, `"`)
		attr := NestedAttrInfo{Name: attrName}

		// Analyze the value to determine type
		switch v := kvExpr.Value.(type) {
		case *ast.SelectorExpr:
			// e.g., types.StringType, types.BoolType
			if x, ok := v.X.(*ast.Ident); ok {
				attr.TypeName = x.Name + "." + v.Sel.Name
			}
		case *ast.CompositeLit:
			// e.g., types.ListType{...}, types.ObjectType{...}
			if sel, ok := v.Type.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok {
					typeName := x.Name + "." + sel.Sel.Name
					attr.TypeName = typeName
					switch typeName {
					case "types.ObjectType":
						attr.IsObject = true
						if ref := extractAttrTypesNestedRef(v.Elts); ref != "" {
							attr.NestedType = ref
							nestedRefs = append(nestedRefs, ref)
						}
					case "types.ListType":
						attr.IsList = true
						attr.ElementType, attr.IsObject, attr.NestedType = extractListTypeInfo(v.Elts, &nestedRefs)
					}
				}
			}
		}

		attrs = append(attrs, attr)
		return true
	})

	return attrs, nestedRefs
}

// extractAttrTypesNestedRef extracts nested type reference from AttrTypes field in an ObjectType
func extractAttrTypesNestedRef(elts []ast.Expr) string {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "AttrTypes" {
			continue
		}
		call, ok := kv.Value.(*ast.CallExpr)
		if !ok {
			continue
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok {
			continue
		}
		return fn.Name
	}
	return ""
}

// extractListElemNestedType extracts nested type info from a ListType's element type
func extractListElemNestedType(elemValue ast.Expr) (isObject bool, nestedType string) {
	compLit, ok := elemValue.(*ast.CompositeLit)
	if !ok {
		return false, ""
	}
	sel, ok := compLit.Type.(*ast.SelectorExpr)
	if !ok {
		return false, ""
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != "types" || sel.Sel.Name != "ObjectType" {
		return false, ""
	}
	return true, extractAttrTypesNestedRef(compLit.Elts)
}

// extractListTypeInfo extracts element type info from a ListType's elements
func extractListTypeInfo(elts []ast.Expr, nestedRefs *[]string) (elemType string, isObject bool, nestedType string) {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "ElemType" {
			continue
		}
		elemType = extractElementType(kv.Value)
		isObject, nestedType = extractListElemNestedType(kv.Value)
		if nestedType != "" {
			*nestedRefs = append(*nestedRefs, nestedType)
		}
		return elemType, isObject, nestedType
	}
	return "", false, ""
}

// extractElementType extracts element type from a list type expression
func extractElementType(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	case *ast.CompositeLit:
		if sel, ok := v.Type.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok {
				return x.Name + "." + sel.Sel.Name
			}
		}
	}
	return ""
}

// findTypeFuncName finds the type function name for an attribute, trying both exported and unexported variants
func findTypeFuncName(attrName string, nestedTypes map[string]NestedTypeInfo) string {
	// First try the exported variant (Get*Type)
	exportedName := attributeNameToTypeFuncName(attrName)
	if _, ok := nestedTypes[exportedName]; ok {
		return exportedName
	}

	// Try the unexported variant (get*Type)
	unexportedName := "g" + strings.TrimPrefix(exportedName, "G")
	if _, ok := nestedTypes[unexportedName]; ok {
		return unexportedName
	}

	// If not found, return the exported name (will generate a fallback)
	return exportedName
}

// typeFuncToCompareFuncName converts a type function name to a compare function name
func typeFuncToCompareFuncName(typeFuncName string) string {
	// Handle both Get*Type and get*Type
	name := strings.TrimPrefix(typeFuncName, "Get")
	name = strings.TrimPrefix(name, "get")
	name = strings.TrimSuffix(name, "Type")
	return "compare" + name
}

// generateNestedCompareFunc generates a compare function for a nested type
func generateNestedCompareFunc(buf *bytes.Buffer, typeFuncName string, nestedTypes map[string]NestedTypeInfo, generatedFuncs map[string]bool) error {
	compareFuncName := typeFuncToCompareFuncName(typeFuncName)

	if generatedFuncs[compareFuncName] {
		return nil // Already generated
	}
	generatedFuncs[compareFuncName] = true

	typeInfo, ok := nestedTypes[typeFuncName]
	if !ok {
		// If we don't have type info, generate a simple null-check function
		attrName := funcNameToAttrName(typeFuncName)
		fmt.Fprintf(buf, `// %s compares %s objects
func %s(field string, a, b types.Object) []compare.FieldDiff {
	if d := compare.ObjectNullCheck(field, a, b); d != nil {
		return []compare.FieldDiff{*d}
	}
	if a.IsNull() {
		return nil
	}
	// No deep comparison available for %s
	return nil
}

`, compareFuncName, attrName, compareFuncName, attrName)
		return nil
	}

	fmt.Fprintf(buf, `// %s compares %s objects
func %s(field string, a, b types.Object) []compare.FieldDiff {
	if d := compare.ObjectNullCheck(field, a, b); d != nil {
		return []compare.FieldDiff{*d}
	}
	if a.IsNull() {
		return nil
	}

	var diffs []compare.FieldDiff
	aAttrs := a.Attributes()
	bAttrs := b.Attributes()

`, compareFuncName, typeInfo.AttrName, compareFuncName)

	// Sort attributes for consistent output
	attrs := typeInfo.Attributes
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Name < attrs[j].Name
	})

	for _, attr := range attrs {
		fieldPath := fmt.Sprintf(`field + ".%s"`, attr.Name)

		switch {
		case attr.TypeName == "types.StringType":
			fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.String); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.String); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\t\tcompare.Collect(&diffs, compare.String(%s, aVal, bVal))\n", fieldPath)
			buf.WriteString("\t\t}\n\t}\n")

		case attr.TypeName == "types.BoolType":
			fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.Bool); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.Bool); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\t\tcompare.Collect(&diffs, compare.Bool(%s, aVal, bVal))\n", fieldPath)
			buf.WriteString("\t\t}\n\t}\n")

		case attr.TypeName == "types.Int64Type":
			fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.Int64); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.Int64); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\t\tcompare.Collect(&diffs, compare.Int64(%s, aVal, bVal))\n", fieldPath)
			buf.WriteString("\t\t}\n\t}\n")

		case attr.TypeName == "types.Int32Type":
			fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.Int32); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.Int32); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\t\tcompare.Collect(&diffs, compare.Int32(%s, aVal, bVal))\n", fieldPath)
			buf.WriteString("\t\t}\n\t}\n")

		case attr.TypeName == "types.ListType":
			fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.List); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.List); ok {\n", attr.Name)
			fmt.Fprintf(buf, "\t\t\tcompare.Collect(&diffs, compare.List(%s, aVal, bVal))\n", fieldPath)
			buf.WriteString("\t\t}\n\t}\n")

		case attr.TypeName == "types.ObjectType" || attr.IsObject:
			// Nested object - call nested compare function
			if attr.NestedType != "" {
				nestedCompareFuncName := typeFuncToCompareFuncName(attr.NestedType)
				fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.Object); ok {\n", attr.Name)
				fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.Object); ok {\n", attr.Name)
				fmt.Fprintf(buf, "\t\t\tcompare.CollectAll(&diffs, %s(%s, aVal, bVal))\n", nestedCompareFuncName, fieldPath)
				buf.WriteString("\t\t}\n\t}\n")
			} else {
				// Unknown nested object - just check null status
				fmt.Fprintf(buf, "\tif aVal, ok := aAttrs[%q].(types.Object); ok {\n", attr.Name)
				fmt.Fprintf(buf, "\t\tif bVal, ok := bAttrs[%q].(types.Object); ok {\n", attr.Name)
				fmt.Fprintf(buf, "\t\t\tcompare.Collect(&diffs, compare.Object(%s, aVal, bVal))\n", fieldPath)
				buf.WriteString("\t\t}\n\t}\n")
			}
		}
	}

	buf.WriteString("\n\treturn diffs\n}\n\n")

	// Recursively generate compare functions for nested types
	for _, nestedRef := range typeInfo.NestedRefs {
		if err := generateNestedCompareFunc(buf, nestedRef, nestedTypes, generatedFuncs); err != nil {
			return err
		}
	}

	return nil
}
