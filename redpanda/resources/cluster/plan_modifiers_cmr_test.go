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
	"errors"
	"maps"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/clustermask"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// cmrTypes holds the tftypes shapes the guard tests build values from.
type cmrTypes struct {
	obj, cmr, gcp, aws, account, sg, rpsql tftypes.Object
}

func newCMRTypes(t *testing.T) cmrTypes {
	t.Helper()
	s := ResourceClusterSchema(context.Background())
	obj := mustTFType[tftypes.Object](t, s.Type().TerraformType(context.Background()))
	cmr := mustTFType[tftypes.Object](t, obj.AttributeTypes["customer_managed_resources"])
	gcp := mustTFType[tftypes.Object](t, cmr.AttributeTypes["gcp"])
	aws := mustTFType[tftypes.Object](t, cmr.AttributeTypes["aws"])
	return cmrTypes{
		obj: obj, cmr: cmr, gcp: gcp, aws: aws,
		account: mustTFType[tftypes.Object](t, gcp.AttributeTypes["redpanda_connect_service_account"]),
		sg:      mustTFType[tftypes.Object](t, aws.AttributeTypes["redpanda_connect_security_group"]),
		rpsql:   mustTFType[tftypes.Object](t, obj.AttributeTypes["rpsql"]),
	}
}

func (ct cmrTypes) email(v string) tftypes.Value {
	return nullsExcept(ct.account, map[string]tftypes.Value{"email": tftypes.NewValue(tftypes.String, v)})
}

func (ct cmrTypes) arn(v string) tftypes.Value {
	return nullsExcept(ct.sg, map[string]tftypes.Value{"arn": tftypes.NewValue(tftypes.String, v)})
}

func (ct cmrTypes) withGCP(blocks map[string]tftypes.Value) map[string]tftypes.Value {
	return map[string]tftypes.Value{"customer_managed_resources": nullsExcept(ct.cmr, map[string]tftypes.Value{"gcp": nullsExcept(ct.gcp, blocks)})}
}

func (ct cmrTypes) withAWS(blocks map[string]tftypes.Value) map[string]tftypes.Value {
	return map[string]tftypes.Value{"customer_managed_resources": nullsExcept(ct.cmr, map[string]tftypes.Value{"aws": nullsExcept(ct.aws, blocks)})}
}

func (ct cmrTypes) rpsqlOn() map[string]tftypes.Value {
	return map[string]tftypes.Value{"rpsql": nullsExcept(ct.rpsql, map[string]tftypes.Value{"enabled": tftypes.NewValue(tftypes.Bool, true)})}
}

func merge(ms ...map[string]tftypes.Value) map[string]tftypes.Value {
	out := map[string]tftypes.Value{}
	for _, m := range ms {
		maps.Copy(out, m)
	}
	return out
}

func unknown(typ tftypes.Type) tftypes.Value { return tftypes.NewValue(typ, tftypes.UnknownValue) }

