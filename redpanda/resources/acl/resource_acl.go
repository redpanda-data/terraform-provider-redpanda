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

package acl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	aclmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/acl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

// Per-RPC retry budget for dataplane calls (e.g., the freshly-provisioned-cluster
// DNS-propagation window). Sized to fit ~5 attempts under Retry's 1s→60s cap.
const dataplaneRetryTimeout = 2 * time.Minute

// ACL represents the ACL Terraform resource.
type ACL struct {
	base.ResourceBase

	ACLClient dataplanev1grpc.ACLServiceClient

	resData config.Resource
}

// NewACL constructs an ACL resource.
func NewACL() *ACL {
	a := &ACL{}
	a.ResourceBase = base.NewResourceBase(
		"redpanda_acl",
		ResourceACLSchema,
		func(p config.Resource) { a.resData = p },
	)
	return a
}

var (
	_ resource.Resource                 = &ACL{}
	_ resource.ResourceWithConfigure    = &ACL{}
	_ resource.ResourceWithImportState  = &ACL{}
	_ resource.ResourceWithUpgradeState = &ACL{}
)

// UpgradeState migrates v0 state to v1, normalizing the legacy host:443
// cluster_api_url form to the canonical https://host the control plane now
// returns so the RequiresReplace plan modifier does not fire on the format
// change alone.
func (*ACL) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	prior := ResourceACLSchema(ctx)
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &prior,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var model aclmodel.ResourceModel
				resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if !model.ClusterAPIURL.IsNull() && !model.ClusterAPIURL.IsUnknown() {
					model.ClusterAPIURL = types.StringValue(utils.NormalizeClusterAPIURL(model.ClusterAPIURL.ValueString()))
				}
				resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
			},
		},
	}
}

// Create creates a new ACL resource. CreateACL has no useful response payload,
// so the model is persisted from plan input with the ID computed via
// GenerateID.
func (a *ACL) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model aclmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	if err := a.createACLClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		response.Diagnostics.AddError("failed to create ACL client", utils.DeserializeGrpcError(err))
		return
	}

	pbReq, diags := aclmodel.ExpandCreate(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	listFilter := buildACLFilter(&model)
	probeACLExists := func() bool {
		listResp, listErr := a.ACLClient.ListACLs(ctx, &dataplanev1.ListACLsRequest{Filter: listFilter})
		if listErr != nil {
			return false
		}
		for _, res := range listResp.GetResources() {
			if res.GetResourceName() == model.ResourceName.ValueString() &&
				res.GetResourceType() == pbReq.GetResourceType() &&
				res.GetResourcePatternType() == pbReq.GetResourcePatternType() {
				return true
			}
		}
		return false
	}

	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		_, rpcErr := a.ACLClient.CreateACL(ctx, pbReq)
		if rpcErr == nil {
			return nil
		}
		// Adopt the existing ACL on AlreadyExists from a prior retry's lost response.
		if utils.IsAlreadyExists(rpcErr) {
			if probeACLExists() {
				return nil
			}
			return utils.NonRetryableError(rpcErr)
		}
		// Probe before retrying so the next attempt doesn't trip AlreadyExists.
		if utils.IsUnavailable(rpcErr) {
			if probeACLExists() {
				return nil
			}
			return utils.RetryableError(rpcErr)
		}
		return utils.NonRetryableError(rpcErr)
	})
	if err != nil {
		response.Diagnostics.AddError("Failed to create ACL", utils.DeserializeGrpcError(err))
		return
	}

	model.ID = types.StringValue(model.GenerateID())
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

// Read confirms the ACL exists in the cluster by listing matching records
// against a filter built from the model. The matched ListACLsResponse_Resource
// is fed to the generated Flatten, which keeps user-supplied identifying
// fields (principal, host, operation, permission_type) preserved from prev
// rather than echoed from the API.
func (a *ACL) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model aclmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	if model.ClusterAPIURL.IsNull() || model.ClusterAPIURL.IsUnknown() || model.ClusterAPIURL.ValueString() == "" {
		response.State.RemoveResource(ctx)
		return
	}

	if err := a.createACLClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "ACL", model.GenerateID(), model.AllowDeletion, err, "create ACL client")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	filter := buildACLFilter(&model)
	var aclList *dataplanev1.ListACLsResponse
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		var rpcErr error
		aclList, rpcErr = a.ACLClient.ListACLs(ctx, &dataplanev1.ListACLsRequest{Filter: filter})
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "ACL", model.GenerateID(), model.AllowDeletion, err, "list ACLs")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	wantType := enums.StringToACLResourceType(model.ResourceType.ValueString())
	wantPattern := enums.StringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	for _, res := range aclList.Resources {
		if res.ResourceName != model.ResourceName.ValueString() ||
			res.ResourceType != wantType ||
			res.ResourcePatternType != wantPattern {
			continue
		}
		persist, diags := aclmodel.Flatten(ctx, res, &model)
		response.Diagnostics.Append(diags...)
		if response.Diagnostics.HasError() {
			return
		}
		response.Diagnostics.Append(response.State.Set(ctx, persist)...)
		return
	}

	// ACL not found
	action, diags := utils.HandleGracefulRemoval(ctx, "ACL", model.GenerateID(), model.AllowDeletion, utils.NotFoundError{Message: "ACL not found in cluster"}, "find ACL")
	response.Diagnostics.Append(diags...)
	if action == utils.RemoveFromState {
		response.State.RemoveResource(ctx)
	}
}

