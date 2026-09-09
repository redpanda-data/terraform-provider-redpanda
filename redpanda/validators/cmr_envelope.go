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

package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// CustomerManagedResourcesEnvelope raises at plan time the two envelope rules
// the control plane applies to customer_managed_resources on apply
// (validateCustomerManagedResources in cloudv2's network and redpanda
// services): the block requires cluster_type "byoc", and the populated arm
// must name cloud_provider. The control plane checks the arm only for GCP;
// an AWS or Azure arm on the wrong provider is stored as written and fails in
// the agent, so the provider checks every arm. Unknown values defer to apply.
// cluster and network share the attribute names, so one rule serves both.
func CustomerManagedResourcesEnvelope(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	var cmr types.Object
	diags.Append(cfg.GetAttribute(ctx, path.Root("customer_managed_resources"), &cmr)...)
	if diags.HasError() || cmr.IsNull() || cmr.IsUnknown() {
		return
	}
	var cloudProvider, clusterType types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("cloud_provider"), &cloudProvider)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("cluster_type"), &clusterType)...)
	if diags.HasError() {
		return
	}

	if !clusterType.IsNull() && !clusterType.IsUnknown() && clusterType.ValueString() != "byoc" {
		diags.AddAttributeError(path.Root("customer_managed_resources"),
			"Customer Managed Resources Require BYOC",
			fmt.Sprintf("customer_managed_resources is set but cluster_type is %q; only \"byoc\" uses customer-managed resources", clusterType.ValueString()))
	}

	if cloudProvider.IsNull() || cloudProvider.IsUnknown() {
		return
	}
	cp := cloudProvider.ValueString()
	for arm, v := range cmr.Attributes() {
		if v.IsNull() || v.IsUnknown() || arm == cp {
			continue
		}
		diags.AddAttributeError(path.Root("customer_managed_resources").AtName(arm),
			"Customer Managed Resources Provider Mismatch",
			fmt.Sprintf("customer_managed_resources.%s is set but cloud_provider is %q; set customer_managed_resources.%s instead", arm, cp, cp))
	}
}
