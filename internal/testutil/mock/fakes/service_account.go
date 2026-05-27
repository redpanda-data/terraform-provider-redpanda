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
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/iam/v1/iamv1grpc"
	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// saFakeNamespace seeds the per-fake derivation so generated IDs are
// reproducible across test runs but distinct from real service-account
// identifiers.
const saFakeNamespace = "tfrp-mock-sa-fake-namespace-6"

// saIDFor builds a deterministic 20-character lowercase alphanumeric id
// mimicking the xid shape the IAM backend uses for ServiceAccount IDs (the
// proto's protovalidate rule requires id length == 20).
func saIDFor(seq uint64) string {
	h := sha256.Sum256(fmt.Appendf(nil, "%s-%d", saFakeNamespace, seq))
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(h[:]))
	return s[:20]
}

// ServiceAccountFake is a stateful in-memory implementation of the IAM
// ServiceAccountService RPCs. Models the write-only client_secret contract:
// Create returns it once; subsequent Gets / Updates omit it.
type ServiceAccountFake struct {
	iamv1grpc.UnimplementedServiceAccountServiceServer

	mu    sync.Mutex
	store map[string]*iamv1.ServiceAccount
	seq   atomic.Uint64
}

// NewServiceAccountFake returns an empty ServiceAccountFake.
func NewServiceAccountFake() *ServiceAccountFake {
	return &ServiceAccountFake{store: map[string]*iamv1.ServiceAccount{}}
}

// CreateServiceAccount stores a new service account with a deterministic
// UUID and synthetic Auth0 client credentials. The returned response
// includes ClientSecret — the one-time exposure.
func (f *ServiceAccountFake) CreateServiceAccount(_ context.Context, req *iamv1.CreateServiceAccountRequest) (*iamv1.CreateServiceAccountResponse, error) {
	in := req.GetServiceAccount()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "service_account is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sa := range f.store {
		if sa.GetName() == in.GetName() {
			return nil, status.Errorf(codes.AlreadyExists, "service_account %q already exists", in.GetName())
		}
	}
	seq := f.seq.Add(1)
	id := saIDFor(seq)
	now := timestamppb.Now()
	clientSecret := fmt.Sprintf("fake-client-secret-%d", seq)
	sa := &iamv1.ServiceAccount{
		Id:          id,
		Name:        in.GetName(),
		Description: in.GetDescription(),
		CreatedAt:   now,
		UpdatedAt:   now,
		Auth0ClientCredentials: &iamv1.ServiceAccountCredentials{
			ClientId: fmt.Sprintf("fake-client-id-%d", seq),
		},
	}
	f.store[id] = sa
	resp := cloneSA(sa)
	resp.Auth0ClientCredentials.ClientSecret = &clientSecret
	return &iamv1.CreateServiceAccountResponse{ServiceAccount: resp}, nil
}

// GetServiceAccount returns the stored SA without the client_secret.
func (f *ServiceAccountFake) GetServiceAccount(_ context.Context, req *iamv1.GetServiceAccountRequest) (*iamv1.GetServiceAccountResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sa, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "service_account %q not found", req.GetId())
	}
	return &iamv1.GetServiceAccountResponse{ServiceAccount: cloneSA(sa)}, nil
}

// ListServiceAccounts iterates the store; Filter.Name does exact match.
func (f *ServiceAccountFake) ListServiceAccounts(_ context.Context, req *iamv1.ListServiceAccountsRequest) (*iamv1.ListServiceAccountsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	wantName := ""
	if flt := req.GetFilter(); flt != nil {
		wantName = flt.GetName()
	}
	out := make([]*iamv1.ServiceAccount, 0, len(f.store))
	for _, sa := range f.store {
		if wantName != "" && sa.GetName() != wantName {
			continue
		}
		out = append(out, cloneSA(sa))
	}
	return &iamv1.ListServiceAccountsResponse{ServiceAccounts: out}, nil
}

// UpdateServiceAccount honors the FieldMask: only paths listed in the mask
// overwrite the stored fields. Returns the updated SA without client_secret.
func (f *ServiceAccountFake) UpdateServiceAccount(_ context.Context, req *iamv1.UpdateServiceAccountRequest) (*iamv1.UpdateServiceAccountResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sa, ok := f.store[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "service_account %q not found", req.GetId())
	}
	upd := req.GetServiceAccount()
	if upd == nil {
		return nil, status.Error(codes.InvalidArgument, "service_account is required")
	}
	maskPaths := req.GetUpdateMask().GetPaths()
	for _, p := range maskPaths {
		switch p {
		case "name":
			sa.Name = upd.GetName()
		case "description":
			sa.Description = upd.GetDescription()
		default:
		}
	}
	sa.UpdatedAt = timestamppb.Now()
	return &iamv1.UpdateServiceAccountResponse{ServiceAccount: cloneSA(sa)}, nil
}

// DeleteServiceAccount removes the SA; NotFound if absent.
func (f *ServiceAccountFake) DeleteServiceAccount(_ context.Context, req *iamv1.DeleteServiceAccountRequest) (*iamv1.DeleteServiceAccountResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "service_account %q not found", req.GetId())
	}
	delete(f.store, req.GetId())
	return &iamv1.DeleteServiceAccountResponse{}, nil
}

// cloneSA returns a deep copy of the stored SA with credentials reset to
// the stored shape (ClientId set, ClientSecret nil). Callers that need to
// expose ClientSecret (only Create) set it on the returned value.
func cloneSA(sa *iamv1.ServiceAccount) *iamv1.ServiceAccount {
	out := &iamv1.ServiceAccount{
		Id:          sa.GetId(),
		Name:        sa.GetName(),
		Description: sa.GetDescription(),
		CreatedAt:   sa.GetCreatedAt(),
		UpdatedAt:   sa.GetUpdatedAt(),
	}
	if sa.GetAuth0ClientCredentials() != nil {
		out.Auth0ClientCredentials = &iamv1.ServiceAccountCredentials{
			ClientId: sa.GetAuth0ClientCredentials().GetClientId(),
		}
	}
	return out
}