// Update is a no-op: ACL fields are all RequiresReplace except for
// allow_deletion, which is reflected back into state from plan.
func (*ACL) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan, state aclmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	state.AllowDeletion = plan.AllowDeletion
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

// Delete deletes an ACL resource via the matching filter.
func (a *ACL) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model aclmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	aclID := model.GenerateID()

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		response.Diagnostics.AddError(
			"Cannot delete ACL",
			fmt.Sprintf("Deletion of ACL for principal %s on resource %s is not allowed. Set allow_deletion=true to allow deletion of this resource.", model.Principal.ValueString(), model.ResourceName.ValueString()),
		)
		return
	}

	if err := a.createACLClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "ACL", aclID, model.AllowDeletion, err, "create ACL client")
		response.Diagnostics.Append(diags...)
		return
	}

	pbReq, diags := aclmodel.ExpandDelete(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	var deleteResponse *dataplanev1.DeleteACLsResponse
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		var rpcErr error
		deleteResponse, rpcErr = a.ACLClient.DeleteACLs(ctx, pbReq)
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		_, ddiags := utils.HandleGracefulRemoval(ctx, "ACL", aclID, model.AllowDeletion, err, "delete ACL")
		response.Diagnostics.Append(ddiags...)
		return
	}

	for _, matchingACL := range deleteResponse.MatchingAcls {
		if matchingACL.Error != nil && matchingACL.Error.Code != 0 {
			response.Diagnostics.AddError("Error deleting ACL", matchingACL.Error.Message)
			return
		}
	}
}

// buildACLFilter constructs a ListACLsRequest_Filter populated from the
// model, mirroring the same field-shape the generated ExpandDelete builds
// for DeleteACLsRequest_Filter (separate proto types, same semantics).
func buildACLFilter(m *aclmodel.ResourceModel) *dataplanev1.ListACLsRequest_Filter {
	return &dataplanev1.ListACLsRequest_Filter{
		ResourceType:        enums.StringToACLResourceType(m.ResourceType.ValueString()),
		ResourceName:        utils.PointerOrNil(m.ResourceName, types.String.ValueString),
		ResourcePatternType: enums.StringToACLResourcePatternType(m.ResourcePatternType.ValueString()),
		Principal:           utils.PointerOrNil(m.Principal, types.String.ValueString),
		Host:                utils.PointerOrNil(m.Host, types.String.ValueString),
		Operation:           enums.StringToACLOperation(m.Operation.ValueString()),
		PermissionType:      enums.StringToACLPermissionType(m.PermissionType.ValueString()),
	}
}

func (a *ACL) createACLClient(ctx context.Context, clusterURL string) error {
	if a.ACLClient != nil {
		return nil
	}
	if a.resData.DataplaneConnPool == nil {
		return errors.New("provider not configured: dataplane connection pool is nil")
	}
	conn, err := a.resData.DataplaneConnPool.GetConnection(ctx, clusterURL)
	if err != nil {
		return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
	}
	a.ACLClient = dataplanev1grpc.NewACLServiceClient(conn)
	return nil
}

// ImportState imports an ACL identified by the comma-separated tuple
// <cluster_id>,<resource_type>,<resource_name>,<resource_pattern_type>,<principal>,<host>,<operation>,<permission_type>.
// The cluster_id is resolved to a dataplane URL via the control-plane
// client, then written into cluster_api_url; the rest seed the seven ACL
// identity attrs. allow_deletion + id are left for the next Read to
// populate (id is computed from the identity tuple).
func (a *ACL) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	const expectedFields = 8
	parts := strings.Split(req.ID, ",")
	if len(parts) != expectedFields {
		resp.Diagnostics.AddError(
			fmt.Sprintf("wrong ACL import ID format: %q", req.ID),
			"format is <cluster_id>,<resource_type>,<resource_name>,<resource_pattern_type>,<principal>,<host>,<operation>,<permission_type>",
		)
		return
	}
	clusterID := parts[0]

	cp := cloud.NewControlPlaneClientSet(a.resData.ControlPlaneConnection)
	var dataplaneURL string
	if cl, err := cp.ClusterForID(ctx, clusterID); err == nil && cl != nil {
		dataplaneURL = cl.DataplaneApi.Url
	} else if sl, serr := cp.ServerlessClusterForID(ctx, clusterID); serr == nil && sl != nil {
		dataplaneURL = sl.DataplaneApi.Url
	} else {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to resolve dataplane URL for cluster %q", clusterID),
			utils.DeserializeGrpcError(err)+utils.DeserializeGrpcError(serr),
		)
		return
	}

	for _, f := range []struct {
		path  path.Path
		value string
	}{
		{path.Root("cluster_api_url"), dataplaneURL},
		{path.Root("resource_type"), parts[1]},
		{path.Root("resource_name"), parts[2]},
		{path.Root("resource_pattern_type"), parts[3]},
		{path.Root("principal"), parts[4]},
		{path.Root("host"), parts[5]},
		{path.Root("operation"), parts[6]},
		{path.Root("permission_type"), parts[7]},
	} {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, f.path, types.StringValue(f.value))...)
	}
	// Compute id from the identity tuple so ImportStateVerify matches the
	// post-Create id (which uses the same GenerateID() shape).
	id := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		parts[1], parts[2], parts[3], parts[4], parts[5], parts[6], parts[7])
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(id))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)
}
