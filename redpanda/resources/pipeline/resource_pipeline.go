// Copyright 2023 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

// Package pipeline contains the implementation of the Pipeline resource following the Terraform framework interfaces.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	pipelinemodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/pipeline"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var (
	_ resource.Resource                = &Pipeline{}
	_ resource.ResourceWithConfigure   = &Pipeline{}
	_ resource.ResourceWithImportState = &Pipeline{}
)

// DefaultPipelineOperationTimeout is the default timeout for pipeline state transitions.
// This allows sufficient time for the pipeline to start/stop gracefully while preventing
// indefinite hangs. Based on empirical testing, most pipelines transition within 30-60 seconds.
const DefaultPipelineOperationTimeout = 2 * time.Minute

// ClientFactory creates a PipelineServiceClient for the given cluster URL
type ClientFactory func(clusterURL, authToken, providerVersion, terraformVersion string) (dataplanev1grpc.PipelineServiceClient, error)

// Pipeline represents a pipeline managed resource
type Pipeline struct {
	base.ResourceBase

	PipelineClient dataplanev1grpc.PipelineServiceClient

	resData       config.Resource
	clientFactory ClientFactory
}

// NewPipeline constructs a Pipeline resource.
func NewPipeline() *Pipeline {
	p := &Pipeline{}
	p.ResourceBase = base.NewResourceBase(
		"redpanda_pipeline",
		ResourcePipelineSchema,
		func(d config.Resource) { p.resData = d },
	)
	return p
}

