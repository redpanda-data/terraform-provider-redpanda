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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

func TestIPAddressValidator(t *testing.T) {
	testCases := []struct {
		name        string
		value       types.String
		expectError bool
	}{
		{name: "null - skip validation", value: types.StringNull()},
		{name: "unknown - skip validation", value: types.StringUnknown()},
		{name: "valid IPv4", value: types.StringValue("10.1.0.4")},
		{name: "valid IPv4 broadcast-ish", value: types.StringValue("192.168.255.254")},
		{name: "valid IPv6", value: types.StringValue("2001:db8::1")},
		{name: "valid IPv6 loopback", value: types.StringValue("::1")},
		{name: "invalid - empty string", value: types.StringValue(""), expectError: true},
		{name: "invalid - hostname", value: types.StringValue("firewall.internal"), expectError: true},
		{name: "invalid - CIDR suffix", value: types.StringValue("10.1.0.0/24"), expectError: true},
		{name: "invalid - octet out of range", value: types.StringValue("10.1.0.256"), expectError: true},
		{name: "invalid - trailing space", value: types.StringValue("10.1.0.4 "), expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				Path:           path.Root("firewall_private_ip"),
				PathExpression: path.Root("firewall_private_ip").Expression(),
				ConfigValue:    tc.value,
			}
			var resp validator.StringResponse

			validators.IPAddressValidator{}.ValidateString(context.Background(), req, &resp)

			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestIPAddressValidator_Description(t *testing.T) {
	v := validators.IPAddressValidator{}

	desc := v.Description(context.Background())
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "IP")

	mdDesc := v.MarkdownDescription(context.Background())
	assert.NotEmpty(t, mdDesc)
	assert.Contains(t, mdDesc, "IP")
}
