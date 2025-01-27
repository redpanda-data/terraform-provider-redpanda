package validators

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.String = RequirePrivateConnectionValidator{}

type RequirePrivateConnectionValidator struct{}

func (v RequirePrivateConnectionValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

func (v RequirePrivateConnectionValidator) MarkdownDescription(_ context.Context) string {
	return "Ensure that if customer_managed_resources is set, then connection_type must be private"
}

func (v RequirePrivateConnectionValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
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
