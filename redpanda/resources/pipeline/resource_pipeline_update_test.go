// Copyright 2026 Redpanda Data, Inc.
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

package pipeline

import (
	"context"
	"testing"
	"time"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	pipelinemodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/pipeline"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const abortedMessage = "the pipeline was modified by another request: reload it and retry"

// pipelineSim answers the dataplane calls Update makes against one pipeline.
// Start and Stop move the state instantly, as the integration fake does.
type pipelineSim struct {
	state   dataplanev1.Pipeline_State
	failAll error
	updates int
	stops   int
	starts  int
}

func newPipelineSim(t *testing.T, initial dataplanev1.Pipeline_State) (*pipelineSim, *mocks.MockPipelineServiceClient) {
	t.Helper()
	utils.SetTestModeWaits()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	sim := &pipelineSim{state: initial}
	client := mocks.NewMockPipelineServiceClient(ctrl)
	client.EXPECT().GetPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *dataplanev1.GetPipelineRequest, ...any) (*dataplanev1.GetPipelineResponse, error) {
			return &dataplanev1.GetPipelineResponse{Pipeline: sim.pipeline()}, nil
		}).AnyTimes()
	client.EXPECT().StopPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *dataplanev1.StopPipelineRequest, ...any) (*dataplanev1.StopPipelineResponse, error) {
			sim.stops++
			sim.state = dataplanev1.Pipeline_STATE_STOPPED
			return &dataplanev1.StopPipelineResponse{}, nil
		}).AnyTimes()
	client.EXPECT().StartPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *dataplanev1.StartPipelineRequest, ...any) (*dataplanev1.StartPipelineResponse, error) {
			sim.starts++
			sim.state = dataplanev1.Pipeline_STATE_RUNNING
			return &dataplanev1.StartPipelineResponse{}, nil
		}).AnyTimes()
	return sim, client
}

func (s *pipelineSim) pipeline() *dataplanev1.Pipeline {
	return createMockPipeline(testPipelineID, testDisplayName, testDescription, testConfigYaml, testPipelineURL, s.state, nil, nil)
}

// expectUpdates answers UpdatePipeline from errs in order, then with success.
func (s *pipelineSim) expectUpdates(client *mocks.MockPipelineServiceClient, errs ...error) {
	client.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *dataplanev1.UpdatePipelineRequest, ...any) (*dataplanev1.UpdatePipelineResponse, error) {
			s.updates++
			if s.failAll != nil {
				return nil, s.failAll
			}
			if s.updates <= len(errs) {
				return nil, errs[s.updates-1]
			}
			return &dataplanev1.UpdatePipelineResponse{Pipeline: s.pipeline()}, nil
		}).AnyTimes()
}

func pipelineStateModel(desiredState string) pipelinemodel.ResourceModel {
	m := newModelBuilder().WithID(testPipelineID).WithState(desiredState).Build()
	m.URL = types.StringValue(testPipelineURL)
	m.Status = types.ObjectNull(pipelinemodel.StatusAttrTypes())
	return m
}

func runUpdate(t *testing.T, client *mocks.MockPipelineServiceClient, state, plan pipelinemodel.ResourceModel) resource.UpdateResponse {
	t.Helper()
	ctx := context.Background()
	r := setupPipelineResource(client)
	sch := ResourcePipelineSchema(ctx)

	req := resource.UpdateRequest{
		State:  tfsdk.State{Schema: sch},
		Plan:   tfsdk.Plan{Schema: sch},
		Config: tfsdk.Config{Schema: sch},
	}
	require.False(t, req.State.Set(ctx, &state).HasError())
	require.False(t, req.Plan.Set(ctx, &plan).HasError())
	req.Config.Raw = req.Plan.Raw

	resp := resource.UpdateResponse{State: tfsdk.State{Schema: sch}}
	r.Update(ctx, req, &resp)
	return resp
}

func recordedState(t *testing.T, resp resource.UpdateResponse) string {
	t.Helper()
	var s types.String
	require.False(t, resp.State.GetAttribute(context.Background(), path.Root("state"), &s).HasError())
	return s.ValueString()
}

func TestUnit_Pipeline_Update_StateOnlyChangeSkipsTheWrite(t *testing.T) {
	tests := []struct {
		name         string
		initial      dataplanev1.Pipeline_State
		from, to     string
		wantStops    int
		wantStarts   int
		wantRecorded string
	}{
		{"running to stopped", dataplanev1.Pipeline_STATE_RUNNING, pipelinemodel.StateRunning, pipelinemodel.StateStopped, 1, 0, pipelinemodel.StateStopped},
		{"stopped to running", dataplanev1.Pipeline_STATE_STOPPED, pipelinemodel.StateStopped, pipelinemodel.StateRunning, 0, 1, pipelinemodel.StateRunning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim, client := newPipelineSim(t, tt.initial)
			sim.expectUpdates(client)

			resp := runUpdate(t, client, pipelineStateModel(tt.from), pipelineStateModel(tt.to))

			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.Zero(t, sim.updates, "a state-only change must not issue UpdatePipeline")
			assert.Equal(t, tt.wantStops, sim.stops)
			assert.Equal(t, tt.wantStarts, sim.starts)
			assert.Equal(t, tt.wantRecorded, recordedState(t, resp))
		})
	}
}

