// Copyright 2026 Redpanda Data, Inc.
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

package cluster

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// clusterConfigWithCMR builds a config whose customer_managed_resources sets
// the named arm (every leaf null) beside the given cloud_provider and
// cluster_type; "" for a string leaves it null.
func clusterConfigWithCMR(t *testing.T, arm, cloudProvider, clusterType string) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	objType := mustTFType[tftypes.Object](t, s.Type().TerraformType(ctx))
	cmrType := mustTFType[tftypes.Object](t, objType.AttributeTypes["customer_managed_resources"])
	armType := mustTFType[tftypes.Object](t, cmrType.AttributeTypes[arm])
	set := map[string]tftypes.Value{
		"customer_managed_resources": nullsExcept(cmrType, map[string]tftypes.Value{
			arm: nullsExcept(armType, nil),
		}),
	}
	if cloudProvider != "" {
		set["cloud_provider"] = tftypes.NewValue(tftypes.String, cloudProvider)
	}
	if clusterType != "" {
		set["cluster_type"] = tftypes.NewValue(tftypes.String, clusterType)
	}
	return tfsdk.Config{Schema: s, Raw: nullsExcept(objType, set)}
}

func hasSummary(resp *resource.ValidateConfigResponse, want string) bool {
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == want {
			return true
		}
	}
	return false
}

// TestValidateConfigCMRArmMustMatchCloudProvider pins the plan-time rule the
// control plane only enforces for one arm: the customer_managed_resources arm
// must be the configured cloud_provider, on every provider.
func TestValidateConfigCMRArmMustMatchCloudProvider(t *testing.T) {
	for _, arm := range []string{"aws", "gcp", "azure"} {
		for _, cp := range []string{"aws", "gcp", "azure"} {
			t.Run(arm+"-arm/"+cp, func(t *testing.T) {
				var resp resource.ValidateConfigResponse
				(&Cluster{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{
					Config: clusterConfigWithCMR(t, arm, cp, "byoc"),
				}, &resp)
				got := hasSummary(&resp, "Customer Managed Resources Provider Mismatch")
				if want := arm != cp; got != want {
					t.Errorf("arm %s on cloud_provider %s: mismatch diagnostic = %v, want %v", arm, cp, got, want)
				}
			})
		}
	}
}

// TestValidateConfigCMRRequiresBYOC pins that customer_managed_resources on a
// non-BYOC cluster fails at plan; the control plane rejects it only at apply.
func TestValidateConfigCMRRequiresBYOC(t *testing.T) {
	for _, ct := range []string{"byoc", "dedicated"} {
		t.Run(ct, func(t *testing.T) {
			var resp resource.ValidateConfigResponse
			(&Cluster{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{
				Config: clusterConfigWithCMR(t, "azure", "azure", ct),
			}, &resp)
			got := hasSummary(&resp, "Customer Managed Resources Require BYOC")
			if want := ct != "byoc"; got != want {
				t.Errorf("cluster_type %s: byoc diagnostic = %v, want %v", ct, got, want)
			}
		})
	}
}

// TestValidateConfigCMRUnknownEnvelopeDefers pins that an unknown
// cloud_provider or cluster_type (a value resolved from another resource at
// apply) raises neither rule; a false positive here would block every
// module-fed config.
func TestValidateConfigCMRUnknownEnvelopeDefers(t *testing.T) {
	ctx := context.Background()
	s := ResourceClusterSchema(ctx)
	objType := mustTFType[tftypes.Object](t, s.Type().TerraformType(ctx))
	cmrType := mustTFType[tftypes.Object](t, objType.AttributeTypes["customer_managed_resources"])
	armType := mustTFType[tftypes.Object](t, cmrType.AttributeTypes["azure"])
	cfg := tfsdk.Config{Schema: s, Raw: nullsExcept(objType, map[string]tftypes.Value{
		"customer_managed_resources": nullsExcept(cmrType, map[string]tftypes.Value{"azure": nullsExcept(armType, nil)}),
		"cloud_provider":             tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"cluster_type":               tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})}
	var resp resource.ValidateConfigResponse
	(&Cluster{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: cfg}, &resp)
	if hasSummary(&resp, "Customer Managed Resources Provider Mismatch") || hasSummary(&resp, "Customer Managed Resources Require BYOC") {
		t.Errorf("unknown envelope raised a CMR rule: %v", resp.Diagnostics.Errors())
	}
}
