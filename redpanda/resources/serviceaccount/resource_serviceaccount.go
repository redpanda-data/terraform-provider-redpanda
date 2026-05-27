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

package serviceaccount

import (
	"context"
	"strings"

	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	serviceaccountmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serviceaccount"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &ServiceAccount{}
	_ resource.ResourceWithConfigure   = &ServiceAccount{}
	_ resource.ResourceWithImportState = &ServiceAccount{}
)

// ServiceAccount represents the Redpanda Cloud service-account resource.
type ServiceAccount struct {
	base.ResourceBase
}

// NewServiceAccount constructs a ServiceAccount resource.
func NewServiceAccount() *ServiceAccount {
	r := &ServiceAccount{}
	r.ResourceBase = base.NewResourceBase("redpanda_service_account", ResourceServiceAccountSchema, nil)
	return r
}

// Create creates a new ServiceAccount.
func (s *ServiceAccount) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceaccountmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pbReq, diags := serviceaccountmodel.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := s.CpCl.ServiceAccount.CreateServiceAccount(ctx, pbReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create service account", utils.DeserializeGrpcError(err))
		return
	}
	sa := apiResp.GetServiceAccount()
	if sa == nil {
		resp.Diagnostics.AddError("failed to create service account", "API response did not contain a service account; please report this bug to Redpanda Support")
		return
	}

	state, flatDiags := serviceaccountmodel.Flatten(ctx, sa, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read refreshes ServiceAccount state.
func (s *ServiceAccount) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceaccountmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := s.CpCl.ServiceAccountForID(ctx, state.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read service account", utils.DeserializeGrpcError(err))
		return
	}

	newState, flatDiags := serviceaccountmodel.Flatten(ctx, sa, &state)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Update applies name/description changes via FieldMask.
func (s *ServiceAccount) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceaccountmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state serviceaccountmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planPayload, eDiags := serviceaccountmodel.ExpandUpdate(ctx, &plan)
	resp.Diagnostics.Append(eDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	statePayload, eDiags := serviceaccountmodel.ExpandUpdate(ctx, &state)
	resp.Diagnostics.Append(eDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, mask := utils.PlanPayloadWithUpdateMask(planPayload, statePayload)
	updateReq := &iamv1.UpdateServiceAccountRequest{
		Id:             plan.ID.ValueString(),
		ServiceAccount: payload,
		UpdateMask:     mask,
	}

	var sa *iamv1.ServiceAccount
	if len(mask.GetPaths()) != 0 {
		apiResp, err := s.CpCl.ServiceAccount.UpdateServiceAccount(ctx, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("failed to update service account", utils.DeserializeGrpcError(err))
			return
		}
		sa = apiResp.GetServiceAccount()
	} else {
		fetched, err := s.CpCl.ServiceAccountForID(ctx, plan.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("failed to refresh service account", utils.DeserializeGrpcError(err))
			return
		}
		sa = fetched
	}
	if sa == nil {
		resp.Diagnostics.AddError("failed to update service account", "API response did not contain a service account; please report this bug to Redpanda Support")
		return
	}

	newState, flatDiags := serviceaccountmodel.Flatten(ctx, sa, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Delete deletes the ServiceAccount.
func (s *ServiceAccount) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceaccountmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pbReq, diags := serviceaccountmodel.ExpandDelete(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := s.CpCl.ServiceAccount.DeleteServiceAccount(ctx, pbReq); err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete service account", utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState imports a ServiceAccount via "<id>:<client_secret>". client_secret
// is server-issued exactly once on Create and never echoed by subsequent reads,
// so an import without it would leave state with a null secret — silently
// breaking any downstream output that consumes it. The composite ID requires
// the operator to supply the secret they captured at creation time; Read fills
// in name, description, and client_id from the server.
func (*ServiceAccount) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, secret, ok := strings.Cut(req.ID, ":")
	if !ok || id == "" || secret == "" {
		resp.Diagnostics.AddError(
			"invalid import ID",
			"redpanda_service_account import requires the format \"<id>:<client_secret>\". client_secret is returned only on creation and cannot be recovered from the server.",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	creds := &serviceaccountmodel.Auth0ClientCredentialsModel{
		ClientID:     types.StringNull(),
		ClientSecret: types.StringValue(secret),
	}
	obj, oDiags := serviceaccountmodel.Auth0ClientCredentialsToObject(ctx, creds)
	resp.Diagnostics.Append(oDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("auth0_client_credentials"), obj)...)
}
