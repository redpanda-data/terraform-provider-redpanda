package validators

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.String = CIDRBlockValidator{}

// CIDRBlockValidator ensures that cidr_block is only set when customer_managed_resources is not set
type CIDRBlockValidator struct{}

// Description returns a human-readable description of the validator
func (CIDRBlockValidator) Description(_ context.Context) string {
	return "ensures that cidr_block is set only when customer_managed_resources is not set"
}

// MarkdownDescription returns a markdown-formatted description of the validator
func (CIDRBlockValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that `cidr_block` is set only when `customer_managed_resources` is not set"
}

// ValidateString validates the cidr_block attribute
func (CIDRBlockValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	var cmr types.Object
	if diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("customer_managed_resources"), &cmr); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// If customer_managed_resources is set, cidr_block must not be set
	if !cmr.IsNull() && !req.ConfigValue.IsNull() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Configuration",
			"cidr_block cannot be set when customer_managed_resources is set",
		)
		return
	}

	// If customer_managed_resources is not set, cidr_block must be set
	if cmr.IsNull() && req.ConfigValue.IsNull() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Configuration",
			"cidr_block must be set when customer_managed_resources is not set",
		)
		return
	}

	// Validate CIDR block format if a value is provided
	if !req.ConfigValue.IsNull() {
		cidrRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}/(\d{1,2})$`)
		if !cidrRegex.MatchString(req.ConfigValue.ValueString()) {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid CIDR Block",
				"The value must be a valid CIDR block (e.g., 192.168.0.0/16)",
			)
		}
	}
}
