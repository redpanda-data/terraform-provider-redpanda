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

// networkIDBase offsets the fake's id sequence so generated ids don't collide
// with other resource fakes that share the [a-v0-9]{20} alphabet.
const networkIDBase uint64 = 0x4000_0000_0000_0000

// NetworkFake is a stateful in-memory implementation of NetworkService. The
// schema has no Update RPC (all configurable fields are RequiresReplace), so
// Create + Get + Delete is the entire mutable surface. Create and Delete are
// async — both publish a completed Operation via op.Set so the provider's
// AreWeDoneYet polling loop resolves on the first GetOperation call.
type NetworkFake struct {
	controlplanev1grpc.UnimplementedNetworkServiceServer

	op       *OperationFake
	mu       sync.Mutex
	networks map[string]*controlplanev1.Network
	seq      atomic.Uint64
}

// NewNetworkFake returns an empty NetworkFake bound to op.
func NewNetworkFake(op *OperationFake) *NetworkFake {
	return &NetworkFake{op: op, networks: map[string]*controlplanev1.Network{}}
}

// CreateNetwork stores a new network in STATE_READY (test mode short-circuits
// the real CREATING → READY transition) and returns a completed Operation.
func (f *NetworkFake) CreateNetwork(_ context.Context, req *controlplanev1.CreateNetworkRequest) (*controlplanev1.CreateNetworkOperation, error) {
	in := req.GetNetwork()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "network is required")
	}
	id := xidLike(networkIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	nw := &controlplanev1.Network{
		Id:                       id,
		Name:                     in.GetName(),
		ResourceGroupId:          in.GetResourceGroupId(),
		CloudProvider:            in.GetCloudProvider(),
		Region:                   in.GetRegion(),
		CidrBlock:                in.GetCidrBlock(),
		ClusterType:              in.GetClusterType(),
		CustomerManagedResources: in.GetCustomerManagedResources(),
		State:                    controlplanev1.Network_STATE_READY,
		CreatedAt:                now,
		UpdatedAt:                now,
		Zones:                    []string{"use1-az1"},
	}
	if in.GetCustomerManagedResources() != nil {
		nw.CidrBlock = "0.0.0.0/0"
	}

	f.mu.Lock()
	f.networks[id] = nw
	f.mu.Unlock()

	return &controlplanev1.CreateNetworkOperation{Operation: completedOp(f.op, id)}, nil
}

// GetNetwork returns the stored network or NotFound.
func (f *NetworkFake) GetNetwork(_ context.Context, req *controlplanev1.GetNetworkRequest) (*controlplanev1.GetNetworkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	nw, ok := f.networks[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "network %q not found", req.GetId())
	}
	return &controlplanev1.GetNetworkResponse{Network: nw}, nil
}

// DeleteNetwork removes the stored network and returns a completed Operation
// so AreWeDoneYet resolves immediately. Returns NotFound if absent so the
// provider's IsNotFound short-circuit fires.
func (f *NetworkFake) DeleteNetwork(_ context.Context, req *controlplanev1.DeleteNetworkRequest) (*controlplanev1.DeleteNetworkOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.networks[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "network %q not found", req.GetId())
	}
	delete(f.networks, req.GetId())
	return &controlplanev1.DeleteNetworkOperation{Operation: completedOp(f.op, req.GetId())}, nil
}