func TestGuardCustomerManagedResourcesTransition(t *testing.T) {
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	ct := newCMRTypes(t)
	typed := func(clusterType string, network tftypes.Value, cmr bool) tftypes.Value {
		attrs := map[string]tftypes.Value{
			"network_id":   network,
			"cluster_type": tftypes.NewValue(tftypes.String, clusterType),
		}
		if cmr {
			attrs["customer_managed_resources"] = nullsExcept(ct.cmr, map[string]tftypes.Value{"gcp": nullsExcept(ct.gcp, nil)})
		}
		return nullsExcept(ct.obj, attrs)
	}
	cluster := func(network tftypes.Value, cmr bool) tftypes.Value { return typed("byoc", network, cmr) }
	dedicated := func(network tftypes.Value) tftypes.Value { return typed("dedicated", network, false) }
	netA := tftypes.NewValue(tftypes.String, "net-a")
	netB := tftypes.NewValue(tftypes.String, "net-b")
	netUnknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	lookupFails := errors.New("permission denied")
	withBlock := func(network tftypes.Value, arms map[string]tftypes.Value) tftypes.Value {
		return nullsExcept(ct.obj, map[string]tftypes.Value{
			"network_id":                 network,
			"cluster_type":               tftypes.NewValue(tftypes.String, "byoc"),
			"customer_managed_resources": nullsExcept(ct.cmr, arms),
		})
	}

	cases := []struct {
		name         string
		state, plan  tftypes.Value
		targetHasCMR bool
		lookupErr    error
		wantErr      string
		wantLookups  int
	}{
		{name: "byovpc keeps the block", state: cluster(netA, true), plan: cluster(netA, true)},
		{name: "dedicated never had one", state: cluster(netA, false), plan: cluster(netA, false)},
		{name: "byovpc drops the block", state: cluster(netA, true), plan: cluster(netA, false), wantErr: "cannot be removed"},
		{name: "moving to a network without them drops the block", state: cluster(netA, true), plan: cluster(netB, false), wantLookups: 1},
		{name: "moving to a network with them drops the block", state: cluster(netA, true), plan: cluster(netB, false), targetHasCMR: true, wantErr: "has customer-managed resources", wantLookups: 1},
		{name: "moving to a network with them keeps the block", state: cluster(netA, true), plan: cluster(netB, true), targetHasCMR: true, wantLookups: 1},
		{name: "moving to a network without them keeps the block", state: cluster(netA, true), plan: cluster(netB, true), wantErr: "has no customer-managed resources", wantLookups: 1},
		{name: "moving a cluster without them onto a network with them", state: cluster(netA, false), plan: cluster(netB, false), targetHasCMR: true, wantErr: "has customer-managed resources", wantLookups: 1},
		{name: "moving a cluster without them to a network without them", state: cluster(netA, false), plan: cluster(netB, false), wantLookups: 1},
		{name: "a dedicated cluster moves without a network read", state: dedicated(netA), plan: dedicated(netB)},
		{name: "adding the block on the same network", state: cluster(netA, false), plan: cluster(netA, true), wantErr: "has no customer-managed resources", wantLookups: 1},
		{name: "moving to a network not yet known", state: cluster(netA, true), plan: cluster(netUnknown, false), targetHasCMR: true},
		{name: "network cannot be read", state: cluster(netA, true), plan: cluster(netB, false), lookupErr: lookupFails, wantErr: "could not be read", wantLookups: 1},
		{name: "adding a block without an arm on the same network", state: cluster(netA, false), plan: withBlock(netA, nil), wantErr: "has no customer-managed resources", wantLookups: 1},
		{name: "moving to a network with them keeps the block without an arm", state: cluster(netA, true), plan: withBlock(netB, nil), targetHasCMR: true, wantErr: "has customer-managed resources", wantLookups: 1},
		{name: "arm not known yet", state: cluster(netA, true), plan: withBlock(netA, map[string]tftypes.Value{"gcp": unknown(ct.gcp)})},
	}
	for _, arm := range []string{"aws", "azure", "gcp"} {
		armType := mustTFType[tftypes.Object](t, ct.cmr.AttributeTypes[arm])
		cases = append(cases, struct {
			name         string
			state, plan  tftypes.Value
			targetHasCMR bool
			lookupErr    error
			wantErr      string
			wantLookups  int
		}{
			name:    "byovpc drops the " + arm + " arm and keeps the block",
			state:   withBlock(netA, map[string]tftypes.Value{arm: nullsExcept(armType, nil)}),
			plan:    withBlock(netA, nil),
			wantErr: "cannot be removed",
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				Plan:  tfsdk.Plan{Schema: s, Raw: tc.plan},
				State: tfsdk.State{Schema: s, Raw: tc.state},
			}
			var resp resource.ModifyPlanResponse
			lookups := 0
			guardCustomerManagedResourcesTransition(ctx, req, &resp, func(context.Context, string) (bool, error) {
				lookups++
				return tc.targetHasCMR, tc.lookupErr
			})
			if lookups != tc.wantLookups {
				t.Fatalf("network lookups = %d, want %d", lookups, tc.wantLookups)
			}
			assertDiag(t, resp.Diagnostics, diag.SeverityError, tc.wantErr)
		})
	}
}

