// Copyright 2026 Redpanda Data, Inc.
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

package cloudtest

import (
	"context"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestFakeControlPlane_Smoke is the wiring sanity check for cloudtest.Start.
// If it fails, every higher-level test using the fake fails too — fix this
// first.
func TestFakeControlPlane_Smoke(t *testing.T) {
	_, conn := Start(t)
	cs := controlplanev1grpc.NewClusterServiceClient(conn)
	os := controlplanev1grpc.NewOperationServiceClient(conn)
	ctx := context.Background()

	_, err := cs.GetCluster(ctx, &controlplanev1.GetClusterRequest{Id: "missing"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got %v", err)
	require.Equal(t, codes.NotFound, st.Code())

	createResp, err := cs.CreateCluster(ctx, &controlplanev1.CreateClusterRequest{
		Cluster: &controlplanev1.ClusterCreate{
			Name:            "smoke",
			ResourceGroupId: "rg-smoke",
			ThroughputTier:  "tier-1-aws-v2-arm",
			Type:            controlplanev1.Cluster_TYPE_DEDICATED,
			ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
			NetworkId:       "net-smoke",
			CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
			Region:          "us-east-1",
			Zones:           []string{"use1-az1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, controlplanev1.Operation_STATE_COMPLETED, createResp.GetOperation().GetState())
	clusterID := createResp.GetOperation().GetResourceId()
	require.NotEmpty(t, clusterID)

	getResp, err := cs.GetCluster(ctx, &controlplanev1.GetClusterRequest{Id: clusterID})
	require.NoError(t, err)
	require.Equal(t, "smoke", getResp.GetCluster().GetName())
	require.Equal(t, controlplanev1.Cluster_STATE_READY, getResp.GetCluster().GetState())

	opResp, err := os.GetOperation(ctx, &controlplanev1.GetOperationRequest{Id: "any"})
	require.NoError(t, err)
	require.Equal(t, controlplanev1.Operation_STATE_COMPLETED, opResp.GetOperation().GetState())

	_, err = cs.DeleteCluster(ctx, &controlplanev1.DeleteClusterRequest{Id: clusterID})
	require.NoError(t, err)
	_, err = cs.GetCluster(ctx, &controlplanev1.GetClusterRequest{Id: clusterID})
	require.Error(t, err)
}

// Two Creates must produce distinct IDs; the constant FakeClusterID
// previously caused the second to overwrite the first.
func TestFakeControlPlane_MultiClusterIDsAreUnique(t *testing.T) {
	ctx := context.Background()
	_, conn := Start(t)
	cs := controlplanev1grpc.NewClusterServiceClient(conn)

	mkReq := func(name string) *controlplanev1.CreateClusterRequest {
		return &controlplanev1.CreateClusterRequest{
			Cluster: &controlplanev1.ClusterCreate{
				Name:            name,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ResourceGroupId: "rg-multi",
				ThroughputTier:  "tier-1-aws-v2-arm",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				NetworkId:       "net-multi",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Region:          "us-east-1",
				Zones:           []string{"use1-az1"},
			},
		}
	}
	resp1, err := cs.CreateCluster(ctx, mkReq("a"))
	require.NoError(t, err)
	resp2, err := cs.CreateCluster(ctx, mkReq("b"))
	require.NoError(t, err)
	require.NotEqual(t, resp1.GetOperation().GetResourceId(), resp2.GetOperation().GetResourceId(),
		"each Create must produce a unique cluster ID")

	list, err := cs.ListClusters(ctx, &controlplanev1.ListClustersRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetClusters(), 2, "both clusters must coexist in the fake's store")
}
