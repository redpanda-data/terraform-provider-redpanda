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

// modelgen generates Terraform model structs from schema definitions.
//
// Usage:
//
//	go run ./internal/generate/modelgen \
//	  -pkg=github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/resourcegroup \
//	  -func=resourceGroupSchema \
//	  -type=resource \
//	  -output=./redpanda/models/resourcegroup/resource_model_gen.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	filePermissionMode = 0o600
	dirPermissionMode  = 0o750
)

func main() {
	var (
		pkgPath      = flag.String("pkg", "", "Import path of the schema package")
		funcName     = flag.String("func", "", "Name of the schema function to call")
		schemaType   = flag.String("type", "resource", "Schema type: 'resource' or 'datasource'")
		output       = flag.String("output", "", "Output file path")
		pkgName      = flag.String("pkgname", "", "Package name for generated file (derived from output path if not set)")
		needsContext = flag.Bool("ctx", false, "Schema function takes a context.Context parameter")
	)
	flag.Parse()

	if err := run(*pkgPath, *funcName, *schemaType, *output, *pkgName, *needsContext); err != nil {
		log.Fatalf("modelgen: %v", err)
	}
}

func run(pkgPath, funcName, schemaType, output, pkgName string, needsContext bool) error {
	if pkgPath == "" {
		return errors.New("-pkg is required")
	}
	if funcName == "" {
		return errors.New("-func is required")
	}
	if output == "" {
		return errors.New("-output is required")
	}
	if schemaType != "resource" && schemaType != "datasource" {
		return fmt.Errorf("-type must be 'resource' or 'datasource', got %q", schemaType)
	}

	if pkgName == "" {
		absOutput, err := filepath.Abs(output)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		pkgName = filepath.Base(filepath.Dir(absOutput))
	}

	log.Printf("Extracting schema from %s.%s()...", pkgPath, funcName)

	info, err := extractSchema(pkgPath, funcName, schemaType, needsContext)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	log.Printf("Found %d attributes", len(info.Attributes))
	if info.HasTimeouts {
		log.Print("Schema has timeouts")
	}

	code, err := generate(info, schemaType, pkgName)
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(output), dirPermissionMode); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(output, code, filePermissionMode); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	log.Printf("Generated %s", output)
	return nil
}
