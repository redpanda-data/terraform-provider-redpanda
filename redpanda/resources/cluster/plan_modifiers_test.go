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
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestReleasePinForServerAssign pins the rpsql.zones release matrix: the pin
// only opens on a fresh enable (rise with no retained zones), where the
// control plane assigns the first cluster zone. Everything else pins so the
// leaf-expanded update mask never carries empty zones into the control
// plane's defaulter + immutability checks.
func TestReleasePinForServerAssign(t *testing.T) {
	ctx := context.Background()
	null := types.ListNull(types.StringType)
	empty, d := types.ListValueFrom(ctx, types.StringType, []string{})
	if d.HasError() {
		t.Fatalf("ListValueFrom: %v", d.Errors())
	}
	retained, d := types.ListValueFrom(ctx, types.StringType, []string{"use1-az1"})
	if d.HasError() {
		t.Fatalf("ListValueFrom: %v", d.Errors())
	}

	cases := []struct {
		name         string
		planEnabled  types.Bool
		stateEnabled types.Bool
		stateZones   types.List
		wantRelease  bool
	}{
		{"fresh enable from null state zones", types.BoolValue(true), types.BoolValue(false), null, true},
		{"fresh enable from null-bool enabled", types.BoolValue(true), types.BoolNull(), null, true},
		{"fresh enable from empty state zones", types.BoolValue(true), types.BoolValue(false), empty, true},
		{"re-enable with retained zones pins", types.BoolValue(true), types.BoolValue(false), retained, false},
		{"steady enabled pins", types.BoolValue(true), types.BoolValue(true), retained, false},
		{"steady disabled pins null", types.BoolValue(false), types.BoolValue(false), null, false},
		{"disable pins retained zones", types.BoolValue(false), types.BoolValue(true), retained, false},
		{"unknown plan enabled stays unknown", types.BoolUnknown(), types.BoolValue(false), null, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := releasePinForServerAssign(tc.planEnabled, tc.stateEnabled, tc.stateZones)
			if got != tc.wantRelease {
				t.Errorf("release = %v, want %v", got, tc.wantRelease)
			}
		})
	}
}

// TestReleaseStringPinForServerAssign pins the rpsql.url / rpsql.version
// release matrix: same fresh-enable-only release as zones, where the control
// plane derives the value on enable. Empty string counts as no retained value.
func TestReleaseStringPinForServerAssign(t *testing.T) {
	cases := []struct {
		name         string
		planEnabled  types.Bool
		stateEnabled types.Bool
		stateValue   types.String
		wantRelease  bool
	}{
		{"fresh enable from null state", types.BoolValue(true), types.BoolValue(false), types.StringNull(), true},
		{"fresh enable from null-bool enabled", types.BoolValue(true), types.BoolNull(), types.StringNull(), true},
		{"add disabled block from null", types.BoolValue(false), types.BoolNull(), types.StringNull(), true},
		{"enable from empty retained re-derives", types.BoolValue(true), types.BoolValue(false), types.StringValue(""), true},
		{"re-enable re-derives", types.BoolValue(true), types.BoolValue(false), types.StringValue("oxla:5432"), true},
		{"steady enabled pins", types.BoolValue(true), types.BoolValue(true), types.StringValue("oxla:5432"), false},
		{"steady disabled pins empty", types.BoolValue(false), types.BoolValue(false), types.StringValue(""), false},
		{"disable re-derives", types.BoolValue(false), types.BoolValue(true), types.StringValue("oxla:5432"), true},
		{"unknown plan enabled stays unknown", types.BoolUnknown(), types.BoolValue(false), types.StringNull(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := releaseStringPinForServerAssign(tc.planEnabled, tc.stateEnabled, tc.stateValue)
			if got != tc.wantRelease {
				t.Errorf("release = %v, want %v", got, tc.wantRelease)
			}
		})
	}
}
