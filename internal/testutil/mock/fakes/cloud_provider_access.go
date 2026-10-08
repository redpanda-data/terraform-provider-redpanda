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
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// cloudProviderAccessIDBase offsets the fake's id sequence so generated ids
// don't collide with other resource fakes that share the [a-v0-9]{20}
// alphabet.
const cloudProviderAccessIDBase uint64 = 0x9000_0000_0000_0000

const (
	// FakeOrgID is the organization the fake serves. The control plane sets a
	// cloud provider access's external_id to the caller's organization ID.
	FakeOrgID = "a845616f-0484-4506-9638-45fe28f34865"

	// FakeCloudProviderAccessPrincipalARN is the Redpanda principal the fake
	// reports as the one assuming customer roles.
	FakeCloudProviderAccessPrincipalARN = "arn:aws:iam::123456789012:role/redpanda-ops-worker"
)

// CloudProviderAccessFake is a stateful in-memory CloudProviderAccessService
// mirroring cloudv2's public CloudProviderAccessService: create is synchronous
// and stores STATE_ACTIVE with the server-set external_id, ignoring one the
// request sets as the public create mapper does, name is unique per
// organization, and delete is refused while a network references the access. Rejections carry only a code, because the public API drops the
// control plane's message when no ExternalError detail is attached.
type CloudProviderAccessFake struct {
	controlplanev1grpc.UnimplementedCloudProviderAccessServiceServer

	mu       sync.Mutex
	accesses map[string]*controlplanev1.CloudProviderAccess
	seq      atomic.Uint64

	// Referenced reports whether a live network uses the access; the mock
	// server wires it to the network fake. Nil means never referenced.
	Referenced func(id string) bool
}

// NewCloudProviderAccessFake returns an empty CloudProviderAccessFake.
func NewCloudProviderAccessFake() *CloudProviderAccessFake {
	return &CloudProviderAccessFake{accesses: map[string]*controlplanev1.CloudProviderAccess{}}
}

// CreateCloudProviderAccess stores a new access in STATE_ACTIVE.
func (f *CloudProviderAccessFake) CreateCloudProviderAccess(_ context.Context, req *controlplanev1.CreateCloudProviderAccessRequest) (*controlplanev1.CreateCloudProviderAccessResponse, error) {
	in := req.GetCloudProviderAccess()
	if in.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS {
		return nil, status.Error(codes.InvalidArgument, "")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.accesses {
		if existing.GetName() == in.GetName() {
			return nil, status.Error(codes.AlreadyExists, "")
		}
	}
	id := xidLike(cloudProviderAccessIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	cpa := &controlplanev1.CloudProviderAccess{
		Id:            id,
		Name:          in.GetName(),
		CreatedAt:     now,
		UpdatedAt:     now,
		CloudProvider: in.GetCloudProvider(),
		State:         controlplanev1.CloudProviderAccess_STATE_ACTIVE,
	}
	cpa.SetAws(&controlplanev1.AWSCloudProviderAccess{
		RoleArn:    in.GetAws().GetRoleArn(),
		ExternalId: FakeOrgID,
	})
	f.accesses[id] = cpa
	return &controlplanev1.CreateCloudProviderAccessResponse{CloudProviderAccess: cpa}, nil
}

// Lookup returns the stored access with the given id, or nil.
func (f *CloudProviderAccessFake) Lookup(id string) *controlplanev1.CloudProviderAccess {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accesses[id]
}

// DeleteOutOfBand removes the access as if it was deleted outside Terraform.
// Returns false when no access has that id.
func (f *CloudProviderAccessFake) DeleteOutOfBand(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.accesses[id]; !ok {
		return false
	}
	delete(f.accesses, id)
	return true
}

// GetCloudProviderAccess returns the stored access or NotFound.
func (f *CloudProviderAccessFake) GetCloudProviderAccess(_ context.Context, req *controlplanev1.GetCloudProviderAccessRequest) (*controlplanev1.GetCloudProviderAccessResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cpa, ok := f.accesses[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "cloud provider access %q not found", req.GetId())
	}
	return &controlplanev1.GetCloudProviderAccessResponse{CloudProviderAccess: cpa}, nil
}

// ListCloudProviderAccess returns every stored access matching the filter.
func (f *CloudProviderAccessFake) ListCloudProviderAccess(_ context.Context, req *controlplanev1.ListCloudProviderAccessRequest) (*controlplanev1.ListCloudProviderAccessResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := req.GetFilter().GetCloudProvider()
	out := &controlplanev1.ListCloudProviderAccessResponse{}
	for _, cpa := range f.accesses {
		if want == controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED || cpa.GetCloudProvider() == want {
			out.CloudProviderAccesses = append(out.CloudProviderAccesses, cpa)
		}
	}
	return out, nil
}

// DeleteCloudProviderAccess removes the stored access, or refuses with
// FailedPrecondition while a network references it.
func (f *CloudProviderAccessFake) DeleteCloudProviderAccess(_ context.Context, req *controlplanev1.DeleteCloudProviderAccessRequest) (*controlplanev1.DeleteCloudProviderAccessResponse, error) {
	id := req.GetId()
	// Referenced takes the network fake's lock, and the network fake calls
	// Lookup while holding its own, so it runs before f.mu is taken.
	if f.Referenced != nil && f.Referenced(id) {
		return nil, status.Error(codes.FailedPrecondition, "")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.accesses[id]; !ok {
		return nil, status.Errorf(codes.NotFound, "cloud provider access %q not found", id)
	}
	delete(f.accesses, id)
	return &controlplanev1.DeleteCloudProviderAccessResponse{}, nil
}

// GetCloudProviderAccessPrerequisites returns the AWS trust-policy values, or
// InvalidArgument for any other provider.
func (*CloudProviderAccessFake) GetCloudProviderAccessPrerequisites(_ context.Context, req *controlplanev1.GetCloudProviderAccessPrerequisitesRequest) (*controlplanev1.GetCloudProviderAccessPrerequisitesResponse, error) {
	if req.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS {
		return nil, status.Error(codes.InvalidArgument, "only AWS is supported")
	}
	resp := &controlplanev1.GetCloudProviderAccessPrerequisitesResponse{}
	resp.SetAws(&controlplanev1.AWSCloudProviderAccessPrerequisites{
		ExternalId:   FakeOrgID,
		PrincipalArn: FakeCloudProviderAccessPrincipalARN,
	})
	return resp, nil
}
