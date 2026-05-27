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

package kclients

import (
	"context"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/sr"
)

// TestSetSubjectCompatibility_InvalidLevelReturnsError proves that an
// unrecognised compatibility string is rejected rather than silently substituted
// with BACKWARD. The error is returned before SetCompatibility is called, so a
// nil client is safe to pass.
func TestSetSubjectCompatibility_InvalidLevelReturnsError(t *testing.T) {
	invalidCases := []struct {
		name        string
		input       string
		errContains string
	}{
		{"typo BACKWRD", "BACKWRD", `invalid compatibility level "BACKWRD"`},
		{"garbage value", "INVALID_LEVEL", `invalid compatibility level "INVALID_LEVEL"`},
		{"numeric", "42", `invalid compatibility level "42"`},
		{"mixed case typo", "Backward_Transitivee", `invalid compatibility level "Backward_Transitivee"`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := SetSubjectCompatibility(context.Background(), (*sr.Client)(nil), "test-subject", tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errContains)
			}
		})
	}

	t.Run("empty string is a no-op", func(t *testing.T) {
		err := SetSubjectCompatibility(context.Background(), (*sr.Client)(nil), "test-subject", "")
		if err != nil {
			t.Fatalf("unexpected error for empty string: %v", err)
		}
	})
}

// TestSchemaRegistryAuthOption_Precedence covers the four cases the helper
// must distinguish: explicit Basic creds win, fall back to Bearer, error
// when nothing is available, explicit Basic wins even when Bearer is also
// supplied.
func TestSchemaRegistryAuthOption_Precedence(t *testing.T) {
	tests := []struct {
		name      string
		authToken string
		username  string
		password  string
		wantErr   string
	}{
		{
			name:     "username and password — Basic auth",
			username: "alice",
			password: "p4ssw0rd",
		},
		{
			name:      "authToken only — Bearer auth",
			authToken: "tok-bearer",
		},
		{
			name:      "both Basic creds and authToken — Basic wins",
			authToken: "tok-bearer",
			username:  "alice",
			password:  "p4ssw0rd",
		},
		{
			name:    "no credentials — error",
			wantErr: "no schema registry credentials available",
		},
		{
			name:      "username only with authToken — Bearer (username alone is not enough for Basic)",
			authToken: "tok-bearer",
			username:  "alice",
		},
		{
			name:      "password only with authToken — Bearer (password alone is not enough for Basic)",
			authToken: "tok-bearer",
			password:  "p4ssw0rd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := schemaRegistryAuthOption(tt.authToken, tt.username, tt.password)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opt == nil {
				t.Fatal("expected non-nil sr.ClientOpt")
			}
		})
	}
}
