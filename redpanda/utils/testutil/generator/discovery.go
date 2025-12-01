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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const (
	testutilDir = "testutil"
)

// ModelResourcePair represents a discovered model and its corresponding resource
type ModelResourcePair struct {
	Name           string // Human-readable name (e.g., "Cluster")
	Type           string // "Resource" or "DataSource"
	PackageName    string // Package name (e.g., "cluster")
	ModelStruct    string // Model struct name (e.g., "ResourceModel")
	ResourceStruct string // Resource struct name (e.g., "Cluster")
	ModelImport    string // Import path for model package
	ResourceImport string // Import path for resource package
}

// DiscoverModelResourcePairs scans the models directory and discovers all model-resource pairs
func DiscoverModelResourcePairs(modelsDir string) ([]ModelResourcePair, error) {
	var pairs []ModelResourcePair

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == testutilDir {
			continue
		}

		packageName := entry.Name()
		packagePath := filepath.Join(modelsDir, packageName)

		resourceModelPath := filepath.Join(packagePath, "resource_model.go")
		if _, err := os.Stat(resourceModelPath); err == nil {
			resourceStruct, err := findResourceStruct(packageName)
			if err != nil {
				fmt.Printf("Warning: could not find resource struct for %s: %v\n", packageName, err)
				resourceStruct = capitalizeFirst(packageName)
			}

			pair := ModelResourcePair{
				Name:           capitalizeFirst(packageName),
				Type:           "Resource",
				PackageName:    packageName,
				ModelStruct:    "ResourceModel",
				ResourceStruct: resourceStruct,
				ModelImport:    fmt.Sprintf("github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/%s", packageName),
				ResourceImport: fmt.Sprintf("github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/%s", packageName),
			}
			pairs = append(pairs, pair)
		}

		dataModelPath := filepath.Join(packagePath, "data_model.go")
		if _, err := os.Stat(dataModelPath); err == nil {
			dataSourceStruct, err := findDataSourceStruct(packageName)
			if err != nil {
				fmt.Printf("Warning: could not find data source struct for %s: %v\n", packageName, err)
				dataSourceStruct = "DataSource" + capitalizeFirst(packageName)
			}

			pair := ModelResourcePair{
				Name:           capitalizeFirst(packageName),
				Type:           "DataSource",
				PackageName:    packageName,
				ModelStruct:    "DataModel",
				ResourceStruct: dataSourceStruct,
				ModelImport:    fmt.Sprintf("github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/%s", packageName),
				ResourceImport: fmt.Sprintf("github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/%s", packageName),
			}
			pairs = append(pairs, pair)
		}
	}

	flatFilePairs, err := discoverFlatFileModels(modelsDir)
	if err != nil {
		fmt.Printf("Warning: error discovering flat file models: %v\n", err)
	} else {
		pairs = append(pairs, flatFilePairs...)
	}

	return pairs, nil
}

// findResourceStruct finds the resource struct name by parsing the resource file
func findResourceStruct(packageName string) (string, error) {
	patterns := []string{
		fmt.Sprintf("resource_%s.go", packageName),
		"resource.go",
	}

	modelsDir, _ := filepath.Abs(".")
	// If we're in testutil/ (the one used for this testing) go up one level
	if filepath.Base(modelsDir) == testutilDir {
		modelsDir = filepath.Dir(modelsDir)
	}

	// Resources are in ../resources relative to models directory
	resourcesPath := filepath.Join(filepath.Dir(modelsDir), "resources", packageName)

	for _, pattern := range patterns {
		filePath := filepath.Join(resourcesPath, pattern)
		if _, err := os.Stat(filePath); err == nil {
			structName, err := findStructImplementingInterface(filePath, "resource.Resource")
			if err == nil {
				return structName, nil
			}
		}
	}

	return capitalizeFirst(packageName), nil
}

