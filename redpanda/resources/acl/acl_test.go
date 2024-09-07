package acl

import (
	"testing"

	dataplanev1alpha2 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1alpha2"
)

// These are golden tests, to ensure we parse properly and the string types
// does not change during the proto file updates.

func Test_stringToACLResourceType(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  string
		exp    dataplanev1alpha2.ACL_ResourceType
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1alpha2.ACL_RESOURCE_TYPE_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1alpha2.ACL_RESOURCE_TYPE_ANY, false},
		{"topic", "TOPIC", dataplanev1alpha2.ACL_RESOURCE_TYPE_TOPIC, false},
		{"group", "GROUP", dataplanev1alpha2.ACL_RESOURCE_TYPE_GROUP, false},
		{"transactional", "TRANSACTIONAL_ID", dataplanev1alpha2.ACL_RESOURCE_TYPE_TRANSACTIONAL_ID, false},
		{"delegation token", "DELEGATION_TOKEN", dataplanev1alpha2.ACL_RESOURCE_TYPE_DELEGATION_TOKEN, false},
		{"user", "USER", dataplanev1alpha2.ACL_RESOURCE_TYPE_USER, false},
		{"user", "CLUSTER", dataplanev1alpha2.ACL_RESOURCE_TYPE_CLUSTER, false},
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
		input dataplanev1alpha2.ACL_ResourceType
		exp   string
	}{
		{"unspecified", dataplanev1alpha2.ACL_RESOURCE_TYPE_UNSPECIFIED, "UNSPECIFIED"},
		{"any", dataplanev1alpha2.ACL_RESOURCE_TYPE_ANY, "ANY"},
		{"topic", dataplanev1alpha2.ACL_RESOURCE_TYPE_TOPIC, "TOPIC"},
		{"group", dataplanev1alpha2.ACL_RESOURCE_TYPE_GROUP, "GROUP"},
		{"transactional", dataplanev1alpha2.ACL_RESOURCE_TYPE_TRANSACTIONAL_ID, "TRANSACTIONAL_ID"},
		{"delegation token", dataplanev1alpha2.ACL_RESOURCE_TYPE_DELEGATION_TOKEN, "DELEGATION_TOKEN"},
		{"user", dataplanev1alpha2.ACL_RESOURCE_TYPE_USER, "USER"},
		{"user", dataplanev1alpha2.ACL_RESOURCE_TYPE_CLUSTER, "CLUSTER"},
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
		exp    dataplanev1alpha2.ACL_ResourcePatternType
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_ANY, false},
		{"match", "MATCH", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_MATCH, false},
		{"literal", "LITERAL", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_LITERAL, false},
		{"prefixed", "PREFIXED", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_PREFIXED, false},
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
		input dataplanev1alpha2.ACL_ResourcePatternType
		exp   string
	}{
		{"unspecified", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_UNSPECIFIED, "UNSPECIFIED"},
		{"any", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_ANY, "ANY"},
		{"match", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_MATCH, "MATCH"},
		{"literal", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_LITERAL, "LITERAL"},
		{"prefixed", dataplanev1alpha2.ACL_RESOURCE_PATTERN_TYPE_PREFIXED, "PREFIXED"},
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
		exp    dataplanev1alpha2.ACL_PermissionType
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1alpha2.ACL_PERMISSION_TYPE_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1alpha2.ACL_PERMISSION_TYPE_ANY, false},
		{"match", "DENY", dataplanev1alpha2.ACL_PERMISSION_TYPE_DENY, false},
		{"literal", "ALLOW", dataplanev1alpha2.ACL_PERMISSION_TYPE_ALLOW, false},
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
		exp    dataplanev1alpha2.ACL_Operation
		expErr bool
	}{
		{"unspecified", "UNSPECIFIED", dataplanev1alpha2.ACL_OPERATION_UNSPECIFIED, false},
		{"any", "ANY", dataplanev1alpha2.ACL_OPERATION_ANY, false},
		{"all", "ALL", dataplanev1alpha2.ACL_OPERATION_ALL, false},
		{"read", "READ", dataplanev1alpha2.ACL_OPERATION_READ, false},
		{"write", "WRITE", dataplanev1alpha2.ACL_OPERATION_WRITE, false},
		{"create", "CREATE", dataplanev1alpha2.ACL_OPERATION_CREATE, false},
		{"delete", "DELETE", dataplanev1alpha2.ACL_OPERATION_DELETE, false},
		{"alter", "ALTER", dataplanev1alpha2.ACL_OPERATION_ALTER, false},
		{"describe", "DESCRIBE", dataplanev1alpha2.ACL_OPERATION_DESCRIBE, false},
		{"cluster action", "CLUSTER_ACTION", dataplanev1alpha2.ACL_OPERATION_CLUSTER_ACTION, false},
		{"describe configs", "DESCRIBE_CONFIGS", dataplanev1alpha2.ACL_OPERATION_DESCRIBE_CONFIGS, false},
		{"alter configs", "ALTER_CONFIGS", dataplanev1alpha2.ACL_OPERATION_ALTER_CONFIGS, false},
		{"idempotent write", "IDEMPOTENT_WRITE", dataplanev1alpha2.ACL_OPERATION_IDEMPOTENT_WRITE, false},
		{"create tokens", "CREATE_TOKENS", dataplanev1alpha2.ACL_OPERATION_CREATE_TOKENS, false},
		{"describe tokens", "DESCRIBE_TOKENS", dataplanev1alpha2.ACL_OPERATION_DESCRIBE_TOKENS, false},
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
