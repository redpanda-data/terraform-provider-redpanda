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

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// secretRecord mirrors what the dataplane returns on Get / Update: id +
// labels + scopes. secret_data is write-only and never persisted.
type secretRecord struct {
	id     string
	labels map[string]string
	scopes []dataplanev1.Scope
}

// SecretFake is a stateful in-memory implementation of the 4 SecretService
// RPCs the provider uses (Create/Get/Update/Delete). The other 8 inherit
// Unimplemented.
type SecretFake struct {
	dataplanev1grpc.UnimplementedSecretServiceServer

	mu    sync.Mutex
	store map[string]*secretRecord
}

// NewSecretFake returns an empty SecretFake.
func NewSecretFake() *SecretFake {
	return &SecretFake{store: map[string]*secretRecord{}}
}

// CreateSecret stores a new secret keyed by ID. AlreadyExists if present.
// SecretData is write-only and not retained.
func (f *SecretFake) CreateSecret(_ context.Context, req *dataplanev1.CreateSecretRequest) (*dataplanev1.CreateSecretResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[req.GetId()]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "secret %q already exists", req.GetId())
	}
	rec := &secretRecord{
		id:     req.GetId(),
		labels: copyStringMap(req.GetLabels()),
		scopes: append([]dataplanev1.Scope(nil), req.GetScopes()...),
	}
	f.store[rec.id] = rec
	return &dataplanev1.CreateSecretResponse{Secret: &dataplanev1.Secret{
		Id:     rec.id,
		Labels: copyStringMap(rec.labels),
		Scopes: append([]dataplanev1.Scope(nil), rec.scopes...),
	}}, nil
}

// GetSecret returns the stored Secret without secret_data.
func (f *SecretFake) GetSecret(_ context.Context, req *dataplanev1.GetSecretRequest) (*dataplanev1.GetSecretResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", req.GetId())
	}
	return &dataplanev1.GetSecretResponse{Secret: &dataplanev1.Secret{
		Id:     rec.id,
		Labels: copyStringMap(rec.labels),
		Scopes: append([]dataplanev1.Scope(nil), rec.scopes...),
	}}, nil
}

// UpdateSecret rewrites scopes and labels (full replacement; no FieldMask
// on the proto). SecretData is write-only and not retained.
func (f *SecretFake) UpdateSecret(_ context.Context, req *dataplanev1.UpdateSecretRequest) (*dataplanev1.UpdateSecretResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", req.GetId())
	}
	rec.scopes = append([]dataplanev1.Scope(nil), req.GetScopes()...)
	if req.GetLabels() != nil {
		rec.labels = copyStringMap(req.GetLabels())
	}
	return &dataplanev1.UpdateSecretResponse{Secret: &dataplanev1.Secret{
		Id:     rec.id,
		Labels: copyStringMap(rec.labels),
		Scopes: append([]dataplanev1.Scope(nil), rec.scopes...),
	}}, nil
}

// DeleteSecret removes the secret; NotFound if absent.
func (f *SecretFake) DeleteSecret(_ context.Context, req *dataplanev1.DeleteSecretRequest) (*dataplanev1.DeleteSecretResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", req.GetId())
	}
	delete(f.store, req.GetId())
	return &dataplanev1.DeleteSecretResponse{}, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
