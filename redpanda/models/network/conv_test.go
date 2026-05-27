// Copyright 2026 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package network

import (
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/stretchr/testify/require"
)

// TestCidrBlockFromProto is the direct regression for the v1.9.0 upgrade-path
// bug: the backend returned cidr_block="0.0.0.0/0" on BYOVPC reads, which
// v1.9.0 wrote straight into state. CIDRBlockValidator forbids cidr_block
// when customer_managed_resources is set, so refreshing such state produced
// a forced-replacement diff. Stripping the sentinel in Flatten converges
// state to null.
func TestCidrBlockFromProto(t *testing.T) {
	cmr := &controlplanev1.Network_CustomerManagedResources{}
	cases := []struct {
		name    string
		cmr     *controlplanev1.Network_CustomerManagedResources
		cidr    string
		wantNul bool
		wantVal string
	}{
		{"byovpc sentinel -> null", cmr, "0.0.0.0/0", true, ""},
		{"byovpc real cidr -> passthrough", cmr, "10.0.0.0/16", false, "10.0.0.0/16"},
		{"non-byovpc real cidr -> passthrough", nil, "10.0.0.0/16", false, "10.0.0.0/16"},
		{"non-byovpc sentinel -> passthrough (not ours to interpret)", nil, "0.0.0.0/0", false, "0.0.0.0/0"},
		{"non-byovpc empty -> empty (unchanged)", nil, "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cidrBlockFromProto(&fakeCIDRProto{cidr: tc.cidr, cmr: tc.cmr})
			if tc.wantNul {
				require.True(t, got.IsNull(), "expected null, got %q", got.ValueString())
				return
			}
			require.False(t, got.IsNull(), "expected non-null")
			require.Equal(t, tc.wantVal, got.ValueString())
		})
	}

	var _ cidrBlockProto = (*controlplanev1.Network)(nil)
}

type fakeCIDRProto struct {
	cidr string
	cmr  *controlplanev1.Network_CustomerManagedResources
}

func (f *fakeCIDRProto) GetCidrBlock() string {
	return f.cidr
}

func (f *fakeCIDRProto) GetCustomerManagedResources() *controlplanev1.Network_CustomerManagedResources {
	return f.cmr
}
