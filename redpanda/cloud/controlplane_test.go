// Copyright 2026 Redpanda Data, Inc.
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

package cloud

import (
	"strings"
	"testing"
)

// TestUnit_DataplaneURLOrNotReady pins the nil-DataplaneApi guard that
// replaces the previous direct `.DataplaneApi.Url` access (which nil-derefed
// for clusters in STATE_CREATING / pre-dataplane BYOC). The happy paths
// through ClusterForID / ServerlessClusterForID are covered by the per-
// resource ImportRoundTrip integration tests against the bufconn fake; this
// isolates the panic-prevention contract.
func TestUnit_DataplaneURLOrNotReady(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		present       bool
		clusterID     string
		serverless    bool
		wantURL       string
		wantErrSubstr []string
	}{
		{
			name:      "regular cluster — dataplane ready",
			url:       "https://api.cluster-123.example.com",
			present:   true,
			clusterID: "cluster-123",
			wantURL:   "https://api.cluster-123.example.com",
		},
		{
			name:          "regular cluster — dataplane nil (STATE_CREATING)",
			url:           "",
			present:       false,
			clusterID:     "cluster-creating-001",
			wantErrSubstr: []string{"cluster-creating-001", "dataplane API URL yet", "READY state"},
		},
		{
			name:       "serverless cluster — dataplane ready",
			url:        "https://api.serverless-456.example.com",
			present:    true,
			clusterID:  "serverless-456",
			serverless: true,
			wantURL:    "https://api.serverless-456.example.com",
		},
		{
			name:          "serverless cluster — dataplane nil",
			url:           "",
			present:       false,
			clusterID:     "serverless-creating-002",
			serverless:    true,
			wantErrSubstr: []string{"serverless cluster", "serverless-creating-002", "no dataplane API URL yet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dataplaneURLOrNotReady(tt.url, tt.present, tt.clusterID, tt.serverless)
			if len(tt.wantErrSubstr) > 0 {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tt.wantErrSubstr)
				}
				for _, s := range tt.wantErrSubstr {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("error must contain %q; got %v", s, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantURL {
				t.Errorf("got %q, want %q", got, tt.wantURL)
			}
		})
	}
}
