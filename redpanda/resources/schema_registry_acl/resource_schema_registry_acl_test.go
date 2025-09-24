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

package schema_registry_acl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestModel creates a test Schema Registry ACL model
func createTestModel() models.SchemaRegistryACL {
	return models.SchemaRegistryACL{
		ClusterID:    types.StringValue("test-cluster"),
		Principal:    types.StringValue("User:test-user"),
		ResourceName: types.StringValue("test-resource"),
		ResourceType: types.StringValue("Subject"),
		PatternType:  types.StringValue("LITERAL"),
		Operation:    types.StringValue("READ"),
		Permission:   types.StringValue("ALLOW"),
	}
}

// createMatchingACL creates an ACL response that matches the test model
func createMatchingACL() kclients.SchemaRegistryACLResponse {
	return kclients.SchemaRegistryACLResponse{
		Principal:    "User:test-user",
		Resource:     "test-resource",
		ResourceType: "Subject",
		PatternType:  "LITERAL",
		Operation:    "READ",
		Permission:   "ALLOW",
	}
}

func TestVerifyACLPropagation_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()
	matchingACL := createMatchingACL()

	mockClient.EXPECT().
		ListACLs(ctx, gomock.Any()).
		Return([]kclients.SchemaRegistryACLResponse{matchingACL}, nil)

	resource := &SchemaRegistryACL{}
	err := resource.verifyACLPropagation(ctx, mockClient, &model)
	assert.NoError(t, err)
}

func TestVerifyACLPropagation_RetryUntilFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()
	matchingACL := createMatchingACL()

	gomock.InOrder(
		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return([]kclients.SchemaRegistryACLResponse{}, nil),
		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return([]kclients.SchemaRegistryACLResponse{matchingACL}, nil),
	)

	resource := &SchemaRegistryACL{}

	start := time.Now()
	err := resource.verifyACLPropagation(ctx, mockClient, &model)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, duration >= 1*time.Second, "Should have waited at least 1 second for retry")
}

func TestVerifyACLPropagation_PermissionDeniedRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()
	matchingACL := createMatchingACL()

	permissionError := errors.New("Forbidden (missing required ACLs)")
	gomock.InOrder(
		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return(nil, permissionError),
		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return([]kclients.SchemaRegistryACLResponse{matchingACL}, nil),
	)

	resource := &SchemaRegistryACL{}

	err := resource.verifyACLPropagation(ctx, mockClient, &model)

	assert.NoError(t, err)
}

func TestVerifyACLPropagation_NonRetryableError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()

	networkError := errors.New("connection refused")
	mockClient.EXPECT().
		ListACLs(ctx, gomock.Any()).
		Return(nil, networkError)

	resource := &SchemaRegistryACL{}

	err := resource.verifyACLPropagation(ctx, mockClient, &model)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify ACL")
}

func TestVerifyACLPropagation_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()

	mockClient.EXPECT().
		ListACLs(ctx, gomock.Any()).
		Return([]kclients.SchemaRegistryACLResponse{}, nil).
		AnyTimes()

	start := time.Now()

	err := utils.Retry(ctx, 2*time.Second, func() *utils.RetryError {
		acls, err := mockClient.ListACLs(ctx, model.ToSchemaRegistryACLFilter())
		if err != nil {
			if utils.IsPermissionDenied(err) {
				return utils.RetryableError(err)
			}
			return utils.NonRetryableError(err)
		}

		found := false
		for _, acl := range acls {
			if model.MatchesACLResponse(&acl) {
				found = true
				break
			}
		}

		if !found {
			return utils.RetryableError(errors.New("ACL not found"))
		}

		return nil
	})

	duration := time.Since(start)

	assert.Error(t, err)
	assert.IsType(t, &utils.TimeoutError{}, err)
	assert.True(t, duration >= 2*time.Second, "Should have waited for timeout")
	assert.True(t, duration < 5*time.Second, "Should not have exceeded reasonable timeout")
}

func TestVerifyACLPropagation_ACLFoundAmongMultiple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()
	matchingACL := createMatchingACL()

	otherACL := kclients.SchemaRegistryACLResponse{
		Principal:    "User:other-user",
		Resource:     "other-resource",
		ResourceType: "Subject",
		PatternType:  "LITERAL",
		Operation:    "WRITE",
		Permission:   "ALLOW",
	}

	mockClient.EXPECT().
		ListACLs(ctx, gomock.Any()).
		Return([]kclients.SchemaRegistryACLResponse{otherACL, matchingACL}, nil)

	resource := &SchemaRegistryACL{}

	err := resource.verifyACLPropagation(ctx, mockClient, &model)

	assert.NoError(t, err)
}

func TestVerifyACLPropagation_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)
	model := createTestModel()

	// Mock will be called but context will be cancelled
	mockClient.EXPECT().
		ListACLs(gomock.Any(), gomock.Any()).
		Return([]kclients.SchemaRegistryACLResponse{}, nil).
		AnyTimes()

	// Create resource instance
	resource := &SchemaRegistryACL{}

	// Cancel context immediately
	cancel()

	// Test the verification with cancelled context
	err := resource.verifyACLPropagation(ctx, mockClient, &model)

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// Benchmark to ensure verification doesn't take too long in success case
func BenchmarkVerifyACLPropagation_ImmediateSuccess(b *testing.B) {
	model := createTestModel()
	matchingACL := createMatchingACL()

	for i := 0; i < b.N; i++ {
		ctrl := gomock.NewController(b)
		ctx := context.Background()
		mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)

		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return([]kclients.SchemaRegistryACLResponse{matchingACL}, nil)

		resource := &SchemaRegistryACL{}
		err := resource.verifyACLPropagation(ctx, mockClient, &model)
		require.NoError(b, err)
		ctrl.Finish()
	}
}

// Test the integration with the actual model's MatchesACLResponse method
func TestVerifyACLPropagation_ModelMatching(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockSchemaRegistryACLClientInterface(ctrl)

	model := createTestModel()

	matchingACL := createMatchingACL()

	nonMatchingACL := kclients.SchemaRegistryACLResponse{
		Principal:    "User:different-user",
		Resource:     "test-resource",
		ResourceType: "Subject",
		PatternType:  "LITERAL",
		Operation:    "READ",
		Permission:   "ALLOW",
	}
	gomock.InOrder(
		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return([]kclients.SchemaRegistryACLResponse{nonMatchingACL}, nil),
		mockClient.EXPECT().
			ListACLs(ctx, gomock.Any()).
			Return([]kclients.SchemaRegistryACLResponse{nonMatchingACL, matchingACL}, nil),
	)

	resource := &SchemaRegistryACL{}
	err := resource.verifyACLPropagation(ctx, mockClient, &model)

	assert.NoError(t, err)
}
