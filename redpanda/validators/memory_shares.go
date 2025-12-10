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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ validator.String = MemorySharesValidator{}

// MemorySharesValidator is a custom validator to ensure memory_shares follows Kubernetes quantity format.
type MemorySharesValidator struct{}

// Description provides a description of the validator.
func (MemorySharesValidator) Description(_ context.Context) string {
	return "memory_shares must follow Kubernetes quantity format (e.g., '256Mi', '1Gi', '512M', '2G')"
}

// MarkdownDescription provides a description of the validator in markdown format.
func (MemorySharesValidator) MarkdownDescription(_ context.Context) string {
	return "memory_shares must follow Kubernetes quantity format (e.g., '256Mi', '1Gi', '512M', '2G')"
}

// ValidateString validates a string attribute.
func (MemorySharesValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	_, err := utils.ParseMemoryToBytes(value)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid memory shares format",
			fmt.Sprintf("Could not parse memory_shares value %q: %s. Use Kubernetes quantity format (e.g., '256Mi', '1Gi', '512M', '2G').", value, err),
		)
	}
}
