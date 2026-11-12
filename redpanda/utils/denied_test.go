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
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func TestControlPlaneNotFound(t *testing.T) {
	// Resource IDs are alphanumeric, so "404" turns up in them, and the client
	// wraps every lookup error with the ID.
	const id = "d404rourdb3c73bqnl4g"
	wrap := func(err error) error { return fmt.Errorf("unable to request cluster with ID %q: %w", id, err) }
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"no error", nil, false},
		{"not found", grpcstatus.Error(grpccodes.NotFound, "cluster not found"), true},
		{"not found, wrapped by the client", wrap(grpcstatus.Error(grpccodes.NotFound, "cluster not found")), true},
		{"client-side not found", NotFoundError{Message: "cluster not found"}, true},
		{"denied, wrapped with an ID containing 404", wrap(deniedFor(t, id)), false},
		{"internal, wrapped with an ID containing 404", wrap(grpcstatus.Error(grpccodes.Internal, "boom")), false},
		{"not a status", errors.New("connection reset"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ControlPlaneNotFound(tt.err))
		})
	}
	require.True(t, IsNotFound(wrap(deniedFor(t, id))), "the substring classifier is what this one exists to avoid")
}

func TestControlPlaneResourceError(t *testing.T) {
	const id = "dbplrourdb3c73bqnl4g"
	const summary = "failed to read cluster " + id
	tests := []struct {
		name       string
		err        error
		wantDenial bool
	}{
		{"denied on this resource", deniedFor(t, id), true},
		{"same, wrapped by the client", fmt.Errorf("unable to request cluster with ID %q: %w", id, deniedFor(t, id)), true},
		{"denied on something else", deniedFor(t, "a845616f-0484-4506-9638-45fe28f34865"), false},
		{"denied without saying on what", grpcstatus.Error(grpccodes.PermissionDenied, "forbidden"), false},
		{"not found", grpcstatus.Error(grpccodes.NotFound, "cluster not found"), false},
		{"internal", grpcstatus.Error(grpccodes.Internal, "boom"), false},
		{"not a status", errors.New("connection reset"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ControlPlaneResourceError(summary, "cluster", id, tt.err)

			assert.Equal(t, diag.SeverityError, d.Severity())
			assert.Contains(t, d.Detail(), DeserializeGrpcError(tt.err), "the original error must survive")
			if !tt.wantDenial {
				assert.Equal(t, summary, d.Summary())
				assert.Equal(t, DeserializeGrpcError(tt.err), d.Detail())
				return
			}
			assert.Equal(t, "Redpanda Cloud denied access to cluster "+id, d.Summary())
			assert.Contains(t, d.Detail(), "no longer exists")
			assert.Contains(t, d.Detail(), "terraform state rm <resource address>")
			assert.Contains(t, d.Detail(), "role bindings")
		})
	}
}

func TestControlPlaneDataSourceError(t *testing.T) {
	const id = "dbplrourdb3c73bqnl4g"
	const summary = "failed to read cluster " + id

	denied := ControlPlaneDataSourceError(summary, "cluster", id, deniedFor(t, id))
	assert.Equal(t, diag.SeverityError, denied.Severity())
	assert.Equal(t, "Redpanda Cloud denied access to cluster "+id, denied.Summary())
	assert.Contains(t, denied.Detail(), "no longer exists")
	assert.Contains(t, denied.Detail(), "role bindings")
	assert.Contains(t, denied.Detail(), "Missing required permission read")
	assert.NotContains(t, denied.Detail(), "terraform state rm")

	other := ControlPlaneDataSourceError(summary, "cluster", id, grpcstatus.Error(grpccodes.Internal, "boom"))
	assert.Equal(t, summary, other.Summary())
	assert.NotContains(t, other.Detail(), "no longer exists")

	byName := ControlPlaneDataSourceError(summary, "resource group", "", grpcstatus.Error(grpccodes.PermissionDenied, "forbidden"))
	assert.Equal(t, summary, byName.Summary())
}

func TestRetryGetCluster_Denied(t *testing.T) {
	const id = "dbplrourdb3c73bqnl4g"
	deleting := &controlplanev1.Cluster{Id: id, State: controlplanev1.Cluster_STATE_DELETING}
	stillDeleting := func(*controlplanev1.Cluster) *RetryError {
		return RetryableError(errors.New("still deleting"))
	}
	notFound := grpcstatus.Error(grpccodes.NotFound, "cluster not found")
	unnamed := grpcstatus.Error(grpccodes.PermissionDenied, "forbidden")

	tests := []struct {
		name        string
		wait        func(context.Context, time.Duration, string, cloud.CpClientSet, func(*controlplanev1.Cluster) *RetryError) (*controlplanev1.Cluster, error)
		finalErr    error
		wantErrCode grpccodes.Code
	}{
		{"create poll: a denial naming the cluster is an error", RetryGetCluster, deniedFor(t, id), grpccodes.PermissionDenied},
		{"create poll: any other denial is an error", RetryGetCluster, unnamed, grpccodes.PermissionDenied},
		{"create poll: not found ends the wait", RetryGetCluster, notFound, grpccodes.OK},
		{"create poll: a failure on an ID containing 404 is an error", RetryGetCluster, fmt.Errorf("unable to request cluster with ID %q: %w", "d404rourdb3c73bqnl4g", grpcstatus.Error(grpccodes.Internal, "boom")), grpccodes.Internal},
		{"delete poll: a denial naming the cluster ends the wait", RetryGetClusterUntilDeleted, deniedFor(t, id), grpccodes.OK},
		{"delete poll: any other denial is an error", RetryGetClusterUntilDeleted, unnamed, grpccodes.PermissionDenied},
		{"delete poll: a denial naming something else is an error", RetryGetClusterUntilDeleted, deniedFor(t, "another"), grpccodes.PermissionDenied},
		{"delete poll: not found ends the wait", RetryGetClusterUntilDeleted, notFound, grpccodes.OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockCpClientSet(gomock.NewController(t))
			gomock.InOrder(
				client.EXPECT().ClusterForID(gomock.Any(), id).Return(deleting, nil),
				client.EXPECT().ClusterForID(gomock.Any(), id).Return(nil, tt.finalErr),
			)

			cl, err := tt.wait(context.Background(), time.Minute, id, client, stillDeleting)

			assert.Nil(t, cl)
			assert.Equal(t, tt.wantErrCode, grpcstatus.Code(err))
		})
	}
}