// TestNetworkHasCMR pins the lookup behind the transition guard: a transient
// failure is retried, anything else is returned so the plan fails closed.
func TestNetworkHasCMR(t *testing.T) {
	utils.SetTestModeWaits()
	ctx := context.Background()
	withCMR := &controlplanev1.Network{Id: "net-b", CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
		CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{
			NetworkName:      "tfrp-vpc",
			NetworkProjectId: "tfrp-proj",
			ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{Name: "tfrp-mgmt"},
		}},
	}}
	cases := []struct {
		name    string
		answers []func() (*controlplanev1.Network, error)
		want    bool
		wantErr bool
	}{
		{"has them", []func() (*controlplanev1.Network, error){func() (*controlplanev1.Network, error) { return withCMR, nil }}, true, false},
		{"does not have them", []func() (*controlplanev1.Network, error){func() (*controlplanev1.Network, error) { return &controlplanev1.Network{Id: "net-b"}, nil }}, false, false},
		{"transient failure then answer", []func() (*controlplanev1.Network, error){
			func() (*controlplanev1.Network, error) { return nil, status.Error(codes.Unavailable, "try later") },
			func() (*controlplanev1.Network, error) { return withCMR, nil },
		}, true, false},
		{"not found", []func() (*controlplanev1.Network, error){func() (*controlplanev1.Network, error) { return nil, status.Error(codes.NotFound, "no such network") }}, false, true},
		{"permission denied", []func() (*controlplanev1.Network, error){func() (*controlplanev1.Network, error) { return nil, status.Error(codes.PermissionDenied, "no") }}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := &Cluster{}
			c.CpCl = &cloud.ControlPlaneClientSet{Network: stubNetworkClient{get: func() (*controlplanev1.Network, error) {
				answer := tc.answers[min(calls, len(tc.answers)-1)]
				calls++
				return answer()
			}}}
			got, err := c.networkHasCMR(ctx, "net-b")
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, err=%v)", got, err, tc.want, tc.wantErr)
			}
			if calls != len(tc.answers) {
				t.Fatalf("GetNetwork calls = %d, want %d", calls, len(tc.answers))
			}
		})
	}
}

// stubNetworkClient answers GetNetwork from a script; every other method of
// the client is unused by the lookup under test.
type stubNetworkClient struct {
	controlplanev1grpc.NetworkServiceClient
	get func() (*controlplanev1.Network, error)
}

func (s stubNetworkClient) GetNetwork(context.Context, *controlplanev1.GetNetworkRequest, ...grpc.CallOption) (*controlplanev1.GetNetworkResponse, error) {
	nw, err := s.get()
	if err != nil {
		return nil, err
	}
	return &controlplanev1.GetNetworkResponse{Network: nw}, nil
}

