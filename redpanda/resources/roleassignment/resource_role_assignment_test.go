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

package roleassignment

import (
	"strings"
	"testing"
)

func TestUnit_RoleAssignment_ImportIDFormat(t *testing.T) {
	tests := []struct {
		name              string
		importID          string
		expectError       bool
		expectedRole      string
		expectedPrincipal string
	}{
		{
			name:              "valid import format",
			importID:          "developer:testuser",
			expectError:       false,
			expectedRole:      "developer",
			expectedPrincipal: "testuser",
		},
		{
			name:              "valid import format with User: prefix preserved",
			importID:          "admin:User:testuser",
			expectError:       false,
			expectedRole:      "admin",
			expectedPrincipal: "User:testuser",
		},
		{
			name:              "valid import format with Group: prefix preserved",
			importID:          "developer:Group:engineers",
			expectError:       false,
			expectedRole:      "developer",
			expectedPrincipal: "Group:engineers",
		},
		{
			name:        "invalid format - missing principal",
			importID:    "developer",
			expectError: true,
		},
		{
			name:              "valid format with colon in principal",
			importID:          "developer:testuser:extra",
			expectError:       false,
			expectedRole:      "developer",
			expectedPrincipal: "testuser:extra",
		},
		{
			name:        "empty import ID",
			importID:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test validates the expected import ID format
			// The actual ImportState method would be tested in integration tests
			parts := splitImportID(tt.importID)

			if tt.expectError {
				if len(parts) == 2 {
					t.Errorf("Expected error for import ID %q, but parsing succeeded", tt.importID)
				}
			} else {
				if len(parts) != 2 {
					t.Errorf("Expected valid parsing for import ID %q, but got %d parts", tt.importID, len(parts))
					return
				}

				role := parts[0]
				principal := parts[1]

				if role != tt.expectedRole {
					t.Errorf("Expected role %q, got %q", tt.expectedRole, role)
				}

				if principal != tt.expectedPrincipal {
					t.Errorf("Expected principal %q, got %q", tt.expectedPrincipal, principal)
				}
			}
		})
	}
}

// Helper function to simulate import ID parsing - mimics strings.SplitN(importID, ":", 2)
func splitImportID(importID string) []string {
	if importID == "" {
		return []string{}
	}

	// Find first colon
	colonIndex := -1
	for i, r := range importID {
		if r == ':' {
			colonIndex = i
			break
		}
	}

	if colonIndex == -1 {
		return []string{importID}
	}

	// Split into exactly 2 parts at the first colon
	parts := []string{
		importID[:colonIndex],
		importID[colonIndex+1:],
	}

	return parts
}

// TestUnit_RoleAssignment_ImportIDFormat_WithURL exercises the optional
// |cluster_api_url suffix added so a single `terraform import` produces a
// state file usable without a follow-up destroy+create cycle.
// Format: <role>:<principal>[|<cluster_api_url>]
func TestUnit_RoleAssignment_ImportIDFormat_WithURL(t *testing.T) {
	tests := []struct {
		name              string
		importID          string
		expectError       bool
		expectedRole      string
		expectedPrincipal string
		expectedURL       string
	}{
		{
			name:              "role:bare without url",
			importID:          "developer:alice",
			expectedRole:      "developer",
			expectedPrincipal: "alice",
			expectedURL:       "",
		},
		{
			name:              "role:User:name with url",
			importID:          "developer:User:alice|https://api.example.com",
			expectedRole:      "developer",
			expectedPrincipal: "User:alice",
			expectedURL:       "https://api.example.com",
		},
		{
			name:              "role:Group:name with url",
			importID:          "admin:Group:engineers|bufnet",
			expectedRole:      "admin",
			expectedPrincipal: "Group:engineers",
			expectedURL:       "bufnet",
		},
		{
			name:              "url with embedded colon survives pipe-split",
			importID:          "developer:User:alice|https://host:8080/path",
			expectedRole:      "developer",
			expectedPrincipal: "User:alice",
			expectedURL:       "https://host:8080/path",
		},
		{
			name:        "missing principal",
			importID:    "developer|bufnet",
			expectError: true,
		},
		{
			name:        "empty role",
			importID:    ":User:alice|bufnet",
			expectError: true,
		},
		{
			name:        "empty principal",
			importID:    "developer:|bufnet",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, principal, url, ok := parseImportIDSim(tt.importID)
			if tt.expectError {
				if ok {
					t.Errorf("expected parse error for %q, got role=%q principal=%q url=%q", tt.importID, role, principal, url)
				}
				return
			}
			if !ok {
				t.Fatalf("expected successful parse for %q, got error", tt.importID)
			}
			if role != tt.expectedRole {
				t.Errorf("role: want %q, got %q", tt.expectedRole, role)
			}
			if principal != tt.expectedPrincipal {
				t.Errorf("principal: want %q, got %q", tt.expectedPrincipal, principal)
			}
			if url != tt.expectedURL {
				t.Errorf("url: want %q, got %q", tt.expectedURL, url)
			}
		})
	}
}

// parseImportIDSim mirrors the parse logic in ImportState — kept here so
// the unit test pins the parser shape without needing to construct a full
// resource.ImportStateRequest/Response pair. Drift between this helper and
// ImportState is caught by the integration test's Import subtests.
func parseImportIDSim(id string) (role, principal, url string, ok bool) {
	rest, urlPart, _ := strings.Cut(id, "|")
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], urlPart, true
}
