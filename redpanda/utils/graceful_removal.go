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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// RemovalAction represents what action to take after checking error conditions
type RemovalAction int

const (
	// RemoveFromState indicates the resource should be removed from Terraform state
	RemoveFromState RemovalAction = iota
	// KeepInState indicates the resource should remain in Terraform state
	KeepInState
	// ErrorNotHandled indicates the error is not a graceful removal case and should be handled by the caller
	ErrorNotHandled
)

// HandleGracefulRemoval handles the common pattern when a resource cannot be
// accessed due to NotFound, ClusterUnreachable, or PermissionDenied errors.
//
// Parameters:
//   - ctx: Context for logging
//   - resourceType: e.g., "pipeline", "topic", "user"
//   - resourceID: The resource identifier for messages
//   - allowDeletion: The allow_deletion attribute value
//   - err: The original error
//   - operation: Description of the failed operation (e.g., "find user", "create client", "delete topic")
//
// Returns:
//   - RemovalAction: RemoveFromState, KeepInState, or ErrorNotHandled
//   - diag.Diagnostics: Warning (if removing), Error (if blocked or unhandled error)
//
// Behavior:
//   - For NotFound, ClusterUnreachable, PermissionDenied, or nil errors:
//   - If allowDeletion is null or true: Logs info, returns (RemoveFromState, warning or none)
//   - If allowDeletion is explicitly false: Logs warning, returns (KeepInState, error)
//   - NotFound and nil errors don't produce warnings (clean removal)
//   - For other errors: Returns (ErrorNotHandled, error diagnostic with "failed to {operation}: {err}")
func HandleGracefulRemoval(
	ctx context.Context,
	resourceType, resourceID string,
	allowDeletion types.Bool,
	err error,
	operation string,
) (RemovalAction, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Determine error type for appropriate messaging
	var reason string
	isCleanRemoval := err == nil || IsNotFound(err)

	switch {
	case err == nil:
		reason = "no error"
	case IsNotFound(err):
		reason = "not found"
	case IsClusterUnreachable(err):
		reason = "cluster unreachable"
	case IsPermissionDenied(err):
		reason = "permission denied"
	default:
		// Not a graceful removal case - return error diagnostic
		diags.AddError(
			fmt.Sprintf("Failed to %s", operation),
			fmt.Sprintf("failed to %s: %s", operation, DeserializeGrpcError(err)),
		)
		return ErrorNotHandled, diags
	}

	errMsg := ""
	if err != nil {
		errMsg = DeserializeGrpcError(err)
	}

	// Check if removal is blocked (allow_deletion explicitly set to false)
	if !allowDeletion.IsNull() && !allowDeletion.ValueBool() {
		tflog.Warn(ctx, fmt.Sprintf("%s %s %s, keeping in state since allow_deletion is false", resourceType, resourceID, reason))
		detail := fmt.Sprintf("%s %s cannot be removed (%s) and allow_deletion is false. Set allow_deletion = true to remove from state.",
			resourceType, resourceID, reason)
		if errMsg != "" {
			detail += " Error: " + errMsg
		}
		diags.AddError(fmt.Sprintf("Cannot Remove %s", resourceType), detail)
		return KeepInState, diags
	}

	// Removal is allowed (null or true)
	tflog.Info(ctx, fmt.Sprintf("%s %s %s, removing from state", resourceType, resourceID, reason))

	// Only add warning for non-clean cases (NotFound/nil are expected/clean)
	if !isCleanRemoval {
		detail := fmt.Sprintf("%s %s: %s. Resource removed from Terraform state but may still exist.",
			resourceType, resourceID, reason)
		if errMsg != "" {
			detail += " Error: " + errMsg
		}
		diags.AddWarning(fmt.Sprintf("Removing %s From State", resourceType), detail)
	}

	return RemoveFromState, diags
}
