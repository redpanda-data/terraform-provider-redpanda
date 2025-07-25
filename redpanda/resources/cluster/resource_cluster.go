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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Cluster{}
	_ resource.ResourceWithConfigure   = &Cluster{}
	_ resource.ResourceWithImportState = &Cluster{}
)

// Cluster represents a cluster managed resource
type Cluster struct {
	CpCl *cloud.ControlPlaneClientSet
	Byoc *utils.ByocClient
}

// Metadata returns the full name of the Cluster resource
func (*Cluster) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_cluster"
}

// Configure uses provider level data to configure Cluster's clients
func (c *Cluster) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(config.Resource)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	c.Byoc = p.ByocClient
	c.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the Cluster resource
func (*Cluster) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceClusterSchema()
}

// Create creates a new Cluster resource. It updates the state if the resource is successfully created
func (c *Cluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model cluster.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	clusterReq, d := model.GetClusterCreate(ctx)
	if d.HasError() {
		resp.Diagnostics.Append(d...)
		resp.Diagnostics.AddError("unable to parse CreateCluster request", "")
		return
	}

	clResp, err := c.CpCl.Cluster.CreateCluster(ctx, &controlplanev1.CreateClusterRequest{Cluster: clusterReq})
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster", utils.DeserializeGrpcError(err))
		return
	}
	op := clResp.Operation
	clusterID := op.GetResourceId()

	// wait for creation to complete, running "byoc apply" if we see STATE_CREATING_AGENT
	var ranByoc bool
	cl, err := utils.RetryGetCluster(ctx, 90*time.Minute, clusterID, c.CpCl, func(cluster *controlplanev1.Cluster) *utils.RetryError {
		switch cluster.GetState() {
		case controlplanev1.Cluster_STATE_CREATING:
			return utils.RetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cluster.GetState()))
		case controlplanev1.Cluster_STATE_CREATING_AGENT:
			if cluster.Type == controlplanev1.Cluster_TYPE_BYOC && !ranByoc {
				err = c.Byoc.RunByoc(ctx, clusterID, "apply")
				if err != nil {
					return utils.NonRetryableError(err)
				}
				ranByoc = true
			}
			return utils.RetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cluster.GetState()))
		case controlplanev1.Cluster_STATE_READY:
			return nil
		case controlplanev1.Cluster_STATE_FAILED:
			return utils.NonRetryableError(fmt.Errorf("expected cluster to be ready but was in state %v", cluster.GetState()))
		default:
			return utils.NonRetryableError(fmt.Errorf("unhandled state %v. please report this issue to the provider developers", cluster.GetState()))
		}
	})
	if err != nil {
		// append minimal state because we failed
		resp.Diagnostics.Append(resp.State.Set(ctx, cluster.GenerateMinimalResourceModel(clusterID))...)
		resp.Diagnostics.AddError(fmt.Sprintf("failed to create cluster with ID %q", clusterID), utils.DeserializeGrpcError(err))
		return
	}

	var clusterConfig cluster.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &clusterConfig)...)

	// there are various states where cluster can be nil in which case we should default to the minimal model already persisted
	if cl != nil {
		p, dg := model.GetUpdatedModel(ctx, cl, cluster.ContingentFields{
			RedpandaVersion:       model.RedpandaVersion,
			AllowDeletion:         model.AllowDeletion,
			Tags:                  model.Tags,
			GcpGlobalAccessConfig: clusterConfig.GCPGlobalAccessEnabled,
		})
		if dg.HasError() {
			// append minimal state because we failed
			resp.Diagnostics.Append(resp.State.Set(ctx, cluster.GenerateMinimalResourceModel(clusterID))...)
			resp.Diagnostics.AddError("failed to generate model for state during cluster.Create", "")
			resp.Diagnostics.Append(d...)
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, p)...)
	}
}

