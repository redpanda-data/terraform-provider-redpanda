// Copyright 2024 Redpanda Data, Inc.
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

package schema

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseImportID(t *testing.T) {
	tests := []struct {
		name        string
		importID    string
		want        *importIDComponents
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid import - basic schema",
			importID: "cluster-123:my-subject:1:myuser:mypass",
			want: &importIDComponents{
				clusterID: "cluster-123",
				subject:   "my-subject",
				version:   1,
				username:  "myuser",
				password:  "mypass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with hyphens",
			importID: "cluster-456:my-test-subject:5:testuser:testpass",
			want: &importIDComponents{
				clusterID: "cluster-456",
				subject:   "my-test-subject",
				version:   5,
				username:  "testuser",
				password:  "testpass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with dots",
			importID: "cluster-789:com.example.schema:10:admin:secret123",
			want: &importIDComponents{
				clusterID: "cluster-789",
				subject:   "com.example.schema",
				version:   10,
				username:  "admin",
				password:  "secret123",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with underscores",
			importID: "prod-cluster:user_profile_schema:2:svc_account:p@ssw0rd",
			want: &importIDComponents{
				clusterID: "prod-cluster",
				subject:   "user_profile_schema",
				version:   2,
				username:  "svc_account",
				password:  "p@ssw0rd",
			},
			wantErr: false,
		},
		{
			name:     "valid import - version zero",
			importID: "cluster-1:test-subject:0:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "test-subject",
				version:   0,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - large version number",
			importID: "cluster-1:schema:999999:admin:adminpass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "schema",
				version:   999999,
				username:  "admin",
				password:  "adminpass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with mixed case",
			importID: "cluster-1:MyTestSchema:1:TestUser:TestPass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "MyTestSchema",
				version:   1,
				username:  "TestUser",
				password:  "TestPass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - subject with numbers",
			importID: "cluster-1:schema123:1:user123:pass123",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "schema123",
				version:   1,
				username:  "user123",
				password:  "pass123",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - missing password",
			importID:    "cluster-1:my-subject:1:myuser",
			want:        nil,
			wantErr:     true,
			errContains: "got 4 parts (expected 5)",
		},
		{
			name:        "invalid import - missing username and password",
			importID:    "cluster-1:my-subject:1",
			want:        nil,
			wantErr:     true,
			errContains: "got 3 parts (expected 5)",
		},
		{
			name:        "invalid import - missing version, username, and password",
			importID:    "cluster-1:my-subject",
			want:        nil,
			wantErr:     true,
			errContains: "got 2 parts (expected 5)",
		},
		{
			name:        "invalid import - only cluster_id",
			importID:    "cluster-1",
			want:        nil,
			wantErr:     true,
			errContains: "got 1 parts (expected 5)",
		},
		{
			name:        "invalid import - empty string",
			importID:    "",
			want:        nil,
			wantErr:     true,
			errContains: "got 1 parts (expected 5)",
		},
		{
			name:        "invalid import - too many parts",
			importID:    "cluster-1:subject:1:user:pass:extra",
			want:        nil,
			wantErr:     true,
			errContains: "got 6 parts (expected 5)",
		},
		{
			name:        "invalid import - too many colons",
			importID:    "cluster-1:subject:1:user:pass:extra:more",
			want:        nil,
			wantErr:     true,
			errContains: "got 7 parts (expected 5)",
		},
		{
			name:     "valid import - empty cluster_id",
			importID: ":my-subject:1:user:pass",
			want: &importIDComponents{
				clusterID: "",
				subject:   "my-subject",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - empty subject",
			importID: "cluster-1::1:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - empty version",
			importID:    "cluster-1:my-subject::user:pass",
			want:        nil,
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:     "valid import - empty username",
			importID: "cluster-1:my-subject:1::pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   1,
				username:  "",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - empty password",
			importID: "cluster-1:my-subject:1:user:",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   1,
				username:  "user",
				password:  "",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - all empty",
			importID:    "::::",
			want:        nil,
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:     "valid import - special characters in subject",
			importID: "cluster-1:test-subject-$special:1:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "test-subject-$special",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - special characters in password",
			importID: "cluster-1:subject:1:user:p@ss!w0rd#",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "subject",
				version:   1,
				username:  "user",
				password:  "p@ss!w0rd#",
			},
			wantErr: false,
		},
		{
			name:     "valid import - version with leading zeros",
			importID: "cluster-1:my-subject:001:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:        "invalid import - non-numeric version",
			importID:    "cluster-1:my-subject:latest:user:pass",
			want:        nil,
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:     "valid import - negative version (parses successfully)",
			importID: "cluster-1:my-subject:-1:user:pass",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "my-subject",
				version:   -1,
				username:  "user",
				password:  "pass",
			},
			wantErr: false,
		},
		{
			name:        "valid import - colons in password",
			importID:    "cluster-1:subject:1:user:pass:with:colons",
			want:        nil,
			wantErr:     true,
			errContains: "got 7 parts (expected 5)",
		},
		{
			name:     "valid import - UUID cluster ID",
			importID: "c7n1234567890ab:my-subject:1:admin:secretpass",
			want: &importIDComponents{
				clusterID: "c7n1234567890ab",
				subject:   "my-subject",
				version:   1,
				username:  "admin",
				password:  "secretpass",
			},
			wantErr: false,
		},
		{
			name:     "valid import - complex subject name",
			importID: "cluster-1:com.example.avro.User-v2:42:svc_user:complex_P@ss123",
			want: &importIDComponents{
				clusterID: "cluster-1",
				subject:   "com.example.avro.User-v2",
				version:   42,
				username:  "svc_user",
				password:  "complex_P@ss123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseImportID(tt.importID)

			if tt.wantErr {
				require.Error(t, err, "parseImportID should return error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "error message should contain expected string")
				}
				assert.Nil(t, got, "result should be nil on error")
				return
			}

			require.NoError(t, err, "parseImportID should not return error")
			require.NotNil(t, got, "result should not be nil")

			assert.Equal(t, tt.want.clusterID, got.clusterID, "clusterID should match")
			assert.Equal(t, tt.want.subject, got.subject, "subject should match")
			assert.Equal(t, tt.want.version, got.version, "version should match")
			assert.Equal(t, tt.want.username, got.username, "username should match")
			assert.Equal(t, tt.want.password, got.password, "password should match")
		})
	}
}

// TestImportTypeConversions validates that parseImportID returns correct types
// and that they convert properly to Terraform types (types.StringValue(), types.Int64Value()).
func TestImportTypeConversions(t *testing.T) {
	tests := []struct {
		name            string
		importID        string
		expectClusterID string
		expectSubject   string
		expectVersion   int64
		expectUsername  string
		expectPassword  string
		wantErr         bool
		errContains     string
	}{
		{
			name:            "basic types convert correctly",
			importID:        "cluster-123:test-subject:42:testuser:testpass",
			expectClusterID: "cluster-123",
			expectSubject:   "test-subject",
			expectVersion:   42,
			expectUsername:  "testuser",
			expectPassword:  "testpass",
			wantErr:         false,
		},
		{
			name:            "version zero",
			importID:        "cluster-1:subject:0:user:pass",
			expectClusterID: "cluster-1",
			expectSubject:   "subject",
			expectVersion:   0,
			expectUsername:  "user",
			expectPassword:  "pass",
			wantErr:         false,
		},
		{
			name:            "large version number",
			importID:        "cluster-1:subject:999999:user:pass",
			expectClusterID: "cluster-1",
			expectSubject:   "subject",
			expectVersion:   999999,
			expectUsername:  "user",
			expectPassword:  "pass",
			wantErr:         false,
		},
		{
			name:            "negative version",
			importID:        "cluster-1:subject:-1:user:pass",
			expectClusterID: "cluster-1",
			expectSubject:   "subject",
			expectVersion:   -1,
			expectUsername:  "user",
			expectPassword:  "pass",
			wantErr:         false,
		},
		{
			name:            "empty strings",
			importID:        "::1::",
			expectClusterID: "",
			expectSubject:   "",
			expectVersion:   1,
			expectUsername:  "",
			expectPassword:  "",
			wantErr:         false,
		},
		{
			name:            "special characters in strings",
			importID:        "cluster-123:test-$subject:1:user@example.com:p@ss!w0rd#",
			expectClusterID: "cluster-123",
			expectSubject:   "test-$subject",
			expectVersion:   1,
			expectUsername:  "user@example.com",
			expectPassword:  "p@ss!w0rd#",
			wantErr:         false,
		},
		{
			name:        "non-numeric version fails",
			importID:    "cluster-1:subject:latest:user:pass",
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
		{
			name:        "empty version fails",
			importID:    "cluster-1:subject::user:pass",
			wantErr:     true,
			errContains: "version must be a valid integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components, err := parseImportID(tt.importID)

			if tt.wantErr {
				require.Error(t, err, "parseImportID should return error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, components)

			// Validate Go types
			assert.IsType(t, "", components.clusterID)
			assert.IsType(t, "", components.subject)
			assert.IsType(t, int64(0), components.version)
			assert.IsType(t, "", components.username)
			assert.IsType(t, "", components.password)

			// Validate values
			assert.Equal(t, tt.expectClusterID, components.clusterID)
			assert.Equal(t, tt.expectSubject, components.subject)
			assert.Equal(t, tt.expectVersion, components.version)
			assert.Equal(t, tt.expectUsername, components.username)
			assert.Equal(t, tt.expectPassword, components.password)

			// Validate Terraform type conversions
			clusterIDValue := types.StringValue(components.clusterID)
			assert.False(t, clusterIDValue.IsNull())
			assert.Equal(t, components.clusterID, clusterIDValue.ValueString())

			subjectValue := types.StringValue(components.subject)
			assert.False(t, subjectValue.IsNull())
			assert.Equal(t, components.subject, subjectValue.ValueString())

			versionValue := types.Int64Value(components.version)
			assert.False(t, versionValue.IsNull())
			assert.Equal(t, components.version, versionValue.ValueInt64())

			usernameValue := types.StringValue(components.username)
			assert.False(t, usernameValue.IsNull())
			assert.Equal(t, components.username, usernameValue.ValueString())

			passwordValue := types.StringValue(components.password)
			assert.False(t, passwordValue.IsNull())
			assert.Equal(t, components.password, passwordValue.ValueString())
		})
	}
}
