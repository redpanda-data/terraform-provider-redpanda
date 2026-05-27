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

package user

import (
	"context"
	"fmt"
	"testing"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mechanism is the user resource's Optional+Computed proto-presence field —
// server populates a default (e.g. SCRAM-SHA-256) when the user leaves it
// unset. Mirrors cluster's gcp_global_access_enabled matrix; the post-import
// case (null prev + proto present → use proto) is the load-bearing row.

type stubUser struct {
	name      string
	hasMech   bool
	mechanism dataplanev1.SASLMechanism
}

func (s stubUser) GetName() string                         { return s.name }
func (s stubUser) HasMechanism() bool                      { return s.hasMech }
func (s stubUser) GetMechanism() dataplanev1.SASLMechanism { return s.mechanism }

func stringifyString(s types.String) string {
	switch {
	case s.IsNull():
		return "Null"
	case s.IsUnknown():
		return "Unknown"
	default:
		return fmt.Sprintf("StringValue(%q)", s.ValueString())
	}
}

func TestFlattenMechanismMatrix(t *testing.T) {
	cases := []struct {
		name      string
		prevValue types.String
		protoHas  bool
		protoVal  dataplanev1.SASLMechanism
		want      types.String
	}{
		{"Null prev + Has=true (post-import)", types.StringNull(), true, dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256, types.StringValue("scram-sha-256")},
		{"Null prev + Has=false", types.StringNull(), false, 0, types.StringNull()},
		{"StringValue prev + Has=true match", types.StringValue("scram-sha-256"), true, dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256, types.StringValue("scram-sha-256")},
		{"StringValue prev + Has=true drift", types.StringValue("scram-sha-256"), true, dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512, types.StringValue("scram-sha-512")},
		{"StringValue prev + Has=false (preserve)", types.StringValue("scram-sha-256"), false, 0, types.StringValue("scram-sha-256")},
		{"Unknown prev + Has=true", types.StringUnknown(), true, dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512, types.StringValue("scram-sha-512")},
		{"Unknown prev + Has=false", types.StringUnknown(), false, 0, types.StringNull()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proto := stubUser{name: "u", hasMech: tc.protoHas, mechanism: tc.protoVal}
			prev := &ResourceModel{Mechanism: tc.prevValue}
			out, diags := Flatten(context.Background(), proto, prev)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags.Errors())
			}
			if !out.Mechanism.Equal(tc.want) {
				t.Errorf("got %s, want %s (prev=%s, proto.Has=%t)",
					stringifyString(out.Mechanism),
					stringifyString(tc.want),
					stringifyString(tc.prevValue),
					tc.protoHas,
				)
			}
		})
	}
}

func TestFlattenMechanism_NilPrev(t *testing.T) {
	proto := stubUser{name: "u", hasMech: true, mechanism: dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256}
	out, diags := Flatten(context.Background(), proto, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if !out.Mechanism.Equal(types.StringValue("scram-sha-256")) {
		t.Errorf("nil prev: got %s, want scram-sha-256", stringifyString(out.Mechanism))
	}
}
