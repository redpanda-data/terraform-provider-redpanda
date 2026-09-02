// Copyright 2025 Redpanda Data, Inc.
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

package acc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDataplaneGates(t *testing.T) {
	t.Run("commented-out resources declare nothing", func(t *testing.T) {
		// BYOVPC's fixtures carry their dataplane resources commented out. Matching
		// them would gate steps on for a cluster that never creates them.
		gates := ParseDataplaneGates(`
resource "redpanda_cluster" "test" {
  name = "x"
}
# resource "redpanda_user" "test" {
#   name = "u"
# }
// resource "redpanda_topic" "test" {
//   name = "t"
// }
`)
		assert.False(t, gates.User, "commented user must not gate on")
		assert.False(t, gates.Topic, "commented topic must not gate on")
		assert.False(t, gates.Any(), "fixture declares no live dataplane resources")
	})

	t.Run("live resources gate on", func(t *testing.T) {
		gates := ParseDataplaneGates(`
resource "redpanda_user" "test" {}
resource "redpanda_topic" "test" {}
resource "redpanda_secret" "test" {}
resource "redpanda_pipeline" "test" {}
`)
		assert.True(t, gates.User)
		assert.True(t, gates.Topic)
		assert.True(t, gates.Secret)
		assert.True(t, gates.Pipeline)
		assert.False(t, gates.Role)
		assert.True(t, gates.Any())
	})

	t.Run("acl resource name follows the declared acl", func(t *testing.T) {
		admin := ParseDataplaneGates(`resource "redpanda_acl" "cluster_admin" {}`)
		assert.True(t, admin.ACL)
		assert.Equal(t, ClusterAdminACLResourceName, admin.ACLResourceName)

		topic := ParseDataplaneGates(`resource "redpanda_acl" "topic_access" {}`)
		assert.True(t, topic.ACL)
		assert.Equal(t, TopicAccessACLResourceName, topic.ACLResourceName)

		// The flip steps assert against ACLResourceName, so a fixture declaring
		// only the RedpandaRole-principal ACL must resolve to that one — not to
		// a better-known name it never declared.
		role := ParseDataplaneGates(`resource "redpanda_acl" "role_topic_read" {}`)
		assert.True(t, role.ACL)
		assert.Equal(t, RoleTopicReadACLResourceName, role.ACLResourceName)

		// topic_access wins when several are present.
		both := ParseDataplaneGates(`
resource "redpanda_acl" "cluster_admin" {}
resource "redpanda_acl" "topic_access" {}
resource "redpanda_acl" "role_topic_read" {}
`)
		assert.Equal(t, TopicAccessACLResourceName, both.ACLResourceName)
	})

	t.Run("no acl means no acl name", func(t *testing.T) {
		// Naming an ACL here would point the flip steps at a resource the
		// fixture never declares.
		gates := ParseDataplaneGates(`resource "redpanda_topic" "test" {}`)
		assert.False(t, gates.ACL)
		assert.Empty(t, gates.ACLResourceName)
	})
}

// TestDataplaneGatesForFixtures pins where dataplane coverage actually lives.
//
// A dedicated/BYOC/BYOVPC cluster takes 45 minutes to hours to stand up, so those
// fixtures declare only dataplane resources serverless cannot serve, plus topic as
// a cheap, stable canary proving the dataplane is reachable at all. Serverless
// declares everything else.
//
// If this test fails, a fixture gained or lost a dataplane resource. Confirm that
// was intended before updating the expectation.
func TestDataplaneGatesForFixtures(t *testing.T) {
	for _, tt := range []struct {
		name string
		dir  string
		want DataplaneGates
	}{
		{
			name: "byovpc aws declares no dataplane resources",
			dir:  AwsByocVpcClusterDir,
			want: DataplaneGates{},
		},
		{
			name: "byovpc gcp declares no dataplane resources",
			dir:  GcpByoVpcClusterDir,
			want: DataplaneGates{},
		},
		{
			// Topic alone: a canary proving this cluster's dataplane is
			// reachable. Every other dataplane resource is exercised on
			// serverless, where a failure costs a short run instead of a
			// cluster build.
			name: "byoc aws declares the topic canary only",
			dir:  AwsByocClusterDir,
			want: DataplaneGates{
				Topic:             true,
				TopicConfigurable: true,
			},
		},
		{
			name: "byoc gcp declares the topic canary only",
			dir:  GcpByocClusterDir,
			want: DataplaneGates{
				Topic:             true,
				TopicConfigurable: true,
			},
		},
		{
			// Only what serverless cannot serve: RBAC — role, role_assignment and
			// the RedpandaRole-principal ACL — plus the user they bind to, and the
			// topic canary. Everything else is exercised on serverless.
			name: "cluster aws declares rbac and the topic canary",
			dir:  AwsDedicatedClusterDir,
			want: DataplaneGates{
				User: true,
				ACL:  true,
				Role: true,
				// AWS alone threads var.user_password_wo; the GCP fixture does not.
				PasswordWo:        true,
				Topic:             true,
				TopicConfigurable: true,
				ACLResourceName:   RoleTopicReadACLResourceName,
			},
		},
		{
			name: "cluster gcp declares rbac and the topic canary",
			dir:  GcpDedicatedClusterDir,
			want: DataplaneGates{
				User:              true,
				ACL:               true,
				Role:              true,
				Topic:             true,
				TopicConfigurable: true,
				ACLResourceName:   RoleTopicReadACLResourceName,
			},
		},
		{
			// Serverless is the home for dataplane coverage. Role/role_assignment
			// are absent on purpose: the console SecurityService answers
			// Unimplemented there, so RBAC can only be exercised on dedicated.
			name: "serverless declares the full dataplane suite",
			dir:  ServerlessClusterDir,
			want: DataplaneGates{
				User:              true,
				ACL:               true,
				SchemaRegistryACL: true,
				Schema:            true,
				Topic:             true,
				Pipeline:          true,
				Secret:            true,
				PasswordWo:        true,
				TopicConfigurable: true,
				ACLResourceName:   TopicAccessACLResourceName,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SniffDataplaneGates(tt.dir)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
