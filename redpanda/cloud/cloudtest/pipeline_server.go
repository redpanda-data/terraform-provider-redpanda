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

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pipelineServer is an in-memory PipelineServiceServer. Pipelines come
// up in STATE_RUNNING by default, matching the real dataplane's
// "start on create" behavior. Start/Stop mutate State so the
// provider's state-management flow round-trips.
type pipelineServer struct {
	dataplanev1grpc.UnimplementedPipelineServiceServer

	mu        sync.Mutex
	pipelines map[string]*dataplanev1.Pipeline
	nextID    int
}

func newPipelineServer() *pipelineServer {
	return &pipelineServer{pipelines: make(map[string]*dataplanev1.Pipeline)}
}

func (s *pipelineServer) CreatePipeline(_ context.Context, req *dataplanev1.CreatePipelineRequest) (*dataplanev1.CreatePipelineResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec := req.GetPipeline()
	if spec == nil {
		return nil, status.Error(codes.InvalidArgument, "missing pipeline spec")
	}
	s.nextID++
	id := fmt.Sprintf("ppl-fake%012d", s.nextID)
	p := EchoFromPipelineCreate(id, spec)
	s.pipelines[id] = p
	return &dataplanev1.CreatePipelineResponse{Pipeline: p}, nil
}

func (s *pipelineServer) GetPipeline(_ context.Context, req *dataplanev1.GetPipelineRequest) (*dataplanev1.GetPipelineResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pipelines[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	return &dataplanev1.GetPipelineResponse{Pipeline: p}, nil
}

func (s *pipelineServer) UpdatePipeline(_ context.Context, req *dataplanev1.UpdatePipelineRequest) (*dataplanev1.UpdatePipelineResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.GetId()
	cur, ok := s.pipelines[id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", id)
	}
	patch := req.GetPipeline()
	if patch != nil {
		if patch.GetDisplayName() != "" {
			cur.DisplayName = patch.GetDisplayName()
		}
		if patch.GetConfigYaml() != "" {
			cur.ConfigYaml = patch.GetConfigYaml()
		}
		if patch.GetDescription() != "" {
			cur.Description = patch.GetDescription()
		}
		if r := patch.GetResources(); r != nil {
			cur.Resources = r
		}
		if len(patch.GetTags()) > 0 {
			cur.Tags = patch.GetTags()
		}
	}
	return &dataplanev1.UpdatePipelineResponse{Pipeline: cur}, nil
}

func (s *pipelineServer) DeletePipeline(_ context.Context, req *dataplanev1.DeletePipelineRequest) (*dataplanev1.DeletePipelineResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pipelines, req.GetId())
	return &dataplanev1.DeletePipelineResponse{}, nil
}

func (s *pipelineServer) StartPipeline(_ context.Context, req *dataplanev1.StartPipelineRequest) (*dataplanev1.StartPipelineResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.pipelines[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	cur.State = dataplanev1.Pipeline_STATE_RUNNING
	return &dataplanev1.StartPipelineResponse{Pipeline: cur}, nil
}

func (s *pipelineServer) StopPipeline(_ context.Context, req *dataplanev1.StopPipelineRequest) (*dataplanev1.StopPipelineResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.pipelines[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	cur.State = dataplanev1.Pipeline_STATE_STOPPED
	return &dataplanev1.StopPipelineResponse{Pipeline: cur}, nil
}
