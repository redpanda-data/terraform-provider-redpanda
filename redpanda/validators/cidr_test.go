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

package validators_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

var cmrTfType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"placeholder": tftypes.String,
	},
}

var testSchema = schema.Schema{
	Attributes: map[string]schema.Attribute{
		"cidr_block": schema.StringAttribute{Optional: true},
		"customer_managed_resources": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"placeholder": schema.StringAttribute{Optional: true},
			},
		},
	},
}

var configObjectType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"cidr_block":                 tftypes.String,
		"customer_managed_resources": cmrTfType,
	},
}

func buildConfig(cidrValue, cmrValue tftypes.Value) tfsdk.Config {
	return tfsdk.Config{
		Schema: testSchema,
		Raw: tftypes.NewValue(configObjectType, map[string]tftypes.Value{
			"cidr_block":                 cidrValue,
			"customer_managed_resources": cmrValue,
		}),
	}
}

func cmrNull() tftypes.Value {
	return tftypes.NewValue(cmrTfType, nil)
}

func cmrUnknown() tftypes.Value {
	return tftypes.NewValue(cmrTfType, tftypes.UnknownValue)
}

func cmrSet() tftypes.Value {
	return tftypes.NewValue(cmrTfType, map[string]tftypes.Value{
		"placeholder": tftypes.NewValue(tftypes.String, "value"),
	})
}

func TestCIDRBlockValidator(t *testing.T) {
	testCases := []struct {
		name          string
		cidrBlock     types.String
		cidrRaw       tftypes.Value
		cmrRaw        tftypes.Value
		expectError   bool
		errorSummary  string
		errorContains string
	}{
		// Unknown values
		{
			name:      "unknown cidr_block with null cmr - skip validation",
			cidrBlock: types.StringUnknown(),
			cidrRaw:   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			cmrRaw:    cmrNull(),
		},
		{
			name:      "unknown cidr_block with set cmr - skip validation",
			cidrBlock: types.StringUnknown(),
			cidrRaw:   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			cmrRaw:    cmrSet(),
		},
		{
			name:      "known cidr_block with unknown cmr - skip validation",
			cidrBlock: types.StringValue("10.0.0.0/16"),
			cidrRaw:   tftypes.NewValue(tftypes.String, "10.0.0.0/16"),
			cmrRaw:    cmrUnknown(),
		},
		{
			name:      "null cidr_block with unknown cmr - skip validation",
			cidrBlock: types.StringNull(),
			cidrRaw:   tftypes.NewValue(tftypes.String, nil),
			cmrRaw:    cmrUnknown(),
		},

		// Valid CIDRs
		{
			name:      "valid CIDR 10.0.0.0/16",
			cidrBlock: types.StringValue("10.0.0.0/16"),
			cidrRaw:   tftypes.NewValue(tftypes.String, "10.0.0.0/16"),
			cmrRaw:    cmrNull(),
		},
		{
			name:      "valid CIDR 192.168.0.0/24",
			cidrBlock: types.StringValue("192.168.0.0/24"),
			cidrRaw:   tftypes.NewValue(tftypes.String, "192.168.0.0/24"),
			cmrRaw:    cmrNull(),
		},
		{
			name:      "valid CIDR 172.16.0.0/12",
			cidrBlock: types.StringValue("172.16.0.0/12"),
			cidrRaw:   tftypes.NewValue(tftypes.String, "172.16.0.0/12"),
			cmrRaw:    cmrNull(),
		},
		{
			name:      "valid CIDR 10.0.0.0/20",
			cidrBlock: types.StringValue("10.0.0.0/20"),
			cidrRaw:   tftypes.NewValue(tftypes.String, "10.0.0.0/20"),
			cmrRaw:    cmrNull(),
		},

		// Invalid CIDRs
		{
			name:          "invalid CIDR - missing prefix length",
			cidrBlock:     types.StringValue("10.0.0.0"),
			cidrRaw:       tftypes.NewValue(tftypes.String, "10.0.0.0"),
			cmrRaw:        cmrNull(),
			expectError:   true,
			errorSummary:  "Invalid CIDR Block",
			errorContains: "valid CIDR block",
		},
		{
			name:          "invalid CIDR - not an IP",
			cidrBlock:     types.StringValue("not-a-cidr"),
			cidrRaw:       tftypes.NewValue(tftypes.String, "not-a-cidr"),
			cmrRaw:        cmrNull(),
			expectError:   true,
			errorSummary:  "Invalid CIDR Block",
			errorContains: "valid CIDR block",
		},
		{
			name:          "invalid CIDR - empty string",
			cidrBlock:     types.StringValue(""),
			cidrRaw:       tftypes.NewValue(tftypes.String, ""),
			cmrRaw:        cmrNull(),
			expectError:   true,
			errorSummary:  "Invalid CIDR Block",
			errorContains: "valid CIDR block",
		},

		// Mutual exclusivity
		{
			name:          "both cidr_block and cmr set - error",
			cidrBlock:     types.StringValue("10.0.0.0/16"),
			cidrRaw:       tftypes.NewValue(tftypes.String, "10.0.0.0/16"),
			cmrRaw:        cmrSet(),
			expectError:   true,
			errorSummary:  "Invalid Configuration",
			errorContains: "cannot be set when customer_managed_resources is set",
		},
		{
			name:          "neither cidr_block nor cmr set - error",
			cidrBlock:     types.StringNull(),
			cidrRaw:       tftypes.NewValue(tftypes.String, nil),
			cmrRaw:        cmrNull(),
			expectError:   true,
			errorSummary:  "Invalid Configuration",
			errorContains: "must be set when customer_managed_resources is not set",
		},

		// BYOVPC
		{
			name:      "null cidr_block with cmr set - valid BYOVPC",
			cidrBlock: types.StringNull(),
			cidrRaw:   tftypes.NewValue(tftypes.String, nil),
			cmrRaw:    cmrSet(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := buildConfig(tc.cidrRaw, tc.cmrRaw)

			req := validator.StringRequest{
				Path:           path.Root("cidr_block"),
				PathExpression: path.Root("cidr_block").Expression(),
				ConfigValue:    tc.cidrBlock,
				Config:         config,
			}
			var resp validator.StringResponse

			validators.CIDRBlockValidator{}.ValidateString(context.Background(), req, &resp)

			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
				if tc.errorSummary != "" {
					found := false
					for _, d := range resp.Diagnostics {
						if d.Severity() == diag.SeverityError && d.Summary() == tc.errorSummary {
							found = true
							if tc.errorContains != "" {
								assert.Contains(t, d.Detail(), tc.errorContains, "error detail doesn't contain expected text")
							}
							break
						}
					}
					assert.True(t, found, "expected error with summary %q not found in diagnostics", tc.errorSummary)
				}
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestCIDRBlockValidator_Description(t *testing.T) {
	v := validators.CIDRBlockValidator{}

	desc := v.Description(context.Background())
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "cidr_block")

	mdDesc := v.MarkdownDescription(context.Background())
	assert.NotEmpty(t, mdDesc)
	assert.Contains(t, mdDesc, "cidr_block")
}
