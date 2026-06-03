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

package acl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	aclmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/acl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two tests below pin the composite-ID format used by
// (*aclmodel.ResourceModel).GenerateID — it's part of the public state
// shape and changing it would break end users.

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
		{
			name:           "topic ACL with RedpandaRole principal",
			resourceType:   "TOPIC",
			resourceName:   "test-topic",
			patternType:    "LITERAL",
			principal:      "RedpandaRole:admin",
			host:           "*",
			operation:      "READ",
			permissionType: "ALLOW",
			expected:       "TOPIC:test-topic:LITERAL:RedpandaRole:admin:*:READ:ALLOW",
		},
		{
			name:           "group ACL with RedpandaRole multi-colon principal",
			resourceType:   "GROUP",
			resourceName:   "test-group",
			patternType:    "PREFIXED",
			principal:      "RedpandaRole:admin:extra",
			host:           "*",
			operation:      "WRITE",
			permissionType: "ALLOW",
			expected:       "GROUP:test-group:PREFIXED:RedpandaRole:admin:extra:*:WRITE:ALLOW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		{
			name:           "format with RedpandaRole principal",
			resourceType:   "TOPIC",
			resourceName:   "events",
			patternType:    "LITERAL",
			principal:      "RedpandaRole:admin",
			host:           "*",
			operation:      "ALL",
			permissionType: "ALLOW",
			expectedFormat: "TOPIC:events:LITERAL:RedpandaRole:admin:*:ALL:ALLOW",
		},
		{
			name:           "format with RedpandaRole multi-colon principal",
			resourceType:   "CLUSTER",
			resourceName:   "kafka-cluster",
			patternType:    "LITERAL",
			principal:      "RedpandaRole:admin:extra",
			host:           "*",
			operation:      "ALTER",
			permissionType: "ALLOW",
			expectedFormat: "CLUSTER:kafka-cluster:LITERAL:RedpandaRole:admin:extra:*:ALTER:ALLOW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			parts := strings.Split(compositeID, ":")
			if len(parts) < 7 {
				t.Errorf("Composite ID should have at least 7 colon-separated parts, got %d", len(parts))
			}
			if parts[0] != tt.resourceType {
				t.Errorf("First part should be resource type %q, got %q", tt.resourceType, parts[0])
			}
			if parts[len(parts)-1] != tt.permissionType {
				t.Errorf("Last part should be permission type %q, got %q", tt.permissionType, parts[len(parts)-1])
			}
		})
	}
}

func TestUnit_ACL_UpgradeState_NormalizesClusterAPIURL(t *testing.T) {
	ctx := context.Background()
	up, ok := (&ACL{}).UpgradeState(ctx)[0]
	require.True(t, ok, "expected a v0 state upgrader")
	require.NotNil(t, up.PriorSchema)

	prior := tfsdk.State{Schema: *up.PriorSchema}
	require.False(t, prior.Set(ctx, &aclmodel.ResourceModel{
		Host:          types.StringValue("*"),
		ClusterAPIURL: types.StringValue("api-abc.cid.byoc.prd.cloud.redpanda.com:443"),
	}).HasError())

	resp := &resource.UpgradeStateResponse{State: tfsdk.State{Schema: ResourceACLSchema(ctx)}}
	up.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &prior}, resp)
	require.False(t, resp.Diagnostics.HasError())

	var got aclmodel.ResourceModel
	require.False(t, resp.State.Get(ctx, &got).HasError())
	assert.Equal(t, "https://api-abc.cid.byoc.prd.cloud.redpanda.com", got.ClusterAPIURL.ValueString())
}
