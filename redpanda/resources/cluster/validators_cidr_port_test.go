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

package cluster

import (
	"strings"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rpvalidate "github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/protovalidate"
)

// TestCidrPortPortEndProtoValidation proves that the buf.validate CEL
// constraint on Cluster_CidrPort —
//
//	this.port_end == 0 || this.port_end >= this.port_start
//
// is enforced by rpvalidate.Validate (the same function the protoValidator
// calls at plan time). No separate attribute-level validator is needed
// because ConfigValidators already covers this constraint.
func TestCidrPortPortEndProtoValidation(t *testing.T) {
	cases := []struct {
		name      string
		portStart int32
		portEnd   int32
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "single port (portEnd=0)",
			portStart: 5432,
			portEnd:   0,
			wantErr:   false,
		},
		{
			name:      "valid range",
			portStart: 5432,
			portEnd:   5500,
			wantErr:   false,
		},
		{
			name:      "same start and end",
			portStart: 5432,
			portEnd:   5432,
			wantErr:   false,
		},
		{
			name:      "portEnd < portStart",
			portStart: 5432,
			portEnd:   5431,
			wantErr:   true,
			errSubstr: "port_end",
		},
		{
			name:      "portEnd < portStart large gap",
			portStart: 1000,
			portEnd:   999,
			wantErr:   true,
			errSubstr: "port_end",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &controlplanev1.Cluster_CidrPort{
				Cidr:      "10.0.0.0/8",
				PortStart: tc.portStart,
				PortEnd:   tc.portEnd,
			}
			diags := rpvalidate.Validate(path.Empty(), msg)
			if tc.wantErr && !diags.HasError() {
				t.Errorf("expected validation error for portStart=%d portEnd=%d, got none",
					tc.portStart, tc.portEnd)
			}
			if !tc.wantErr && diags.HasError() {
				t.Errorf("expected no error for portStart=%d portEnd=%d, got: %v",
					tc.portStart, tc.portEnd, diags)
			}
			if tc.wantErr && tc.errSubstr != "" {
				found := false
				for _, d := range diags {
					if strings.Contains(d.Summary(), tc.errSubstr) ||
						strings.Contains(d.Detail(), tc.errSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected diagnostic containing %q; got: %v", tc.errSubstr, diags)
				}
			}
		})
	}
}
