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

// Package cloudprovideraccess contains the implementation of the CloudProviderAccess
// resource following the Terraform framework interfaces.
package cloudprovideraccess

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	model "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cloudprovideraccess"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

var (
	_ resource.Resource                   = &CloudProviderAccess{}
	_ resource.ResourceWithConfigure      = &CloudProviderAccess{}
	_ resource.ResourceWithImportState    = &CloudProviderAccess{}
	_ resource.ResourceWithValidateConfig = &CloudProviderAccess{}
)

// CloudProviderAccess represents a cross-account cloud provider access managed resource.
type CloudProviderAccess struct {
	base.ResourceBase
}

// NewCloudProviderAccess constructs a CloudProviderAccess resource.
func NewCloudProviderAccess() *CloudProviderAccess {
	r := &CloudProviderAccess{}
	r.ResourceBase = base.NewResourceBase("redpanda_cloud_provider_access", ResourceCloudProviderAccessSchema, nil)
	return r
}

// ValidateConfig enforces the cloud_provider / config-block pairing that the
// schema alone cannot express: the aws block is required when cloud_provider
// is "aws", and aws is the only provider the API currently supports.
func (*CloudProviderAccess) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cloudProvider types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("cloud_provider"), &cloudProvider)...)
	var awsObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("aws"), &awsObj)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cloudProvider.IsUnknown() || cloudProvider.IsNull() || awsObj.IsUnknown() {
		return
	}
	if cloudProvider.ValueString() != enums.CloudProviderStringAws {
		resp.Diagnostics.AddAttributeError(path.Root("cloud_provider"),
			"unsupported cloud provider",
			fmt.Sprintf("cloud provider %q is not supported for cloud provider access; only %q is currently supported", cloudProvider.ValueString(), enums.CloudProviderStringAws))
		return
	}
	if awsObj.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("aws"),
			"missing aws configuration",
			"the aws block is required when cloud_provider is \"aws\"")
	}
}

// Create creates a new CloudProviderAccess resource.
func (r *CloudProviderAccess) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tflog.Info(ctx, "creating cloud provider access", map[string]any{"name": plan.Name.ValueString()})

	pbReq, diags := model.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.CpCl.CloudProviderAccess.CreateCloudProviderAccess(ctx, pbReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create cloud provider access", utils.DeserializeGrpcError(err))
		return
	}

	cpa := apiResp.GetCloudProviderAccess()
	if cpa == nil || cpa.GetId() == "" {
		resp.Diagnostics.AddError("failed to create cloud provider access", "API returned an empty cloud provider access; please report this issue to the provider developers")
		return
	}

	persist, diags := model.Flatten(ctx, cpa, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)

	// State is persisted above so a failed resource is tracked (and tainted)
	// rather than orphaned server-side.
	if cpa.GetState() == controlplanev1.CloudProviderAccess_STATE_FAILED {
		resp.Diagnostics.AddError("cloud provider access creation failed",
			fmt.Sprintf("cloud provider access %s is in %s state", cpa.GetId(), enums.CloudProviderAccessStateToString(cpa.GetState())))
		return
	}
	tflog.Info(ctx, "cloud provider access created", map[string]any{"id": cpa.GetId()})
}

// Read reads CloudProviderAccess resource's values and updates the state.
func (r *CloudProviderAccess) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cpa, err := r.CpCl.CloudProviderAccessForID(ctx, state.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read cloud provider access", utils.DeserializeGrpcError(err))
		return
	}

	if cpa.GetState() == controlplanev1.CloudProviderAccess_STATE_DELETED {
		resp.State.RemoveResource(ctx)
		return
	}

	tflog.Debug(ctx, "read cloud provider access", map[string]any{"id": cpa.GetId()})
	persist, diags := model.Flatten(ctx, cpa, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update is a no-op; CloudProviderAccess has no Update RPC. All mutable
// attributes use RequiresReplace, so only non-API changes (timeouts) land
// here, and the framework copies the plan into state automatically.
func (*CloudProviderAccess) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the CloudProviderAccess resource.
func (r *CloudProviderAccess) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := state.Timeouts.Delete(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tflog.Info(ctx, "deleting cloud provider access", map[string]any{"id": state.ID.ValueString()})

	pbReq, diags := model.ExpandDelete(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.CpCl.CloudProviderAccess.DeleteCloudProviderAccess(ctx, pbReq); err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete cloud provider access", utils.DeserializeGrpcError(err))
		return
	}
	tflog.Info(ctx, "cloud provider access deleted", map[string]any{"id": state.ID.ValueString()})
}

// ImportState imports the resource by ID; the subsequent Read populates the
// remaining attributes from the API.
func (*CloudProviderAccess) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
