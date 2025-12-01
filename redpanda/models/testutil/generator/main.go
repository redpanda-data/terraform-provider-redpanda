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
	// When go generate is run from testutil/ directory, we need to scan the parent models/ directory
	// Get current directory (testutil/)
	currentDir, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Models directory is the parent directory
	modelsDir := filepath.Dir(currentDir)

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
	nestedValidations, err := DiscoverNestedObjectValidations()
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
}