// Create creates a new Pipeline resource
func (p *Pipeline) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	var cfg pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(restoreServiceAccountClientSecret(ctx, &model, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := p.createPipelineClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}

	createReq, expandDiags := pipelinemodel.ExpandCreate(ctx, &model)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := p.PipelineClient.CreatePipeline(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create pipeline", utils.DeserializeGrpcError(err))
		return
	}

	pipeline := createResp.GetPipeline()

	createTimeout, diags := model.Timeouts.Create(ctx, DefaultPipelineOperationTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	desiredState := model.State.ValueString()
	if desiredState == "" {
		desiredState = pipelinemodel.StateStopped
	}

	currentAPIState := pipeline.GetState()
	isCurrentlyRunning := currentAPIState == dataplanev1.Pipeline_STATE_RUNNING || currentAPIState == dataplanev1.Pipeline_STATE_STARTING

	if desiredState == pipelinemodel.StateRunning && !isCurrentlyRunning {
		updatedPipeline, warning, ok := p.startPipeline(ctx, pipeline.GetId(), createTimeout)
		if !ok {
			resp.Diagnostics.AddWarning("pipeline failed to reach desired state",
				fmt.Sprintf("Pipeline %s was created but failed to start within the timeout. Run 'terraform apply' again to retry: %s", pipeline.GetId(), warning))
		} else if updatedPipeline != nil {
			pipeline = updatedPipeline
		}
	} else if desiredState == pipelinemodel.StateStopped && isCurrentlyRunning {
		updatedPipeline, warning, ok := p.stopPipeline(ctx, pipeline.GetId(), createTimeout)
		if !ok {
			resp.Diagnostics.AddWarning("pipeline failed to reach desired state",
				fmt.Sprintf("Pipeline %s was created but failed to stop within the timeout. Run 'terraform apply' again to retry: %s", pipeline.GetId(), warning))
		} else if updatedPipeline != nil {
			pipeline = updatedPipeline
		}
	}

	state, diags := pipelinemodel.Flatten(ctx, pipeline, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read reads Pipeline resource's values and updates the state
func (p *Pipeline) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if model.ClusterAPIURL.IsNull() || model.ClusterAPIURL.IsUnknown() || model.ClusterAPIURL.ValueString() == "" {
		if model.AllowDeletion.IsNull() || model.AllowDeletion.ValueBool() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddWarning(
			"Missing Cluster API URL",
			fmt.Sprintf("Pipeline %s has no cluster API URL configured. Resource will remain in state because allow_deletion is false.", model.ID.ValueString()),
		)
		return
	}

	err := p.createPipelineClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		if utils.IsClusterUnreachable(err) {
			if model.AllowDeletion.IsNull() || model.AllowDeletion.ValueBool() {
				tflog.Info(ctx, fmt.Sprintf("cluster unreachable for pipeline %s, removing from state since allow_deletion is true", model.ID.ValueString()))
				resp.State.RemoveResource(ctx)
				return
			}
			tflog.Warn(ctx, fmt.Sprintf("cluster unreachable for pipeline %s, keeping in state since allow_deletion is false", model.ID.ValueString()))
			resp.Diagnostics.AddWarning(
				"Cluster Unreachable",
				fmt.Sprintf("Unable to reach cluster for pipeline %s. Resource will remain in state because allow_deletion is false. Error: %s", model.ID.ValueString(), utils.DeserializeGrpcError(err)),
			)
			return
		}
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) || utils.IsClusterUnreachable(err) {
			if model.AllowDeletion.IsNull() || model.AllowDeletion.ValueBool() {
				tflog.Info(ctx, fmt.Sprintf("pipeline %s not found or cluster unreachable, removing from state since allow_deletion is true", model.ID.ValueString()))
				resp.State.RemoveResource(ctx)
				return
			}
			tflog.Warn(ctx, fmt.Sprintf("pipeline %s not found or cluster unreachable, keeping in state since allow_deletion is false", model.ID.ValueString()))
			if utils.IsNotFound(err) {
				resp.Diagnostics.AddWarning(
					"Pipeline Not Found",
					fmt.Sprintf("Pipeline %s was not found on the cluster. Resource will remain in state because allow_deletion is false.", model.ID.ValueString()),
				)
			} else {
				resp.Diagnostics.AddWarning(
					"Cluster Unreachable",
					fmt.Sprintf("Unable to reach cluster for pipeline %s. Resource will remain in state because allow_deletion is false. Error: %s", model.ID.ValueString(), utils.DeserializeGrpcError(err)),
				)
			}
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to get pipeline %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}

	readState, diags := pipelinemodel.Flatten(ctx, getResp.GetPipeline(), &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// While a pipeline is in state=running, the Cloud backend's GetPipeline
	// RPC omits ServiceAccount from its response.
	// Without this restore, Flatten drops the wrapper from in-memory state
	// and the next plan reports `+ service_account` even though nothing
	// changed. Mirror the Update-path fix at the wrapper level.
	if getResp.GetPipeline().GetServiceAccount() == nil && !model.ServiceAccount.IsNull() {
		readState.ServiceAccount = model.ServiceAccount
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, readState)...)
}

// Update updates a Pipeline resource
func (p *Pipeline) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var cfg pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(restoreServiceAccountClientSecret(ctx, &plan, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := p.createPipelineClient(plan.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}

	pipelineID := state.ID.ValueString()

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to get pipeline %s", pipelineID), utils.DeserializeGrpcError(err))
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, DefaultPipelineOperationTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentAPIState := getResp.GetPipeline().GetState()
	isCurrentlyRunning := currentAPIState == dataplanev1.Pipeline_STATE_RUNNING || currentAPIState == dataplanev1.Pipeline_STATE_STARTING

	if isCurrentlyRunning {
		tflog.Info(ctx, fmt.Sprintf("stopping pipeline %s before update", pipelineID))
		_, err := p.PipelineClient.StopPipeline(ctx, &dataplanev1.StopPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to stop pipeline %s before update", pipelineID), utils.DeserializeGrpcError(err))
			return
		}

		err = p.waitForPipelineState(ctx, pipelineID, dataplanev1.Pipeline_STATE_STOPPED, updateTimeout)
		if err != nil {
			resp.Diagnostics.AddWarning(
				"pipeline may not have fully stopped",
				fmt.Sprintf("Timed out waiting for pipeline %s to stop: %s", pipelineID, err.Error()),
			)
		}
	}

	desiredState := plan.State.ValueString()
	if desiredState == "" {
		desiredState = pipelinemodel.StateStopped
	}

	updateReq, expandDiags := pipelinemodel.ExpandUpdate(ctx, &plan)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateReq.Id = pipelineID

	// Only include service_account in the update when:
	// - Adding service account for the first time (state was null), OR
	// - client_id changed, OR
	// - secret_version changed (signals intent to update the write-only secret).
	// Otherwise null it out so we don't clobber the write-only client_secret
	// server-side with the empty value Terraform reads back from state.
	if !plan.ServiceAccount.IsNull() && !plan.ServiceAccount.IsUnknown() {
		shouldUpdateServiceAccount := state.ServiceAccount.IsNull()
		if !shouldUpdateServiceAccount {
			var planSA, stateSA pipelinemodel.ServiceAccountModel
			resp.Diagnostics.Append(plan.ServiceAccount.As(ctx, &planSA, basetypes.ObjectAsOptions{})...)
			resp.Diagnostics.Append(state.ServiceAccount.As(ctx, &stateSA, basetypes.ObjectAsOptions{})...)
			if resp.Diagnostics.HasError() {
				return
			}

			clientIDChanged := !planSA.ClientID.Equal(stateSA.ClientID)
			secretVersionChanged := !planSA.SecretVersion.Equal(stateSA.SecretVersion)
			shouldUpdateServiceAccount = clientIDChanged || secretVersionChanged
		}
		if !shouldUpdateServiceAccount {
			updateReq.Pipeline.ServiceAccount = nil
		}
	} else {
		updateReq.Pipeline.ServiceAccount = nil
	}

	updateResp, err := p.PipelineClient.UpdatePipeline(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to update pipeline %s", pipelineID), utils.DeserializeGrpcError(err))
		return
	}

	pipeline := updateResp.GetPipeline()

	if desiredState == pipelinemodel.StateRunning {
		updatedPipeline, warning, ok := p.startPipeline(ctx, pipelineID, updateTimeout)
		if !ok {
			resp.Diagnostics.AddWarning("pipeline failed to reach desired state",
				fmt.Sprintf("Pipeline %s was updated but failed to start within the timeout. Run 'terraform apply' again to retry: %s", pipelineID, warning))
		} else if updatedPipeline != nil {
			pipeline = updatedPipeline
		}
	}

	newState, diags := pipelinemodel.Flatten(ctx, pipeline, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// When we omit ServiceAccount from the Update request (to protect the
	// write-only client_secret), the server response echoes
	// SA=nil and Flatten drops the wrapper from state — even though
	// nothing on the SA changed. Restore the prior-state block so the
	// framework's post-apply consistency check sees it unchanged.
	if updateReq.Pipeline.ServiceAccount == nil && !state.ServiceAccount.IsNull() {
		newState.ServiceAccount = state.ServiceAccount
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Delete deletes the Pipeline resource
func (p *Pipeline) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError(
			"Deletion Not Allowed",
			fmt.Sprintf("Pipeline %s cannot be deleted because allow_deletion is set to false. Set allow_deletion = true to delete this resource.", model.ID.ValueString()),
		)
		return
	}

	err := p.createPipelineClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "pipeline", model.ID.ValueString(), model.AllowDeletion, err, "create pipeline client")
		resp.Diagnostics.Append(diags...)
		return
	}

	pipelineID := model.ID.ValueString()

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		if utils.IsNotFound(err) {
			tflog.Info(ctx, fmt.Sprintf("pipeline %s already deleted", pipelineID))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to get pipeline %s before deletion", pipelineID), utils.DeserializeGrpcError(err))
		return
	}

	deleteTimeout, diags := model.Timeouts.Delete(ctx, DefaultPipelineOperationTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentState := getResp.GetPipeline().GetState()

	if currentState == dataplanev1.Pipeline_STATE_RUNNING || currentState == dataplanev1.Pipeline_STATE_STARTING {
		_, warning, ok := p.stopPipeline(ctx, pipelineID, deleteTimeout)
		if !ok {
			resp.Diagnostics.AddWarning("failed to stop pipeline before deletion",
				fmt.Sprintf("Pipeline %s could not be stopped before deletion: %s. Attempting to delete anyway.", pipelineID, warning))
		}
	}

	_, err = p.PipelineClient.DeletePipeline(ctx, &dataplanev1.DeletePipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		if utils.IsNotFound(err) {
			tflog.Info(ctx, fmt.Sprintf("pipeline %s already deleted", pipelineID))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete pipeline %s", pipelineID), utils.DeserializeGrpcError(err))
		return
	}

	tflog.Info(ctx, fmt.Sprintf("successfully deleted pipeline %s", pipelineID))
}

// ImportState imports and update the state of the pipeline resource
func (p *Pipeline) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	split := strings.SplitN(req.ID, ",", 2)
	if len(split) != 2 {
		resp.Diagnostics.AddError(
			fmt.Sprintf("wrong import ID format: %v", req.ID),
			"Import ID format is <pipeline_id>,<cluster_id>",
		)
		return
	}

	pipelineID, clusterID := split[0], split[1]

	dataplaneURL, err := p.CpCl.DataplaneURLForCluster(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to resolve dataplane URL for cluster %q; make sure import ID format is <pipeline_id>,<cluster_id>", clusterID),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(pipelineID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), types.StringValue(dataplaneURL))...)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourcePipelineSchema(ctx), &resp.State, "allow_deletion")...)
}

// startPipeline starts the pipeline and waits for it to reach running state.
// Returns the updated pipeline, a warning message (if any), and whether the operation succeeded.
func (p *Pipeline) startPipeline(ctx context.Context, pipelineID string, timeout time.Duration) (*dataplanev1.Pipeline, string, bool) {
	tflog.Info(ctx, fmt.Sprintf("starting pipeline %s", pipelineID))
	_, err := p.PipelineClient.StartPipeline(ctx, &dataplanev1.StartPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		return nil, fmt.Sprintf("failed to start pipeline: %s", utils.DeserializeGrpcError(err)), false
	}

	err = p.waitForPipelineState(ctx, pipelineID, dataplanev1.Pipeline_STATE_RUNNING, timeout)
	if err != nil {
		return nil, fmt.Sprintf("timed out waiting for pipeline to reach running state: %s", err.Error()), false
	}

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		return nil, "", true // Started but couldn't refresh state
	}
	return getResp.GetPipeline(), "", true
}

// stopPipeline stops the pipeline and waits for it to reach stopped state.
// Returns the updated pipeline, a warning message (if any), and whether the operation succeeded.
func (p *Pipeline) stopPipeline(ctx context.Context, pipelineID string, timeout time.Duration) (*dataplanev1.Pipeline, string, bool) {
	tflog.Info(ctx, fmt.Sprintf("stopping pipeline %s", pipelineID))
	_, err := p.PipelineClient.StopPipeline(ctx, &dataplanev1.StopPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		return nil, fmt.Sprintf("failed to stop pipeline: %s", utils.DeserializeGrpcError(err)), false
	}

	err = p.waitForPipelineState(ctx, pipelineID, dataplanev1.Pipeline_STATE_STOPPED, timeout)
	if err != nil {
		return nil, fmt.Sprintf("timed out waiting for pipeline to reach stopped state: %s", err.Error()), false
	}

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		return nil, "", true // Stopped but couldn't refresh state
	}
	return getResp.GetPipeline(), "", true
}

func (p *Pipeline) createPipelineClient(clusterURL string) error {
	if p.PipelineClient != nil {
		return nil
	}
	if clusterURL == "" {
		return errors.New("unable to create client with empty target cluster API URL")
	}

	if p.clientFactory != nil {
		client, err := p.clientFactory(clusterURL, p.resData.AuthToken, p.resData.ProviderVersion, p.resData.TerraformVersion)
		if err != nil {
			return err
		}
		p.PipelineClient = client
		return nil
	}

	if p.resData.DataplaneConnPool == nil {
		return errors.New("provider not configured: dataplane connection pool is nil")
	}
	conn, err := p.resData.DataplaneConnPool.GetConnection(clusterURL)
	if err != nil {
		return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
	}
	p.PipelineClient = dataplanev1grpc.NewPipelineServiceClient(conn)
	return nil
}

func (p *Pipeline) waitForPipelineState(ctx context.Context, pipelineID string, targetState dataplanev1.Pipeline_State, timeout time.Duration) error {
	return utils.Retry(ctx, timeout, func() *utils.RetryError {
		getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			return utils.NonRetryableError(fmt.Errorf("failed to get pipeline state: %w", err))
		}
		state := getResp.GetPipeline().GetState()
		if state == targetState {
			return nil
		}
		if state == dataplanev1.Pipeline_STATE_ERROR {
			return utils.NonRetryableError(errors.New("pipeline entered error state"))
		}
		return utils.RetryableError(fmt.Errorf("pipeline in state %s, waiting for %s", state, targetState))
	})
}

// restoreServiceAccountClientSecret copies cfg.ServiceAccount.ClientSecret
// into model.ServiceAccount.ClientSecret. client_secret is WriteOnly so the
// framework strips it from Plan/State per its contract — but the server's
// regex validator requires the `${secrets.NAME}` value to reach the API on
// Create/Update. Same pattern as `redpanda_schema`'s `password_wo` handling.
// No-op when cfg has no service_account block; safe to call unconditionally.
func restoreServiceAccountClientSecret(ctx context.Context, model, cfg *pipelinemodel.ResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if cfg.ServiceAccount.IsNull() || cfg.ServiceAccount.IsUnknown() {
		return diags
	}
	cfgSA, d := cfg.AsServiceAccount(ctx)
	diags.Append(d...)
	if diags.HasError() || cfgSA == nil {
		return diags
	}
	modelSA, d := model.AsServiceAccount(ctx)
	diags.Append(d...)
	if diags.HasError() || modelSA == nil {
		return diags
	}
	modelSA.ClientSecret = cfgSA.ClientSecret
	obj, d := pipelinemodel.ServiceAccountToObject(ctx, modelSA)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.ServiceAccount = obj
	return diags
}
