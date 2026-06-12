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

// aclKey is the 7-tuple uniquely identifying an ACL record.
type aclKey struct {
	resourceType        dataplanev1.ACL_ResourceType
	resourceName        string
	resourcePatternType dataplanev1.ACL_ResourcePatternType
	principal           string
	host                string
	operation           dataplanev1.ACL_Operation
	permissionType      dataplanev1.ACL_PermissionType
}

// ACLFake is a stateful in-memory implementation of the ACLService RPC
// surface. Safe for concurrent use.
type ACLFake struct {
	dataplanev1grpc.UnimplementedACLServiceServer

	mu    sync.Mutex
	store map[aclKey]struct{}
}

// NewACLFake returns an empty ACLFake.
func NewACLFake() *ACLFake {
	return &ACLFake{store: map[aclKey]struct{}{}}
}

// CreateACL stores a new ACL keyed by the 7-tuple. Returns AlreadyExists if a
// record with the same key is present.
func (f *ACLFake) CreateACL(_ context.Context, req *dataplanev1.CreateACLRequest) (*dataplanev1.CreateACLResponse, error) {
	k := aclKey{
		resourceType:        req.GetResourceType(),
		resourceName:        req.GetResourceName(),
		resourcePatternType: req.GetResourcePatternType(),
		principal:           req.GetPrincipal(),
		host:                req.GetHost(),
		operation:           req.GetOperation(),
		permissionType:      req.GetPermissionType(),
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[k]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "acl already exists")
	}
	f.store[k] = struct{}{}
	return &dataplanev1.CreateACLResponse{}, nil
}

// ListACLs returns records matching the filter, grouped by
// (resource_type, resource_name, resource_pattern_type) into Resource entries
// each carrying its set of Policy entries.
func (f *ACLFake) ListACLs(_ context.Context, req *dataplanev1.ListACLsRequest) (*dataplanev1.ListACLsResponse, error) {
	flt := req.GetFilter()
	f.mu.Lock()
	defer f.mu.Unlock()
	type resGroupKey struct {
		rtype    dataplanev1.ACL_ResourceType
		rname    string
		rpattern dataplanev1.ACL_ResourcePatternType
	}
	groups := map[resGroupKey][]*dataplanev1.ListACLsResponse_Policy{}
	order := []resGroupKey{}
	for k := range f.store {
		if flt != nil && !matchACLFilter(flt, k) {
			continue
		}
		g := resGroupKey{rtype: k.resourceType, rname: k.resourceName, rpattern: k.resourcePatternType}
		if _, seen := groups[g]; !seen {
			order = append(order, g)
		}
		groups[g] = append(groups[g], &dataplanev1.ListACLsResponse_Policy{
			Principal:      k.principal,
			Host:           k.host,
			Operation:      k.operation,
			PermissionType: k.permissionType,
		})
	}
	out := make([]*dataplanev1.ListACLsResponse_Resource, 0, len(order))
	for _, g := range order {
		out = append(out, &dataplanev1.ListACLsResponse_Resource{
			ResourceType:        g.rtype,
			ResourceName:        g.rname,
			ResourcePatternType: g.rpattern,
			Acls:                groups[g],
		})
	}
	return &dataplanev1.ListACLsResponse{Resources: out}, nil
}

// DeleteACLs removes every record matching the single filter and returns them
// as MatchingAcls with nil Error fields (success).
func (f *ACLFake) DeleteACLs(_ context.Context, req *dataplanev1.DeleteACLsRequest) (*dataplanev1.DeleteACLsResponse, error) {
	flt := req.GetFilter()
	f.mu.Lock()
	defer f.mu.Unlock()
	var matched []*dataplanev1.DeleteACLsResponse_MatchingACL
	for k := range f.store {
		if flt != nil && !matchACLFilter(flt, k) {
			continue
		}
		matched = append(matched, &dataplanev1.DeleteACLsResponse_MatchingACL{
			ResourceType:        k.resourceType,
			ResourceName:        k.resourceName,
			ResourcePatternType: k.resourcePatternType,
			Principal:           k.principal,
			Host:                k.host,
			Operation:           k.operation,
			PermissionType:      k.permissionType,
		})
		delete(f.store, k)
	}
	return &dataplanev1.DeleteACLsResponse{MatchingAcls: matched}, nil
}

// aclFilter is the common shape of the List and Delete ACL request filters.
type aclFilter interface {
	GetResourceType() dataplanev1.ACL_ResourceType
	GetResourcePatternType() dataplanev1.ACL_ResourcePatternType
	GetOperation() dataplanev1.ACL_Operation
	GetPermissionType() dataplanev1.ACL_PermissionType
	HasResourceName() bool
	GetResourceName() string
	HasPrincipal() bool
	GetPrincipal() string
	HasHost() bool
	GetHost() string
}

// matchACLFilter applies wildcard-aware match semantics: enums at 0
// (UNSPECIFIED) wildcard; proto3-optional string fields wildcard when unset. A
// nil filter (match-all) is handled by callers before reaching here.
func matchACLFilter(flt aclFilter, k aclKey) bool {
	if flt.GetResourceType() != 0 && flt.GetResourceType() != k.resourceType {
		return false
	}
	if flt.GetResourcePatternType() != 0 && flt.GetResourcePatternType() != k.resourcePatternType {
		return false
	}
	if flt.GetOperation() != 0 && flt.GetOperation() != k.operation {
		return false
	}
	if flt.GetPermissionType() != 0 && flt.GetPermissionType() != k.permissionType {
		return false
	}
	if flt.HasResourceName() && flt.GetResourceName() != k.resourceName {
		return false
	}
	if flt.HasPrincipal() && flt.GetPrincipal() != k.principal {
		return false
	}
	if flt.HasHost() && flt.GetHost() != k.host {
		return false
	}
	return true
}
