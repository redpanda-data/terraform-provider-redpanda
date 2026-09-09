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

package network

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func mustObject(t *testing.T, v tftypes.Type) tftypes.Object {
	t.Helper()
	obj, ok := v.(tftypes.Object)
	if !ok {
		t.Fatalf("type is %T, want tftypes.Object", v)
	}
	return obj
}

func nullsExcept(obj tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	vals := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, at := range obj.AttributeTypes {
		if v, ok := set[name]; ok {
			vals[name] = v
		} else {
			vals[name] = tftypes.NewValue(at, nil)
		}
	}
	return tftypes.NewValue(obj, vals)
}

// networkConfigWithCMR builds a config whose customer_managed_resources sets
// the named arm (every leaf null) beside the given cloud_provider and
// cluster_type; "" for a string leaves it null, "?" makes it unknown.
func networkConfigWithCMR(t *testing.T, arm, cloudProvider, clusterType string) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	s := ResourceNetworkSchema(ctx)
	objType := mustObject(t, s.Type().TerraformType(ctx))
	cmrType := mustObject(t, objType.AttributeTypes["customer_managed_resources"])
	armType := mustObject(t, cmrType.AttributeTypes[arm])
	set := map[string]tftypes.Value{
		"customer_managed_resources": nullsExcept(cmrType, map[string]tftypes.Value{arm: nullsExcept(armType, nil)}),
	}
	str := func(name, v string) {
		switch v {
		case "":
		case "?":
			set[name] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		default:
			set[name] = tftypes.NewValue(tftypes.String, v)
		}
	}
	str("cloud_provider", cloudProvider)
	str("cluster_type", clusterType)
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

// TestValidateConfigCMRArmMustMatchCloudProvider pins the plan-time rule on
// the network: the customer_managed_resources arm must be the configured
// cloud_provider on every provider, not only the GCP arm the control plane
// checks.
func TestValidateConfigCMRArmMustMatchCloudProvider(t *testing.T) {
	for _, arm := range []string{"aws", "gcp", "azure"} {
		for _, cp := range []string{"aws", "gcp", "azure"} {
			t.Run(arm+"-arm/"+cp, func(t *testing.T) {
				var resp resource.ValidateConfigResponse
				(&Network{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{
					Config: networkConfigWithCMR(t, arm, cp, "byoc"),
				}, &resp)
				got := hasSummary(&resp, "Customer Managed Resources Provider Mismatch")
				if want := arm != cp; got != want {
					t.Errorf("arm %s on cloud_provider %s: mismatch diagnostic = %v, want %v", arm, cp, got, want)
				}
			})
		}
	}
}

// TestValidateConfigCMRRequiresBYOC mirrors the control plane's
// validateCustomerManagedResources at plan time: a network with
// customer_managed_resources must be cluster_type "byoc".
func TestValidateConfigCMRRequiresBYOC(t *testing.T) {
	for _, ct := range []string{"byoc", "dedicated"} {
		t.Run(ct, func(t *testing.T) {
			var resp resource.ValidateConfigResponse
			(&Network{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{
				Config: networkConfigWithCMR(t, "azure", "azure", ct),
			}, &resp)
			got := hasSummary(&resp, "Customer Managed Resources Require BYOC")
			if want := ct != "byoc"; got != want {
				t.Errorf("cluster_type %s: byoc diagnostic = %v, want %v", ct, got, want)
			}
		})
	}
}

// TestValidateConfigCMRUnknownEnvelopeDefers pins that unknown cloud_provider
// and cluster_type raise neither rule.
func TestValidateConfigCMRUnknownEnvelopeDefers(t *testing.T) {
	var resp resource.ValidateConfigResponse
	(&Network{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{
		Config: networkConfigWithCMR(t, "azure", "?", "?"),
	}, &resp)
	if hasSummary(&resp, "Customer Managed Resources Provider Mismatch") || hasSummary(&resp, "Customer Managed Resources Require BYOC") {
		t.Errorf("unknown envelope raised a CMR rule: %v", resp.Diagnostics.Errors())
	}
}