// findDataSourceStruct finds the data source struct name by parsing the data source file
func findDataSourceStruct(packageName string) (string, error) {
	patterns := []string{
		fmt.Sprintf("data_%s.go", packageName),
		"data.go",
	}

	modelsDir, _ := filepath.Abs(".")
	if filepath.Base(modelsDir) == testutilDir {
		modelsDir = filepath.Dir(modelsDir)
	}

	resourcesPath := filepath.Join(filepath.Dir(modelsDir), "resources", packageName)

	for _, pattern := range patterns {
		filePath := filepath.Join(resourcesPath, pattern)
		if _, err := os.Stat(filePath); err == nil {
			structName, err := findStructImplementingInterface(filePath, "datasource.DataSource")
			if err == nil {
				return structName, nil
			}
		}
	}

	return "DataSource" + capitalizeFirst(packageName), nil
}

// findStructImplementingInterface parses a Go file and finds a struct that likely implements the given interface
func findStructImplementingInterface(filePath, interfaceName string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	var structs []string

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			_, isStruct := typeSpec.Type.(*ast.StructType)
			if isStruct {
				structs = append(structs, typeSpec.Name.Name)
			}
		}
	}

	if len(structs) == 0 {
		return "", fmt.Errorf("no struct found in %s", filePath)
	}

	// For data sources, prefer structs starting with "DataSource"
	// For resources, prefer structs NOT starting with "DataSource"
	isDataSource := strings.Contains(interfaceName, "datasource")

	for _, name := range structs {
		hasDataSourcePrefix := strings.HasPrefix(name, "DataSource")
		if isDataSource && hasDataSourcePrefix {
			return name, nil
		}
		if !isDataSource && !hasDataSourcePrefix {
			return name, nil
		}
	}

	return structs[0], nil
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// discoverFlatFileModels discovers models that are defined as flat files in the models directory
func discoverFlatFileModels(modelsDir string) ([]ModelResourcePair, error) {
	var pairs []ModelResourcePair

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory: %w", err)
	}

	for _, entry := range entries {
		// Skip directories and test files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		// Skip special files
		fileName := entry.Name()
		if fileName == "generate.go" || strings.HasSuffix(fileName, "_test.go") {
			continue
		}

		filePath := filepath.Join(modelsDir, fileName)
		structName, err := findMainStructInFile(filePath)
		if err != nil {
			fmt.Printf("Warning: could not find struct in %s: %v\n", fileName, err)
			continue
		}

		// Convert file name to resource package name (remove .go and convert underscores) for example "role_assignment.go" -> "roleassignment"
		baseName := strings.TrimSuffix(fileName, ".go")
		resourcePackageName := strings.ReplaceAll(baseName, "_", "")

		// Check if this is a resource, data source, or both
		resourcesPath := filepath.Join(modelsDir, "..", "resources", resourcePackageName)
		hasResource := fileExists(filepath.Join(resourcesPath, fmt.Sprintf("resource_%s.go", baseName)))
		hasDataSource := fileExists(filepath.Join(resourcesPath, fmt.Sprintf("data_%s.go", baseName)))

		// If neither exists, try without underscores in the file name
		if !hasResource && !hasDataSource {
			hasResource = fileExists(filepath.Join(resourcesPath, fmt.Sprintf("resource_%s.go", resourcePackageName)))
			hasDataSource = fileExists(filepath.Join(resourcesPath, fmt.Sprintf("data_%s.go", resourcePackageName)))
		}

		if hasResource {
			resourceStruct, err := findResourceStructInPackage(resourcePackageName, baseName)
			if err != nil {
				fmt.Printf("Warning: could not find resource struct for %s: %v\n", baseName, err)
				resourceStruct = structName
			}

			pair := ModelResourcePair{
				Name:           structName,
				Type:           "Resource",
				PackageName:    "models", // Flat files are in the models package directly
				ModelStruct:    structName,
				ResourceStruct: resourceStruct,
				ModelImport:    "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models",
				ResourceImport: fmt.Sprintf("github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/%s", resourcePackageName),
			}
			pairs = append(pairs, pair)
		}

		// Create pairs for data sources
		if hasDataSource {
			dataSourceStruct, err := findDataSourceStructInPackage(resourcePackageName, baseName)
			if err != nil {
				fmt.Printf("Warning: could not find data source struct for %s: %v\n", baseName, err)
				dataSourceStruct = "DataSource" + structName
			}

			pair := ModelResourcePair{
				Name:           structName,
				Type:           "DataSource",
				PackageName:    "models",
				ModelStruct:    structName,
				ResourceStruct: dataSourceStruct,
				ModelImport:    "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models",
				ResourceImport: fmt.Sprintf("github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/%s", resourcePackageName),
			}
			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}

// findMainStructInFile parses a Go file and returns the name of the main struct
func findMainStructInFile(filePath string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	// Look for struct declarations
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			_, isStruct := typeSpec.Type.(*ast.StructType)
			if isStruct {
				// Return the first public struct found
				if ast.IsExported(typeSpec.Name.Name) {
					return typeSpec.Name.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no public struct found in %s", filePath)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// findResourceStructInPackage finds the resource struct name in a package
func findResourceStructInPackage(packageName, baseName string) (string, error) {
	patterns := []string{
		fmt.Sprintf("resource_%s.go", baseName),
		fmt.Sprintf("resource_%s.go", packageName),
		"resource.go",
	}

	modelsDir, _ := filepath.Abs(".")
	if filepath.Base(modelsDir) == testutilDir {
		modelsDir = filepath.Dir(modelsDir)
	}

	resourcesPath := filepath.Join(filepath.Dir(modelsDir), "resources", packageName)

	for _, pattern := range patterns {
		filePath := filepath.Join(resourcesPath, pattern)
		if fileExists(filePath) {
			structName, err := findStructImplementingInterface(filePath, "resource.Resource")
			if err == nil {
				return structName, nil
			}
		}
	}

	return capitalizeFirst(packageName), nil
}

// findDataSourceStructInPackage finds the data source struct name in a package
func findDataSourceStructInPackage(packageName, baseName string) (string, error) {
	patterns := []string{
		fmt.Sprintf("data_%s.go", baseName),
		fmt.Sprintf("data_%s.go", packageName),
		"data.go",
	}

	modelsDir, _ := filepath.Abs(".")
	if filepath.Base(modelsDir) == testutilDir {
		modelsDir = filepath.Dir(modelsDir)
	}

	resourcesPath := filepath.Join(filepath.Dir(modelsDir), "resources", packageName)

	for _, pattern := range patterns {
		filePath := filepath.Join(resourcesPath, pattern)
		if fileExists(filePath) {
			structName, err := findStructImplementingInterface(filePath, "datasource.DataSource")
			if err == nil {
				return structName, nil
			}
		}
	}

	return "DataSource" + capitalizeFirst(packageName), nil
}

// NestedObjectValidation represents a discovered nested object that needs validation
type NestedObjectValidation struct {
	AttributeName string // e.g., "kafka_api"
	TypeDefFunc   string // e.g., "GetKafkaAPIType"
}

// NestedObjectValidationSkipList defines attributes that should skip validation
var nestedObjectValidationSkipList = map[string]string{
	// Format: "attribute_name": "reason for skipping"
	// Example: "deprecated_field": "Being removed in v2.0"
}

// DiscoverNestedObjectValidations discovers all nested SingleNestedAttribute fields in cluster schema
// and matches them to their type definition functions
func DiscoverNestedObjectValidations(modelsDir string) ([]NestedObjectValidation, error) {
	// redpandaDir is the parent of models (e.g., /path/to/redpanda)
	redpandaDir := filepath.Dir(modelsDir)

	// Path to cluster schema file
	schemaPath := filepath.Join(redpandaDir, "resources", "cluster", "schema_resource.go")

	// Parse schema file to find nested attributes
	nestedAttrs, err := findNestedAttributesInSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find nested attributes: %w", err)
	}

	// Path to object_definitions.go
	objectDefsPath := filepath.Join(modelsDir, "cluster", "object_definitions.go")

	// Check which type definition functions exist
	var validations []NestedObjectValidation
	var missing []string

	for _, attrName := range nestedAttrs {
		// Check if in skip list
		if reason, skip := nestedObjectValidationSkipList[attrName]; skip {
			fmt.Printf("ℹ️  Skipping validation for '%s' (in allowlist)\n", attrName)
			fmt.Printf("   Reason: %s\n", reason)
			continue
		}

		// Convert attribute name to expected function name
		funcName := attributeNameToTypeFuncName(attrName)

		// Check if function exists
		if !typeFunctionExists(objectDefsPath, funcName) {
			missing = append(missing, fmt.Sprintf("  - %s (expected function: %s)", attrName, funcName))
			continue
		}

		validations = append(validations, NestedObjectValidation{
			AttributeName: attrName,
			TypeDefFunc:   funcName,
		})
	}

	// FAIL if missing type definitions
	if len(missing) > 0 {
		return nil, fmt.Errorf("❌ VALIDATION COVERAGE GAP: Found nested attributes without type definition functions:\n%s\n\nThese attributes will NOT be validated. You must:\n  1. Export the missing type definition functions in object_definitions.go, OR\n  2. Add them to nestedObjectValidationSkipList with reason\n\nValidation cannot proceed with coverage gaps", strings.Join(missing, "\n"))
	}

	return validations, nil
}

// findNestedAttributesInSchema parses a schema file and returns all SingleNestedAttribute field names
func findNestedAttributesInSchema(schemaPath string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, schemaPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema file: %w", err)
	}

	var nestedAttrs []string

	// Look for any function that returns schema.Schema (could be Schema() method or resourceClusterSchema() function)
	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if function returns schema.Schema
		if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
			return true
		}

		// Look for schema.Schema{} composite literal
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			compLit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}

			// Check if this is schema.Schema
			if !isSchemaStruct(compLit.Type) {
				return true
			}

			// Find Attributes field
			for _, elt := range compLit.Elts {
				kvExpr, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}

				ident, ok := kvExpr.Key.(*ast.Ident)
				if !ok || ident.Name != "Attributes" {
					continue
				}

				// Parse attributes map
				attrsMap, ok := kvExpr.Value.(*ast.CompositeLit)
				if !ok {
					continue
				}

				// Find all SingleNestedAttribute entries
				for _, attrElt := range attrsMap.Elts {
					attrKV, ok := attrElt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}

					// Get attribute name
					attrNameLit, ok := attrKV.Key.(*ast.BasicLit)
					if !ok || attrNameLit.Kind != token.STRING {
						continue
					}

					attrName := strings.Trim(attrNameLit.Value, `"`)

					// Check if value is schema.SingleNestedAttribute
					if isSingleNestedAttribute(attrKV.Value) {
						nestedAttrs = append(nestedAttrs, attrName)
					}
				}
			}

			return false
		})

		return false
	})

	return nestedAttrs, nil
}

