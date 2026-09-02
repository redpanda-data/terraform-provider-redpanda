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
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	corev2 "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/admin/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestShadowLinkFake_RoleSyncEchoAlwaysPopulated pins fake parity with the
// control plane's shadowLinkRoleSyncOptionsCPToPublicAPI, which returns a
// non-nil role_sync_options on every GET (even for links that never
// configured it) specifically to carry effective_interval.
func TestShadowLinkFake_RoleSyncEchoAlwaysPopulated(t *testing.T) {
	f := NewShadowLinkFake(NewOperationFake())
	ctx := context.Background()

	op, err := f.CreateShadowLink(ctx, &controlplanev1.CreateShadowLinkRequest{
		ShadowLink: &controlplanev1.ShadowLinkCreate{
			Name:             "no-role-sync",
			ShadowRedpandaId: "shadow-id",
		},
	})
	if err != nil {
		t.Fatalf("CreateShadowLink: %v", err)
	}
	id := op.GetOperation().GetResourceId()

	got, err := f.GetShadowLink(ctx, &controlplanev1.GetShadowLinkRequest{Id: id})
	if err != nil {
		t.Fatalf("GetShadowLink: %v", err)
	}
	rs := got.GetShadowLink().GetRoleSyncOptions()
	if rs == nil {
		t.Fatal("role_sync_options is nil on GET; the control plane always returns a non-nil block carrying effective_interval")
	}
	if secs := rs.GetEffectiveInterval().GetSeconds(); secs != 30 {
		t.Fatalf("effective_interval = %ds, want the 30s server default", secs)
	}
}

// TestShadowLinkFake_NameFilterWildcardRequiresLiteral pins the documented
// NameFilter constraint on every surface that carries one: a "*" name must be
// the only character AND use PATTERN_TYPE_LITERAL. The real backend rejects
// the combination; a lenient fake would let an invalid fixture pass.
func TestShadowLinkFake_NameFilterWildcardRequiresLiteral(t *testing.T) {
	badFilter := &corev2.NameFilter{
		Name:        "*",
		PatternType: corev2.PatternType_PATTERN_TYPE_PREFIX,
		FilterType:  corev2.FilterType_FILTER_TYPE_INCLUDE,
	}

	cases := []struct {
		name string
		link *controlplanev1.ShadowLinkCreate
	}{
		{"role_name_filters", &controlplanev1.ShadowLinkCreate{
			Name:             "bad-role-filter",
			ShadowRedpandaId: "shadow-id",
			RoleSyncOptions:  &corev2.RoleSyncOptions{RoleNameFilters: []*corev2.NameFilter{badFilter}},
		}},
		{"group_filters", &controlplanev1.ShadowLinkCreate{
			Name:                      "bad-group-filter",
			ShadowRedpandaId:          "shadow-id",
			ConsumerOffsetSyncOptions: &corev2.ConsumerOffsetSyncOptions{GroupFilters: []*corev2.NameFilter{badFilter}},
		}},
		{"auto_create_shadow_topic_filters", &controlplanev1.ShadowLinkCreate{
			Name:                     "bad-topic-filter",
			ShadowRedpandaId:         "shadow-id",
			TopicMetadataSyncOptions: &corev2.TopicMetadataSyncOptions{AutoCreateShadowTopicFilters: []*corev2.NameFilter{badFilter}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewShadowLinkFake(NewOperationFake())
			_, err := f.CreateShadowLink(context.Background(), &controlplanev1.CreateShadowLinkRequest{ShadowLink: tc.link})
			if err == nil {
				t.Fatal("CreateShadowLink accepted a \"*\" name with PATTERN_TYPE_PREFIX; the backend requires PATTERN_TYPE_LITERAL")
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
		})
	}
}
