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

package fakes

import (
	"context"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const serverlessClusterIDBase uint64 = 0x6000_0000_0000_0000

// ServerlessClusterFake is a stateful in-memory implementation of
// ServerlessClusterService. All three mutating RPCs are async — each publishes
// a completed Operation via op.Set.
//
// UpdateServerlessClusterRequest has no FieldMask; networking_config, tags, and
// private_link_id are full-replacement top-level fields.
type ServerlessClusterFake struct {
	controlplanev1grpc.UnimplementedServerlessClusterServiceServer

	op       *OperationFake
	mu       sync.Mutex
	clusters map[string]*controlplanev1.ServerlessCluster
	seq      atomic.Uint64
}

// NewServerlessClusterFake returns an empty fake bound to op.
func NewServerlessClusterFake(op *OperationFake) *ServerlessClusterFake {
	return &ServerlessClusterFake{op: op, clusters: map[string]*controlplanev1.ServerlessCluster{}}
}

// CreateServerlessCluster stores a new cluster in STATE_READY with every
// Computed-only sub-message populated (kafka_api, schema_registry,
// dataplane_api, console_url, prometheus) so the provider's Flatten lands a
// stable state on first apply.
func (f *ServerlessClusterFake) CreateServerlessCluster(_ context.Context, req *controlplanev1.CreateServerlessClusterRequest) (*controlplanev1.CreateServerlessClusterOperation, error) {
	in := req.GetServerlessCluster()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "serverless_cluster is required")
	}
	id := xidLike(serverlessClusterIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	cluster := &controlplanev1.ServerlessCluster{
		Id:               id,
		Name:             in.GetName(),
		ResourceGroupId:  in.GetResourceGroupId(),
		ServerlessRegion: in.GetServerlessRegion(),
		State:            controlplanev1.ServerlessCluster_STATE_READY,
		CreatedAt:        now,
		UpdatedAt:        now,
		KafkaApi: &controlplanev1.ServerlessCluster_KafkaAPI{
			SeedBrokers:        []string{"mock-broker-0.mock.redpanda.cloud:9092"},
			PrivateSeedBrokers: []string{"mock-broker-0.private.mock.redpanda.cloud:9092"},
		},
		SchemaRegistry: &controlplanev1.ServerlessCluster_SchemaRegistryStatus{
			Url:        "https://mock.schema-registry.redpanda.cloud",
			PrivateUrl: "https://mock.schema-registry.private.redpanda.cloud",
		},
		DataplaneApi: &controlplanev1.ServerlessCluster_DataplaneAPI{
			Url:        "bufnet",
			PrivateUrl: "bufnet-private",
		},
		ConsoleUrl:        "https://mock.console.redpanda.cloud",
		ConsolePrivateUrl: "https://mock.console.private.redpanda.cloud",
		Prometheus: &controlplanev1.ServerlessCluster_Prometheus{
			Url:        "https://mock.prometheus.redpanda.cloud",
			PrivateUrl: "https://mock.prometheus.private.redpanda.cloud",
		},
		NetworkingConfig: in.GetNetworkingConfig(),
		Tags:             in.GetTags(),
	}
	if in.HasPrivateLinkId() {
		v := in.GetPrivateLinkId()
		cluster.PrivateLinkId = &v
	}
	if cluster.NetworkingConfig == nil {
		cluster.NetworkingConfig = &controlplanev1.ServerlessNetworkingConfig{
			Public:  controlplanev1.ServerlessNetworkingConfig_STATE_ENABLED,
			Private: controlplanev1.ServerlessNetworkingConfig_STATE_DISABLED,
		}
	}

	f.mu.Lock()
	f.clusters[id] = cluster
	f.mu.Unlock()

	return &controlplanev1.CreateServerlessClusterOperation{Operation: completedOp(f.op, id)}, nil
}

// GetServerlessCluster returns the stored cluster or NotFound.
func (f *ServerlessClusterFake) GetServerlessCluster(_ context.Context, req *controlplanev1.GetServerlessClusterRequest) (*controlplanev1.GetServerlessClusterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cl, ok := f.clusters[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless_cluster %q not found", req.GetId())
	}
	return &controlplanev1.GetServerlessClusterResponse{ServerlessCluster: cl}, nil
}

// UpdateServerlessCluster overwrites networking_config, tags, and
// private_link_id (full replacement, no FieldMask). Bumps updated_at.
func (f *ServerlessClusterFake) UpdateServerlessCluster(_ context.Context, req *controlplanev1.UpdateServerlessClusterRequest) (*controlplanev1.UpdateServerlessClusterOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cl, ok := f.clusters[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless_cluster %q not found", req.GetId())
	}
	if req.GetNetworkingConfig() != nil {
		cl.NetworkingConfig = req.GetNetworkingConfig()
	}
	if req.HasPrivateLinkId() {
		v := req.GetPrivateLinkId()
		cl.PrivateLinkId = &v
	}
	if req.GetTags() != nil {
		cl.Tags = req.GetTags()
	}
	cl.UpdatedAt = timestamppb.Now()
	return &controlplanev1.UpdateServerlessClusterOperation{Operation: completedOp(f.op, req.GetId())}, nil
}

// DeleteServerlessCluster removes the stored cluster and publishes a completed
// Operation. Returns NotFound if absent.
func (f *ServerlessClusterFake) DeleteServerlessCluster(_ context.Context, req *controlplanev1.DeleteServerlessClusterRequest) (*controlplanev1.DeleteServerlessClusterOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.clusters[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "serverless_cluster %q not found", req.GetId())
	}
	delete(f.clusters, req.GetId())
	return &controlplanev1.DeleteServerlessClusterOperation{Operation: completedOp(f.op, req.GetId())}, nil
}
