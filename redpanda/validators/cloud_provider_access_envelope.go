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

// CloudProviderAccessEnvelope raises at plan time the rules the control plane
// applies to a network's cloud_provider_access_id on create
// (resolveCloudProviderAccess in cloudv2's network service): it requires
// cluster_type "byoc" and cloud_provider "aws", and it excludes
// customer_managed_resources. Unknown values defer to apply.
func CloudProviderAccessEnvelope(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	var cpaID types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("cloud_provider_access_id"), &cpaID)...)
	if diags.HasError() || cpaID.IsNull() {
		return
	}
	var cmr types.Object
	var cloudProvider, clusterType types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("customer_managed_resources"), &cmr)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("cloud_provider"), &cloudProvider)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("cluster_type"), &clusterType)...)
	if diags.HasError() {
		return
	}

	attr := path.Root("cloud_provider_access_id")
	if !cmr.IsNull() && !cmr.IsUnknown() {
		diags.AddAttributeError(attr,
			"Conflicting Network Provisioning Modes",
			"cloud_provider_access_id and customer_managed_resources are mutually exclusive; with a cloud provider access Redpanda creates the network resources in your account")
	}
	if !clusterType.IsNull() && !clusterType.IsUnknown() && clusterType.ValueString() != "byoc" {
		diags.AddAttributeError(attr,
			"Cloud Provider Access Requires BYOC",
			fmt.Sprintf("cloud_provider_access_id is set but cluster_type is %q; only \"byoc\" networks use a cloud provider access", clusterType.ValueString()))
	}
	if !cloudProvider.IsNull() && !cloudProvider.IsUnknown() && cloudProvider.ValueString() != "aws" {
		diags.AddAttributeError(attr,
			"Cloud Provider Access Requires AWS",
			fmt.Sprintf("cloud_provider_access_id is set but cloud_provider is %q; only \"aws\" is supported", cloudProvider.ValueString()))
	}
}
