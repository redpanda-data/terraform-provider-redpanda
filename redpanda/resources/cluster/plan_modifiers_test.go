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

package cluster

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
)

// TestReleasePinForServerAssign pins the rpsql.zones release matrix: the pin
// opens on a fresh enable (rise with no retained zones), where the control
// plane assigns the first cluster zone, and on a disable, where the control
// plane clears zones. Steady states pin so an unrelated update plan does not
// churn the leaf to "known after apply".
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
		{"disable releases, server clears zones", types.BoolValue(false), types.BoolValue(true), retained, true},
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

// TestImpliedConnectionType pins the control plane's derivation of
// connection_type from a service's projected connections: any private entry
// reads back private, otherwise public; an unresolved list yields no verdict.
func TestImpliedConnectionType(t *testing.T) {
	elemType := types.ObjectType{AttrTypes: clustermodel.KafkaAPIConnectionsAttrTypes()}
	conn := func(connType string) attr.Value {
		auth, d := types.ObjectValue(clustermodel.KafkaAPIConnectionsAuthAttrTypes(), map[string]attr.Value{"mode": types.StringValue("sasl")})
		if d.HasError() {
			t.Fatalf("auth: %v", d)
		}
		obj, d := types.ObjectValue(clustermodel.KafkaAPIConnectionsAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(connType), "auth": auth, "endpoint": types.StringNull(),
		})
		if d.HasError() {
			t.Fatalf("conn: %v", d)
		}
		return obj
	}
	list := func(els ...attr.Value) types.List {
		l, d := types.ListValue(elemType, els)
		if d.HasError() {
			t.Fatalf("list: %v", d)
		}
		return l
	}
	cases := []struct {
		name   string
		conns  types.List
		want   string
		wantOK bool
	}{
		{"public only", list(conn("public")), "public", true},
		{"private only", list(conn("private")), "private", true},
		{"dual", list(conn("public"), conn("private")), "private", true},
		{"empty projects public", list(), "public", true},
		{"null list no verdict", types.ListNull(elemType), "", false},
		{"unknown list no verdict", types.ListUnknown(elemType), "", false},
		{"unknown element no verdict", list(types.ObjectUnknown(clustermodel.KafkaAPIConnectionsAttrTypes())), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := impliedConnectionType(tc.conns)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("impliedConnectionType = %q/%v, want %q/%v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
