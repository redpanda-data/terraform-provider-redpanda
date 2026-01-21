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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	pipelinemodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/pipeline"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// Ensure provider defined types fully satisfy framework interfaces.
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
type ClientFactory func(clusterURL, authToken, providerVersion, terraformVersion string) (dataplanev1grpc.PipelineServiceClient, *grpc.ClientConn, error)

// Pipeline represents a pipeline managed resource
type Pipeline struct {
	PipelineClient dataplanev1grpc.PipelineServiceClient

	resData       config.Resource
	dataplaneConn *grpc.ClientConn
	clientFactory ClientFactory
	pollInterval  time.Duration // For testing; if zero, uses default
}

// getPollInterval returns the poll interval for waiting on state changes.
// Uses the configured interval if set, otherwise returns the default of 1 second.
func (p *Pipeline) getPollInterval() time.Duration {
	if p.pollInterval > 0 {
		return p.pollInterval
	}
	return 1 * time.Second
}

// Metadata returns the full name of the Pipeline resource
func (*Pipeline) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_pipeline"
}

// Configure uses provider level data to configure Pipeline's clients
func (p *Pipeline) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.Resource, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	p.resData = pd
}

// Schema returns the schema for the Pipeline resource
func (*Pipeline) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourcePipelineSchema(ctx)
}

// Create creates a new Pipeline resource
func (p *Pipeline) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := p.createPipelineClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}
	defer func() {
		if p.dataplaneConn != nil {
			_ = p.dataplaneConn.Close()
		}
	}()

	pipelineCreate := &dataplanev1.PipelineCreate{
		DisplayName: model.DisplayName.ValueString(),
		ConfigYaml:  model.ConfigYaml.ValueString(),
	}

	if !model.Description.IsNull() && !model.Description.IsUnknown() {
		pipelineCreate.Description = model.Description.ValueString()
	}

	if !model.Resources.IsNull() && !model.Resources.IsUnknown() {
		resources, diags := model.ExtractResources(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if resources != nil {
			pipelineCreate.Resources = resources
		}
	}

	if !model.Tags.IsNull() && !model.Tags.IsUnknown() {
		tags, diags := model.ExtractTags(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		pipelineCreate.Tags = tags
	}

	createResp, err := p.PipelineClient.CreatePipeline(ctx, &dataplanev1.CreatePipelineRequest{
		Pipeline: pipelineCreate,
	})
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

	// contingent holds the fields that are not returned by the API but need to be preserved in state.
	contingent := pipelinemodel.ContingentFields{
		ClusterAPIURL: model.ClusterAPIURL,
		AllowDeletion: model.AllowDeletion,
		Resources:     model.Resources,
		State:         model.State,
		Timeouts:      model.Timeouts,
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

	state := &pipelinemodel.ResourceModel{}
	state, diags = state.GetUpdatedModel(ctx, pipeline, contingent)
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
	defer func() {
		if p.dataplaneConn != nil {
			_ = p.dataplaneConn.Close()
		}
	}()

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

	readState := &pipelinemodel.ResourceModel{}
	readState, diags := readState.GetUpdatedModel(ctx, getResp.GetPipeline(), pipelinemodel.ContingentFields{
		ClusterAPIURL: model.ClusterAPIURL,
		AllowDeletion: model.AllowDeletion,
		Resources:     model.Resources,
		State:         model.State,
		Timeouts:      model.Timeouts,
	})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, readState)...)
}

