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

package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func notFoundError() error {
	return status.Error(codes.NotFound, "resource not found")
}

func clusterUnreachableError() error {
	return errors.New("rpc error: code = Unavailable desc = name resolver error: produced zero addresses")
}

func permissionDeniedError() error {
	return status.Error(codes.PermissionDenied, "permission denied")
}

func TestHandleGracefulRemoval(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		resourceType    string
		resourceID      string
		allowDeletion   types.Bool
		err             error
		operation       string
		expectedAction  RemovalAction
		expectError     bool
		expectWarning   bool
		expectNoDiags   bool
		errorContains   string
		warningContains string
	}{
		// NotFound errors
		{
			name:           "NotFound_AllowDeletionNull_RemovesCleanly",
			resourceType:   "pipeline",
			resourceID:     "pipe-123",
			allowDeletion:  types.BoolNull(),
			err:            notFoundError(),
			operation:      "find pipeline",
			expectedAction: RemoveFromState,
			expectNoDiags:  true,
		},
		{
			name:           "NotFound_AllowDeletionTrue_RemovesCleanly",
			resourceType:   "pipeline",
			resourceID:     "pipe-123",
			allowDeletion:  types.BoolValue(true),
			err:            notFoundError(),
			operation:      "find pipeline",
			expectedAction: RemoveFromState,
			expectNoDiags:  true,
		},
		{
			name:           "NotFound_AllowDeletionFalse_BlocksRemoval",
			resourceType:   "pipeline",
			resourceID:     "pipe-123",
			allowDeletion:  types.BoolValue(false),
			err:            notFoundError(),
			operation:      "find pipeline",
			expectedAction: KeepInState,
			expectError:    true,
			errorContains:  "allow_deletion is false",
		},
		// ClusterUnreachable errors
		{
			name:            "Unreachable_AllowDeletionNull_RemovesWithWarning",
			resourceType:    "topic",
			resourceID:      "my-topic",
			allowDeletion:   types.BoolNull(),
			err:             clusterUnreachableError(),
			operation:       "create topic client",
			expectedAction:  RemoveFromState,
			expectWarning:   true,
			warningContains: "cluster unreachable",
		},
		{
			name:            "Unreachable_AllowDeletionTrue_RemovesWithWarning",
			resourceType:    "topic",
			resourceID:      "my-topic",
			allowDeletion:   types.BoolValue(true),
			err:             clusterUnreachableError(),
			operation:       "create topic client",
			expectedAction:  RemoveFromState,
			expectWarning:   true,
			warningContains: "may still exist",
		},
		{
			name:           "Unreachable_AllowDeletionFalse_BlocksRemoval",
			resourceType:   "topic",
			resourceID:     "my-topic",
			allowDeletion:  types.BoolValue(false),
			err:            clusterUnreachableError(),
			operation:      "create topic client",
			expectedAction: KeepInState,
			expectError:    true,
			errorContains:  "allow_deletion = true to remove",
		},
		// PermissionDenied errors
		{
			name:            "PermDenied_AllowDeletionNull_RemovesWithWarning",
			resourceType:    "schema",
			resourceID:      "schema-abc",
			allowDeletion:   types.BoolNull(),
			err:             permissionDeniedError(),
			operation:       "read schema",
			expectedAction:  RemoveFromState,
			expectWarning:   true,
			warningContains: "permission denied",
		},
		{
			name:           "PermDenied_AllowDeletionTrue_RemovesWithWarning",
			resourceType:   "schema",
			resourceID:     "schema-abc",
			allowDeletion:  types.BoolValue(true),
			err:            permissionDeniedError(),
			operation:      "read schema",
			expectedAction: RemoveFromState,
			expectWarning:  true,
		},
		{
			name:           "PermDenied_AllowDeletionFalse_BlocksRemoval",
			resourceType:   "schema",
			resourceID:     "schema-abc",
			allowDeletion:  types.BoolValue(false),
			err:            permissionDeniedError(),
			operation:      "read schema",
			expectedAction: KeepInState,
			expectError:    true,
			errorContains:  "permission denied",
		},
		// Edge cases
		{
			name:           "NilError_AllowDeletionNull_RemovesCleanly",
			resourceType:   "pipeline",
			resourceID:     "pipe-nil",
			allowDeletion:  types.BoolNull(),
			err:            nil,
			operation:      "find pipeline",
			expectedAction: RemoveFromState,
			expectNoDiags:  true,
		},
		{
			name:           "NilError_AllowDeletionFalse_BlocksRemoval",
			resourceType:   "pipeline",
			resourceID:     "pipe-nil",
			allowDeletion:  types.BoolValue(false),
			err:            nil,
			operation:      "find pipeline",
			expectedAction: KeepInState,
			expectError:    true,
			errorContains:  "allow_deletion is false",
		},
		{
			name:           "UnknownError_AllowDeletionNull_ReturnsErrorNotHandled",
			resourceType:   "user",
			resourceID:     "test-user",
			allowDeletion:  types.BoolNull(),
			err:            errors.New("some unknown error"),
			operation:      "find user",
			expectedAction: ErrorNotHandled,
			expectError:    true,
			errorContains:  "failed to find user",
		},
		{
			name:           "UnknownError_AllowDeletionFalse_ReturnsErrorNotHandled",
			resourceType:   "user",
			resourceID:     "test-user",
			allowDeletion:  types.BoolValue(false),
			err:            errors.New("some unknown error"),
			operation:      "find user",
			expectedAction: ErrorNotHandled,
			expectError:    true,
			errorContains:  "some unknown error",
		},
		{
			name:           "ResourceTypeInErrorMessage",
			resourceType:   "schemaregistryacl",
			resourceID:     "acl-xyz",
			allowDeletion:  types.BoolValue(false),
			err:            notFoundError(),
			operation:      "find schemaregistryacl",
			expectedAction: KeepInState,
			expectError:    true,
			errorContains:  "schemaregistryacl acl-xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, diags := HandleGracefulRemoval(ctx, tt.resourceType, tt.resourceID, tt.allowDeletion, tt.err, tt.operation)

			// Check action
			assert.Equal(t, tt.expectedAction, action, "unexpected RemovalAction")

			// Check diagnostics
			if tt.expectNoDiags {
				assert.False(t, diags.HasError(), "expected no errors")
				assert.Empty(t, diags.Warnings(), "expected no warnings")
				return
			}

			if tt.expectError {
				require.True(t, diags.HasError(), "expected error diagnostic")
				assert.Empty(t, diags.Warnings(), "expected no warnings when error")
				if tt.errorContains != "" {
					errSummary := diags.Errors()[0].Summary() + diags.Errors()[0].Detail()
					assert.Contains(t, errSummary, tt.errorContains)
				}
			}

			if tt.expectWarning {
				assert.False(t, diags.HasError(), "expected no errors when warning")
				require.NotEmpty(t, diags.Warnings(), "expected warning diagnostic")
				if tt.warningContains != "" {
					warnSummary := diags.Warnings()[0].Summary() + diags.Warnings()[0].Detail()
					assert.Contains(t, warnSummary, tt.warningContains)
				}
			}
		})
	}
}

