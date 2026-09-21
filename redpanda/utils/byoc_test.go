// Copyright 2026 Redpanda Data, Inc.
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

package utils

import (
	"strings"
	"testing"
)

func TestProgressLine(t *testing.T) {
	zap := func(level, msg string) string {
		return "2026-09-21T10:00:00.000Z\t" + level + "\tbyoc\tfile.go:1\t" + msg
	}
	cases := []struct {
		name string
		line string
		want string
		ok   bool
	}{
		{"info message forwarded", zap("INFO", "Reconciling agent infrastructure..."), "Reconciling agent infrastructure...", true},
		{"warn message forwarded", zap("WARN", "retrying"), "retrying", true},
		{"error message forwarded", zap("ERROR", "apply failed"), "apply failed", true},
		{"debug dropped", zap("DEBUG", "terraform plan output"), "", false},
		{"non-zap line forwarded raw", "Error: required flag(s) not set", "Error: required flag(s) not set", true},
		{"empty line dropped", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := progressLine(tc.line)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("progressLine(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestByocClient_CheckCloudConfig(t *testing.T) {
	cases := []struct {
		name    string
		conf    ByocClientConfig
		cloud   string
		wantErr string
	}{
		{"aws never checked", ByocClientConfig{}, "aws", ""},
		{"gcp missing project", ByocClientConfig{}, "gcp", "gcp_project_id"},
		{"gcp with project", ByocClientConfig{GcpProject: "p"}, "gcp", ""},
		{"azure missing subscription", ByocClientConfig{}, "azure", "azure_subscription_id"},
		{"azure with subscription", ByocClientConfig{AzureSubscriptionID: "s"}, "azure", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewByocClient(tc.conf).CheckCloudConfig(tc.cloud)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("CheckCloudConfig(%s) = %v, want nil", tc.cloud, err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("CheckCloudConfig(%s) = %v, want error mentioning %q", tc.cloud, err, tc.wantErr)
			default:
			}
		})
	}
}
