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

var _ validator.String = CPUSharesValidator{}

// CPUSharesValidator is a custom validator to ensure cpu_shares is a multiple of 100m.
type CPUSharesValidator struct{}

// Description provides a description of the validator.
func (CPUSharesValidator) Description(_ context.Context) string {
	return "cpu_shares must be a multiple of 100m (e.g., '100m', '200m', '1' for 1 core)"
}

// MarkdownDescription provides a description of the validator in markdown format.
func (CPUSharesValidator) MarkdownDescription(_ context.Context) string {
	return "cpu_shares must be a multiple of 100m (e.g., '100m', '200m', '1' for 1 core)"
}

// ValidateString validates a string attribute.
func (CPUSharesValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	millicores, err := utils.ParseCPUToMillicores(value)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid CPU shares format",
			fmt.Sprintf("Could not parse cpu_shares value %q: %s. Use Kubernetes quantity format (e.g., '100m', '500m', '1').", value, err),
		)
		return
	}

	if millicores%100 != 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid CPU shares value",
			fmt.Sprintf("cpu_shares must be a multiple of 100m, got %q (%d millicores). Valid examples: '100m', '200m', '500m', '1', '2'.", value, millicores),
		)
		return
	}

	// Minimum 100m (1 compute unit) per Redpanda Cloud documentation
	const minCPUMillicores = 100
	if millicores < minCPUMillicores {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"CPU below minimum",
			fmt.Sprintf("cpu_shares must be at least 100m (1 compute unit), got %q (%dm).", value, millicores),
		)
	}
}