func TestHandleGracefulRemoval_ReadScenario(t *testing.T) {
	ctx := context.Background()

	t.Run("Read_ClusterDeleted_DefaultBehavior_RemovesState", func(t *testing.T) {
		action, diags := HandleGracefulRemoval(ctx, "pipeline", "pipe-123", types.BoolNull(), clusterUnreachableError(), "create client")

		assert.Equal(t, RemoveFromState, action)
		assert.False(t, diags.HasError())
		assert.NotEmpty(t, diags.Warnings())
	})

	t.Run("Read_ClusterDeleted_DeletionBlocked_KeepsState", func(t *testing.T) {
		action, diags := HandleGracefulRemoval(ctx, "pipeline", "pipe-123", types.BoolValue(false), clusterUnreachableError(), "create client")

		assert.Equal(t, KeepInState, action)
		assert.True(t, diags.HasError())
	})
}

func TestHandleGracefulRemoval_DeleteScenario(t *testing.T) {
	ctx := context.Background()

	t.Run("Delete_ClusterDeleted_DefaultBehavior_Succeeds", func(t *testing.T) {
		_, diags := HandleGracefulRemoval(ctx, "pipeline", "pipe-123", types.BoolNull(), clusterUnreachableError(), "delete pipeline")

		assert.False(t, diags.HasError())
	})

	t.Run("Delete_ClusterDeleted_DeletionBlocked_Fails", func(t *testing.T) {
		_, diags := HandleGracefulRemoval(ctx, "pipeline", "pipe-123", types.BoolValue(false), clusterUnreachableError(), "delete pipeline")

		assert.True(t, diags.HasError())
	})

	t.Run("Delete_ResourceAlreadyGone_Succeeds", func(t *testing.T) {
		_, diags := HandleGracefulRemoval(ctx, "topic", "my-topic", types.BoolValue(true), notFoundError(), "delete topic")

		assert.False(t, diags.HasError())
		assert.Empty(t, diags.Warnings())
	})
}
