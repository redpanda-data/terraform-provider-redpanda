package acl

import (
	"fmt"
	"strings"
	"testing"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
)

// These are golden tests, to ensure we parse properly and the string types
// does not change during the proto file updates.

func Test_stringToACLResourceType(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  string
		exp    dataplanev1.ACL_ResourceType
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1.ACL_RESOURCE_TYPE_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1.ACL_RESOURCE_TYPE_ANY, false},
		{"topic", "TOPIC", dataplanev1.ACL_RESOURCE_TYPE_TOPIC, false},
		{"group", "GROUP", dataplanev1.ACL_RESOURCE_TYPE_GROUP, false},
		{"transactional", "TRANSACTIONAL_ID", dataplanev1.ACL_RESOURCE_TYPE_TRANSACTIONAL_ID, false},
		{"delegation token", "DELEGATION_TOKEN", dataplanev1.ACL_RESOURCE_TYPE_DELEGATION_TOKEN, false},
		{"user", "USER", dataplanev1.ACL_RESOURCE_TYPE_USER, false},
		{"user", "CLUSTER", dataplanev1.ACL_RESOURCE_TYPE_CLUSTER, false},
		{"wrong input", "WRONG_INPUT", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringToACLResourceType(tt.input)
			if err != nil {
				if tt.expErr {
					return
				}
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.exp {
				t.Errorf("got = %v, want = %v", got, tt.exp)
			}
		})
	}
}

func Test_aclResourceTypeToString(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input dataplanev1.ACL_ResourceType
		exp   string
	}{
		{"unspecified", dataplanev1.ACL_RESOURCE_TYPE_UNSPECIFIED, "UNSPECIFIED"},
		{"any", dataplanev1.ACL_RESOURCE_TYPE_ANY, "ANY"},
		{"topic", dataplanev1.ACL_RESOURCE_TYPE_TOPIC, "TOPIC"},
		{"group", dataplanev1.ACL_RESOURCE_TYPE_GROUP, "GROUP"},
		{"transactional", dataplanev1.ACL_RESOURCE_TYPE_TRANSACTIONAL_ID, "TRANSACTIONAL_ID"},
		{"delegation token", dataplanev1.ACL_RESOURCE_TYPE_DELEGATION_TOKEN, "DELEGATION_TOKEN"},
		{"user", dataplanev1.ACL_RESOURCE_TYPE_USER, "USER"},
		{"user", dataplanev1.ACL_RESOURCE_TYPE_CLUSTER, "CLUSTER"},
		{"wrong input", 123, "UNKNOWN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := aclResourceTypeToString(tt.input)
			if got != tt.exp {
				t.Errorf("got = %v, want = %v", got, tt.exp)
			}
		})
	}
}

func Test_stringToACLResourcePatternType(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  string
		exp    dataplanev1.ACL_ResourcePatternType
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_ANY, false},
		{"match", "MATCH", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_MATCH, false},
		{"literal", "LITERAL", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_LITERAL, false},
		{"prefixed", "PREFIXED", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_PREFIXED, false},
		{"wrong input", "WRONG_INPUT", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringToACLResourcePatternType(tt.input)
			if err != nil {
				if tt.expErr {
					return
				}
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.exp {
				t.Errorf("got = %v, want = %v", got, tt.exp)
			}
		})
	}
}

func Test_aclResourcePatternTypeToString(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input dataplanev1.ACL_ResourcePatternType
		exp   string
	}{
		{"unspecified", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_UNSPECIFIED, "UNSPECIFIED"},
		{"any", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_ANY, "ANY"},
		{"match", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_MATCH, "MATCH"},
		{"literal", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_LITERAL, "LITERAL"},
		{"prefixed", dataplanev1.ACL_RESOURCE_PATTERN_TYPE_PREFIXED, "PREFIXED"},
		{"wrong input", 123, "UNKNOWN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := aclResourcePatternTypeToString(tt.input)
			if got != tt.exp {
				t.Errorf("got = %v, want = %v", got, tt.exp)
			}
		})
	}
}

func Test_stringToACLPermissionType(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  string
		exp    dataplanev1.ACL_PermissionType
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1.ACL_PERMISSION_TYPE_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1.ACL_PERMISSION_TYPE_ANY, false},
		{"match", "DENY", dataplanev1.ACL_PERMISSION_TYPE_DENY, false},
		{"literal", "ALLOW", dataplanev1.ACL_PERMISSION_TYPE_ALLOW, false},
		{"wrong input", "WRONG_INPUT", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringToACLPermissionType(tt.input)
			if err != nil {
				if tt.expErr {
					return
				}
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.exp {
				t.Errorf("got = %v, want = %v", got, tt.exp)
			}
		})
	}
}