// Update updates a Pipeline resource
func (p *Pipeline) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state pipelinemodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := p.createPipelineClient(plan.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}
	defer func() {
		if p.dataplaneConn != nil {
			_ = p.dataplaneConn.Close()
		}
	}()

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

	pipelineUpdate := &dataplanev1.PipelineUpdate{
		DisplayName: plan.DisplayName.ValueString(),
		ConfigYaml:  plan.ConfigYaml.ValueString(),
		Description: plan.Description.ValueString(),
	}

	if !plan.Resources.IsNull() && !plan.Resources.IsUnknown() {
		resources, diags := plan.ExtractResources(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if resources != nil {
			pipelineUpdate.Resources = resources
		}
	}

	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags, diags := plan.ExtractTags(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		pipelineUpdate.Tags = tags
	}

	updateResp, err := p.PipelineClient.UpdatePipeline(ctx, &dataplanev1.UpdatePipelineRequest{
		Id:       pipelineID,
		Pipeline: pipelineUpdate,
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to update pipeline %s", pipelineID), utils.DeserializeGrpcError(err))
		return
	}

	pipeline := updateResp.GetPipeline()

	// contingent holds the fields that are not returned by the API but need to be preserved in state.
	contingent := pipelinemodel.ContingentFields{
		ClusterAPIURL: plan.ClusterAPIURL,
		AllowDeletion: plan.AllowDeletion,
		Resources:     plan.Resources,
		State:         plan.State,
		Timeouts:      plan.Timeouts,
	}

	if desiredState == pipelinemodel.StateRunning {
		updatedPipeline, warning, ok := p.startPipeline(ctx, pipelineID, updateTimeout)
		if !ok {
			resp.Diagnostics.AddWarning("pipeline failed to reach desired state",
				fmt.Sprintf("Pipeline %s was updated but failed to start within the timeout. Run 'terraform apply' again to retry: %s", pipelineID, warning))
		} else if updatedPipeline != nil {
			pipeline = updatedPipeline
		}
	}

	newState := &pipelinemodel.ResourceModel{}
	newState, diags = newState.GetUpdatedModel(ctx, pipeline, contingent)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
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
		if utils.IsClusterUnreachable(err) {
			tflog.Warn(ctx, fmt.Sprintf("cluster unreachable for pipeline %s, considering deleted", model.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}
	defer func() {
		if p.dataplaneConn != nil {
			_ = p.dataplaneConn.Close()
		}
	}()

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

	client := cloud.NewControlPlaneClientSet(p.resData.ControlPlaneConnection)
	cluster, err := client.ClusterForID(ctx, clusterID)
	var dataplaneURL string

	if err == nil && cluster != nil && cluster.DataplaneApi != nil {
		dataplaneURL = cluster.DataplaneApi.Url
	} else {
		serverlessCluster, serr := client.ServerlessClusterForID(ctx, clusterID)
		if serr != nil || serverlessCluster == nil || serverlessCluster.DataplaneApi == nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("failed to find cluster with ID %q; make sure import ID format is <pipeline_id>,<cluster_id>", clusterID),
				utils.DeserializeGrpcError(err)+utils.DeserializeGrpcError(serr),
			)
			return
		}
		dataplaneURL = serverlessCluster.DataplaneApi.Url
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(pipelineID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), types.StringValue(dataplaneURL))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)
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
		client, conn, err := p.clientFactory(clusterURL, p.resData.AuthToken, p.resData.ProviderVersion, p.resData.TerraformVersion)
		if err != nil {
			return err
		}
		p.PipelineClient = client
		p.dataplaneConn = conn
		return nil
	}

	if p.dataplaneConn == nil {
		conn, err := cloud.SpawnConn(clusterURL, p.resData.AuthToken, p.resData.ProviderVersion, p.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
		}
		p.dataplaneConn = conn
	}
	p.PipelineClient = dataplanev1grpc.NewPipelineServiceClient(p.dataplaneConn)
	return nil
}

func (p *Pipeline) waitForPipelineState(ctx context.Context, pipelineID string, targetState dataplanev1.Pipeline_State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	baseInterval := p.getPollInterval()
	interval := baseInterval
	maxInterval := baseInterval * 10

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			return fmt.Errorf("failed to get pipeline state: %w", err)
		}

		state := getResp.GetPipeline().GetState()
		if state == targetState {
			return nil
		}
		if state == dataplanev1.Pipeline_STATE_ERROR {
			return errors.New("pipeline entered error state")
		}

		time.Sleep(interval)
		interval = min(interval*2, maxInterval)
	}
	return fmt.Errorf("timeout waiting for pipeline to reach state %s", targetState.String())
}
