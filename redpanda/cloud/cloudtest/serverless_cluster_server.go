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
	"sync"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// serverlessClusterServer is an in-memory ServerlessClusterServiceServer.
// Clusters come up in STATE_READY synchronously; every Computed endpoint
// URL is populated by EchoFromServerlessClusterCreate so schema plan
// modifiers have non-null state to work with.
type serverlessClusterServer struct {
	controlplanev1grpc.UnimplementedServerlessClusterServiceServer

	mu       sync.Mutex
	clusters map[string]*controlplanev1.ServerlessCluster
	nextID   int
}

func newServerlessClusterServer() *serverlessClusterServer {
	return &serverlessClusterServer{clusters: make(map[string]*controlplanev1.ServerlessCluster)}
}

func (s *serverlessClusterServer) CreateServerlessCluster(_ context.Context, req *controlplanev1.CreateServerlessClusterRequest) (*controlplanev1.CreateServerlessClusterOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec := req.GetServerlessCluster()
	if spec == nil {
		return nil, status.Error(codes.InvalidArgument, "missing serverless cluster spec")
	}
	s.nextID++
	id := fmt.Sprintf("rp-scl-fake%012d", s.nextID)
	c := EchoFromServerlessClusterCreate(id, spec)
	s.clusters[id] = c
	return &controlplanev1.CreateServerlessClusterOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-screate-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

func (s *serverlessClusterServer) GetServerlessCluster(_ context.Context, req *controlplanev1.GetServerlessClusterRequest) (*controlplanev1.GetServerlessClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clusters[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless cluster %q not found", req.GetId())
	}
	return &controlplanev1.GetServerlessClusterResponse{ServerlessCluster: c}, nil
}

func (s *serverlessClusterServer) UpdateServerlessCluster(_ context.Context, req *controlplanev1.UpdateServerlessClusterRequest) (*controlplanev1.UpdateServerlessClusterOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.GetId()
	cur, ok := s.clusters[id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless cluster %q not found", id)
	}
	if req.PrivateLinkId != nil {
		cur.PrivateLinkId = req.PrivateLinkId
	}
	if req.NetworkingConfig != nil {
		cur.NetworkingConfig = req.NetworkingConfig
	}
	return &controlplanev1.UpdateServerlessClusterOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-supdate-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

func (s *serverlessClusterServer) DeleteServerlessCluster(_ context.Context, req *controlplanev1.DeleteServerlessClusterRequest) (*controlplanev1.DeleteServerlessClusterOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.GetId()
	delete(s.clusters, id)
	return &controlplanev1.DeleteServerlessClusterOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-sdelete-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}
