// Copyright 2025 Redpanda Data, Inc.
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

// Package kclients provides utilities for creating franz-go Kafka clients
package kclients

import (
	"context"
	"fmt"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/twmb/franz-go/pkg/sr"
)

const (
	// DefaultCompatibilityLevel is the default compatibility level for Schema Registry subjects
	DefaultCompatibilityLevel = "BACKWARD"
)

// GetSchemaRegistryClientForCluster creates a Schema Registry client for a specific cluster
func GetSchemaRegistryClientForCluster(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID, username, password string) (*sr.Client, error) {
	cluster, err := cpCl.ClusterForID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster details: %w", err)
	}

	if !cluster.HasSchemaRegistry() {
		return nil, fmt.Errorf("schema registry is not enabled for cluster %s", clusterID)
	}

	schemaRegistry := cluster.GetSchemaRegistry()
	if schemaRegistry.GetUrl() == "" {
		return nil, fmt.Errorf("schema registry URL is empty for cluster %s", clusterID)
	}

	client, err := sr.NewClient(
		sr.URLs(schemaRegistry.GetUrl()),
		sr.BasicAuth(username, password),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema registry client: %w", err)
	}

	return client, nil
}

// FetchSchema fetches a schema by subject and optional version
// If version is nil, it returns the latest version
func FetchSchema(ctx context.Context, client *sr.Client, subject string, version *int) (sr.SubjectSchema, error) {
	if version != nil {
		// Fetch specific version
		return client.SchemaByVersion(ctx, subject, *version)
	}

	schemas, err := client.Schemas(ctx, subject)
	if err != nil {
		return sr.SubjectSchema{}, err
	}

	if len(schemas) == 0 {
		return sr.SubjectSchema{}, fmt.Errorf("no schemas found for subject %s", subject)
	}

	return schemas[len(schemas)-1], nil
}

// SetSubjectCompatibility sets the compatibility level for a subject
func SetSubjectCompatibility(ctx context.Context, client *sr.Client, subject, compatibility string) error {
	if compatibility == "" {
		return nil // No compatibility to set
	}

	var level sr.CompatibilityLevel
	if err := level.UnmarshalText([]byte(strings.ToUpper(compatibility))); err != nil {
		level = sr.CompatBackward // Default to BACKWARD on error
	}

	setCompat := sr.SetCompatibility{
		Level: level,
	}

	results := client.SetCompatibility(ctx, setCompat, subject)
	for _, result := range results {
		if result.Err != nil {
			return result.Err
		}
	}
	return nil
}

// GetSubjectCompatibility gets the compatibility level for a subject
func GetSubjectCompatibility(ctx context.Context, client *sr.Client, subject string) (string, error) {
	// Use Compatibility method to get subject compatibility
	results := client.Compatibility(ctx, subject)

	// Check results for the subject
	for _, result := range results {
		if result.Err != nil {
			return "", fmt.Errorf("failed to get compatibility for subject %s: %w", subject, result.Err)
		}
		if result.Subject == subject {
			return result.Level.String(), nil
		}
	}

	// If no specific result found, check if we have any results
	if len(results) > 0 && results[0].Err == nil {
		return results[0].Level.String(), nil
	}

	// No compatibility level found for the subject
	return "", fmt.Errorf("no compatibility level found for subject %s", subject)
}
