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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Pipeline{}
	_ resource.ResourceWithConfigure   = &Pipeline{}
	_ resource.ResourceWithImportState = &Pipeline{}
)

// Pipeline represents the Pipeline Terraform resource.
type Pipeline struct {
	PipelineClient dataplanev1grpc.PipelineServiceClient

	resData       config.Resource
	dataplaneConn *grpc.ClientConn
}

// Metadata returns the metadata for the Pipeline resource.
func (*Pipeline) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_pipeline"
}

// Configure configures the Pipeline resource.
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

// Schema returns the schema for the Pipeline resource.
func (*Pipeline) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourcePipelineSchema()
}

// Create creates a Pipeline resource.
func (p *Pipeline) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Pipeline
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := p.createPipelineClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}
	defer p.dataplaneConn.Close()

	pipelineCreate := &dataplanev1.PipelineCreate{
		DisplayName: model.DisplayName.ValueString(),
		ConfigYaml:  model.ConfigYaml.ValueString(),
	}

	if !model.Description.IsNull() && !model.Description.IsUnknown() {
		pipelineCreate.Description = model.Description.ValueString()
	}

	if !model.Resources.IsNull() && !model.Resources.IsUnknown() {
		resources, diags := extractResources(ctx, model.Resources)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if resources != nil {
			pipelineCreate.Resources = resources
		}
	}

	if !model.Tags.IsNull() && !model.Tags.IsUnknown() {
		tags, diags := extractTags(ctx, model.Tags)
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

	if model.StartAfterCreate.ValueBool() {
		tflog.Info(ctx, fmt.Sprintf("starting pipeline %s after creation", pipeline.GetId()))
		_, err := p.PipelineClient.StartPipeline(ctx, &dataplanev1.StartPipelineRequest{
			Id: pipeline.GetId(),
		})
		if err != nil {
			resp.Diagnostics.AddWarning(
				"pipeline created but failed to start",
				fmt.Sprintf("Pipeline %s was created successfully but failed to start: %s", pipeline.GetId(), utils.DeserializeGrpcError(err)),
			)
		} else {
			getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
				Id: pipeline.GetId(),
			})
			if err == nil {
				pipeline = getResp.GetPipeline()
			}
		}
	}

	state, diags := pipelineToModel(ctx, pipeline, model.ClusterAPIURL, model.StartAfterCreate)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read reads the state of the Pipeline resource.
