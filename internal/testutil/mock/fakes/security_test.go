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
	"testing"

	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
)

// TestSecurityFake_PrincipalCanonicalization pins fake parity with the
// dataplane SecurityService: UpdateRoleMembership stores a bare principal
// canonicalized to "User:<name>"; ListRoleMembers returns the canonical form.
func TestSecurityFake_PrincipalCanonicalization(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"bare username canonicalized to User: form", "alice", "User:alice"},
		{"User: prefix preserved verbatim", "User:bob", "User:bob"},
		{"Group: prefix preserved verbatim", "Group:engineers", "Group:engineers"},
		{"RedpandaRole: prefix preserved verbatim", "RedpandaRole:admin", "RedpandaRole:admin"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewSecurityFake()
			ctx := context.Background()

			updateReq := &consolev1alpha1.UpdateRoleMembershipRequest{
				Request: &dataplanev1.UpdateRoleMembershipRequest{
					RoleName: "role",
					Create:   true,
					Add:      []*dataplanev1.RoleMembership{{Principal: tc.input}},
				},
			}
			if _, err := f.UpdateRoleMembership(ctx, updateReq); err != nil {
				t.Fatalf("UpdateRoleMembership: %v", err)
			}

			listReq := &consolev1alpha1.ListRoleMembersRequest{
				Request: &dataplanev1.ListRoleMembersRequest{RoleName: "role"},
			}
			listResp, err := f.ListRoleMembers(ctx, listReq)
			if err != nil {
				t.Fatalf("ListRoleMembers: %v", err)
			}
			members := listResp.GetResponse().GetMembers()
			if len(members) != 1 {
				t.Fatalf("want 1 member, got %d", len(members))
			}
			if got := members[0].GetPrincipal(); got != tc.expected {
				t.Errorf("principal: want %q, got %q", tc.expected, got)
			}
		})
	}
}
