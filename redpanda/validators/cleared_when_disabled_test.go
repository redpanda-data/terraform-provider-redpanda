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

package validators_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

var rpsqlTfType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"enabled":  tftypes.Bool,
		"replicas": tftypes.Number,
		"zones":    tftypes.List{ElementType: tftypes.String},
	},
}

var rpsqlSchema = schema.Schema{
	Attributes: map[string]schema.Attribute{
		"rpsql": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"enabled":  schema.BoolAttribute{Optional: true},
				"replicas": schema.Int32Attribute{Optional: true},
				"zones":    schema.ListAttribute{Optional: true, ElementType: types.StringType},
			},
		},
	},
}

func rpsqlConfig(enabled, replicas, zones tftypes.Value) tfsdk.Config {
	return tfsdk.Config{
		Schema: rpsqlSchema,
		Raw: tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{"rpsql": rpsqlTfType},
		}, map[string]tftypes.Value{
			"rpsql": tftypes.NewValue(rpsqlTfType, map[string]tftypes.Value{
				"enabled":  enabled,
				"replicas": replicas,
				"zones":    zones,
			}),
		}),
	}
}

func zonesVal(zones ...string) tftypes.Value {
	vals := make([]tftypes.Value, len(zones))
	for i, z := range zones {
		vals[i] = tftypes.NewValue(tftypes.String, z)
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, vals)
}

func boolVal(b bool) tftypes.Value { return tftypes.NewValue(tftypes.Bool, b) }
func boolNull() tftypes.Value      { return tftypes.NewValue(tftypes.Bool, nil) }
func boolUnknown() tftypes.Value   { return tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue) }
func numNull() tftypes.Value       { return tftypes.NewValue(tftypes.Number, nil) }
func numVal(n int64) tftypes.Value { return tftypes.NewValue(tftypes.Number, n) }
func zonesNull() tftypes.Value {
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
}

func TestClearedWhenDisabledListValidator(t *testing.T) {
	enabledPath := path.Root("rpsql").AtName("enabled")

	testCases := []struct {
		name        string
		value       types.List
		enabled     tftypes.Value
		expectError bool
	}{
		{name: "null zones with disabled - ok", value: types.ListNull(types.StringType), enabled: boolVal(false)},
		{name: "unknown zones with disabled - ok", value: types.ListUnknown(types.StringType), enabled: boolVal(false)},
		{name: "zones with enabled - ok", value: mustList(t, "use1-az1"), enabled: boolVal(true)},
		{name: "zones with unknown enabled - defer", value: mustList(t, "use1-az1"), enabled: boolUnknown()},
		{name: "zones with disabled - error", value: mustList(t, "use1-az1"), enabled: boolVal(false), expectError: true},
		{name: "empty zones with disabled - error", value: mustList(t), enabled: boolVal(false), expectError: true},
		{name: "zones with omitted enabled - error", value: mustList(t, "use1-az1"), enabled: boolNull(), expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			zonesRaw := zonesNull()
			if !tc.value.IsNull() && !tc.value.IsUnknown() {
				elems := make([]string, 0, len(tc.value.Elements()))
				for _, e := range tc.value.Elements() {
					elems = append(elems, e.(types.String).ValueString())
				}
				zonesRaw = zonesVal(elems...)
			}
			req := validator.ListRequest{
				Path:           path.Root("rpsql").AtName("zones"),
				PathExpression: path.MatchRoot("rpsql").AtName("zones"),
				ConfigValue:    tc.value,
				Config:         rpsqlConfig(tc.enabled, numNull(), zonesRaw),
			}
			var resp validator.ListResponse

			validators.ClearedWhenDisabledListValidator{Enabled: enabledPath}.ValidateList(context.Background(), req, &resp)

			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestClearedWhenDisabledInt32Validator(t *testing.T) {
	enabledPath := path.Root("rpsql").AtName("enabled")

	testCases := []struct {
		name        string
		value       types.Int32
		enabled     tftypes.Value
		expectError bool
	}{
		{name: "null replicas with disabled - ok", value: types.Int32Null(), enabled: boolVal(false)},
		{name: "unknown replicas with disabled - ok", value: types.Int32Unknown(), enabled: boolVal(false)},
		{name: "zero replicas with disabled - ok, matches server reset", value: types.Int32Value(0), enabled: boolVal(false)},
		{name: "replicas with enabled - ok", value: types.Int32Value(3), enabled: boolVal(true)},
		{name: "replicas with unknown enabled - defer", value: types.Int32Value(3), enabled: boolUnknown()},
		{name: "replicas with disabled - error", value: types.Int32Value(3), enabled: boolVal(false), expectError: true},
		{name: "replicas with omitted enabled - error", value: types.Int32Value(3), enabled: boolNull(), expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			replicasRaw := numNull()
			if !tc.value.IsNull() && !tc.value.IsUnknown() {
				replicasRaw = numVal(int64(tc.value.ValueInt32()))
			}
			req := validator.Int32Request{
				Path:           path.Root("rpsql").AtName("replicas"),
				PathExpression: path.MatchRoot("rpsql").AtName("replicas"),
				ConfigValue:    tc.value,
				Config:         rpsqlConfig(tc.enabled, replicasRaw, zonesNull()),
			}
			var resp validator.Int32Response

			validators.ClearedWhenDisabledInt32Validator{Enabled: enabledPath}.ValidateInt32(context.Background(), req, &resp)

			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func mustList(t *testing.T, elems ...string) types.List {
	t.Helper()
	vals := make([]types.String, len(elems))
	for i, e := range elems {
		vals[i] = types.StringValue(e)
	}
	l, d := types.ListValueFrom(context.Background(), types.StringType, vals)
	assert.False(t, d.HasError())
	return l
}
