// Copyright 2026 Redpanda Data, Inc.
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

package serviceaccount

import (
	"context"

	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// ThreadCreateExtras lifts TF-only fields onto the CreateServiceAccountRequest
// payload after ExpandCreate has run. Plugged into the schemagen
// post_expand_hook so the same code path runs in both
// resource_serviceaccount.go::Create() and the generated proto-validator.
//
// role_bindings is `extra: true` in schema.yaml because the proto only has it
// on the Create payload (not on the canonical ServiceAccount). The generator
// omits it from ExpandCreate; this hook threads it through.
func ThreadCreateExtras(ctx context.Context, m *ResourceModel, req *iamv1.CreateServiceAccountRequest) diag.Diagnostics {
	bindings, diags := m.AsRoleBindings(ctx)
	if diags.HasError() || len(bindings) == 0 {
		return diags
	}
	out := make([]*iamv1.ServiceAccountCreate_RoleBinding, 0, len(bindings))
	for i := range bindings {
		b := &bindings[i]
		rb := &iamv1.ServiceAccountCreate_RoleBinding{
			RoleName: b.RoleName.ValueString(),
		}
		if sc, scDiags := DecodeRoleBindingsScope(ctx, b); scDiags.HasError() {
			diags.Append(scDiags...)
			return diags
		} else if sc != nil {
			scope := &iamv1.RoleBinding_Scope{
				ResourceType: iamv1.RoleBinding_ScopeResourceType(
					iamv1.RoleBinding_ScopeResourceType_value["SCOPE_RESOURCE_TYPE_"+sc.ResourceType.ValueString()],
				),
				ResourceId: sc.ResourceID.ValueString(),
			}
			if !sc.DataplaneID.IsNull() && !sc.DataplaneID.IsUnknown() {
				scope.SetDataplaneId(sc.DataplaneID.ValueString())
			}
			rb.Scope = scope
		}
		out = append(out, rb)
	}
	req.GetServiceAccount().SetRoleBindings(out)
	return diags
}
