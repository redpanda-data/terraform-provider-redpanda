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
	"strings"
	"testing"
)

// TestSupportedOperations exercises the default → exclude resolution and the
// cross-validation against api: blocks.
func TestSupportedOperations(t *testing.T) {
	allCRUD := func() *APIConfig {
		return &APIConfig{
			Create: &RPCConfig{Request: "CreateXReq"},
			Update: &RPCConfig{Request: "UpdateXReq"},
			Delete: &RPCConfig{Request: "DeleteXReq"},
		}
	}

	tests := []struct {
		name       string
		cfg        *Config
		schemaType string
		want       []string
		wantErr    string
	}{
		{
			name:       "resource_default_full_crud",
			cfg:        &Config{API: allCRUD()},
			schemaType: "resource",
			want:       []string{"create", "delete", "read", "update"},
		},
		{
			name: "resource_immutable_excludes_update",
			cfg: &Config{
				ExcludeOperations: []string{"update"},
				API: &APIConfig{
					Create: &RPCConfig{Request: "CreateXReq"},
					Delete: &RPCConfig{Request: "DeleteXReq"},
				},
			},
			schemaType: "resource",
			want:       []string{"create", "delete", "read"},
		},
		{
			name:       "datasource_default_read_only",
			cfg:        &Config{API: &APIConfig{}},
			schemaType: "datasource",
			want:       []string{"read"},
		},
		{
			name: "rejects_excluding_read",
			cfg: &Config{
				ExcludeOperations: []string{"read"},
				API:               &APIConfig{},
			},
			schemaType: "resource",
			wantErr:    `cannot be excluded`,
		},
		{
			name: "rejects_unknown_op",
			cfg: &Config{
				ExcludeOperations: []string{"replace"},
				API:               &APIConfig{},
			},
			schemaType: "resource",
			wantErr:    `not a valid CRUD op`,
		},
		{
			name: "rejects_api_block_for_excluded_op",
			cfg: &Config{
				ExcludeOperations: []string{"update"},
				API: &APIConfig{
					Create: &RPCConfig{Request: "CreateXReq"},
					Update: &RPCConfig{Request: "UpdateXReq"},
					Delete: &RPCConfig{Request: "DeleteXReq"},
				},
			},
			schemaType: "resource",
			wantErr:    `api.update is declared but "update" is in exclude_operations`,
		},
		{
			name: "tolerates_missing_api_block_for_supported_op",
			cfg: &Config{
				API: &APIConfig{
					Create: &RPCConfig{Request: "CreateXReq"},
					Delete: &RPCConfig{Request: "DeleteXReq"},
				},
			},
			schemaType: "resource",
			want:       []string{"create", "delete", "read", "update"},
		},
		{
			name: "datasource_can_have_exclude_for_listed_default",
			cfg: &Config{
				API:               &APIConfig{},
				ExcludeOperations: []string{"create"},
			},
			schemaType: "datasource",
			wantErr:    `not a valid CRUD op for datasource schema`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.cfg.SupportedOperations(tc.schemaType)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotKeys := make([]string, 0, len(got))
			for k := range got {
				gotKeys = append(gotKeys, k)
			}
			sortStrings(gotKeys)
			if !equalStrings(gotKeys, tc.want) {
				t.Fatalf("ops mismatch: got %v want %v", gotKeys, tc.want)
			}
		})
	}
}

func sortStrings(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
