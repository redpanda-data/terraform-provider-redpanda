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

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// canonicalizePrincipal mirrors the dataplane SecurityService behavior of
// prefixing a bare principal with "User:" when no type prefix is supplied.
func canonicalizePrincipal(p string) string {
	switch {
	case strings.HasPrefix(p, "User:"),
		strings.HasPrefix(p, "Group:"),
		strings.HasPrefix(p, "RedpandaRole:"):
		return p
	default:
		return "User:" + p
	}
}

// roleRecord backs both redpanda_role (name) and redpanda_role_assignment
// (members on that name).
type roleRecord struct {
	name    string
	members []string
}

// SecurityFake is a stateful in-memory implementation of the console
// SecurityService RPCs. Backs both redpanda_role and redpanda_role_assignment
// resources — membership state lives under the owning role record.
type SecurityFake struct {
	consolev1alpha1grpc.UnimplementedSecurityServiceServer

	mu    sync.Mutex
	roles map[string]*roleRecord
}

// NewSecurityFake returns an empty SecurityFake.
func NewSecurityFake() *SecurityFake {
	return &SecurityFake{roles: map[string]*roleRecord{}}
}

// SeedRoleWithMembers pre-populates the fake with a role and its members
// for tests that need to drive Read/Delete code paths against state seeded
// outside the normal Create flow (e.g. ImportState-with-bare-principal
// scenarios that simulate legacy pre-validator state). Principals are
// stored canonicalized exactly as UpdateRoleMembership.Add would store
// them; bare inputs get the "User:" prefix.
func (f *SecurityFake) SeedRoleWithMembers(name string, principals ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec := &roleRecord{name: name}
	for _, p := range principals {
		rec.members = append(rec.members, canonicalizePrincipal(p))
	}
	f.roles[name] = rec
}

// CreateRole stores a new role keyed by name. AlreadyExists if present.
func (f *SecurityFake) CreateRole(_ context.Context, req *consolev1alpha1.CreateRoleRequest) (*consolev1alpha1.CreateRoleResponse, error) {
	inner := req.GetRequest()
	if inner == nil || inner.GetRole() == nil {
		return nil, status.Error(codes.InvalidArgument, "role is required")
	}
	name := inner.GetRole().GetName()
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.roles[name]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "role %q already exists", name)
	}
	f.roles[name] = &roleRecord{name: name}
	return &consolev1alpha1.CreateRoleResponse{Response: &dataplanev1.CreateRoleResponse{
		Role: &dataplanev1.Role{Name: name},
	}}, nil
}

// GetRole returns the role by name. NotFound if absent.
func (f *SecurityFake) GetRole(_ context.Context, req *consolev1alpha1.GetRoleRequest) (*consolev1alpha1.GetRoleResponse, error) {
	inner := req.GetRequest()
	if inner == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.roles[inner.GetRoleName()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "role %q not found", inner.GetRoleName())
	}
	return &consolev1alpha1.GetRoleResponse{Response: &dataplanev1.GetRoleResponse{
		Role:    &dataplanev1.Role{Name: rec.name},
		Members: membersToProto(rec.members),
	}}, nil
}

// DeleteRole removes the role; NotFound if absent.
func (f *SecurityFake) DeleteRole(_ context.Context, req *consolev1alpha1.DeleteRoleRequest) (*consolev1alpha1.DeleteRoleResponse, error) {
	inner := req.GetRequest()
	if inner == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.roles[inner.GetRoleName()]; !ok {
		return nil, status.Errorf(codes.NotFound, "role %q not found", inner.GetRoleName())
	}
	delete(f.roles, inner.GetRoleName())
	return &consolev1alpha1.DeleteRoleResponse{}, nil
}

// ListRoleMembers returns the members of a role. NotFound if absent.
func (f *SecurityFake) ListRoleMembers(_ context.Context, req *consolev1alpha1.ListRoleMembersRequest) (*consolev1alpha1.ListRoleMembersResponse, error) {
	inner := req.GetRequest()
	if inner == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.roles[inner.GetRoleName()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "role %q not found", inner.GetRoleName())
	}
	return &consolev1alpha1.ListRoleMembersResponse{Response: &dataplanev1.ListRoleMembersResponse{
		RoleName: rec.name,
		Members:  membersToProto(rec.members),
	}}, nil
}

// UpdateRoleMembership applies add + remove lists to the role's membership.
// NotFound if the role doesn't exist and inner.Create is false; if Create is
// true, the role is created and only the Add list is honored.
func (f *SecurityFake) UpdateRoleMembership(_ context.Context, req *consolev1alpha1.UpdateRoleMembershipRequest) (*consolev1alpha1.UpdateRoleMembershipResponse, error) {
	inner := req.GetRequest()
	if inner == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	name := inner.GetRoleName()
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.roles[name]
	if !ok {
		if !inner.GetCreate() {
			return nil, status.Errorf(codes.NotFound, "role %q not found", name)
		}
		rec = &roleRecord{name: name}
		f.roles[name] = rec
	}
	var added []*dataplanev1.RoleMembership
	for _, m := range inner.GetAdd() {
		p := canonicalizePrincipal(m.GetPrincipal())
		if !containsString(rec.members, p) {
			rec.members = append(rec.members, p)
			added = append(added, &dataplanev1.RoleMembership{Principal: p})
		}
	}
	var removed []*dataplanev1.RoleMembership
	if !inner.GetCreate() {
		for _, m := range inner.GetRemove() {
			p := canonicalizePrincipal(m.GetPrincipal())
			if i := indexOfString(rec.members, p); i >= 0 {
				rec.members = append(rec.members[:i], rec.members[i+1:]...)
				removed = append(removed, &dataplanev1.RoleMembership{Principal: p})
			}
		}
	}
	return &consolev1alpha1.UpdateRoleMembershipResponse{Response: &dataplanev1.UpdateRoleMembershipResponse{
		RoleName: name,
		Added:    added,
		Removed:  removed,
	}}, nil
}

func membersToProto(in []string) []*dataplanev1.RoleMembership {
	out := make([]*dataplanev1.RoleMembership, 0, len(in))
	for _, p := range in {
		out = append(out, &dataplanev1.RoleMembership{Principal: p})
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func indexOfString(haystack []string, needle string) int {
	for i, s := range haystack {
		if s == needle {
			return i
		}
	}
	return -1
}
