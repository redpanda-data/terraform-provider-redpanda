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
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// publicSubnetsWriteOnce is the customer_managed_resources.aws.public_subnets
// plan modifier referenced by the generated schema. The control plane lets the
// field be set on a network that has none, then refuses to change or clear
// it, comparing ARNs as a set. So: unset in state leaves the plan alone (the
// update path adds it in place); a removed block or the same set in another
// order keeps the state value so the plan is empty; a different set forces
// replacement like the sibling customer-managed leaves.
func publicSubnetsWriteOnce() planmodifier.Object {
	return publicSubnetsWriteOnceModifier{}
}

type publicSubnetsWriteOnceModifier struct{}

func (publicSubnetsWriteOnceModifier) Description(_ context.Context) string {
	return "Public subnets can be added to an existing network but not changed or removed; a different set forces replacement."
}

func (m publicSubnetsWriteOnceModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyObject implements planmodifier.Object.
func (publicSubnetsWriteOnceModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.State.Raw.IsNull() || req.StateValue.IsNull() {
		return
	}
	if req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
		resp.PlanValue = req.StateValue
		return
	}
	planARNs, planKnown, diags := subnetARNs(ctx, req.PlanValue)
	resp.Diagnostics.Append(diags...)
	stateARNs, stateKnown, diags := subnetARNs(ctx, req.StateValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !planKnown || !stateKnown {
		return
	}
	if sameARNSet(planARNs, stateARNs) {
		resp.PlanValue = req.StateValue
		return
	}
	resp.RequiresReplace = true
}

func subnetARNs(ctx context.Context, obj types.Object) (arns []string, known bool, diags diag.Diagnostics) {
	list, ok := obj.Attributes()["arns"].(types.List)
	if !ok || list.IsUnknown() || list.IsNull() {
		return nil, false, nil
	}
	diags = list.ElementsAs(ctx, &arns, false)
	return arns, !diags.HasError(), diags
}

func sameARNSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
