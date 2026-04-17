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
	"fmt"
	"sort"
	"sync"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// clusterServer is an in-memory ClusterServiceServer. All operations
// complete synchronously and clusters are returned in STATE_READY on the
// first poll, so the resource's RetryGetCluster / AreWeDoneYet loops exit
// immediately. No timer-based state transitions.
type clusterServer struct {
	controlplanev1grpc.UnimplementedClusterServiceServer

	mu       sync.Mutex
	clusters map[string]*controlplanev1.Cluster
	nextID   int
}

func newClusterServer() *clusterServer {
	return &clusterServer{
		clusters: make(map[string]*controlplanev1.Cluster),
	}
}

func (s *clusterServer) CreateCluster(_ context.Context, req *controlplanev1.CreateClusterRequest) (*controlplanev1.CreateClusterOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := EchoFromClusterCreate(req.GetCluster())
	if c == nil {
		return nil, status.Error(codes.InvalidArgument, "missing cluster spec")
	}
	// Override the default FakeClusterID so multi-cluster tests do not
	// collide on the same map key. Direct callers of EchoFromClusterCreate
	// (unit tests) keep the stable const.
	s.nextID++
	c.Id = fmt.Sprintf("rp-fake%015d", s.nextID)
	s.clusters[c.GetId()] = c

	id := c.GetId()
	return &controlplanev1.CreateClusterOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-create-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

func (s *clusterServer) GetCluster(_ context.Context, req *controlplanev1.GetClusterRequest) (*controlplanev1.GetClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.clusters[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "cluster %q not found", req.GetId())
	}
	return &controlplanev1.GetClusterResponse{Cluster: c}, nil
}

