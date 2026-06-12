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

// Package cluster contains the implementation of the Cluster resource
// following the Terraform framework interfaces.
package cluster

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/clustermask"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &Cluster{}
	_ resource.ResourceWithConfigure   = &Cluster{}
	_ resource.ResourceWithImportState = &Cluster{}
	_ resource.ResourceWithModifyPlan  = &Cluster{}
)

// Cluster represents a cluster managed resource
type Cluster struct {
	base.ResourceBase
	Byoc *utils.ByocClient
}

// NewCluster constructs a Cluster resource.
func NewCluster() *Cluster {
	c := &Cluster{}
	c.ResourceBase = base.NewResourceBase(
		"redpanda_cluster",
		ResourceClusterSchema,
		func(p config.Resource) { c.Byoc = p.ByocClient },
	)
	return c
}

// Create creates a new Cluster resource. It updates the state if the resource is successfully created
func (c *Cluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clustermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, expandDiags := clustermodel.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clResp, err := c.CpCl.Cluster.CreateCluster(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster", utils.DeserializeGrpcError(err))
		return
	}
	op := clResp.Operation
	clusterID := op.GetResourceId()
	tflog.Info(ctx, "creating cluster", map[string]any{"cluster_id": clusterID})

	createTimeout, diags := plan.Timeouts.Create(ctx, 90*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ranByoc bool
	cl, err := utils.RetryGetCluster(ctx, createTimeout, clusterID, c.CpCl, func(cl *controlplanev1.Cluster) *utils.RetryError {
		switch cl.GetState() {
		case controlplanev1.Cluster_STATE_CREATING:
			return utils.RetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cl.GetState()))
		case controlplanev1.Cluster_STATE_CREATING_AGENT:
			if cl.Type == controlplanev1.Cluster_TYPE_BYOC && !ranByoc {
				err = c.Byoc.RunByoc(ctx, clusterID, "apply")
				if err != nil {
					if utils.IsRetryableByocError(err) {
						tflog.Debug(ctx, fmt.Sprintf("Retryable byoc error during apply: %v", err))
						return utils.RetryableError(err)
					}
					return utils.NonRetryableError(err)
				}
				ranByoc = true
			}
			return utils.RetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cl.GetState()))
		case controlplanev1.Cluster_STATE_READY:
			return nil
		case controlplanev1.Cluster_STATE_FAILED:
			return utils.NonRetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cl.GetState()))
		case controlplanev1.Cluster_STATE_DELETING, controlplanev1.Cluster_STATE_DELETING_AGENT:
			return utils.NonRetryableError(fmt.Errorf("cluster is being deleted (state %v), cannot complete creation", cl.GetState()))
		default:
			return utils.NonRetryableError(fmt.Errorf("unhandled state %v. please report this issue to the provider developers", cl.GetState()))
		}
	})
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, clustermodel.GenerateMinimalResourceModel(types.StringValue(clusterID), plan.Timeouts))...)
		resp.Diagnostics.AddError(fmt.Sprintf("failed to create cluster with ID %q", clusterID), utils.DeserializeGrpcError(err))
		return
	}

	if cl != nil {
		state, flatDiags := clustermodel.Flatten(ctx, cl, &plan)
		resp.Diagnostics.Append(flatDiags...)
		if resp.Diagnostics.HasError() {
			resp.Diagnostics.Append(resp.State.Set(ctx, clustermodel.GenerateMinimalResourceModel(types.StringValue(clusterID), plan.Timeouts))...)
			resp.Diagnostics.AddError("failed to generate model for state during cluster.Create", "")
			return
		}
		tflog.Info(ctx, "cluster created", map[string]any{"cluster_id": clusterID})
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

// Read reads Cluster resource's values and updates the state.
func (c *Cluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model clustermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	cl, err := c.CpCl.ClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	logPrivateLinkResponse(ctx, cl)
	tflog.Debug(ctx, "read cluster", map[string]any{"cluster_id": cl.GetId(), "state": cl.GetState().String()})

	if cl.GetState() == controlplanev1.Cluster_STATE_DELETING || cl.GetState() == controlplanev1.Cluster_STATE_DELETING_AGENT {
		resp.Diagnostics.Append(resp.State.Set(ctx, clustermodel.GenerateMinimalResourceModel(types.StringValue(cl.GetId()), model.Timeouts))...)
		resp.Diagnostics.AddWarning(fmt.Sprintf("cluster %s is in state %s", model.ID.ValueString(), cl.GetState()), "")
		return
	}

	state, flatDiags := clustermodel.Flatten(ctx, cl, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Read", "")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update a Redpanda cluster
func (c *Cluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan clustermodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state clustermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "updating cluster", map[string]any{"cluster_id": plan.ID.ValueString()})

	diffedPayload, mask, expandDiags := clustermodel.ExpandUpdateWithMask(ctx, &plan, &state)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The public-API mapper accepts rpsql and kafka_connect only at leaf
	// granularity, not the top-level path the diff emits, so expand those before
	// the request (see internal/clustermask).
	clustermask.ExpandLeafPaths(mask)
	diffedPayload.Id = plan.ID.ValueString()
	updateReq := &controlplanev1.UpdateClusterRequest{
		Cluster:    diffedPayload,
		UpdateMask: mask,
	}

	if len(updateReq.UpdateMask.Paths) != 0 {
		op, err := c.CpCl.Cluster.UpdateCluster(ctx, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("failed to send cluster update request", utils.DeserializeGrpcError(err))
			return
		}
		updateTimeout, diags := plan.Timeouts.Update(ctx, 180*time.Minute)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := utils.AreWeDoneYet(ctx, op.GetOperation(), updateTimeout, c.CpCl.Operation); err != nil {
			resp.Diagnostics.AddError("failed while waiting to update cluster", utils.DeserializeGrpcError(err))
			return
		}
	}

	cl, err := c.CpCl.ClusterForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", plan.ID), utils.DeserializeGrpcError(err))
		return
	}

	newState, flatDiags := clustermodel.Flatten(ctx, cl, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Update", "")
		return
	}
	tflog.Info(ctx, "cluster updated", map[string]any{"cluster_id": plan.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Delete deletes the Cluster resource.
func (c *Cluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model clustermodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("cluster deletion not allowed", "allow_deletion is set to false")
		return
	}

	clusterID := model.ID.ValueString()
	tflog.Info(ctx, "deleting cluster", map[string]any{"cluster_id": clusterID})
	cl, err := c.CpCl.ClusterForID(ctx, clusterID)
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	if cl.GetState() != controlplanev1.Cluster_STATE_DELETING && cl.GetState() != controlplanev1.Cluster_STATE_DELETING_AGENT {
		delReq, _ := clustermodel.ExpandDelete(ctx, &model)
		if _, err := c.CpCl.Cluster.DeleteCluster(ctx, delReq); err != nil {
			resp.Diagnostics.AddError("failed to delete cluster", utils.DeserializeGrpcError(err))
			return
		}
	}

	deleteTimeout, diags := model.Timeouts.Delete(ctx, 90*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ranByoc := false
	_, err = utils.RetryGetCluster(ctx, deleteTimeout, clusterID, c.CpCl, func(cl *controlplanev1.Cluster) *utils.RetryError {
		if cl.GetState() == controlplanev1.Cluster_STATE_DELETING {
			return utils.RetryableError(fmt.Errorf("expected cluster to be deleted but was in state %v", cl.GetState()))
		}
		if cl.GetState() == controlplanev1.Cluster_STATE_DELETING_AGENT {
			if cl.Type == controlplanev1.Cluster_TYPE_BYOC && !ranByoc {
				err = c.Byoc.RunByoc(ctx, clusterID, "destroy")
				if err != nil {
					if utils.IsRetryableByocError(err) {
						tflog.Debug(ctx, fmt.Sprintf("Retryable byoc error during destroy: %v", err))
						return utils.RetryableError(err)
					}
					return utils.NonRetryableError(err)
				}
				ranByoc = true
			}
			return utils.RetryableError(fmt.Errorf("expected cluster to be deleted but was in state %v", cl.GetState()))
		}
		return utils.NonRetryableError(fmt.Errorf("unhandled state %v. please report this issue to the provider developers", cl.GetState()))
	})
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}
	tflog.Info(ctx, "cluster deleted", map[string]any{"cluster_id": clusterID})
}

// ImportState imports and update the state of the cluster resource.
func (*Cluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceClusterSchema(ctx), &resp.State, "allow_deletion")...)
}

// ModifyPlan marks endpoint and private-link.status objects Unknown when
// private-link config changes, so populated→null transitions don't trip
// "inconsistent result after apply" (terraform-plugin-framework#1211).
// Pairs with the parent objectplanmodifier.UseNonNullStateForUnknown
// migration in schema_resource.go: that handles null→populated; this
// handles populated→null.
func (*Cluster) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	plChanged, d := privateLinkConfigChanged(ctx, req.State, req.Plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() || !plChanged {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("kafka_api").AtName("all_seed_brokers"), types.ObjectUnknown(clustermodel.KafkaAPIAllSeedBrokersAttrTypes()))...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("http_proxy").AtName("all_urls"), types.ObjectUnknown(clustermodel.HTTPProxyAllUrlsAttrTypes()))...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("schema_registry").AtName("all_urls"), types.ObjectUnknown(clustermodel.SchemaRegistryAllUrlsAttrTypes()))...)
	for _, pl := range []struct {
		blockPath  path.Path
		statusType map[string]attr.Type
	}{
		{path.Root("aws_private_link"), clustermodel.AWSPrivateLinkStatusAttrTypes()},
		{path.Root("gcp_private_service_connect"), clustermodel.GCPPrivateServiceConnectStatusAttrTypes()},
		{path.Root("azure_private_link"), clustermodel.AzurePrivateLinkStatusAttrTypes()},
	} {
		var planVal types.Object
		resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, pl.blockPath, &planVal)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if planVal.IsNull() || planVal.IsUnknown() {
			continue
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, pl.blockPath.AtName("status"), types.ObjectUnknown(pl.statusType))...)
	}
}

