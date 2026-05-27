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
	"fmt"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// opSeq mints monotonic operation IDs so concurrent completedOp calls don't
// collide (Operation.Id is the OperationFake map key).
var opSeq atomic.Uint64

// xidAlphabet matches the buf.validate id pattern `[a-v0-9]` used by every
// controlplane resource (network/cluster/serverless_*). xidLike turns a
// monotonic seq into a 20-char deterministic id from that alphabet so the
// fake's emitted ids pass the protovalidate interceptor.
const xidAlphabet = "0123456789abcdefghijklmnopqrstuv"

// xidLike returns a 20-char id from xidAlphabet, deterministically derived
// from seq (high bits first, padded with the alphabet's zero).
func xidLike(seq uint64) string {
	const n = 20
	out := make([]byte, n)
	for i := range out {
		out[i] = xidAlphabet[0]
	}
	pos := n - 1
	v := seq
	for v > 0 && pos >= 0 {
		out[pos] = xidAlphabet[v&0x1f]
		v >>= 5
		pos--
	}
	return string(out)
}

// completedOp builds a COMPLETED Operation wrapping resourceID and registers
// it on the OperationFake. Async Create/Update/Delete fakes call this so the
// provider's AreWeDoneYet polling loop terminates on the first GetOperation.
// Shared with the per-fake files (network, shadow_link, serverless_*) — the
// alternative was a copy per fake, which obscures the contract that every
// async fake must publish its operation here.
func completedOp(op *OperationFake, resourceID string) *controlplanev1.Operation {
	o := &controlplanev1.Operation{
		Id:         fmt.Sprintf("op-%d", opSeq.Add(1)),
		State:      controlplanev1.Operation_STATE_COMPLETED,
		ResourceId: proto.String(resourceID),
	}
	op.Set(o)
	return o
}

// OperationFake is an in-memory implementation of OperationServiceServer.
// Async fakes (shadow_link, serverless_private_link, network, cluster)
// register their long-running operations here so the provider's
// AreWeDoneYet polling loop resolves them via GetOperation.
type OperationFake struct {
	controlplanev1grpc.UnimplementedOperationServiceServer

	mu  sync.Mutex
	ops map[string]*controlplanev1.Operation
}

// NewOperationFake returns an empty OperationFake.
func NewOperationFake() *OperationFake {
	return &OperationFake{ops: map[string]*controlplanev1.Operation{}}
}

// Set stores or replaces an operation, keyed by op.Id.
func (f *OperationFake) Set(op *controlplanev1.Operation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops[op.GetId()] = op
}

// GetOperation returns the stored operation or NotFound.
func (f *OperationFake) GetOperation(_ context.Context, req *controlplanev1.GetOperationRequest) (*controlplanev1.GetOperationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	op, ok := f.ops[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "operation %q not found", req.GetId())
	}
	return &controlplanev1.GetOperationResponse{Operation: op}, nil
}
