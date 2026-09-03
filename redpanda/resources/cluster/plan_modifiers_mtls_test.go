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

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mtlsGuardFixture builds config and state for one service: whether the config
// carries an mtls block, whether it manages connections, and whether state
// has mTLS enabled (nil state mtls means the block is absent).
type mtlsGuardFixture struct {
	configHasMTLS   bool
	configHasConns  bool
	stateMTLS       *bool
	configMTLSValue bool
}

func (f mtlsGuardFixture) request(t *testing.T, svc string) resource.ModifyPlanRequest {
	t.Helper()
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	objType := mustTFType[tftypes.Object](t, s.Type().TerraformType(ctx))
	svcType := mustTFType[tftypes.Object](t, objType.AttributeTypes[svc])
	mtlsType := mustTFType[tftypes.Object](t, svcType.AttributeTypes["mtls"])
	connsType := mustTFType[tftypes.List](t, svcType.AttributeTypes["connections"])
	connType := mustTFType[tftypes.Object](t, connsType.ElementType)
	authType := mustTFType[tftypes.Object](t, connType.AttributeTypes["auth"])

	mtls := func(enabled bool) tftypes.Value {
		return nullsExcept(mtlsType, map[string]tftypes.Value{"enabled": tftypes.NewValue(tftypes.Bool, enabled)})
	}
	cfgSvc := map[string]tftypes.Value{}
	if f.configHasMTLS {
		cfgSvc["mtls"] = mtls(f.configMTLSValue)
	}
	if f.configHasConns {
		conn := nullsExcept(connType, map[string]tftypes.Value{
			"type": tftypes.NewValue(tftypes.String, "public"),
			"auth": nullsExcept(authType, map[string]tftypes.Value{"mode": tftypes.NewValue(tftypes.String, "sasl")}),
		})
		cfgSvc["connections"] = tftypes.NewValue(connsType, []tftypes.Value{conn})
	}
	stSvc := map[string]tftypes.Value{}
	if f.stateMTLS != nil {
		stSvc["mtls"] = mtls(*f.stateMTLS)
	}
	cfg := nullsExcept(objType, map[string]tftypes.Value{svc: nullsExcept(svcType, cfgSvc)})
	st := nullsExcept(objType, map[string]tftypes.Value{svc: nullsExcept(svcType, stSvc)})
	return resource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: s, Raw: cfg},
		Plan:   tfsdk.Plan{Schema: s, Raw: cfg},
		State:  tfsdk.State{Schema: s, Raw: st},
	}
}

// fakePrivate stands in for framework private state, whose concrete type is
// internal to the framework and cannot be built in a unit test.
type fakePrivate map[string][]byte

func (f fakePrivate) GetKey(_ context.Context, key string) ([]byte, diag.Diagnostics) {
	return f[key], nil
}

func (f fakePrivate) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	f[key] = value
	return nil
}

// TestGuardLegacyMTLSRemoval pins when dropping a service's mtls block is
// rejected: only when the block was in the config at the last applied change
// (the private-state marker), state has mTLS enabled, and the config neither
// carries the block nor manages connections. Import and out-of-band enablement
// leave no marker and plan clean. The marker follows the config: true while
// the block is present, false once a plan without it passes.
func TestGuardLegacyMTLSRemoval(t *testing.T) {
	on, off := true, false
	cases := []struct {
		name       string
		fixture    mtlsGuardFixture
		marker     string
		wantErr    bool
		wantMarker string
	}{
		{"block removed while enabled", mtlsGuardFixture{stateMTLS: &on}, "true", true, "true"},
		{"never configured while enabled (import, out-of-band)", mtlsGuardFixture{stateMTLS: &on}, "", false, "false"},
		{"block removed while disabled clears the marker", mtlsGuardFixture{stateMTLS: &off}, "true", false, "false"},
		{"block removed with no state block", mtlsGuardFixture{}, "true", false, "false"},
		{"block kept enabled", mtlsGuardFixture{configHasMTLS: true, configMTLSValue: true, stateMTLS: &on}, "", false, "true"},
		{"block explicitly disabled", mtlsGuardFixture{configHasMTLS: true, configMTLSValue: false, stateMTLS: &on}, "true", false, "true"},
		{"connections managed", mtlsGuardFixture{configHasConns: true, stateMTLS: &on}, "true", false, "false"},
	}
	for _, svc := range []string{"kafka_api", "http_proxy", "schema_registry"} {
		for _, tc := range cases {
			t.Run(svc+"/"+tc.name, func(t *testing.T) {
				req := tc.fixture.request(t, svc)
				priv := fakePrivate{}
				if tc.marker != "" {
					priv[mtlsConfiguredKey(svc)] = []byte(tc.marker)
				}
				var diags diag.Diagnostics
				checkLegacyMTLSRemoval(context.Background(), req.Config, req.State, priv, priv, &diags)
				if got := diags.HasError(); got != tc.wantErr {
					t.Errorf("error = %v, want %v: %v", got, tc.wantErr, diags)
				}
				if got := string(priv[mtlsConfiguredKey(svc)]); got != tc.wantMarker {
					t.Errorf("marker after plan = %q, want %q", got, tc.wantMarker)
				}
			})
		}
	}
}
