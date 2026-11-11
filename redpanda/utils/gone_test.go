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

package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func deniedFor(t *testing.T, resourceID string) error {
	t.Helper()
	st, err := grpcstatus.New(grpccodes.PermissionDenied, "Missing required permission read").WithDetails(&errdetails.ErrorInfo{
		Reason:   "REASON_PERMISSION_DENIED",
		Domain:   "redpanda.com/controlplane",
		Metadata: map[string]string{"permission": "read", "resource_type": "controlplane_cluster", "resource_id": resourceID},
	})
	require.NoError(t, err)
	return st.Err()
}

func TestControlPlaneGone(t *testing.T) {
	const id = "dbplrourdb3c73bqnl4g"
	tests := []struct {
		name     string
		err      error
		wantGone bool
		wantWarn bool
	}{
		{"no error", nil, false, false},
		{"not found", grpcstatus.Error(grpccodes.NotFound, "cluster not found"), true, false},
		{"deleted: denied on this resource", deniedFor(t, id), true, true},
		{"deleted: same, wrapped by the client", fmt.Errorf("unable to request cluster with ID %q: %w", id, deniedFor(t, id)), true, true},
		{"denied on something else", deniedFor(t, "a845616f-0484-4506-9638-45fe28f34865"), false, false},
		{"denied without saying on what", grpcstatus.Error(grpccodes.PermissionDenied, "forbidden"), false, false},
		{"internal", grpcstatus.Error(grpccodes.Internal, "boom"), false, false},
		{"not a status", errors.New("connection reset"), false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gone, diags := ControlPlaneGone("cluster", id, tt.err)

			assert.Equal(t, tt.wantGone, gone)
			assert.False(t, diags.HasError())
			if !tt.wantWarn {
				assert.Empty(t, diags)
				return
			}
			require.Len(t, diags, 1)
			assert.Contains(t, diags[0].Summary(), id)
			assert.Contains(t, diags[0].Detail(), "cannot read")
		})
	}
}

func TestControlPlaneReadError(t *testing.T) {
	const id = "dbplrourdb3c73bqnl4g"
	assert.Contains(t, ControlPlaneReadError(id, deniedFor(t, id)), "has been deleted")
	assert.NotContains(t, ControlPlaneReadError(id, grpcstatus.Error(grpccodes.Internal, "boom")), "has been deleted")
}

func TestControlPlaneGoneOnDelete(t *testing.T) {
	const id = "dbplrourdb3c73bqnl4g"
	notFound := grpcstatus.Error(grpccodes.NotFound, "not found")
	tests := []struct {
		name       string
		deleteErr  error
		lookupErr  error
		wantLookup bool
		wantGone   bool
		wantWarn   bool
	}{
		{"delete says not found", notFound, nil, false, true, false},
		{"deleted: delete and read both denied", deniedFor(t, id), deniedFor(t, id), true, true, true},
		{"deleted in between: read says not found", deniedFor(t, id), notFound, true, true, false},
		{"readable but not deletable: still there", deniedFor(t, id), nil, true, false, false},
		{"denied, and the read fails some other way", deniedFor(t, id), grpcstatus.Error(grpccodes.Unavailable, "try later"), true, false, false},
		{"denied on something else", deniedFor(t, "another"), nil, false, false, false},
		{"internal", grpcstatus.Error(grpccodes.Internal, "boom"), nil, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			looked := false
			gone, diags := ControlPlaneGoneOnDelete("network", id, tt.deleteErr, func() error {
				looked = true
				return tt.lookupErr
			})

			assert.Equal(t, tt.wantLookup, looked)
			assert.Equal(t, tt.wantGone, gone)
			assert.False(t, diags.HasError())
			assert.Equal(t, tt.wantWarn, len(diags) == 1)
		})
	}
}