func TestUnit_Pipeline_Update_AnyUpdatableFieldChangeIssuesTheWrite(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pipelinemodel.ResourceModel)
	}{
		{"display_name", func(m *pipelinemodel.ResourceModel) { m.DisplayName = types.StringValue("renamed") }},
		{"description", func(m *pipelinemodel.ResourceModel) { m.Description = types.StringValue("new description") }},
		{"config_yaml", func(m *pipelinemodel.ResourceModel) {
			m.ConfigYaml = types.StringValue("input:\n  generate: {}\noutput:\n  drop: {}")
		}},
		{"tags", func(m *pipelinemodel.ResourceModel) { m.Tags = createTagsMap(map[string]string{"env": "prod"}) }},
		{"resources", func(m *pipelinemodel.ResourceModel) { m.Resources = createResourcesObject("100m", "256Mi") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim, client := newPipelineSim(t, dataplanev1.Pipeline_STATE_STOPPED)
			sim.expectUpdates(client)

			plan := pipelineStateModel(pipelinemodel.StateStopped)
			tt.mutate(&plan)
			resp := runUpdate(t, client, pipelineStateModel(pipelinemodel.StateStopped), plan)

			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.Equal(t, 1, sim.updates)
		})
	}
}

func TestUnit_Pipeline_Update_RetriesAbortedWrite(t *testing.T) {
	sim, client := newPipelineSim(t, dataplanev1.Pipeline_STATE_RUNNING)
	sim.expectUpdates(client, status.Error(codes.Aborted, abortedMessage))

	plan := pipelineStateModel(pipelinemodel.StateRunning)
	plan.DisplayName = types.StringValue("renamed")
	resp := runUpdate(t, client, pipelineStateModel(pipelinemodel.StateRunning), plan)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, 2, sim.updates, "one Aborted, then success")
	assert.Equal(t, 1, sim.starts, "the pipeline is restarted after the write")
	assert.Equal(t, pipelinemodel.StateRunning, recordedState(t, resp))
}

func TestUnit_Pipeline_Update_AbortedThatNeverClearsFails(t *testing.T) {
	prev := updateConflictBudget
	updateConflictBudget = 50 * time.Millisecond
	t.Cleanup(func() { updateConflictBudget = prev })

	sim, client := newPipelineSim(t, dataplanev1.Pipeline_STATE_RUNNING)
	sim.failAll = status.Error(codes.Aborted, abortedMessage)
	sim.expectUpdates(client)

	plan := pipelineStateModel(pipelinemodel.StateRunning)
	plan.DisplayName = types.StringValue("renamed")
	resp := runUpdate(t, client, pipelineStateModel(pipelinemodel.StateRunning), plan)

	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), testPipelineID)
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), abortedMessage)
	assert.Greater(t, sim.updates, 1)
}

func TestUnit_Pipeline_Update_OtherWriteErrorsAreNotRetried(t *testing.T) {
	for _, code := range []codes.Code{codes.Internal, codes.InvalidArgument, codes.FailedPrecondition} {
		t.Run(code.String(), func(t *testing.T) {
			sim, client := newPipelineSim(t, dataplanev1.Pipeline_STATE_STOPPED)
			sim.expectUpdates(client, status.Error(code, "rejected"))

			plan := pipelineStateModel(pipelinemodel.StateStopped)
			plan.DisplayName = types.StringValue("renamed")
			resp := runUpdate(t, client, pipelineStateModel(pipelinemodel.StateStopped), plan)

			require.True(t, resp.Diagnostics.HasError())
			assert.Equal(t, 1, sim.updates)
		})
	}
}

func TestUnit_Pipeline_Update_ProviderOnlyChangeLeavesARunningPipelineAlone(t *testing.T) {
	sim, client := newPipelineSim(t, dataplanev1.Pipeline_STATE_RUNNING)
	sim.expectUpdates(client)

	state := pipelineStateModel(pipelinemodel.StateRunning)
	plan := pipelineStateModel(pipelinemodel.StateRunning)
	plan.AllowDeletion = types.BoolValue(true)
	resp := runUpdate(t, client, state, plan)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Zero(t, sim.stops, "allow_deletion is not part of the pipeline; it must not bounce it")
	assert.Zero(t, sim.updates)
	assert.Zero(t, sim.starts)
	assert.Equal(t, pipelinemodel.StateRunning, recordedState(t, resp))
}
