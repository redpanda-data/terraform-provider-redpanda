// Copyright 2025 Redpanda Data, Inc.
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
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_             validator.List = AWSZoneIDValidator{}
	zoneIDPattern                = regexp.MustCompile(`^[a-z]{2,5}\d-az\d$`)
)

// AWSZoneIDValidator is a custom validator to ensure that when cloud_provider is "aws",
// the zones attribute values follow the AWS zone ID format (e.g., use1-az1)
type AWSZoneIDValidator struct{}

// Description provides a description of the validator
func (AWSZoneIDValidator) Description(_ context.Context) string {
	return "ensures that when cloud_provider is aws, zones contain valid AWS zone IDs (format: use1-az1)"
}

// MarkdownDescription provides a description of the validator in markdown format
func (AWSZoneIDValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that when `cloud_provider` is `aws`, zones contain valid AWS zone IDs (format: `use1-az1`)"
}

// ValidateList validates a list attribute to ensure the elements follow AWS zone ID format when cloud_provider is aws
func (AWSZoneIDValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// Check if cloud_provider is aws
	var cloudProvider types.String
	if diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("cloud_provider"), &cloudProvider); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// If cloud_provider is not aws or unknown, skip validation
	if cloudProvider.IsUnknown() || cloudProvider.ValueString() != "aws" {
		return
	}

	// Get list elements
	var zones []types.String
	diags := req.ConfigValue.ElementsAs(ctx, &zones, false)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Validate each zone ID
	for i, zone := range zones {
		if zone.IsNull() || zone.IsUnknown() {
			continue
		}

		zoneValue := zone.ValueString()
		if !zoneIDPattern.MatchString(zoneValue) {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid AWS Zone ID",
				fmt.Sprintf(
					"Zone at index %d (%s) does not match the AWS zone ID format. AWS zone IDs must follow the pattern 'use1-az1'.",
					i, zoneValue,
				),
			)
		}
	}
}