// isSchemaStruct checks if a type is schema.Schema
func isSchemaStruct(expr ast.Expr) bool {
	selExpr, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "schema" && selExpr.Sel.Name == "Schema"
}

// isSingleNestedAttribute checks if an expression is a schema.SingleNestedAttribute
func isSingleNestedAttribute(expr ast.Expr) bool {
	compLit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}

	selExpr, ok := compLit.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "schema" && selExpr.Sel.Name == "SingleNestedAttribute"
}

// attributeNameToTypeFuncName converts an attribute name to the expected type function name
// e.g., "kafka_api" -> "GetKafkaAPIType"
// e.g., "http_proxy" -> "GetHTTPProxyType"
func attributeNameToTypeFuncName(attrName string) string {
	// Special cases for known acronyms and abbreviations (must match actual function names)
	acronyms := map[string]string{
		"api":   "API",
		"http":  "HTTP",
		"https": "HTTPS",
		"aws":   "Aws",
		"gcp":   "Gcp",
		"azure": "Azure",
		"mtls":  "Mtls",
		"dns":   "DNS",
		"vpc":   "Vpc",
		"cmr":   "Cmr",
		"arn":   "Arn",
		"ipv4":  "IPv4",
		"id":    "ID",
		"url":   "URL",
		"psc":   "Psc",
		"nat":   "Nat",
		"k8s":   "K8s",
	}

	// Split on underscore and capitalize each part
	parts := strings.Split(attrName, "_")
	for i, part := range parts {
		if acronym, ok := acronyms[part]; ok {
			parts[i] = acronym
		} else {
			parts[i] = capitalizeFirst(part)
		}
	}

	return "Get" + strings.Join(parts, "") + "Type"
}

// typeFunctionExists checks if a type function exists in object_definitions.go
func typeFunctionExists(objectDefsPath, funcName string) bool {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, objectDefsPath, nil, parser.ParseComments)
	if err != nil {
		return false
	}

	// Look for function declaration
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if funcDecl.Name.Name == funcName {
			return true
		}
	}

	return false
}
