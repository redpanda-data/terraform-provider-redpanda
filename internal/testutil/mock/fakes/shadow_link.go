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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const shadowLinkIDBase uint64 = 0x7000_0000_0000_0000

// ShadowLinkFake is a stateful in-memory implementation of ShadowLinkService.
// All three mutating RPCs (Create/Update/Delete) are async — each publishes a
// completed Operation via op.Set so AreWeDoneYet resolves on the first poll.
//
// UpdateShadowLink honors UpdateMask: only the listed top-level fields on the
// shadow_link payload are written through; unmasked fields retain their prior
// stored value. The mask paths come from utils.GenerateProtobufDiffAndUpdateMask
// which produces top-level field names (client_options, topic_metadata_sync_options,
// consumer_offset_sync_options, security_sync_options, schema_registry_sync_options).
type ShadowLinkFake struct {
	controlplanev1grpc.UnimplementedShadowLinkServiceServer

	op    *OperationFake
	mu    sync.Mutex
	links map[string]*controlplanev1.ShadowLink
	seq   atomic.Uint64
}

// NewShadowLinkFake returns an empty fake bound to op.
func NewShadowLinkFake(op *OperationFake) *ShadowLinkFake {
	return &ShadowLinkFake{op: op, links: map[string]*controlplanev1.ShadowLink{}}
}

// CreateShadowLink stores a new shadow link in STATE_ACTIVE and returns a
// completed Operation.
func (f *ShadowLinkFake) CreateShadowLink(_ context.Context, req *controlplanev1.CreateShadowLinkRequest) (*controlplanev1.CreateShadowLinkOperation, error) {
	in := req.GetShadowLink()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "shadow_link is required")
	}
	id := xidLike(shadowLinkIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	sl := &controlplanev1.ShadowLink{
		Id:                        id,
		Name:                      in.GetName(),
		ShadowRedpandaId:          in.GetShadowRedpandaId(),
		ClientOptions:             in.GetClientOptions(),
		TopicMetadataSyncOptions:  in.GetTopicMetadataSyncOptions(),
		ConsumerOffsetSyncOptions: in.GetConsumerOffsetSyncOptions(),
		SecuritySyncOptions:       in.GetSecuritySyncOptions(),
		SchemaRegistrySyncOptions: in.GetSchemaRegistrySyncOptions(),
		State:                     controlplanev1.ShadowLink_STATE_ACTIVE,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	f.mu.Lock()
	f.links[id] = sl
	f.mu.Unlock()

	return &controlplanev1.CreateShadowLinkOperation{Operation: completedOp(f.op, id)}, nil
}

// maskSensitiveClientOptions returns a clone of sl with sensitive fields
// zeroed, mirroring the real backend's behavior on Read.
func maskSensitiveClientOptions(sl *controlplanev1.ShadowLink) *controlplanev1.ShadowLink {
	out, ok := proto.Clone(sl).(*controlplanev1.ShadowLink)
	if !ok {
		return sl
	}
	co := out.GetClientOptions()
	if co == nil {
		return out
	}
	if tls := co.GetTlsSettings(); tls != nil {
		tls.Key = ""
	}
	if auth := co.GetAuthenticationConfiguration(); auth != nil {
		if scram := auth.GetScramConfiguration(); scram != nil {
			scram.Password = ""
		}
		if plain := auth.GetPlainConfiguration(); plain != nil {
			plain.Password = ""
		}
	}
	return out
}

// GetShadowLink returns the stored shadow link with sensitive fields masked,
// mirroring the real backend's behavior (tls.key, scram.password,
// plain.password are never returned on Read).
func (f *ShadowLinkFake) GetShadowLink(_ context.Context, req *controlplanev1.GetShadowLinkRequest) (*controlplanev1.GetShadowLinkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sl, ok := f.links[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "shadow_link %q not found", req.GetId())
	}
	return &controlplanev1.GetShadowLinkResponse{ShadowLink: maskSensitiveClientOptions(sl)}, nil
}

// UpdateShadowLink applies req.UpdateMask.Paths against the stored record:
// for each top-level path, the value from req.ShadowLink is written through.
// Unmasked fields are left untouched.
func (f *ShadowLinkFake) UpdateShadowLink(_ context.Context, req *controlplanev1.UpdateShadowLinkRequest) (*controlplanev1.UpdateShadowLinkOperation, error) {
	upd := req.GetShadowLink()
	if upd == nil {
		return nil, status.Error(codes.InvalidArgument, "shadow_link is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	sl, ok := f.links[upd.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "shadow_link %q not found", upd.GetId())
	}

	dstR := sl.ProtoReflect()
	srcR := upd.ProtoReflect()
	for _, path := range req.GetUpdateMask().GetPaths() {
		dstFD := dstR.Descriptor().Fields().ByName(protoreflect.Name(path))
		srcFD := srcR.Descriptor().Fields().ByName(protoreflect.Name(path))
		if dstFD == nil || srcFD == nil {
			continue
		}
		dstR.Set(dstFD, srcR.Get(srcFD))
	}
	sl.UpdatedAt = timestamppb.Now()

	return &controlplanev1.UpdateShadowLinkOperation{Operation: completedOp(f.op, upd.GetId())}, nil
}

// DeleteShadowLink removes the stored shadow link and publishes a completed
// Operation. Returns NotFound if absent.
func (f *ShadowLinkFake) DeleteShadowLink(_ context.Context, req *controlplanev1.DeleteShadowLinkRequest) (*controlplanev1.DeleteShadowLinkOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.links[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "shadow_link %q not found", req.GetId())
	}
	delete(f.links, req.GetId())
	return &controlplanev1.DeleteShadowLinkOperation{Operation: completedOp(f.op, req.GetId())}, nil
}
