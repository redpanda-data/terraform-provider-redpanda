package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// CloudProviderDependentValidator is a custom validator to ensure that an attribute is only set when cloud_provider is a specific value
// For example when using this on aws_private_link it will ensure that the HCL fails validation unless cloud_provider is set to "aws"
// AttributeName should be the name of the attribute that is being validated
// CloudProvider should be the value of cloud_provider that the attribute is dependent on
type CloudProviderDependentValidator struct {
	AttributeName string
	CloudProvider string
}

// Description provides a description of the validator
func (v CloudProviderDependentValidator) Description(_ context.Context) string {
	return fmt.Sprintf("ensures that %s is only set when cloud_provider is %s", v.AttributeName, v.CloudProvider)
}

// MarkdownDescription provides a description of the validator in markdown format
func (v CloudProviderDependentValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Ensures that `%s` is only set when `cloud_provider` is `%s`", v.AttributeName, v.CloudProvider)
}

// ValidateObject validates an object
func (v CloudProviderDependentValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	var cloudProvider types.String
	if diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("cloud_provider"), &cloudProvider); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// If the object is set and cloud_provider is known but doesn't match, add an error
	if !req.ConfigValue.IsNull() && !cloudProvider.IsUnknown() && cloudProvider.ValueString() != v.CloudProvider {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Configuration",
			fmt.Sprintf("%s can only be set when cloud_provider is %s, but it is set to %s",
				v.AttributeName, v.CloudProvider, cloudProvider.ValueString()),
		)
	}
}
