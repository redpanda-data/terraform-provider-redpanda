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

// serverlessPrivateLinkServer is an in-memory
// ServerlessPrivateLinkServiceServer. Links come up in STATE_READY with
// synthetic AWS status populated.
type serverlessPrivateLinkServer struct {
	controlplanev1grpc.UnimplementedServerlessPrivateLinkServiceServer

	mu     sync.Mutex
	links  map[string]*controlplanev1.ServerlessPrivateLink
	nextID int
}

func newServerlessPrivateLinkServer() *serverlessPrivateLinkServer {
	return &serverlessPrivateLinkServer{links: make(map[string]*controlplanev1.ServerlessPrivateLink)}
}

func (s *serverlessPrivateLinkServer) CreateServerlessPrivateLink(_ context.Context, req *controlplanev1.CreateServerlessPrivateLinkRequest) (*controlplanev1.CreateServerlessPrivateLinkOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec := req.GetServerlessPrivateLink()
	if spec == nil {
		return nil, status.Error(codes.InvalidArgument, "missing serverless private link spec")
	}
	s.nextID++
	id := fmt.Sprintf("rp-spl-fake%012d", s.nextID)
	link := EchoFromServerlessPrivateLinkCreate(id, spec)
	s.links[id] = link
	return &controlplanev1.CreateServerlessPrivateLinkOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-splcreate-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

func (s *serverlessPrivateLinkServer) GetServerlessPrivateLink(_ context.Context, req *controlplanev1.GetServerlessPrivateLinkRequest) (*controlplanev1.GetServerlessPrivateLinkResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless private link %q not found", req.GetId())
	}
	return &controlplanev1.GetServerlessPrivateLinkResponse{ServerlessPrivateLink: l}, nil
}

func (s *serverlessPrivateLinkServer) UpdateServerlessPrivateLink(_ context.Context, req *controlplanev1.UpdateServerlessPrivateLinkRequest) (*controlplanev1.UpdateServerlessPrivateLinkOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.GetId()
	cur, ok := s.links[id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless private link %q not found", id)
	}
	if aws := req.GetAwsConfig(); aws != nil {
		cur.CloudProviderConfig = &controlplanev1.ServerlessPrivateLink_AwsConfig{
			AwsConfig: &controlplanev1.ServerlessPrivateLink_AWS{
				AllowedPrincipals: aws.GetAllowedPrincipals(),
			},
		}
	}
	return &controlplanev1.UpdateServerlessPrivateLinkOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-splupdate-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}

func (s *serverlessPrivateLinkServer) DeleteServerlessPrivateLink(_ context.Context, req *controlplanev1.DeleteServerlessPrivateLinkRequest) (*controlplanev1.DeleteServerlessPrivateLinkOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.GetId()
	delete(s.links, id)
	return &controlplanev1.DeleteServerlessPrivateLinkOperation{
		Operation: &controlplanev1.Operation{
			Id:         "op-spldelete-" + id,
			ResourceId: &id,
			State:      controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}
