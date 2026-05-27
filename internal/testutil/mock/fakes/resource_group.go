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

// Package fakes implements in-memory *ServiceServer fakes for the
// integration tier. Each fake is independently constructable and embeds
// the proto-generated Unimplemented*Server for forward compatibility.
package fakes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// rgFakeNamespace seeds the per-fake UUID-v5 derivation so generated IDs are
// reproducible across test runs but distinct from real resource-group UUIDs.
var rgFakeNamespace = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// ResourceGroupFake is a stateful in-memory implementation of the
// ResourceGroupService RPC surface. Safe for concurrent use.
type ResourceGroupFake struct {
	controlplanev1grpc.UnimplementedResourceGroupServiceServer

	mu     sync.Mutex
	groups map[string]*controlplanev1.ResourceGroup
	seq    atomic.Uint64
}

// NewResourceGroupFake returns an empty ResourceGroupFake.
func NewResourceGroupFake() *ResourceGroupFake {
	return &ResourceGroupFake{groups: map[string]*controlplanev1.ResourceGroup{}}
}

// CreateResourceGroup stores a new group with a deterministic UUID (derived
// from a per-fake namespace + monotonic seq, so IDs are reproducible across
// test runs) and populates created_at + updated_at. Returns AlreadyExists if
// a group with the same name is present.
func (f *ResourceGroupFake) CreateResourceGroup(_ context.Context, req *controlplanev1.CreateResourceGroupRequest) (*controlplanev1.CreateResourceGroupResponse, error) {
	if req.GetResourceGroup() == nil {
		return nil, status.Error(codes.InvalidArgument, "resource_group is required")
	}
	name := req.GetResourceGroup().GetName()

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, rg := range f.groups {
		if rg.GetName() == name {
			return nil, status.Errorf(codes.AlreadyExists, "resource_group %q already exists", name)
		}
	}
	id := uuid.NewSHA1(rgFakeNamespace, fmt.Appendf(nil, "rg-%d", f.seq.Add(1))).String()
	now := timestamppb.Now()
	rg := &controlplanev1.ResourceGroup{
		Id:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	f.groups[id] = rg
	return &controlplanev1.CreateResourceGroupResponse{ResourceGroup: rg}, nil
}

// GetResourceGroup looks up by ID; returns NotFound if absent.
func (f *ResourceGroupFake) GetResourceGroup(_ context.Context, req *controlplanev1.GetResourceGroupRequest) (*controlplanev1.GetResourceGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rg, ok := f.groups[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "resource_group %q not found", req.GetId())
	}
	return &controlplanev1.GetResourceGroupResponse{ResourceGroup: rg}, nil
}

// ListResourceGroups returns every stored group. Honors Filter.NameContains
// when set (substring match, mirroring real-backend semantics).
func (f *ResourceGroupFake) ListResourceGroups(_ context.Context, req *controlplanev1.ListResourceGroupsRequest) (*controlplanev1.ListResourceGroupsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := ""
	if req.GetFilter() != nil {
		want = req.GetFilter().GetNameContains()
	}
	out := make([]*controlplanev1.ResourceGroup, 0, len(f.groups))
	for _, rg := range f.groups {
		if want == "" || strings.Contains(rg.GetName(), want) {
			out = append(out, rg)
		}
	}
	return &controlplanev1.ListResourceGroupsResponse{ResourceGroups: out}, nil
}

// UpdateResourceGroup rewrites the name field on an existing group and bumps
// updated_at. Returns NotFound if the ID doesn't exist. UpdateResourceGroup
// has no FieldMask on the proto; full-field replacement is the contract.
func (f *ResourceGroupFake) UpdateResourceGroup(_ context.Context, req *controlplanev1.UpdateResourceGroupRequest) (*controlplanev1.UpdateResourceGroupResponse, error) {
	if req.GetResourceGroup() == nil {
		return nil, status.Error(codes.InvalidArgument, "resource_group is required")
	}
	upd := req.GetResourceGroup()

	f.mu.Lock()
	defer f.mu.Unlock()
	rg, ok := f.groups[upd.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "resource_group %q not found", upd.GetId())
	}
	rg.Name = upd.GetName()
	rg.UpdatedAt = timestamppb.Now()
	return &controlplanev1.UpdateResourceGroupResponse{ResourceGroup: rg}, nil
}

// DeleteResourceGroup removes the group; returns NotFound if absent.
func (f *ResourceGroupFake) DeleteResourceGroup(_ context.Context, req *controlplanev1.DeleteResourceGroupRequest) (*controlplanev1.DeleteResourceGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.groups[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "resource_group %q not found", req.GetId())
	}
	delete(f.groups, req.GetId())
	return &controlplanev1.DeleteResourceGroupResponse{}, nil
}
