// Copyright 2023 Redpanda Data, Inc.
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

package cluster

import (
	"context"
	"fmt"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Regression matrix for Flatten under the Optional+Computed +
// UseStateForUnknown contract. gcp_global_access_enabled is the
// representative scalar case (proto-optional bool, server populates a
// default). Apply-time consistency is handled by the plan modifier, so
// Flatten always defers to the proto when proto.HasGcpGlobalAccessEnabled()
// is true — including the null-prev case that the framework zero-initializes
// on the first Read after ImportState.

func stringifyBool(b types.Bool) string {
	switch {
	case b.IsNull():
		return "Null"
	case b.IsUnknown():
		return "Unknown"
	default:
		return fmt.Sprintf("BoolValue(%t)", b.ValueBool())
	}
}

func buildClusterWithGCPGlobal(has, val bool) *controlplanev1.Cluster {
	c := &controlplanev1.Cluster{}
	if has {
		v := val
		c.GcpGlobalAccessEnabled = &v
	}
	return c
}

func TestFlattenGCPGlobalAccessEnabledMatrix(t *testing.T) {
	cases := []struct {
		name      string
		prevValue types.Bool
		protoHas  bool
		protoVal  bool
		want      types.Bool
	}{
		{"Null prev + Has=true + value=false", types.BoolNull(), true, false, types.BoolValue(false)},
		{"Null prev + Has=true + value=true", types.BoolNull(), true, true, types.BoolValue(true)},
		{"Null prev + Has=false", types.BoolNull(), false, false, types.BoolNull()},
		{"BoolValue(true) prev + Has=true + value=true", types.BoolValue(true), true, true, types.BoolValue(true)},
		{"BoolValue(true) prev + Has=true + value=false (drift)", types.BoolValue(true), true, false, types.BoolValue(false)},
		{"BoolValue(true) prev + Has=false (preserve)", types.BoolValue(true), false, false, types.BoolValue(true)},
		{"BoolValue(false) prev + Has=true + value=true (drift)", types.BoolValue(false), true, true, types.BoolValue(true)},
		{"Unknown prev + Has=true + value=true", types.BoolUnknown(), true, true, types.BoolValue(true)},
		{"Unknown prev + Has=false", types.BoolUnknown(), false, false, types.BoolNull()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proto := buildClusterWithGCPGlobal(tc.protoHas, tc.protoVal)
			prev := &ResourceModel{GCPGlobalAccessEnabled: tc.prevValue}
			out, diags := Flatten(context.Background(), proto, prev)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags.Errors())
			}
			if !out.GCPGlobalAccessEnabled.Equal(tc.want) {
				t.Errorf("got %s, want %s (prev=%s, proto.Has=%t, value=%t)",
					stringifyBool(out.GCPGlobalAccessEnabled),
					stringifyBool(tc.want),
					stringifyBool(tc.prevValue),
					tc.protoHas, tc.protoVal,
				)
			}
		})
	}
}

func TestFlattenGCPGlobalAccessEnabled_NilPrev(t *testing.T) {
	proto := buildClusterWithGCPGlobal(true, false)
	out, diags := Flatten(context.Background(), proto, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if !out.GCPGlobalAccessEnabled.Equal(types.BoolValue(false)) {
		t.Errorf("nil prev: got %s, want BoolValue(false)", stringifyBool(out.GCPGlobalAccessEnabled))
	}
}
