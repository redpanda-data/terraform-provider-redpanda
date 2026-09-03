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
)

// nameFilterConfig builds a config holding one filter object so the validator
// can read pattern_type as the sibling of name. A nil pattern marks unknown.
func nameFilterConfig(name string, pattern *string) tfsdk.Config {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"filter": schema.SingleNestedAttribute{Required: true, Attributes: map[string]schema.Attribute{
			"name":         schema.StringAttribute{Required: true},
			"pattern_type": schema.StringAttribute{Required: true},
		}},
	}}
	filterType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "pattern_type": tftypes.String}}
	patternVal := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	if pattern != nil {
		patternVal = tftypes.NewValue(tftypes.String, *pattern)
	}
	return tfsdk.Config{
		Schema: s,
		Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"filter": filterType}}, map[string]tftypes.Value{
			"filter": tftypes.NewValue(filterType, map[string]tftypes.Value{
				"name":         tftypes.NewValue(tftypes.String, name),
				"pattern_type": patternVal,
			}),
		}),
	}
}

func TestNameFilterWildcardValidator(t *testing.T) {
	literal, prefix := "LITERAL", "PREFIX"
	cases := []struct {
		name      string
		filter    string
		pattern   *string
		wantError bool
	}{
		{"wildcard literal accepted", "*", &literal, false},
		{"wildcard prefix rejected", "*", &prefix, true},
		{"star inside a literal name is a character", "ops-*", &literal, false},
		{"star inside a prefix is a character", "*ops", &prefix, false},
		{"plain literal accepted", "ops", &literal, false},
		{"plain prefix accepted", "ops-", &prefix, false},
		{"wildcard with unknown pattern deferred", "*", nil, false},
	}

	v := validators.NameFilterWildcardValidator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			v.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("filter").AtName("name"),
				ConfigValue: types.StringValue(tc.filter),
				Config:      nameFilterConfig(tc.filter, tc.pattern),
			}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("ValidateString(%q, pattern %v) error = %v, want %v: %v", tc.filter, tc.pattern, got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
