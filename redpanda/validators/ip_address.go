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

package validators

import (
	"context"
	"net"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = IPAddressValidator{}

// IPAddressValidator ensures the value parses as a valid IPv4 or IPv6 address
// literal, mirroring the API's buf.validate (buf.validate.field).string.ip
// constraint (net.ParseIP semantics — no CIDR suffix).
type IPAddressValidator struct{}

// Description provides a description of the validator.
func (IPAddressValidator) Description(_ context.Context) string {
	return "value must be a valid IPv4 or IPv6 address"
}

// MarkdownDescription provides a description of the validator in markdown format.
func (IPAddressValidator) MarkdownDescription(_ context.Context) string {
	return "value must be a valid IPv4 or IPv6 address"
}

// ValidateString validates that the string is a parseable IP address.
func (IPAddressValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if net.ParseIP(req.ConfigValue.ValueString()) == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP Address",
			"value must be a valid IPv4 or IPv6 address, got: "+req.ConfigValue.ValueString(),
		)
	}
}
