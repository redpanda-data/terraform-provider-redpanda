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
					t.Errorf("Expected an error, but got none")
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
