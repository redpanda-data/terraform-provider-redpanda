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
	"strings"
	"sync"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// userRecord mirrors what the dataplane backend returns on List: name +
// optional mechanism. Password is write-only and never persisted.
type userRecord struct {
	name      string
	mechanism dataplanev1.SASLMechanism
}

// UserFake is a stateful in-memory implementation of the UserService RPC
// surface. Safe for concurrent use.
type UserFake struct {
	dataplanev1grpc.UnimplementedUserServiceServer

	mu    sync.Mutex
	store map[string]*userRecord
}

// NewUserFake returns an empty UserFake.
func NewUserFake() *UserFake {
	return &UserFake{store: map[string]*userRecord{}}
}

// CreateUser stores a new user keyed by name. Returns AlreadyExists if a
// record with the same name is present.
func (f *UserFake) CreateUser(_ context.Context, req *dataplanev1.CreateUserRequest) (*dataplanev1.CreateUserResponse, error) {
	u := req.GetUser()
	if u == nil {
		return nil, status.Error(codes.InvalidArgument, "user is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[u.GetName()]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "user %q already exists", u.GetName())
	}
	f.store[u.GetName()] = &userRecord{name: u.GetName(), mechanism: u.GetMechanism()}
	m := u.GetMechanism()
	return &dataplanev1.CreateUserResponse{User: &dataplanev1.CreateUserResponse_User{
		Name:      u.GetName(),
		Mechanism: &m,
	}}, nil
}

// ListUsers returns users matching the filter. Name is exact match;
// NameContains is substring; both unset returns all.
func (f *UserFake) ListUsers(_ context.Context, req *dataplanev1.ListUsersRequest) (*dataplanev1.ListUsersResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	wantName := ""
	wantContains := ""
	if flt := req.GetFilter(); flt != nil {
		wantName = flt.GetName()
		wantContains = flt.GetNameContains()
	}
	out := make([]*dataplanev1.ListUsersResponse_User, 0, len(f.store))
	for _, rec := range f.store {
		if wantName != "" && rec.name != wantName {
			continue
		}
		if wantContains != "" && !strings.Contains(rec.name, wantContains) {
			continue
		}
		m := rec.mechanism
		out = append(out, &dataplanev1.ListUsersResponse_User{
			Name:      rec.name,
			Mechanism: &m,
		})
	}
	return &dataplanev1.ListUsersResponse{Users: out}, nil
}

// UpdateUser rewrites the mechanism on an existing user. Password is
// write-only and not stored. Returns NotFound if absent.
func (f *UserFake) UpdateUser(_ context.Context, req *dataplanev1.UpdateUserRequest) (*dataplanev1.UpdateUserResponse, error) {
	u := req.GetUser()
	if u == nil {
		return nil, status.Error(codes.InvalidArgument, "user is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[u.GetName()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %q not found", u.GetName())
	}
	rec.mechanism = u.GetMechanism()
	m := rec.mechanism
	return &dataplanev1.UpdateUserResponse{User: &dataplanev1.UpdateUserResponse_User{
		Name:      rec.name,
		Mechanism: &m,
	}}, nil
}

// DeleteUser removes the user; returns NotFound if absent.
func (f *UserFake) DeleteUser(_ context.Context, req *dataplanev1.DeleteUserRequest) (*dataplanev1.DeleteUserResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[req.GetName()]; !ok {
		return nil, status.Errorf(codes.NotFound, "user %q not found", req.GetName())
	}
	delete(f.store, req.GetName())
	return &dataplanev1.DeleteUserResponse{}, nil
}
