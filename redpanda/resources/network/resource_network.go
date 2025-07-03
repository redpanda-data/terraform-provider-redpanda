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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &Network{}
	_ resource.ResourceWithConfigure   = &Network{}
	_ resource.ResourceWithImportState = &Network{}
)

// Network represents a network managed resource.
type Network struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the full name of the Network resource.
func (*Network) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_network"
}

// Configure uses provider level data to configure Network's clients.
func (n *Network) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		// we can't add a diagnostic for an unset ProviderData here because
		// during the early part of the terraform lifecycle, the provider data
		// is not set and this is valid, but we also can't do anything until it
		// is set
		return
	}

	p, ok := request.ProviderData.(config.Resource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	n.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the Network resource.
func (*Network) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceNetworkSchema()
}

// Create creates a new Network resource. It updates the state if the resource is successfully created.
func (n *Network) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model network.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)
	m, d := model.GetNetworkCreate(ctx)
	if d.HasError() {
		d.AddError("error creating network", "error getting proto from network model")
	}

	netResp, err := n.CpCl.Network.CreateNetwork(ctx, &controlplanev1.CreateNetworkRequest{
		Network: m,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create network", utils.DeserializeGrpcError(err))
		return
	}
	op := netResp.Operation
	// write initial state so that if network creation fails, we can still track and delete it
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), utils.TrimmedStringValue(op.GetResourceId()))...)

	if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, n.CpCl.Operation); err != nil {
		response.Diagnostics.AddError("failed waiting for network creation", utils.DeserializeGrpcError(err))
		return
	}

	nw, err := n.CpCl.NetworkForID(ctx, op.GetResourceId())
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", op.GetResourceId()), utils.DeserializeGrpcError(err))
		return
	}
	toPersist, dgs := model.GetUpdatedModel(ctx, nw)
	if dgs.HasError() {
		response.Diagnostics = append(response.Diagnostics, dgs...)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, toPersist)...)
}

// Read reads Network resource's values and updates the state.
func (n *Network) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model network.ResourceModel
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
		// null out the state, force it to be destroyed and recreated
		response.State.RemoveResource(ctx)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), nw.Id)...)
		response.Diagnostics.AddWarning(fmt.Sprintf("network %s is in state %s", nw.Id, nw.GetState()), "")
		return
	}
	m, d := model.GetUpdatedModel(ctx, nw)
	if d.HasError() {
		response.Diagnostics = append(response.Diagnostics, d...)
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, m)...)
}

// Update is not supported for network. As a result all configurable schema elements have been marked as RequiresReplace.
func (*Network) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the Network resource.
func (n *Network) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model network.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	netResp, err := n.CpCl.Network.DeleteNetwork(ctx, &controlplanev1.DeleteNetworkRequest{
		Id: model.GetID(),
	})
	if err != nil {
		response.Diagnostics.AddError("failed to delete network", utils.DeserializeGrpcError(err))
		return
	}
	if err := utils.AreWeDoneYet(ctx, netResp.Operation, 15*time.Minute, n.CpCl.Operation); err != nil {
		response.Diagnostics.AddError("failed waiting for network deletion", utils.DeserializeGrpcError(err))
	}
}

// ImportState refreshes the state with the correct ID for the network, allowing
// TF to use Read to get the correct Network name into state see
// https://developer.hashicorp.com/terraform/plugin/framework/resources/import
// for more details.
func (*Network) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
