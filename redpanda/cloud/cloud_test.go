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

package cloud

import (
	"testing"
)

func TestParseHTTPSURLAsGrpc(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expected    string
		expectError bool
	}{
		{
			name:        "URL with incorrect scheme",
			url:         "http://example.com:8080",
			expected:    "",
			expectError: true,
		},
		{
			name:        "URL with scheme, no port",
			url:         "https://example.com",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL without scheme, with port",
			url:         "example.com:9090",
			expected:    "example.com:9090",
			expectError: false,
		},
		{
			name:        "URL without scheme, no port",
			url:         "example.com",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "IP address with port",
			url:         "192.168.1.1:8080",
			expected:    "192.168.1.1:8080",
			expectError: false,
		},
		{
			name:        "IP address without port",
			url:         "192.168.1.1",
			expected:    "192.168.1.1",
			expectError: false,
		},
		{
			name:        "URL with trailing slash",
			url:         "example.com/",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with trailing path",
			url:         "example.com/api/v1",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			expected:    "",
			expectError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseHTTPSURLAsGrpc(tc.url)
			if tc.expectError {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %q, but got %q", tc.expected, result)
				}
			}
		})
	}
}
