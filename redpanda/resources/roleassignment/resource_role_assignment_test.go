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
	"testing"
)

func TestNormalizePrincipal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "principal without User prefix",
			input:    "testuser",
			expected: "testuser",
		},
		{
			name:     "principal with User prefix",
			input:    "User:testuser",
			expected: "testuser",
		},
		{
			name:     "principal with email",
			input:    "user@example.com",
			expected: "user@example.com",
		},
		{
			name:     "principal with User prefix and email",
			input:    "User:user@example.com",
			expected: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePrincipal(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePrincipal(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestImportIDFormat(t *testing.T) {
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
			name:              "valid import format with User prefix",
			importID:          "admin:User:testuser",
			expectError:       false,
			expectedRole:      "admin",
			expectedPrincipal: "testuser",
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
				principal := normalizePrincipal(parts[1])

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