func TestStateOnlySettings(t *testing.T) {
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	ct := newCMRTypes(t)
	emptyArm := ct.withGCP(nil)
	cmrState := merge(emptyArm, ct.rpsqlOn())

	cases := []struct {
		name          string
		state, config map[string]tftypes.Value
		want          []stateOnlySetting
	}{
		{"rpsql enabled only in state beside customer-managed resources", cmrState, emptyArm, []stateOnlySetting{{Path: "rpsql"}}},
		{"rpsql enabled only in state without customer-managed resources", ct.rpsqlOn(), nil, []stateOnlySetting{{Path: "rpsql", Warn: true}}},
		{"rpsql enabled only in state, config gains customer-managed resources", ct.rpsqlOn(), emptyArm, []stateOnlySetting{{Path: "rpsql"}}},
		{"rpsql enabled only in state, config drops customer-managed resources", cmrState, nil, []stateOnlySetting{{Path: "rpsql"}}},
		{"rpsql not known yet", ct.rpsqlOn(), map[string]tftypes.Value{"rpsql": unknown(ct.rpsql)}, nil},
		{"rpsql declared", ct.rpsqlOn(), ct.rpsqlOn(), nil},
		{"rpsql off in state", merge(emptyArm, map[string]tftypes.Value{"rpsql": nullsExcept(ct.rpsql, map[string]tftypes.Value{"enabled": tftypes.NewValue(tftypes.Bool, false)})}), emptyArm, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := tfsdk.State{Schema: s, Raw: nullsExcept(ct.obj, tc.state)}
			config := tfsdk.Config{Schema: s, Raw: nullsExcept(ct.obj, tc.config)}
			got, diags := stateOnlySettings(ctx, state, config)
			if diags.HasError() {
				t.Fatalf("diagnostics: %v", diags)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}

// TestGuardReplaceWithStateOnlySettings pins how the recorded settings reach
// the create pass of a replace: a held setting refuses the plan, one the
// control plane may put back only warns.
func TestGuardReplaceWithStateOnlySettings(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		held     []stateOnlySetting
		wantErr  string
		wantWarn string
	}{
		{name: "nothing recorded"},
		{name: "rpsql held", held: []stateOnlySetting{{Path: "rpsql"}}, wantErr: "Redpanda SQL off"},
		{name: "rpsql the control plane may put back", held: []stateOnlySetting{{Path: "rpsql", Warn: true}}, wantWarn: "Redpanda SQL off"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			priv := fakePrivate{}
			if d := writeStateOnlySettings(ctx, priv, tc.held); d.HasError() {
				t.Fatalf("write marker: %v", d)
			}
			var resp resource.ModifyPlanResponse
			refuseReplaceWithStateOnlySettings(ctx, priv, &resp)
			assertDiag(t, resp.Diagnostics, diag.SeverityError, tc.wantErr)
			assertDiag(t, resp.Diagnostics, diag.SeverityWarning, tc.wantWarn)
		})
	}
}

// TestGuardConnectPairing pins the pairing rule on both passes of a plan. On
// the update pass the planned values decide: a half the config adds or changes
// without its partner is an error, so is leaving one half out while the
// cluster holds it, a lone half the config keeps unchanged is a warning, a
// half whose config is not known yet is left for the apply-time plan, and a
// pair the config leaves out is cleared with a warning that Connect goes off.
// On a create, plain or the create pass of a replace, a lone half in config is
// an error.
func TestGuardConnectPairing(t *testing.T) {
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	ct := newCMRTypes(t)
	const (
		sgARN    = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-connect"
		newSGARN = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-connect-2"
		profile  = "arn:aws:iam::123456789012:instance-profile/tfrp-rp-connect-ng"
	)
	profileType := mustTFType[tftypes.Object](t, ct.aws.AttributeTypes["redpanda_connect_node_group_instance_profile"])
	profileARN := nullsExcept(profileType, map[string]tftypes.Value{"arn": tftypes.NewValue(tftypes.String, profile)})
	emptyProfile := nullsExcept(profileType, map[string]tftypes.Value{"arn": tftypes.NewValue(tftypes.String, "")})
	sg := func(v tftypes.Value) map[string]tftypes.Value {
		return ct.withAWS(map[string]tftypes.Value{"redpanda_connect_security_group": v})
	}
	both := ct.withAWS(map[string]tftypes.Value{"redpanda_connect_security_group": ct.arn(sgARN), "redpanda_connect_node_group_instance_profile": profileARN})
	sgAndEmptyProfile := ct.withAWS(map[string]tftypes.Value{"redpanda_connect_security_group": ct.arn(sgARN), "redpanda_connect_node_group_instance_profile": emptyProfile})
	unknownARN := nullsExcept(ct.sg, map[string]tftypes.Value{"arn": unknown(tftypes.String)})
	none := ct.withAWS(nil)
	value := func(m map[string]tftypes.Value) tftypes.Value { return nullsExcept(ct.obj, m) }

	cases := []struct {
		name              string
		state, cfg, plan  map[string]tftypes.Value
		create            bool
		wantErr, wantWarn string
	}{
		{name: "pair declared", state: both, cfg: both, plan: both},
		{name: "half added", state: none, cfg: sg(ct.arn(sgARN)), plan: sg(ct.arn(sgARN)), wantErr: "needs both blocks"},
		{name: "half changed", state: sg(ct.arn(sgARN)), cfg: sg(ct.arn(newSGARN)), plan: sg(ct.arn(newSGARN)), wantErr: "needs both blocks"},
		{name: "lone half kept unchanged", state: sg(ct.arn(sgARN)), cfg: sg(ct.arn(sgARN)), plan: sg(ct.arn(sgARN)), wantWarn: "needs both blocks"},
		{name: "lone half kept, value not known yet", state: sg(ct.arn(sgARN)), cfg: sg(unknownARN), plan: sg(unknownARN)},
		{name: "lone half the server set, config silent", state: sg(ct.arn(sgARN)), cfg: none, plan: none},
		{name: "partner emptied on the server, half kept", state: sgAndEmptyProfile, cfg: sg(ct.arn(sgARN)), plan: sg(ct.arn(sgARN)), wantWarn: "needs both blocks"},
		{name: "half removed while the other stays", state: both, cfg: sg(ct.arn(sgARN)), plan: sg(ct.arn(sgARN)), wantErr: "Remove both blocks"},
		{name: "half removed and the other changed", state: both, cfg: sg(ct.arn(newSGARN)), plan: sg(ct.arn(newSGARN)), wantErr: "Remove both blocks"},
		{name: "half removed, the other not known yet", state: both, cfg: sg(unknownARN), plan: sg(unknownARN), wantErr: "Remove both blocks"},
		{name: "pair removed", state: both, cfg: none, plan: none, wantWarn: "turns Redpanda Connect off"},
		{name: "pair removed, one half empty on the server", state: sgAndEmptyProfile, cfg: none, plan: none},
		{name: "pair not held, config silent", state: none, cfg: none, plan: none},
		{name: "plain create with a lone half", cfg: sg(ct.arn(sgARN)), plan: sg(ct.arn(sgARN)), create: true, wantErr: "Set both blocks"},
		{name: "create with a lone half not known yet", cfg: sg(unknownARN), plan: sg(unknownARN), create: true, wantErr: "needs both blocks"},
		{name: "create with the pair", cfg: both, plan: both, create: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: s, Raw: value(tc.cfg)},
				Plan:   tfsdk.Plan{Schema: s, Raw: value(tc.plan)},
				State:  tfsdk.State{Schema: s, Raw: tftypes.NewValue(ct.obj, nil)},
			}
			if !tc.create {
				req.State.Raw = value(tc.state)
			}
			var resp resource.ModifyPlanResponse
			guardConnectPairing(ctx, req, &resp)
			assertDiag(t, resp.Diagnostics, diag.SeverityError, tc.wantErr)
			assertDiag(t, resp.Diagnostics, diag.SeverityWarning, tc.wantWarn)
		})
	}
}

