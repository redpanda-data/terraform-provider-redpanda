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

// Package network contains the implementation of the Network resource
// following the Terraform framework interfaces.
package network

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	networkmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &Network{}
	_ resource.ResourceWithConfigure   = &Network{}
	_ resource.ResourceWithImportState = &Network{}
)

// Network represents a network managed resource.
type Network struct {
	base.ResourceBase
}

// NewNetwork constructs a Network resource.
func NewNetwork() *Network {
	n := &Network{}
	n.ResourceBase = base.NewResourceBase("redpanda_network", ResourceNetworkSchema, nil)
	return n
}

// Create creates a new Network resource. It updates the state if the resource is successfully created.
func (n *Network) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan networkmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 15*time.Minute)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	req, expandDiags := networkmodel.ExpandCreate(ctx, &plan)
	response.Diagnostics.Append(expandDiags...)
	if response.Diagnostics.HasError() {
		return
	}

	netResp, err := n.CpCl.Network.CreateNetwork(ctx, req)
	if err != nil {
		response.Diagnostics.AddError("failed to create network", utils.DeserializeGrpcError(err))
		return
	}
	op := netResp.Operation
	// Write initial state so that if network creation fails, we can still track and delete it.
	plan.ID = utils.TrimmedStringValue(op.GetResourceId())
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)

	if err := utils.AreWeDoneYet(ctx, op, createTimeout, n.CpCl.Operation); err != nil {
		response.Diagnostics.AddError("failed waiting for network creation", utils.DeserializeGrpcError(err))
		return
	}

	nw, err := n.CpCl.NetworkForID(ctx, op.GetResourceId())
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", op.GetResourceId()), utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := networkmodel.Flatten(ctx, nw, &plan)
	response.Diagnostics.Append(flatDiags...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, state)...)
}

// Read reads Network resource's values and updates the state.
func (n *Network) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model networkmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	nw, err := n.CpCl.NetworkForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	if nw.GetState() == controlplanev1.Network_STATE_DELETING {
		// Null out the state, force destroy + recreate.
		response.State.RemoveResource(ctx)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), nw.Id)...)
		response.Diagnostics.AddWarning(fmt.Sprintf("network %s is in state %s", nw.Id, nw.GetState()), "")
		return
	}
	state, flatDiags := networkmodel.Flatten(ctx, nw, &model)
	response.Diagnostics.Append(flatDiags...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, state)...)
}

// Update is not supported for network. As a result all configurable schema elements have been marked as RequiresReplace.
func (*Network) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the Network resource.
func (n *Network) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model networkmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	deleteTimeout, diags := model.Timeouts.Delete(ctx, 15*time.Minute)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	delReq, expandDiags := networkmodel.ExpandDelete(ctx, &model)
	response.Diagnostics.Append(expandDiags...)
	if response.Diagnostics.HasError() {
		return
	}

	netResp, err := n.CpCl.Network.DeleteNetwork(ctx, delReq)
	if err != nil {
		response.Diagnostics.AddError("failed to delete network", utils.DeserializeGrpcError(err))
		return
	}
	if err := utils.AreWeDoneYet(ctx, netResp.Operation, deleteTimeout, n.CpCl.Operation); err != nil {
		response.Diagnostics.AddError("failed waiting for network deletion", utils.DeserializeGrpcError(err))
	}
}

// ImportState refreshes the state with the correct ID for the network, allowing
// TF to use Read to get the correct Network name into state. See
// https://developer.hashicorp.com/terraform/plugin/framework/resources/import
// for more details.
func (*Network) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
