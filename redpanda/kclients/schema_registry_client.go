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
	"errors"
	"fmt"
	"net/http"
	"strings"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/twmb/franz-go/pkg/sr"
	"golang.org/x/oauth2"
)

const (
	// DefaultCompatibilityLevel is the default compatibility level for Schema Registry subjects
	DefaultCompatibilityLevel = "BACKWARD"
)

// GetSchemaRegistryClientForCluster creates a Schema Registry client for a specific cluster.
//
// Auth precedence: when both username and password are non-empty, HTTP Basic
// auth is used. Otherwise the provider's cloud-issued Bearer token (sourced
// from the TokenSource per request) is used. Redpanda Cloud SR endpoints
// accept Bearer tokens issued by the same Auth0 IDP that mints the provider's
// control-plane token, so the Bearer path is the recommended default.
func GetSchemaRegistryClientForCluster(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID string, ts oauth2.TokenSource, username, password string) (*sr.Client, error) {
	srURL, err := schemaRegistryURLForCluster(ctx, cpCl, clusterID)
	if err != nil {
		return nil, err
	}

	authOpt, err := schemaRegistryAuthOption(ts, username, password)
	if err != nil {
		return nil, err
	}

	client, err := sr.NewClient(
		sr.URLs(srURL),
		authOpt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema registry client: %w", err)
	}

	return client, nil
}

// schemaRegistryURLForCluster resolves the Schema Registry URL for clusterID.
// A serverless cluster ID is invalid for the dedicated ClusterService and is
// rejected there (PermissionDenied), so any ClusterForID failure falls back to
// ServerlessClusterForID before giving up.
func schemaRegistryURLForCluster(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID string) (string, error) {
	cluster, err := cpCl.ClusterForID(ctx, clusterID)
	if err == nil {
		if !cluster.HasSchemaRegistry() {
			return "", fmt.Errorf("schema registry is not enabled for cluster %s", clusterID)
		}
		if url := cluster.GetSchemaRegistry().GetUrl(); url != "" {
			return url, nil
		}
		return "", fmt.Errorf("schema registry URL is empty for cluster %s", clusterID)
	}

	serverless, serverlessErr := cpCl.ServerlessClusterForID(ctx, clusterID)
	if serverlessErr != nil {
		return "", fmt.Errorf("failed to get cluster details: %w", err)
	}
	if !serverless.HasSchemaRegistry() {
		return "", fmt.Errorf("schema registry is not enabled for serverless cluster %s", clusterID)
	}
	srStatus := serverless.GetSchemaRegistry()
	// A public-disabled serverless cluster rejects the public SR URL with
	// "public network is not enabled"; the private URL is the endpoint
	// reachable from inside the private link.
	if serverless.GetNetworkingConfig().GetPublic() == controlplanev1.ServerlessNetworkingConfig_STATE_DISABLED {
		if url := srStatus.GetPrivateUrl(); url != "" {
			return url, nil
		}
	}
	if url := srStatus.GetUrl(); url != "" {
		return url, nil
	}
	return "", fmt.Errorf("schema registry URL is empty for serverless cluster %s", clusterID)
}

// schemaRegistryAuthOption selects the franz-go sr.Client auth option based on
// which credentials are present. Username+password → Basic; else the
// TokenSource is consulted per HTTP request via sr.PreReq, so the bearer
// header reflects the latest cached or freshly-fetched access token.
// Returns an error when neither set of credentials is available.
func schemaRegistryAuthOption(ts oauth2.TokenSource, username, password string) (sr.ClientOpt, error) {
	if username != "" && password != "" {
		return sr.BasicAuth(username, password), nil
	}
	if ts != nil {
		return sr.PreReq(func(req *http.Request) error {
			tok, err := ts.Token()
			if err != nil {
				return fmt.Errorf("acquire bearer token: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
			return nil
		}), nil
	}
	return nil, errors.New("no schema registry credentials available: provide username+password, or rely on the provider's cloud authentication")
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
		return nil
	}

	var level sr.CompatibilityLevel
	if err := level.UnmarshalText([]byte(strings.ToUpper(compatibility))); err != nil {
		return fmt.Errorf("invalid compatibility level %q: %w", compatibility, err)
	}

	results := client.SetCompatibility(ctx, sr.SetCompatibility{Level: level}, subject)
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