// assertDiag requires exactly one diagnostic of the severity when want is
// non-empty, with want in its detail, and none of that severity otherwise.
func assertDiag(t *testing.T, diags diag.Diagnostics, sev diag.Severity, want string) {
	t.Helper()
	var found []diag.Diagnostic
	for _, d := range diags {
		if d.Severity() == sev {
			found = append(found, d)
		}
	}
	switch {
	case want == "" && len(found) > 0:
		t.Fatalf("unexpected %v: %v", sev, found)
	case want != "" && len(found) != 1:
		t.Fatalf("%v diagnostics = %v, want one containing %q", sev, found, want)
	case want != "" && !strings.Contains(found[0].Detail(), want) && !strings.Contains(found[0].Summary(), want):
		t.Fatalf("%v diagnostic %q / %q does not mention %q", sev, found[0].Summary(), found[0].Detail(), want)
	}
}

// TestConnectBlocksMatchSchema ties the Connect pairs and the operator
// account to the schema: each is a plain Optional nested object, so leaving
// it out of the configuration clears it, and no nested object under
// customer_managed_resources is Optional+Computed, the shape that keeps a
// value the configuration leaves out and so cannot be cleared at all.
func TestConnectBlocksMatchSchema(t *testing.T) {
	ctx := context.Background()
	cmr, ok := ResourceClusterSchema(ctx).Attributes["customer_managed_resources"].(rschema.SingleNestedAttribute)
	if !ok {
		t.Fatal("customer_managed_resources is not a single nested attribute")
	}
	for armName, arm := range cmr.Attributes {
		nested, ok := arm.(rschema.SingleNestedAttribute)
		if !ok {
			continue
		}
		for leafName, leaf := range nested.Attributes {
			if obj, ok := leaf.(rschema.SingleNestedAttribute); ok && obj.IsOptional() && obj.IsComputed() {
				t.Errorf("%s is Optional+Computed, so a configuration can never clear it", cmrPath.AtName(armName).AtName(leafName))
			}
		}
	}
	for _, p := range connectBlocks() {
		steps := p.Steps()
		arm, ok := cmr.Attributes[steps[1].String()].(rschema.SingleNestedAttribute)
		if !ok {
			t.Fatalf("%s: arm is not a single nested attribute", p)
		}
		obj, ok := arm.Attributes[steps[2].String()].(rschema.SingleNestedAttribute)
		if !ok {
			t.Fatalf("%s: not a single nested attribute in the schema", p)
		}
		if !obj.IsOptional() || obj.IsComputed() || obj.IsRequired() {
			t.Errorf("%s: want plain Optional, got optional=%v computed=%v required=%v", p, obj.IsOptional(), obj.IsComputed(), obj.IsRequired())
		}
	}
}

