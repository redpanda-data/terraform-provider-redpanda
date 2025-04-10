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

package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ validator.Object = CloudProviderDependentValidator{}
	_ validator.Bool   = CloudProviderDependentValidator{}
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

// ValidateObject validates an object attribute to ensure it is only set when cloud_provider is a specific value
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

// ValidateBool validates a boolean attribute to ensure it is only set when cloud_provider is a specific value
func (v CloudProviderDependentValidator) ValidateBool(ctx context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	var cloudProvider types.String
	if diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("cloud_provider"), &cloudProvider); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// If the bool is set and cloud_provider is known but doesn't match, add an error
	if !req.ConfigValue.IsNull() && !cloudProvider.IsUnknown() && cloudProvider.ValueString() != v.CloudProvider {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Configuration",
			fmt.Sprintf("%s can only be set when cloud_provider is %s, but it is set to %s",
				v.AttributeName, v.CloudProvider, cloudProvider.ValueString()),
		)
	}
}
