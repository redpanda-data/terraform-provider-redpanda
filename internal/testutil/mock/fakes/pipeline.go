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

package fakes

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type pipelineRecord struct {
	id          string
	displayName string
	description string
	configYaml  string
	state       dataplanev1.Pipeline_State
	tags        map[string]string
	clientID    string

	// staleVersion is set when a start or stop changed the pipeline's state.
	// The controller's status write for that change bumps the resourceVersion,
	// so the next UpdatePipeline writes at a stale version.
	staleVersion bool
}

// PipelineFake is a stateful in-memory implementation of the 6 PipelineService
// RPCs the provider uses (Create/Get/Update/Delete/Start/Stop). The remaining
// 6 RPCs inherit Unimplemented. State transitions are instantaneous: Start sets
// STATE_RUNNING, Stop sets STATE_STOPPED, so the provider's waitForPipelineState
// poller converges on the first poll.
type PipelineFake struct {
	dataplanev1grpc.UnimplementedPipelineServiceServer

	mu    sync.Mutex
	store map[string]*pipelineRecord
	seq   atomic.Uint64

	conflictAfterStateChange bool
}

// NewPipelineFake returns an empty PipelineFake. It models a dataplane whose
// UpdatePipeline does not retry the Kubernetes conflict: the controller's
// status write for a start or stop bumps the resourceVersion, so the first
// update after one fails with Aborted. The fake holds that stale write until
// the next UpdatePipeline, however late it comes, which is stricter than the
// dataplane's window between its own read and write; a caller that retries
// Aborted absorbs the difference. SetConflictAfterStateChange(false) models a
// dataplane that retries the conflict itself and never returns it.
func NewPipelineFake() *PipelineFake {
	return &PipelineFake{store: map[string]*pipelineRecord{}, conflictAfterStateChange: true}
}

// SetConflictAfterStateChange turns the stale write after a start or stop on
// or off.
func (f *PipelineFake) SetConflictAfterStateChange(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conflictAfterStateChange = on
}

func (f *PipelineFake) nextID() string {
	return fmt.Sprintf("tfrp-mock-pipeline-%d", f.seq.Add(1))
}

func pipelineRecordToProto(rec *pipelineRecord) *dataplanev1.Pipeline {
	p := &dataplanev1.Pipeline{
		Id:          rec.id,
		DisplayName: rec.displayName,
		Description: rec.description,
		ConfigYaml:  rec.configYaml,
		State:       rec.state,
		Tags:        copyStringMap(rec.tags),
	}
	if rec.clientID != "" {
		p.ServiceAccount = &dataplanev1.Pipeline_ServiceAccount{
			ClientId: rec.clientID,
		}
	}
	return p
}

// CreatePipeline stores a new pipeline in STATE_STOPPED.
func (f *PipelineFake) CreatePipeline(_ context.Context, req *dataplanev1.CreatePipelineRequest) (*dataplanev1.CreatePipelineResponse, error) {
	pc := req.GetPipeline()
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID()
	rec := &pipelineRecord{
		id:          id,
		displayName: pc.GetDisplayName(),
		description: pc.GetDescription(),
		configYaml:  pc.GetConfigYaml(),
		state:       dataplanev1.Pipeline_STATE_STOPPED,
		tags:        copyStringMap(pc.GetTags()),
	}
	if sa := pc.GetServiceAccount(); sa != nil {
		rec.clientID = sa.GetClientId()
	}
	f.store[id] = rec
	return &dataplanev1.CreatePipelineResponse{Pipeline: pipelineRecordToProto(rec)}, nil
}

// GetPipeline returns the stored pipeline.
func (f *PipelineFake) GetPipeline(_ context.Context, req *dataplanev1.GetPipelineRequest) (*dataplanev1.GetPipelineResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	return &dataplanev1.GetPipelineResponse{Pipeline: pipelineRecordToProto(rec)}, nil
}

// UpdatePipeline writes the payload the way the dataplane does: with no
// update_mask every mutable field is replaced, so an absent service account
// clears the stored one; with a mask only the named fields change.
func (f *PipelineFake) UpdatePipeline(_ context.Context, req *dataplanev1.UpdatePipelineRequest) (*dataplanev1.UpdatePipelineResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	if rec.staleVersion {
		rec.staleVersion = false
		return nil, status.Error(codes.Aborted, "the pipeline was modified by another request: reload it and retry")
	}
	pu := req.GetPipeline()
	paths := req.GetUpdateMask().GetPaths()
	named := func(field string) bool {
		if len(paths) == 0 {
			return true
		}
		for _, p := range paths {
			if p == field {
				return true
			}
		}
		return false
	}
	if named("display_name") {
		rec.displayName = pu.GetDisplayName()
	}
	if named("description") {
		rec.description = pu.GetDescription()
	}
	if named("config_yaml") {
		rec.configYaml = pu.GetConfigYaml()
	}
	if named("tags") {
		rec.tags = copyStringMap(pu.GetTags())
	}
	if named("service_account") {
		rec.clientID = pu.GetServiceAccount().GetClientId()
	}
	return &dataplanev1.UpdatePipelineResponse{Pipeline: pipelineRecordToProto(rec)}, nil
}

// DeletePipeline removes the pipeline; NotFound if absent.
func (f *PipelineFake) DeletePipeline(_ context.Context, req *dataplanev1.DeletePipelineRequest) (*dataplanev1.DeletePipelineResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	delete(f.store, req.GetId())
	return &dataplanev1.DeletePipelineResponse{}, nil
}

// StartPipeline transitions the pipeline to STATE_RUNNING.
func (f *PipelineFake) StartPipeline(_ context.Context, req *dataplanev1.StartPipelineRequest) (*dataplanev1.StartPipelineResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	if f.conflictAfterStateChange && rec.state != dataplanev1.Pipeline_STATE_RUNNING {
		rec.staleVersion = true
	}
	rec.state = dataplanev1.Pipeline_STATE_RUNNING
	return &dataplanev1.StartPipelineResponse{}, nil
}

// StopPipeline transitions the pipeline to STATE_STOPPED.
func (f *PipelineFake) StopPipeline(_ context.Context, req *dataplanev1.StopPipelineRequest) (*dataplanev1.StopPipelineResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "pipeline %q not found", req.GetId())
	}
	if f.conflictAfterStateChange && rec.state == dataplanev1.Pipeline_STATE_RUNNING {
		rec.staleVersion = true
	}
	rec.state = dataplanev1.Pipeline_STATE_STOPPED
	return &dataplanev1.StopPipelineResponse{}, nil
}
