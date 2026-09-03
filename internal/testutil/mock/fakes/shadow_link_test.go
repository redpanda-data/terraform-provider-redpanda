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