// Read reads Cluster resource's values and updates the state.
func (c *Cluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model cluster.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	cl, err := c.CpCl.ClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			// Treat HTTP 404 Not Found status as a signal to recreate resource and return early
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	if cl.GetState() == controlplanev1.Cluster_STATE_DELETING || cl.GetState() == controlplanev1.Cluster_STATE_DELETING_AGENT {
		// null out the state, force it to be destroyed and recreated
		resp.Diagnostics.Append(resp.State.Set(ctx, cluster.GenerateMinimalResourceModel(cl.GetId()))...)
		resp.Diagnostics.AddWarning(fmt.Sprintf("cluster %s is in state %s", model.ID.ValueString(), cl.GetState()), "")
		return
	}

	persist, d := model.GetUpdatedModel(ctx, cl, cluster.ContingentFields{
		RedpandaVersion:       model.RedpandaVersion,
		AllowDeletion:         model.AllowDeletion,
		Tags:                  model.Tags,
		GcpGlobalAccessConfig: model.GCPGlobalAccessEnabled,
	})
	if d.HasError() {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Read", "")
		resp.Diagnostics.Append(d...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update a Redpanda cluster
func (c *Cluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cluster.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state cluster.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	updateReq, ds := plan.GetClusterUpdateRequest(ctx, &state)
	if ds.HasError() {
		resp.Diagnostics.Append(ds...)
		resp.Diagnostics.AddError("unable to parse UpdateCluster request", "")
		return
	}
	if len(updateReq.UpdateMask.Paths) != 0 {
		op, err := c.CpCl.Cluster.UpdateCluster(ctx, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("failed to send cluster update request", utils.DeserializeGrpcError(err))
			return
		}

		if err := utils.AreWeDoneYet(ctx, op.GetOperation(), 180*time.Minute, c.CpCl.Operation); err != nil {
			resp.Diagnostics.AddError("failed while waiting to update cluster", utils.DeserializeGrpcError(err))
			return
		}
	}

	cl, err := c.CpCl.ClusterForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", plan.ID), utils.DeserializeGrpcError(err))
		return
	}

	var clusterConfig cluster.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &clusterConfig)...)

	persist, d := plan.GetUpdatedModel(ctx, cl, cluster.ContingentFields{
		RedpandaVersion:       plan.RedpandaVersion,
		AllowDeletion:         plan.AllowDeletion,
		Tags:                  plan.Tags,
		GcpGlobalAccessConfig: clusterConfig.GCPGlobalAccessEnabled,
	})
	if d.HasError() {
		resp.Diagnostics.AddError("failed to generate model for state during cluster.Update", "")
		resp.Diagnostics.Append(d...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Delete deletes the Cluster resource.
func (c *Cluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model cluster.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("cluster deletion not allowed", "allow_deletion is set to false")
		return
	}

	clusterID := model.ID.ValueString()
	cl, err := c.CpCl.ClusterForID(ctx, clusterID)
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	// call Delete on the cluster, if it's not already in progress. calling Delete on a cluster in
	// STATE_DELETING_AGENT seems to destroy it immediately and we don't want to do that if we haven't
	// cleaned up yet

	if cl.GetState() != controlplanev1.Cluster_STATE_DELETING && cl.GetState() != controlplanev1.Cluster_STATE_DELETING_AGENT {
		_, err = c.CpCl.Cluster.DeleteCluster(ctx, &controlplanev1.DeleteClusterRequest{
			Id: clusterID,
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to delete cluster", utils.DeserializeGrpcError(err))
			return
		}
	}

	// wait for creation to complete, running "byoc apply" if we see STATE_DELETING_AGENT
	ranByoc := false
	_, err = utils.RetryGetCluster(ctx, 90*time.Minute, clusterID, c.CpCl, func(cluster *controlplanev1.Cluster) *utils.RetryError {
		if cluster.GetState() == controlplanev1.Cluster_STATE_DELETING {
			return utils.RetryableError(fmt.Errorf("expected cluster to be deleted but was in state %v", cluster.GetState()))
		}
		if cluster.GetState() == controlplanev1.Cluster_STATE_DELETING_AGENT {
			if cluster.Type == controlplanev1.Cluster_TYPE_BYOC && !ranByoc {
				err = c.Byoc.RunByoc(ctx, clusterID, "destroy")
				if err != nil {
					return utils.NonRetryableError(err)
				}
				ranByoc = true
			}
			return utils.RetryableError(fmt.Errorf("expected cluster to be deleted but was in state %v", cluster.GetState()))
		}

		return utils.NonRetryableError(fmt.Errorf("unhandled state %v. please report this issue to the provider developers", cluster.GetState()))
	})
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}
}

// ImportState imports and update the state of the cluster resource.
func (*Cluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