// TestConnectBlocksCoverMaskTable ties the Connect pairs and the operator
// account to the mask table: every message the table clears whole must be
// one the plan guards judge, or a block added to the table would clear
// without the pairing check.
func TestConnectBlocksCoverMaskTable(t *testing.T) {
	want := map[string]bool{}
	for _, p := range clustermask.CMRUpdatableLeafPaths() {
		seg := strings.Split(p, ".")
		if strings.HasPrefix(seg[2], "redpanda_connect_") || strings.HasPrefix(seg[2], "redpanda_operator_") {
			want[seg[1]+"."+seg[2]] = true
		}
	}
	got := map[string]bool{}
	for _, p := range connectBlocks() {
		steps := p.Steps()
		got[steps[1].String()+"."+steps[2].String()] = true
	}
	if !maps.Equal(got, want) {
		t.Fatalf("connectBlocks = %v, mask table Connect entries = %v", got, want)
	}
}

// TestGuardOperatorAccountRemoved pins that a configuration which leaves the
// operator account out while the cluster holds it plans the clear with a
// warning, and that nothing else draws one.
func TestGuardOperatorAccountRemoved(t *testing.T) {
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	ct := newCMRTypes(t)
	const operator = "rp-op@tfrp-proj.iam.gserviceaccount.com"
	held := ct.withGCP(map[string]tftypes.Value{"redpanda_operator_service_account": ct.email(operator)})
	empty := ct.withGCP(map[string]tftypes.Value{"redpanda_operator_service_account": ct.email("")})
	none := ct.withGCP(nil)
	value := func(m map[string]tftypes.Value) tftypes.Value { return nullsExcept(ct.obj, m) }

	cases := []struct {
		name             string
		state, cfg, plan map[string]tftypes.Value
		create           bool
		wantWarn         string
	}{
		{name: "declared", state: held, cfg: held, plan: held},
		{name: "removed", state: held, cfg: none, plan: none, wantWarn: "operator"},
		{name: "never held", state: none, cfg: none, plan: none},
		{name: "empty on the server", state: empty, cfg: none, plan: none},
		{name: "config not known yet", state: held, cfg: map[string]tftypes.Value{"customer_managed_resources": unknown(ct.cmr)}, plan: map[string]tftypes.Value{"customer_managed_resources": unknown(ct.cmr)}},
		{name: "create", cfg: none, plan: none, create: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: s, Raw: value(tc.cfg)},
				Plan:   tfsdk.Plan{Schema: s, Raw: value(tc.plan)},
				State:  tfsdk.State{Schema: s, Raw: tftypes.NewValue(ct.obj, nil)},
			}
			if !tc.create {
				req.State.Raw = value(tc.state)
			}
			var resp resource.ModifyPlanResponse
			guardOperatorAccountRemoved(ctx, req, &resp)
			assertDiag(t, resp.Diagnostics, diag.SeverityError, "")
			assertDiag(t, resp.Diagnostics, diag.SeverityWarning, tc.wantWarn)
		})
	}
}

// TestRpsqlCMRLeavesCoverMaskTable pins that every rpsql_* entry of the update
// mask table, leaf or scalar, reaches the guard's list.
func TestRpsqlCMRLeavesCoverMaskTable(t *testing.T) {
	want := map[string]bool{}
	for _, p := range clustermask.CMRUpdatableLeafPaths() {
		seg := strings.Split(p, ".")
		if strings.HasPrefix(seg[2], "rpsql_") {
			want[seg[1]+"."+seg[2]] = true
		}
	}
	got := map[string]bool{}
	for _, l := range rpsqlCMRLeaves {
		got[l.arm+"."+l.leaf] = true
	}
	if !maps.Equal(got, want) {
		t.Fatalf("rpsqlCMRLeaves = %v, mask table rpsql entries = %v", got, want)
	}
}
