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

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.String = RequirePrivateConnectionValidator{}

// RequirePrivateConnectionValidator ensures that if customer_managed_resources is set, then connection_type must be private
type RequirePrivateConnectionValidator struct{}

// Description returns a human-readable description of the validator
func (v RequirePrivateConnectionValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown-formatted description of the validator
func (RequirePrivateConnectionValidator) MarkdownDescription(_ context.Context) string {
	return "Ensure that if customer_managed_resources is set, then connection_type must be private"
}

// ValidateString validates whether it is set to private when customer_managed_resources is set
func (RequirePrivateConnectionValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	if !req.Path.Equal(path.Root("connection_type")) {
		return
	}

	var customerManagedResources types.Object
	diags := req.Config.GetAttribute(ctx, path.Root("customer_managed_resources"), &customerManagedResources)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !customerManagedResources.IsNull() && !customerManagedResources.IsUnknown() {
		if req.ConfigValue.ValueString() != "private" {
			resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
				req.Path,
				"Invalid Connection Type",
				"connection_type must be \"private\" when customer_managed_resources is specified",
			))
		}
	}
}
