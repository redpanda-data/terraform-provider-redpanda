// Copyright 2024 Redpanda Data, Inc.
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

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// ServerlessPrivateLinkConfigRemovedMessage explains the cloud_provider_config →
// aws_config migration. Shared by the validator error and the schema
// deprecation message so both read identically.
const ServerlessPrivateLinkConfigRemovedMessage = "cloud_provider_config has been replaced by aws_config. Move your configuration from cloud_provider_config.aws to aws_config.\n\n" +
	"None of this serverless private link resource's attributes are marked RequiresReplace, so updating in place is safe. " +
	"To guarantee the transition will be a no-op, use terraform state rm followed by terraform import on each affected serverless private link resource:\n\n" +
	"  terraform state rm redpanda_serverless_private_link.<resource_name>\n" +
	"  terraform import redpanda_serverless_private_link.<resource_name> <serverless_private_link_id>"

var _ validator.Object = ServerlessPrivateLinkConfigRemovedValidator{}

// ServerlessPrivateLinkConfigRemovedValidator fails the plan when the deprecated
// cloud_provider_config attribute is set, directing the user to aws_config.
type ServerlessPrivateLinkConfigRemovedValidator struct{}

// Description returns a plain-text description of the validator.
func (v ServerlessPrivateLinkConfigRemovedValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown description of the validator.
func (ServerlessPrivateLinkConfigRemovedValidator) MarkdownDescription(_ context.Context) string {
	return "cloud_provider_config is removed; use aws_config instead"
}

// ValidateObject errors when cloud_provider_config is set to a non-null value.
func (ServerlessPrivateLinkConfigRemovedValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
		req.Path,
		"cloud_provider_config has been removed",
		ServerlessPrivateLinkConfigRemovedMessage,
	))
}