func privateLinkConfigChanged(ctx context.Context, state tfsdk.State, plan tfsdk.Plan) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	for _, pc := range []struct {
		path      path.Path
		userAttrs []string
	}{
		{path.Root("aws_private_link"), []string{"enabled", "connect_console", "allowed_principals"}},
		{path.Root("gcp_private_service_connect"), []string{"enabled", "global_access_enabled", "consumer_accept_list"}},
		{path.Root("azure_private_link"), []string{"enabled", "connect_console", "allowed_subscriptions"}},
	} {
		var stateVal, planVal types.Object
		diags.Append(state.GetAttribute(ctx, pc.path, &stateVal)...)
		diags.Append(plan.GetAttribute(ctx, pc.path, &planVal)...)
		if diags.HasError() {
			return false, diags
		}
		if stateVal.IsNull() != planVal.IsNull() {
			return true, diags
		}
		if stateVal.IsNull() {
			continue
		}
		sa := stateVal.Attributes()
		pa := planVal.Attributes()
		for _, a := range pc.userAttrs {
			if !sa[a].Equal(pa[a]) {
				return true, diags
			}
		}
	}
	return false, diags
}

// logPrivateLinkResponse emits DEBUG-level presence flags for the
// PrivateLink-adjacent fields on a cluster response. URL values are
// intentionally omitted — endpoint FQDNs identify the customer's
// PrivateLink service.
func logPrivateLinkResponse(ctx context.Context, cl *controlplanev1.Cluster) {
	if pl := cl.GetAwsPrivateLink(); pl != nil {
		tflog.Debug(ctx, "cluster API response: aws_private_link", map[string]any{
			"enabled":         pl.GetEnabled(),
			"connect_console": pl.GetConnectConsole(),
			"has_status":      pl.HasStatus(),
		})
	}
	tflog.Debug(ctx, "cluster API response: endpoint presence", map[string]any{
		"kafka_api.all_seed_brokers": cl.GetKafkaApi().HasAllSeedBrokers(),
		"http_proxy.all_urls":        cl.GetHttpProxy().HasAllUrls(),
		"schema_registry.all_urls":   cl.GetSchemaRegistry().HasAllUrls(),
	})
}
