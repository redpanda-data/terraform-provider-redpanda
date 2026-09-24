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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithModifyPlan = &Network{}

// ModifyPlan warns when a plan adds a cloud provider access to an existing
// network or removes it. The control plane refuses both on UpdateNetwork, so
// unless the apply replaces the network, it fails and leaves the network
// unchanged. It is a warning rather than an error because a replacement makes
// the same plan valid, and a resource-level ModifyPlan cannot see one: the
// framework hands it an empty RequiresReplace, and terraform apply -replace
// or a taint never reaches the provider.
func (*Network) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	attr := path.Root("cloud_provider_access_id")
	var prior, planned types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, attr, &prior)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, attr, &planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	switch {
	case prior.IsNull() && !planned.IsNull():
		resp.Diagnostics.AddAttributeWarning(attr, "Cloud Provider Access Cannot Be Added In Place",
			"This network was created without a cloud provider access, and Redpanda Cloud refuses to add one to an existing network. Unless this apply replaces the network, because another attribute forces it or with terraform apply -replace, it will fail and leave the network unchanged.")
	case !prior.IsNull() && planned.IsNull():
		resp.Diagnostics.AddAttributeWarning(attr, "Cloud Provider Access Cannot Be Removed In Place",
			"Redpanda Cloud refuses to remove the cloud provider access from a network. Unless this apply replaces the network, it will fail and leave the network unchanged; to rotate the role, re-point it at another cloud provider access instead.")
	default:
	}
}
