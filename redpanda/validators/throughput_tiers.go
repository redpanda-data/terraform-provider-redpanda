package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ThroughputTierValidator struct{}

func (v ThroughputTierValidator) Description(ctx context.Context) string {
	return "ensures that the throughput tier is valid for the given cloud provider, region, and zones"
}

func (v ThroughputTierValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures that the throughput tier is valid for the given cloud provider, region, and zones"
}

func (v ThroughputTierValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	var zones types.List
	var cloudProvider, region types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("cloud_provider"), &cloudProvider)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("region"), &region)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("zones"), &zones)...)
	if resp.Diagnostics.HasError() {
		return
	}
	throughputTier := req.ConfigValue.ValueString()

	// Validate the throughput tier
	validTiers := make(map[string]bool)
	invalidZones := make([]string, 0)

	for _, zoneElement := range zones.Elements() {
		zone := zoneElement.(types.String).ValueString()
		if tiers, ok := validThroughputTiers[cloudProvider.ValueString()][region.ValueString()][zone]; ok {
			for _, tier := range tiers {
				validTiers[tier] = true
			}
		} else {
			invalidZones = append(invalidZones, zone)
		}
	}

	if len(invalidZones) > 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Zones",
			fmt.Sprintf("The following zones are not valid for the given cloud provider (%s) and region (%s): %v",
				cloudProvider.ValueString(), region.ValueString(), invalidZones),
		)
		return
	}

	if _, isValid := validTiers[throughputTier]; !isValid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Throughput Tier",
			fmt.Sprintf("The throughput tier %s is not valid for the given cloud provider (%s), region (%s), and zones. Valid tiers are: %v",
				throughputTier, cloudProvider.ValueString(), region.ValueString(), getKeys(validTiers)),
		)
	}
}

// Helper function to get keys from a map
func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

var validThroughputTiers = map[string]map[string]map[string][]string{
	"aws": {
		"ap-south-1": {
			"aps1-az1": {"tier-1-aws-v2-x86", "tier-2-aws-v2-x86", "tier-3-aws-v2-x86", "tier-4-aws-v2-x86", "tier-5-aws-v2-x86"},
			"aps1-az2": {"tier-1-aws-v2-x86", "tier-2-aws-v2-x86", "tier-3-aws-v2-x86", "tier-4-aws-v2-x86", "tier-5-aws-v2-x86"},
			"aps1-az3": {"tier-1-aws-v2-x86", "tier-2-aws-v2-x86", "tier-3-aws-v2-x86", "tier-4-aws-v2-x86", "tier-5-aws-v2-x86"},
		},
		"ap-southeast-1": {
			"apse1-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"apse1-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"apse1-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"ap-southeast-2": {
			"apse2-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"apse2-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"apse2-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"ca-central-1": {
			"cac1-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"cac1-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"cac1-az4": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"eu-central-1": {
			"euc1-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"euc1-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"euc1-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"eu-west-1": {
			"euw1-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"euw1-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"euw1-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"eu-west-2": {
			"euw2-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"euw2-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"euw2-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"us-east-1": {
			"use1-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"use1-az4": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"use1-az6": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"us-east-2": {
			"use2-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"use2-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"use2-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
		"us-west-2": {
			"usw2-az1": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"usw2-az2": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
			"usw2-az3": {"tier-1-aws-v2-arm", "tier-2-aws-v2-arm", "tier-3-aws-v2-arm", "tier-4-aws-v2-arm", "tier-5-aws-v2-arm"},
		},
	},
	"gcp": {
		"asia-south1": {
			"asia-south1-a": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"asia-south1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"asia-south1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"asia-southeast1": {
			"asia-southeast1-a": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"asia-southeast1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"asia-southeast1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"australia-southeast1": {
			"australia-southeast1-a": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"australia-southeast1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"australia-southeast1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"europe-west1": {
			"europe-west1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"europe-west1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"europe-west1-d": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"europe-west2": {
			"europe-west2-a": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"europe-west2-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"europe-west2-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"northamerica-northeast1": {
			"northamerica-northeast1-a": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"northamerica-northeast1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"northamerica-northeast1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"us-central1": {
			"us-central1-a": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"us-central1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"us-central1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"us-central1-f": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
		"us-east1": {
			"us-east1-b": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"us-east1-c": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
			"us-east1-d": {"tier-1-gcp-v2-x86", "tier-2-gcp-v2-x86", "tier-3-gcp-v2-x86", "tier-4-gcp-v2-x86", "tier-5-gcp-v2-x86"},
		},
	},
	"azure": {
		"eastus": {
			"eastus-az1": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
			"eastus-az2": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
			"eastus-az3": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
		},
		"ukwest": {
			"ukwest-az1": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
			"ukwest-az2": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
			"ukwest-az3": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
		},
		"uksouth": {
			"uksouth-az1": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
			"uksouth-az2": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
			"uksouth-az3": {"tier-1-azure", "tier-2-azure", "tier-3-azure", "tier-4-azure", "tier-5-azure", "tier-6-azure", "tier-7-azure", "tier-8-azure", "tier-9-azure"},
		},
	},
}