// UpdateCluster overlays the patch onto the stored cluster. Only fields
// the resource's update path actually emits are honored — extend as
// needed.
func (s *clusterServer) UpdateCluster(_ context.Context, req *controlplanev1.UpdateClusterRequest) (*controlplanev1.UpdateClusterOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	patch := req.GetCluster()
	if patch == nil {
		return nil, status.Error(codes.InvalidArgument, "missing cluster patch")
	}
	cur, ok := s.clusters[patch.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "cluster %q not found", patch.GetId())
	}

	for _, p := range req.GetUpdateMask().GetPaths() {
		switch p {
		case "name":
			cur.Name = patch.GetName()
		case "throughput_tier":
			cur.ThroughputTier = patch.GetThroughputTier()
		case "cloud_provider_tags":
			cur.CloudProviderTags = patch.GetCloudProviderTags()
		case "read_replica_cluster_ids":
			cur.ReadReplicaClusterIds = patch.GetReadReplicaClusterIds()
		case "kafka_api":
			if patch.GetKafkaApi() != nil {
				cur.KafkaApi = &controlplanev1.Cluster_KafkaAPI{
					Mtls: patch.GetKafkaApi().GetMtls(),
					Sasl: patch.GetKafkaApi().GetSasl(),
				}
			} else {
				cur.KafkaApi = nil
			}
		case "http_proxy":
			if patch.GetHttpProxy() != nil {
				cur.HttpProxy = &controlplanev1.Cluster_HTTPProxyStatus{
					Mtls: patch.GetHttpProxy().GetMtls(),
					Sasl: patch.GetHttpProxy().GetSasl(),
				}
			} else {
				cur.HttpProxy = nil
			}
		case "schema_registry":
			if patch.GetSchemaRegistry() != nil {
				cur.SchemaRegistry = &controlplanev1.Cluster_SchemaRegistryStatus{
					Mtls: patch.GetSchemaRegistry().GetMtls(),
					Sasl: patch.GetSchemaRegistry().GetSasl(),
				}
			} else {
				cur.SchemaRegistry = nil
			}
		case "aws_private_link":
			// Real CP omits aws_private_link when disabled.
			if patch.GetAwsPrivateLink() != nil && patch.GetAwsPrivateLink().GetEnabled() {
				cur.AwsPrivateLink = &controlplanev1.Cluster_AWSPrivateLink{
					Enabled:           patch.GetAwsPrivateLink().GetEnabled(),
					ConnectConsole:    patch.GetAwsPrivateLink().GetConnectConsole(),
					AllowedPrincipals: patch.GetAwsPrivateLink().GetAllowedPrincipals(),
					SupportedRegions:  patch.GetAwsPrivateLink().GetSupportedRegions(),
					Status:            fakeAwsPrivateLinkStatus(patch.GetAwsPrivateLink().GetEnabled()),
				}
			} else {
				cur.AwsPrivateLink = nil
			}
		case "gcp_private_service_connect":
			if patch.GetGcpPrivateServiceConnect() != nil {
				cur.GcpPrivateServiceConnect = &controlplanev1.Cluster_GCPPrivateServiceConnect{
					Enabled:             patch.GetGcpPrivateServiceConnect().GetEnabled(),
					GlobalAccessEnabled: patch.GetGcpPrivateServiceConnect().GetGlobalAccessEnabled(),
					ConsumerAcceptList:  patch.GetGcpPrivateServiceConnect().GetConsumerAcceptList(),
				}
			} else {
				cur.GcpPrivateServiceConnect = nil
			}
		case "azure_private_link":
			if patch.GetAzurePrivateLink() != nil {
				cur.AzurePrivateLink = &controlplanev1.Cluster_AzurePrivateLink{
					Enabled:              patch.GetAzurePrivateLink().GetEnabled(),
					ConnectConsole:       patch.GetAzurePrivateLink().GetConnectConsole(),
					AllowedSubscriptions: patch.GetAzurePrivateLink().GetAllowedSubscriptions(),
				}
			} else {
				cur.AzurePrivateLink = nil
			}
		case "maintenance_window_config":
			cur.MaintenanceWindowConfig = patch.GetMaintenanceWindowConfig()
		case "kafka_connect":
			cur.KafkaConnect = patch.GetKafkaConnect() //nolint:staticcheck // deprecated but still used
		case "cluster_configuration":
			if patch.GetClusterConfiguration() != nil {
				cur.ClusterConfiguration = &controlplanev1.Cluster_ClusterConfiguration{
					CustomProperties: patch.GetClusterConfiguration().GetCustomProperties(),
				}
			} else {
				cur.ClusterConfiguration = nil
			}
		default:
			// Unknown field-mask path: fail loudly so provider/fake drift
			// surfaces as a test failure instead of a silent no-op.
			return nil, status.Errorf(codes.Unimplemented, "fake: unsupported field-mask path %q", p)
		}
	}

	// Keep the private_link_sasl/mtls endpoint URLs in lockstep with the
	// cluster's PL state — without this, plan-modifier bugs that depend
	// on the URL transition aren't reproducible.
	RecomputeClusterEndpoints(cur)

	id := cur.GetId()
	return &controlplanev1.UpdateClusterOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-update-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

// DeleteCluster is idempotent: deleting a missing cluster is a no-op.
func (s *clusterServer) DeleteCluster(_ context.Context, req *controlplanev1.DeleteClusterRequest) (*controlplanev1.DeleteClusterOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := req.GetId()
	delete(s.clusters, id)
	return &controlplanev1.DeleteClusterOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-delete-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

// ListClusters is implemented so sweepers don't see Unimplemented.
// Returns clusters in stable ID order so tests with ordered assertions
// don't flake on map iteration nondeterminism.
func (s *clusterServer) ListClusters(_ context.Context, _ *controlplanev1.ListClustersRequest) (*controlplanev1.ListClustersResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.clusters))
	for id := range s.clusters {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]*controlplanev1.Cluster, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.clusters[id])
	}
	return &controlplanev1.ListClustersResponse{Clusters: out}, nil
}