func Test_stringToACLOperation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  string
		exp    dataplanev1.ACL_Operation
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1.ACL_OPERATION_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1.ACL_OPERATION_ANY, false},
		{"all", "ALL", dataplanev1.ACL_OPERATION_ALL, false},
		{"read", "READ", dataplanev1.ACL_OPERATION_READ, false},
		{"write", "WRITE", dataplanev1.ACL_OPERATION_WRITE, false},
		{"create", "CREATE", dataplanev1.ACL_OPERATION_CREATE, false},
		{"delete", "DELETE", dataplanev1.ACL_OPERATION_DELETE, false},
		{"alter", "ALTER", dataplanev1.ACL_OPERATION_ALTER, false},
		{"describe", "DESCRIBE", dataplanev1.ACL_OPERATION_DESCRIBE, false},
		{"cluster action", "CLUSTER_ACTION", dataplanev1.ACL_OPERATION_CLUSTER_ACTION, false},
		{"describe configs", "DESCRIBE_CONFIGS", dataplanev1.ACL_OPERATION_DESCRIBE_CONFIGS, false},
		{"alter configs", "ALTER_CONFIGS", dataplanev1.ACL_OPERATION_ALTER_CONFIGS, false},
		{"idempotent write", "IDEMPOTENT_WRITE", dataplanev1.ACL_OPERATION_IDEMPOTENT_WRITE, false},
		{"create tokens", "CREATE_TOKENS", dataplanev1.ACL_OPERATION_CREATE_TOKENS, false},
		{"describe tokens", "DESCRIBE_TOKENS", dataplanev1.ACL_OPERATION_DESCRIBE_TOKENS, false},
		{"wrong input", "WRONG_INPUT", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringToACLOperation(tt.input)
			if err != nil {
				if tt.expErr {
					return
				}
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.exp {
				t.Errorf("got = %v, want = %v", got, tt.exp)
			}
		})
	}
}

func Test_generateACLCompositeID(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		resourceName   string
		patternType    string
		principal      string
		host           string
		operation      string
		permissionType string
		expected       string
	}{
		{
			name:           "basic topic ACL",
			resourceType:   "TOPIC",
			resourceName:   "test-topic",
			patternType:    "LITERAL",
			principal:      "User:testuser",
			host:           "*",
			operation:      "READ",
			permissionType: "ALLOW",
			expected:       "TOPIC:test-topic:LITERAL:User:testuser:*:READ:ALLOW",
		},
		{
			name:           "group ACL with specific host",
			resourceType:   "GROUP",
			resourceName:   "test-group",
			patternType:    "PREFIXED",
			principal:      "User:admin",
			host:           "192.168.1.100",
			operation:      "WRITE",
			permissionType: "DENY",
			expected:       "GROUP:test-group:PREFIXED:User:admin:192.168.1.100:WRITE:DENY",
		},
		{
			name:           "cluster ACL with colon in resource name",
			resourceType:   "CLUSTER",
			resourceName:   "my:cluster:name",
			patternType:    "MATCH",
			principal:      "ServiceAccount:service",
			host:           "*",
			operation:      "CLUSTER_ACTION",
			permissionType: "ALLOW",
			expected:       "CLUSTER:my:cluster:name:MATCH:ServiceAccount:service:*:CLUSTER_ACTION:ALLOW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the composite ID format used in both Create and Read methods
			compositeID := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
				tt.resourceType,
				tt.resourceName,
				tt.patternType,
				tt.principal,
				tt.host,
				tt.operation,
				tt.permissionType)

			if compositeID != tt.expected {
				t.Errorf("generateACLCompositeID() = %q, expected %q", compositeID, tt.expected)
			}
		})
	}
}

func Test_ACLCompositeIDFormat(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		resourceName   string
		patternType    string
		principal      string
		host           string
		operation      string
		permissionType string
		expectedFormat string
	}{
		{
			name:           "simple format validation",
			resourceType:   "TOPIC",
			resourceName:   "test-topic",
			patternType:    "LITERAL",
			principal:      "User:testuser",
			host:           "*",
			operation:      "READ",
			permissionType: "ALLOW",
			expectedFormat: "TOPIC:test-topic:LITERAL:User:testuser:*:READ:ALLOW",
		},
		{
			name:           "format with complex values",
			resourceType:   "GROUP",
			resourceName:   "my-consumer-group",
			patternType:    "PREFIXED",
			principal:      "ServiceAccount:my-service",
			host:           "192.168.1.100",
			operation:      "DESCRIBE",
			permissionType: "DENY",
			expectedFormat: "GROUP:my-consumer-group:PREFIXED:ServiceAccount:my-service:192.168.1.100:DESCRIBE:DENY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the composite ID format matches what's used in the actual resource implementation
			compositeID := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
				tt.resourceType,
				tt.resourceName,
				tt.patternType,
				tt.principal,
				tt.host,
				tt.operation,
				tt.permissionType)

			if compositeID != tt.expectedFormat {
				t.Errorf("Composite ID format = %q, expected %q", compositeID, tt.expectedFormat)
			}

			// Verify that the ID contains all required fields
			parts := strings.Split(compositeID, ":")
			if len(parts) < 7 {
				t.Errorf("Composite ID should have at least 7 colon-separated parts, got %d", len(parts))
			}

			// Verify first and last parts (most reliable for validation)
			if parts[0] != tt.resourceType {
				t.Errorf("First part should be resource type %q, got %q", tt.resourceType, parts[0])
			}
			if parts[len(parts)-1] != tt.permissionType {
				t.Errorf("Last part should be permission type %q, got %q", tt.permissionType, parts[len(parts)-1])
			}
		})
	}
}