func (p *Pipeline) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.Pipeline
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if model.ClusterAPIURL.IsNull() || model.ClusterAPIURL.IsUnknown() || model.ClusterAPIURL.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	err := p.createPipelineClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		if utils.IsClusterUnreachable(err) {
			tflog.Warn(ctx, fmt.Sprintf("cluster unreachable for pipeline %s, removing from state", model.ID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to create pipeline client", utils.DeserializeGrpcError(err))
		return
	}
	defer p.dataplaneConn.Close()

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			tflog.Info(ctx, fmt.Sprintf("pipeline %s not found, removing from state", model.ID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		if utils.IsClusterUnreachable(err) {
			tflog.Warn(ctx, fmt.Sprintf("cluster unreachable for pipeline %s, removing from state", model.ID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to get pipeline %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}

	state, diags := pipelineToModel(ctx, getResp.GetPipeline(), model.ClusterAPIURL, model.StartAfterCreate)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the Pipeline resource.
func (p *Pipeline) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state models.Pipeline
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
	defer p.dataplaneConn.Close()

	pipelineID := state.ID.ValueString()

	getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
		Id: pipelineID,
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to get pipeline %s", pipelineID), utils.DeserializeGrpcError(err))
		return
	}

	currentState := getResp.GetPipeline().GetState()
	wasRunning := currentState == dataplanev1.Pipeline_STATE_RUNNING || currentState == dataplanev1.Pipeline_STATE_STARTING

	if wasRunning {
		tflog.Info(ctx, fmt.Sprintf("stopping pipeline %s before update", pipelineID))
		_, err := p.PipelineClient.StopPipeline(ctx, &dataplanev1.StopPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to stop pipeline %s before update", pipelineID), utils.DeserializeGrpcError(err))
			return
		}

		// Wait for pipeline to stop
		err = p.waitForPipelineState(ctx, pipelineID, dataplanev1.Pipeline_STATE_STOPPED, 2*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning(
				"pipeline may not have fully stopped",
				fmt.Sprintf("Timed out waiting for pipeline %s to stop: %s", pipelineID, err.Error()),
			)
		}
	}

	pipelineUpdate := &dataplanev1.PipelineUpdate{
		DisplayName: plan.DisplayName.ValueString(),
		ConfigYaml:  plan.ConfigYaml.ValueString(),
		Description: plan.Description.ValueString(),
	}

	if !plan.Resources.IsNull() && !plan.Resources.IsUnknown() {
		resources, diags := extractResources(ctx, plan.Resources)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if resources != nil {
			pipelineUpdate.Resources = resources
		}
	}

	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags, diags := extractTags(ctx, plan.Tags)
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

	if wasRunning {
		tflog.Info(ctx, fmt.Sprintf("restarting pipeline %s after update", pipelineID))
		_, err := p.PipelineClient.StartPipeline(ctx, &dataplanev1.StartPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			resp.Diagnostics.AddWarning(
				"pipeline updated but failed to restart",
				fmt.Sprintf("Pipeline %s was updated successfully but failed to restart: %s", pipelineID, utils.DeserializeGrpcError(err)),
			)
		} else {
			getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
				Id: pipelineID,
			})
			if err == nil {
				pipeline = getResp.GetPipeline()
			}
		}
	}

	newState, diags := pipelineToModel(ctx, pipeline, plan.ClusterAPIURL, plan.StartAfterCreate)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Delete deletes the Pipeline resource.
func (p *Pipeline) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.Pipeline
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
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
	defer p.dataplaneConn.Close()

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

	currentState := getResp.GetPipeline().GetState()

	if currentState == dataplanev1.Pipeline_STATE_RUNNING || currentState == dataplanev1.Pipeline_STATE_STARTING {
		tflog.Info(ctx, fmt.Sprintf("stopping pipeline %s before deletion", pipelineID))
		_, err := p.PipelineClient.StopPipeline(ctx, &dataplanev1.StopPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			resp.Diagnostics.AddWarning(
				"failed to stop pipeline before deletion",
				fmt.Sprintf("Pipeline %s could not be stopped before deletion: %s. Attempting to delete anyway.", pipelineID, utils.DeserializeGrpcError(err)),
			)
		} else {
			err = p.waitForPipelineState(ctx, pipelineID, dataplanev1.Pipeline_STATE_STOPPED, 2*time.Minute)
			if err != nil {
				resp.Diagnostics.AddWarning(
					"pipeline may not have fully stopped",
					fmt.Sprintf("Timed out waiting for pipeline %s to stop before deletion: %s", pipelineID, err.Error()),
				)
			}
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

// ImportState imports the state of the Pipeline resource.
func (p *Pipeline) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: <pipeline_id>,<cluster_id>
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

	if err == nil && cluster != nil {
		dataplaneURL = cluster.DataplaneApi.Url
	} else {
		serverlessCluster, serr := client.ServerlessClusterForID(ctx, clusterID)
		if serr != nil || serverlessCluster == nil {
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("start_after_create"), types.BoolValue(false))...)
}

func (p *Pipeline) createPipelineClient(clusterURL string) error {
	if p.PipelineClient != nil {
		return nil
	}
	if clusterURL == "" {
		return errors.New("unable to create client with empty target cluster API URL")
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
	for time.Now().Before(deadline) {
		getResp, err := p.PipelineClient.GetPipeline(ctx, &dataplanev1.GetPipelineRequest{
			Id: pipelineID,
		})
		if err != nil {
			return fmt.Errorf("failed to get pipeline state: %w", err)
		}
		if getResp.GetPipeline().GetState() == targetState {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for pipeline to reach state %s", targetState.String())
}

// pipelineToModel converts a dataplane Pipeline to a Terraform model.
func pipelineToModel(ctx context.Context, pipeline *dataplanev1.Pipeline, clusterAPIURL types.String, startAfterCreate types.Bool) (*models.Pipeline, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := &models.Pipeline{
		ID:               types.StringValue(pipeline.GetId()),
		ClusterAPIURL:    clusterAPIURL,
		DisplayName:      types.StringValue(pipeline.GetDisplayName()),
		Description:      types.StringValue(pipeline.GetDescription()),
		ConfigYaml:       types.StringValue(pipeline.GetConfigYaml()),
		StartAfterCreate: startAfterCreate,
		State:            types.StringValue(pipelineStateToString(pipeline.GetState())),
		URL:              types.StringValue(pipeline.GetUrl()),
	}

	if pipeline.HasResources() {
		res := pipeline.GetResources()
		resourcesObj, d := types.ObjectValue(models.GetPipelineResourcesType(), map[string]attr.Value{
			"memory_shares": types.StringValue(res.GetMemoryShares()),
			"cpu_shares":    types.StringValue(res.GetCpuShares()),
		})
		diags.Append(d...)
		model.Resources = resourcesObj
	} else {
		model.Resources = types.ObjectNull(models.GetPipelineResourcesType())
	}

	if pipeline.GetTags() != nil && len(pipeline.GetTags()) > 0 {
		tagsMap, d := types.MapValueFrom(ctx, types.StringType, pipeline.GetTags())
		diags.Append(d...)
		model.Tags = tagsMap
	} else {
		model.Tags = types.MapNull(types.StringType)
	}

	return model, diags
}

func extractResources(ctx context.Context, resourcesObj types.Object) (*dataplanev1.Pipeline_Resources, diag.Diagnostics) {
	var diags diag.Diagnostics

	if resourcesObj.IsNull() || resourcesObj.IsUnknown() {
		return nil, diags
	}

	var resources models.PipelineResources
	diags.Append(resourcesObj.As(ctx, &resources, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	result := &dataplanev1.Pipeline_Resources{}
	if !resources.MemoryShares.IsNull() && !resources.MemoryShares.IsUnknown() {
		result.MemoryShares = resources.MemoryShares.ValueString()
	}
	if !resources.CpuShares.IsNull() && !resources.CpuShares.IsUnknown() {
		result.CpuShares = resources.CpuShares.ValueString()
	}

	return result, diags
}

func extractTags(ctx context.Context, tagsMap types.Map) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if tagsMap.IsNull() || tagsMap.IsUnknown() {
		return nil, diags
	}

	tags := make(map[string]string)
	diags.Append(tagsMap.ElementsAs(ctx, &tags, false)...)

	return tags, diags
}

func pipelineStateToString(state dataplanev1.Pipeline_State) string {
	switch state {
	case dataplanev1.Pipeline_STATE_STARTING:
		return "starting"
	case dataplanev1.Pipeline_STATE_RUNNING:
		return "running"
	case dataplanev1.Pipeline_STATE_STOPPING:
		return "stopping"
	case dataplanev1.Pipeline_STATE_STOPPED:
		return "stopped"
	case dataplanev1.Pipeline_STATE_ERROR:
		return "error"
	case dataplanev1.Pipeline_STATE_COMPLETED:
		return "completed"
	default:
		return "unknown"
	}
}
