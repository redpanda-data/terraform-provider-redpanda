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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// ThroughputTierValidator is a custom validator that ensures that the throughput tier is valid for the given cloud provider, region, and zones
type ThroughputTierValidator struct {
	ThroughputTierClient utils.ThroughputTierClient
	ClusterType          string
}

// Description provides a description of the validator
func (ThroughputTierValidator) Description(_ context.Context) string {
	return "ensures that the throughput tier is valid for the given cloud provider, region, and zones"
}

// MarkdownDescription provides a description of the validator in markdown format
func (ThroughputTierValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that the throughput tier is valid for the given cloud provider, region, and zones"
}

// ValidateString validates a string
func (t ThroughputTierValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	var cloudProvider, region types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("cloud_provider"), &cloudProvider)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("region"), &region)...)
	if resp.Diagnostics.HasError() {
		return
	}
	throughputTier := req.ConfigValue.ValueString()

	tiers, err := utils.GetThroughputTiers(ctx, t.ThroughputTierClient, cloudProvider.ValueString(), t.ClusterType, region.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "throughput tier api error", fmt.Sprintf("failed to get throughput tiers from api: %s", err))
	}

	for _, tier := range tiers {
		if tier.GetName() == throughputTier {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path,
		"invalid throughput tier",
		fmt.Sprintf("invalid throughput tier %s, please select a valid throughput tier", throughputTier))
}
