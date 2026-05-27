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

// splIDBase offsets the SPL fake's id sequence away from network/cluster.
const splIDBase uint64 = 0x5000_0000_0000_0000

// ServerlessPrivateLinkFake is a stateful in-memory implementation of
// ServerlessPrivateLinkService. All three mutating RPCs (Create/Update/Delete)
// are async — each publishes a completed Operation via op.Set.
//
// UpdateServerlessPrivateLinkRequest has no FieldMask: the cloud_provider_config
// oneof is full-replacement, so Update overwrites aws_config.allowed_principals
// wholesale.
type ServerlessPrivateLinkFake struct {
	controlplanev1grpc.UnimplementedServerlessPrivateLinkServiceServer

	op    *OperationFake
	mu    sync.Mutex
	links map[string]*controlplanev1.ServerlessPrivateLink
	seq   atomic.Uint64
}

// NewServerlessPrivateLinkFake returns an empty fake bound to op.
func NewServerlessPrivateLinkFake(op *OperationFake) *ServerlessPrivateLinkFake {
	return &ServerlessPrivateLinkFake{op: op, links: map[string]*controlplanev1.ServerlessPrivateLink{}}
}

// CreateServerlessPrivateLink stores a new link in STATE_READY and returns a
// completed Operation. Status.Aws is pre-populated so the computed-only status
// surface lands in state on first apply (no second-refresh churn).
func (f *ServerlessPrivateLinkFake) CreateServerlessPrivateLink(_ context.Context, req *controlplanev1.CreateServerlessPrivateLinkRequest) (*controlplanev1.CreateServerlessPrivateLinkOperation, error) {
	in := req.GetServerlessPrivateLink()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "serverless_private_link is required")
	}
	id := xidLike(splIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	pl := &controlplanev1.ServerlessPrivateLink{
		Id:               id,
		Name:             in.GetName(),
		ResourceGroupId:  in.GetResourceGroupId(),
		Cloudprovider:    in.GetCloudprovider(),
		ServerlessRegion: in.GetServerlessRegion(),
		State:            controlplanev1.ServerlessPrivateLink_STATE_READY,
		CreatedAt:        now,
		UpdatedAt:        now,
		Status: &controlplanev1.ServerlessPrivateLinkStatus{
			CloudProvider: &controlplanev1.ServerlessPrivateLinkStatus_Aws{
				Aws: &controlplanev1.ServerlessPrivateLinkStatus_AWS{
					VpcEndpointServiceName: "com.amazonaws.vpce.us-east-1.vpce-svc-mockfake",
					AvailabilityZones:      []string{"use1-az1"},
				},
			},
		},
	}
	if aws := in.GetAwsConfig(); aws != nil {
		pl.CloudProviderConfig = &controlplanev1.ServerlessPrivateLink_AwsConfig{
			AwsConfig: &controlplanev1.ServerlessPrivateLink_AWS{
				AllowedPrincipals: append([]string(nil), aws.GetAllowedPrincipals()...),
				AllowedRegions:    append([]string(nil), aws.GetAllowedRegions()...),
			},
		}
	}

	f.mu.Lock()
	f.links[id] = pl
	f.mu.Unlock()

	return &controlplanev1.CreateServerlessPrivateLinkOperation{Operation: completedOp(f.op, id)}, nil
}

// GetServerlessPrivateLink returns the stored link or NotFound.
func (f *ServerlessPrivateLinkFake) GetServerlessPrivateLink(_ context.Context, req *controlplanev1.GetServerlessPrivateLinkRequest) (*controlplanev1.GetServerlessPrivateLinkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pl, ok := f.links[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless_private_link %q not found", req.GetId())
	}
	return &controlplanev1.GetServerlessPrivateLinkResponse{ServerlessPrivateLink: pl}, nil
}

// UpdateServerlessPrivateLink overwrites aws_config.allowed_principals (full
// replacement; the proto has no FieldMask). Bumps updated_at.
func (f *ServerlessPrivateLinkFake) UpdateServerlessPrivateLink(_ context.Context, req *controlplanev1.UpdateServerlessPrivateLinkRequest) (*controlplanev1.UpdateServerlessPrivateLinkOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pl, ok := f.links[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "serverless_private_link %q not found", req.GetId())
	}
	if aws := req.GetAwsConfig(); aws != nil {
		pl.CloudProviderConfig = &controlplanev1.ServerlessPrivateLink_AwsConfig{
			AwsConfig: &controlplanev1.ServerlessPrivateLink_AWS{
				AllowedPrincipals: append([]string(nil), aws.GetAllowedPrincipals()...),
				AllowedRegions:    append([]string(nil), aws.GetAllowedRegions()...),
			},
		}
	}
	pl.UpdatedAt = timestamppb.Now()
	return &controlplanev1.UpdateServerlessPrivateLinkOperation{Operation: completedOp(f.op, req.GetId())}, nil
}

// DeleteServerlessPrivateLink removes the stored link and publishes a
// completed Operation. Returns NotFound if absent.
func (f *ServerlessPrivateLinkFake) DeleteServerlessPrivateLink(_ context.Context, req *controlplanev1.DeleteServerlessPrivateLinkRequest) (*controlplanev1.DeleteServerlessPrivateLinkOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.links[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "serverless_private_link %q not found", req.GetId())
	}
	delete(f.links, req.GetId())
	return &controlplanev1.DeleteServerlessPrivateLinkOperation{Operation: completedOp(f.op, req.GetId())}, nil
}
