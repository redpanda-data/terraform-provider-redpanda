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
	"log"
	"path/filepath"
)

func main() {
	// When go generate is run from utils/testutil/ directory, we need to find the models/ directory
	// Get current directory (utils/testutil/)
	currentDir, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Models directory is at ../../models (testutil -> utils -> redpanda -> models)
	redpandaDir := filepath.Dir(filepath.Dir(currentDir)) // Go up from testutil to utils to redpanda
	modelsDir := filepath.Join(redpandaDir, "models")

	fmt.Printf("Scanning models directory: %s\n", modelsDir)

	// Discover model-resource pairs
	pairs, err := DiscoverModelResourcePairs(modelsDir)
	if err != nil {
		log.Fatalf("Failed to discover model-resource pairs: %v", err)
	}

	fmt.Printf("Discovered %d model-resource pairs\n", len(pairs))
	for _, pair := range pairs {
		fmt.Printf("  - %s (%s)\n", pair.Name, pair.Type)
	}

	// Discover nested object validations for cluster resource
	fmt.Print("\nDiscovering nested object validations for cluster resource...\n")
	nestedValidations, err := DiscoverNestedObjectValidations(modelsDir)
	if err != nil {
		log.Fatalf("Failed to discover nested object validations: %v", err)
	}

	fmt.Printf("Discovered %d nested object validations\n", len(nestedValidations))
	for _, validation := range nestedValidations {
		fmt.Printf("  - %s → %s\n", validation.AttributeName, validation.TypeDefFunc)
	}

	// Generate test file
	outputPath := filepath.Join(currentDir, "schema_validation_generated_test.go")
	if err := GenerateTestFile(pairs, nestedValidations, outputPath); err != nil {
		log.Fatalf("Failed to generate test file: %v", err)
	}

	fmt.Printf("\nSuccessfully generated: %s\n", outputPath)

	// Generate compare functions for all model packages
	fmt.Print("\nGenerating compare functions...\n")

	// Generate for cluster package (ResourceModel only)
	if err := GenerateCompareFile(modelsDir, "cluster"); err != nil {
		log.Fatalf("Failed to generate compare file for cluster: %v", err)
	}

	// Generate for network package (both ResourceModel and DataModel)
	if err := GenerateCompareFile(modelsDir, "network", "ResourceModel", "DataModel"); err != nil {
		log.Fatalf("Failed to generate compare file for network: %v", err)
	}

	// Generate for resourcegroup package (both ResourceModel and DataModel)
	if err := GenerateCompareFile(modelsDir, "resourcegroup", "ResourceModel", "DataModel"); err != nil {
		log.Fatalf("Failed to generate compare file for resourcegroup: %v", err)
	}

	// Generate for schema package (both ResourceModel and DataModel)
	if err := GenerateCompareFile(modelsDir, "schema", "ResourceModel", "DataModel"); err != nil {
		log.Fatalf("Failed to generate compare file for schema: %v", err)
	}

	fmt.Println("Compare function generation complete!")
}
