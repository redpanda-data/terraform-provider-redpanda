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

package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	pipelinemodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// Test constants for common values.
const (
	testClusterAPIURL = "https://api.cluster.example.com"
	testPipelineID    = "pipeline-123"
	testDisplayName   = "test-pipeline"
	testDescription   = "A test pipeline"
	testConfigYaml    = "input:\n  generate: {}\noutput:\n  stdout: {}"
	testPipelineURL   = "https://pipeline.example.com"
	testMemoryShares  = "512Mi"
	testCPUShares     = "500m"
	testAuthToken     = "test-token"
	testProviderVer   = "1.0.0"
	testTerraformVer  = "1.5.0"
)

// pipelineInput defines the input parameters for creating/updating a pipeline.
type pipelineInput struct {
	displayName  string
	description  string
	configYaml   string
	desiredState string
	cpuShares    string
	memoryShares string
	tags         map[string]string
}

// pipelineExistingState represents existing state before an update operation.
type pipelineExistingState struct {
	id          string
	displayName string
	description string
	configYaml  string
	state       string
}

// pipelineAPIResponse represents what the API returns.
type pipelineAPIResponse struct {
	id           string
	state        dataplanev1.Pipeline_State
	url          string
	cpuShares    string
	memoryShares string
	tags         map[string]string
}

// createDefaultContingentFields creates a ContingentFields with default test values.
func createDefaultContingentFields() pipelinemodel.ContingentFields {
	return pipelinemodel.ContingentFields{
		ClusterAPIURL: types.StringValue(testClusterAPIURL),
		AllowDeletion: types.BoolNull(),
		Resources:     types.ObjectNull(pipelinemodel.GetResourcesType()),
		State:         types.StringNull(),
		Timeouts:      timeouts.Value{},
	}
}

// createResourcesObject creates a types.Object for pipeline resources.
// Returns null object if both values are empty.
func createResourcesObject(cpuShares, memoryShares string) types.Object {
	if cpuShares == "" && memoryShares == "" {
		return types.ObjectNull(pipelinemodel.GetResourcesType())
	}
	obj, _ := types.ObjectValue(pipelinemodel.GetResourcesType(), map[string]attr.Value{
		pipelinemodel.FieldCPUShares:    types.StringValue(cpuShares),
		pipelinemodel.FieldMemoryShares: types.StringValue(memoryShares),
	})
	return obj
}

// createTagsMap creates a types.Map for pipeline tags.
// Returns null map if tags is nil or empty.
func createTagsMap(tags map[string]string) types.Map {
	if len(tags) == 0 {
		return types.MapNull(types.StringType)
	}
	tagValues := make(map[string]attr.Value)
	for k, v := range tags {
		tagValues[k] = types.StringValue(v)
	}
	m, _ := types.MapValue(types.StringType, tagValues)
	return m
}

// createMockPipeline creates a dataplanev1.Pipeline from the given parameters.
func createMockPipeline(id, displayName, description, configYaml, url string, state dataplanev1.Pipeline_State, resources *dataplanev1.Pipeline_Resources, tags map[string]string) *dataplanev1.Pipeline {
	p := &dataplanev1.Pipeline{
		Id:          id,
		DisplayName: displayName,
		Description: description,
		ConfigYaml:  configYaml,
		State:       state,
		Url:         url,
	}
	if resources != nil {
		p.Resources = resources
	}
	if len(tags) > 0 {
		p.Tags = tags
	}
	return p
}

// createMockPipelineResources creates a dataplanev1.Pipeline_Resources.
// Returns nil if both values are empty.
func createMockPipelineResources(cpuShares, memoryShares string) *dataplanev1.Pipeline_Resources {
	if cpuShares == "" && memoryShares == "" {
		return nil
	}
	return &dataplanev1.Pipeline_Resources{
		CpuShares:    cpuShares,
		MemoryShares: memoryShares,
	}
}

// setupPipelineResource creates a configured Pipeline resource for testing.
func setupPipelineResource(mockClient dataplanev1grpc.PipelineServiceClient) *Pipeline {
	return &Pipeline{
		clientFactory: func(_, _, _, _ string) (dataplanev1grpc.PipelineServiceClient, *grpc.ClientConn, error) {
			return mockClient, nil, nil
		},
		resData: config.Resource{
			AuthToken:        testAuthToken,
			ProviderVersion:  testProviderVer,
			TerraformVersion: testTerraformVer,
		},
	}
}

// setupCreateMocks configures mock expectations for a create operation.
func setupCreateMocks(mockClient *mocks.MockPipelineServiceClient, apiPipeline *dataplanev1.Pipeline, autoStarts bool) {
	mockClient.EXPECT().
		CreatePipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&dataplanev1.CreatePipelineResponse{Pipeline: apiPipeline}, nil)

	if autoStarts {
		stoppedPipeline := &dataplanev1.Pipeline{
			Id:          apiPipeline.Id,
			DisplayName: apiPipeline.DisplayName,
			Description: apiPipeline.Description,
			ConfigYaml:  apiPipeline.ConfigYaml,
			State:       dataplanev1.Pipeline_STATE_STOPPED,
			Url:         apiPipeline.Url,
			Resources:   apiPipeline.Resources,
			Tags:        apiPipeline.Tags,
		}
		mockClient.EXPECT().
			StopPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&dataplanev1.StopPipelineResponse{}, nil)
		mockClient.EXPECT().
			GetPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&dataplanev1.GetPipelineResponse{Pipeline: stoppedPipeline}, nil).
			AnyTimes()
	} else {
		mockClient.EXPECT().
			GetPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&dataplanev1.GetPipelineResponse{Pipeline: apiPipeline}, nil)
	}
}

// setupUpdateMocks configures mock expectations for an update operation.
func setupUpdateMocks(mockClient *mocks.MockPipelineServiceClient, beforePipeline, afterPipeline *dataplanev1.Pipeline) {
	mockClient.EXPECT().
		GetPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&dataplanev1.GetPipelineResponse{Pipeline: beforePipeline}, nil)

	mockClient.EXPECT().
		UpdatePipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&dataplanev1.UpdatePipelineResponse{Pipeline: afterPipeline}, nil)

	mockClient.EXPECT().
		GetPipeline(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&dataplanev1.GetPipelineResponse{Pipeline: afterPipeline}, nil)
}

func createTestTimeouts() timeouts.Value {
	timeoutsAttrTypes := map[string]attr.Type{
		"create": types.StringType,
		"update": types.StringType,
		"delete": types.StringType,
	}
	timeoutsObj, _ := types.ObjectValueFrom(context.Background(), timeoutsAttrTypes, map[string]attr.Value{
		"create": types.StringNull(),
		"update": types.StringNull(),
		"delete": types.StringNull(),
	})
	return timeouts.Value{Object: timeoutsObj}
}

func TestPipelineStateToString(t *testing.T) {
	tests := []struct {
		name     string
		input    dataplanev1.Pipeline_State
		expected string
	}{
		{"starting", dataplanev1.Pipeline_STATE_STARTING, pipelinemodel.StateStarting},
		{"running", dataplanev1.Pipeline_STATE_RUNNING, pipelinemodel.StateRunning},
		{"stopping", dataplanev1.Pipeline_STATE_STOPPING, pipelinemodel.StateStopping},
		{"stopped", dataplanev1.Pipeline_STATE_STOPPED, pipelinemodel.StateStopped},
		{"error", dataplanev1.Pipeline_STATE_ERROR, pipelinemodel.StateError},
		{"completed", dataplanev1.Pipeline_STATE_COMPLETED, pipelinemodel.StateCompleted},
		{"unspecified", dataplanev1.Pipeline_STATE_UNSPECIFIED, pipelinemodel.StateUnknown},
		{"invalid", dataplanev1.Pipeline_State(999), pipelinemodel.StateUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pipelinemodel.StateToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipelineToModel(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		pipeline        *dataplanev1.Pipeline
		wantID          string
		wantDisplayName string
		wantState       string
	}{
		{
			name: "basic pipeline",
			pipeline: createMockPipeline(
				testPipelineID, testDisplayName, testDescription, testConfigYaml, testPipelineURL,
				dataplanev1.Pipeline_STATE_STOPPED, nil, nil,
			),
			wantID:          testPipelineID,
			wantDisplayName: testDisplayName,
			wantState:       pipelinemodel.StateStopped,
		},
		{
			name: "running pipeline",
			pipeline: createMockPipeline(
				"pipeline-456", "running-pipeline", "", "", "",
				dataplanev1.Pipeline_STATE_RUNNING, nil, nil,
			),
			wantID:          "pipeline-456",
			wantDisplayName: "running-pipeline",
			wantState:       pipelinemodel.StateRunning,
		},
		{
			name: "pipeline with resources",
			pipeline: createMockPipeline(
				"pipeline-789", "resourced-pipeline", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPED,
				createMockPipelineResources(testCPUShares, testMemoryShares),
				nil,
			),
			wantID:          "pipeline-789",
			wantDisplayName: "resourced-pipeline",
			wantState:       pipelinemodel.StateStopped,
		},
		{
			name: "pipeline with tags",
			pipeline: createMockPipeline(
				"pipeline-tags", "tagged-pipeline", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPED, nil,
				map[string]string{"environment": "test", "team": "data"},
			),
			wantID:          "pipeline-tags",
			wantDisplayName: "tagged-pipeline",
			wantState:       pipelinemodel.StateStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &pipelinemodel.ResourceModel{}
			model, diags := model.GetUpdatedModel(ctx, tt.pipeline, createDefaultContingentFields())
			require.False(t, diags.HasError())

			assert.Equal(t, tt.wantID, model.ID.ValueString())
			assert.Equal(t, tt.wantDisplayName, model.DisplayName.ValueString())
			assert.Equal(t, tt.wantState, model.State.ValueString())
			assert.Equal(t, testClusterAPIURL, model.ClusterAPIURL.ValueString())

			if tt.pipeline.HasResources() {
				assert.False(t, model.Resources.IsNull())
			}
			if len(tt.pipeline.GetTags()) > 0 {
				assert.False(t, model.Tags.IsNull())
			}
		})
	}
}

func TestPipelineSchema(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{}

	schemaResp := resource.SchemaResponse{}
	p.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	require.False(t, schemaResp.Diagnostics.HasError())

	schema := schemaResp.Schema

	requiredAttrs := []string{"cluster_api_url", "display_name", "config_yaml"}
	for _, name := range requiredAttrs {
		schemaAttr, ok := schema.Attributes[name]
		require.True(t, ok)
		assert.True(t, schemaAttr.IsRequired())
	}

	computedOnlyAttrs := []string{"id", "url"}
	for _, name := range computedOnlyAttrs {
		schemaAttr, ok := schema.Attributes[name]
		require.True(t, ok)
		assert.True(t, schemaAttr.IsComputed())
	}

	optionalAttrs := []string{"description", "tags", "resources", "state"}
	for _, name := range optionalAttrs {
		schemaAttr, ok := schema.Attributes[name]
		require.True(t, ok)
		assert.True(t, schemaAttr.IsOptional())
	}

	_, ok := schema.Attributes["timeouts"]
	assert.True(t, ok)
}

func TestStatesEquivalent(t *testing.T) {
	tests := []struct {
		name     string
		prior    string
		current  string
		expected bool
	}{
		{"running to running", "running", "running", true},
		{"running to starting", "running", "starting", true},
		{"starting to running", "starting", "running", true},
		{"stopped to stopped", "stopped", "stopped", true},
		{"stopped to stopping", "stopped", "stopping", true},
		{"stopped to completed", "stopped", "completed", true},
		{"running to stopped", "running", "stopped", false},
		{"stopped to running", "stopped", "running", false},
		{"running to error", "running", "error", false},
		{"stopped to error", "stopped", "error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pipelinemodel.StatesEquivalent(tt.prior, tt.current)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDesiredStateFromAPIState(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"starting", pipelinemodel.StateStarting, pipelinemodel.StateRunning},
		{"running", pipelinemodel.StateRunning, pipelinemodel.StateRunning},
		{"stopping", pipelinemodel.StateStopping, pipelinemodel.StateStopped},
		{"stopped", pipelinemodel.StateStopped, pipelinemodel.StateStopped},
		{"completed", pipelinemodel.StateCompleted, pipelinemodel.StateStopped},
		{"error", pipelinemodel.StateError, pipelinemodel.StateStopped},
		{"unknown", pipelinemodel.StateUnknown, pipelinemodel.StateStopped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pipelinemodel.DesiredStateFromAPIState(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipelineMetadata(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{}

	metaResp := resource.MetadataResponse{}
	p.Metadata(ctx, resource.MetadataRequest{}, &metaResp)

	assert.Equal(t, "redpanda_pipeline", metaResp.TypeName)
}

func TestGetUpdatedModel_EdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pipeline      *dataplanev1.Pipeline
		contingent    pipelinemodel.ContingentFields
		expectedState string
		expectedID    string
		resourcesNull bool
		tagsNull      bool
	}{
		{
			name: "handles error state from API",
			pipeline: createMockPipeline(
				"pipeline-error", "error-pipeline", "", "", "",
				dataplanev1.Pipeline_STATE_ERROR, nil, nil,
			),
			contingent:    createDefaultContingentFields(),
			expectedState: pipelinemodel.StateStopped, // error state normalizes to stopped
			expectedID:    "pipeline-error",
			resourcesNull: true,
			tagsNull:      true,
		},
		{
			name: "handles unspecified state from API",
			pipeline: createMockPipeline(
				"pipeline-unspec", "unspecified-pipeline", "", "", "",
				dataplanev1.Pipeline_STATE_UNSPECIFIED, nil, nil,
			),
			contingent:    createDefaultContingentFields(),
			expectedState: pipelinemodel.StateStopped, // unknown state normalizes to stopped
			expectedID:    "pipeline-unspec",
			resourcesNull: true,
			tagsNull:      true,
		},
		{
			name: "handles empty string fields from API",
			pipeline: createMockPipeline(
				"pipeline-empty", "", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPED, nil, nil,
			),
			contingent:    createDefaultContingentFields(),
			expectedState: pipelinemodel.StateStopped,
			expectedID:    "pipeline-empty",
			resourcesNull: true,
			tagsNull:      true,
		},
		{
			name: "handles empty tags map from API",
			pipeline: createMockPipeline(
				"pipeline-empty-tags", "test", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPED, nil, map[string]string{},
			),
			contingent:    createDefaultContingentFields(),
			expectedState: pipelinemodel.StateStopped,
			expectedID:    "pipeline-empty-tags",
			resourcesNull: true,
			tagsNull:      true, // empty map should result in null
		},
		{
			name: "handles nil resources from API",
			pipeline: createMockPipeline(
				"pipeline-nil-resources", "test", "", "", "",
				dataplanev1.Pipeline_STATE_RUNNING, nil, nil,
			),
			contingent:    createDefaultContingentFields(),
			expectedState: pipelinemodel.StateRunning,
			expectedID:    "pipeline-nil-resources",
			resourcesNull: true,
			tagsNull:      true,
		},
		{
			name: "state mismatch between prior and API triggers normalization",
			pipeline: createMockPipeline(
				"pipeline-mismatch", "test", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPED, nil, nil,
			),
			contingent: pipelinemodel.ContingentFields{
				ClusterAPIURL: types.StringValue(testClusterAPIURL),
				AllowDeletion: types.BoolNull(),
				Resources:     types.ObjectNull(pipelinemodel.GetResourcesType()),
				State:         types.StringValue(pipelinemodel.StateRunning), // prior was running, now stopped
				Timeouts:      timeouts.Value{},
			},
			expectedState: pipelinemodel.StateStopped, // should update to actual state
			expectedID:    "pipeline-mismatch",
			resourcesNull: true,
			tagsNull:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &pipelinemodel.ResourceModel{}
			model, diags := model.GetUpdatedModel(ctx, tt.pipeline, tt.contingent)
			require.False(t, diags.HasError(), "unexpected error: %v", diags)

			assert.Equal(t, tt.expectedID, model.ID.ValueString())
			assert.Equal(t, tt.expectedState, model.State.ValueString())
			assert.Equal(t, tt.resourcesNull, model.Resources.IsNull(), "resources null mismatch")
			assert.Equal(t, tt.tagsNull, model.Tags.IsNull(), "tags null mismatch")
		})
	}
}

func TestExtractResources_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Helper to create object with null values for specific fields.
	createResourcesWithNulls := func(cpuNull, memNull bool, cpuVal, memVal string) types.Object {
		cpuAttr := types.StringValue(cpuVal)
		memAttr := types.StringValue(memVal)
		if cpuNull {
			cpuAttr = types.StringNull()
		}
		if memNull {
			memAttr = types.StringNull()
		}
		obj, _ := types.ObjectValue(pipelinemodel.GetResourcesType(), map[string]attr.Value{
			pipelinemodel.FieldCPUShares:    cpuAttr,
			pipelinemodel.FieldMemoryShares: memAttr,
		})
		return obj
	}

	tests := []struct {
		name         string
		resources    types.Object
		expectNil    bool
		expectMemory string
		expectCPU    string
	}{
		{
			name:      "null resources returns nil",
			resources: createResourcesObject("", ""),
			expectNil: true,
		},
		{
			name:      "unknown resources returns nil",
			resources: types.ObjectUnknown(pipelinemodel.GetResourcesType()),
			expectNil: true,
		},
		{
			name:         "partial resources - only memory_shares set",
			resources:    createResourcesWithNulls(true, false, "", testMemoryShares),
			expectNil:    false,
			expectMemory: testMemoryShares,
			expectCPU:    "",
		},
		{
			name:         "partial resources - only cpu_shares set",
			resources:    createResourcesWithNulls(false, true, "250m", ""),
			expectNil:    false,
			expectMemory: "",
			expectCPU:    "250m",
		},
		{
			name:         "both fields null returns empty struct",
			resources:    createResourcesWithNulls(true, true, "", ""),
			expectNil:    false,
			expectMemory: "",
			expectCPU:    "",
		},
		{
			name:         "empty string values",
			resources:    createResourcesObject("", ""),
			expectNil:    true,
			expectMemory: "",
			expectCPU:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &pipelinemodel.ResourceModel{
				Resources: tt.resources,
			}
			result, diags := model.ExtractResources(ctx)
			require.False(t, diags.HasError(), "unexpected error: %v", diags)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectMemory, result.MemoryShares)
				assert.Equal(t, tt.expectCPU, result.CpuShares)
			}
		})
	}
}

func TestExtractTags_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Helper to create non-null empty map (createTagsMap returns null for empty).
	createEmptyTagsMap := func() types.Map {
		m, _ := types.MapValue(types.StringType, map[string]attr.Value{})
		return m
	}

	tests := []struct {
		name      string
		tags      types.Map
		expectNil bool
		expectLen int
	}{
		{
			name:      "null tags returns nil",
			tags:      createTagsMap(nil),
			expectNil: true,
		},
		{
			name:      "unknown tags returns nil",
			tags:      types.MapUnknown(types.StringType),
			expectNil: true,
		},
		{
			name:      "empty tags map returns empty map",
			tags:      createEmptyTagsMap(),
			expectNil: false,
			expectLen: 0,
		},
		{
			name:      "single tag",
			tags:      createTagsMap(map[string]string{"env": "test"}),
			expectNil: false,
			expectLen: 1,
		},
		{
			name:      "multiple tags",
			tags:      createTagsMap(map[string]string{"env": "prod", "team": "data", "app": "pipeline"}),
			expectNil: false,
			expectLen: 3,
		},
		{
			name:      "tags with empty string values",
			tags:      createTagsMap(map[string]string{"empty": ""}),
			expectNil: false,
			expectLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &pipelinemodel.ResourceModel{
				Tags: tt.tags,
			}
			result, diags := model.ExtractTags(ctx)
			require.False(t, diags.HasError(), "unexpected error: %v", diags)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Len(t, result, tt.expectLen)
			}
		})
	}
}

func TestPipelineToModelPreservesPlannedValues(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		pipeline          *dataplanev1.Pipeline
		plannedState      types.String
		plannedResources  types.Object
		expectedState     string
		expectedResources bool
	}{
		{
			name: "preserves running state when API returns starting",
			pipeline: createMockPipeline(
				"pipeline-1", "test", "", "", "",
				dataplanev1.Pipeline_STATE_STARTING, nil, nil,
			),
			plannedState:     types.StringValue(pipelinemodel.StateRunning),
			plannedResources: createResourcesObject("", ""),
			expectedState:    pipelinemodel.StateRunning,
		},
		{
			name: "preserves stopped state when API returns stopping",
			pipeline: createMockPipeline(
				"pipeline-2", "test", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPING, nil, nil,
			),
			plannedState:     types.StringValue(pipelinemodel.StateStopped),
			plannedResources: createResourcesObject("", ""),
			expectedState:    pipelinemodel.StateStopped,
		},
		{
			name: "preserves stopped state when API returns completed",
			pipeline: createMockPipeline(
				"pipeline-3", "test", "", "", "",
				dataplanev1.Pipeline_STATE_COMPLETED, nil, nil,
			),
			plannedState:     types.StringValue(pipelinemodel.StateStopped),
			plannedResources: createResourcesObject("", ""),
			expectedState:    pipelinemodel.StateStopped,
		},
		{
			name: "preserves user-configured resources when API normalizes values",
			pipeline: createMockPipeline(
				"pipeline-4", "test", "", "", "",
				dataplanev1.Pipeline_STATE_STOPPED,
				createMockPipelineResources("300m", "1200M"),
				nil,
			),
			plannedState:      types.StringValue(pipelinemodel.StateStopped),
			plannedResources:  createResourcesObject("100m", "256Mi"),
			expectedState:     pipelinemodel.StateStopped,
			expectedResources: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &pipelinemodel.ResourceModel{}
			model, diags := model.GetUpdatedModel(
				ctx,
				tt.pipeline,
				pipelinemodel.ContingentFields{
					ClusterAPIURL: types.StringValue(testClusterAPIURL),
					AllowDeletion: types.BoolNull(),
					Resources:     tt.plannedResources,
					State:         tt.plannedState,
					Timeouts:      timeouts.Value{},
				},
			)
			require.False(t, diags.HasError())
			assert.Equal(t, tt.expectedState, model.State.ValueString())
			if tt.expectedResources && !tt.plannedResources.IsNull() {
				assert.Equal(t, tt.plannedResources, model.Resources)
			}
		})
	}
}

func TestPipeline_PlanApplyConsistency(t *testing.T) {
	tests := []struct {
		name          string
		operation     string
		inputPipeline pipelineInput
		existingState *pipelineExistingState
		apiReturns    pipelineAPIResponse
		apiAutoStarts bool
		expectedState string
	}{
		{
			name:      "basic_create_read",
			operation: "create",
			inputPipeline: pipelineInput{
				displayName:  testDisplayName,
				description:  testDescription,
				configYaml:   testConfigYaml,
				desiredState: pipelinemodel.StateStopped,
			},
			apiReturns: pipelineAPIResponse{
				id:    testPipelineID,
				state: dataplanev1.Pipeline_STATE_STOPPED,
				url:   testPipelineURL,
			},
			expectedState: pipelinemodel.StateStopped,
		},
		{
			name:      "create_stops_autostarted_pipeline",
			operation: "create",
			inputPipeline: pipelineInput{
				displayName:  testDisplayName,
				configYaml:   testConfigYaml,
				desiredState: pipelinemodel.StateStopped,
			},
			apiReturns: pipelineAPIResponse{
				id:    "pipeline-autostart",
				state: dataplanev1.Pipeline_STATE_RUNNING, // API auto-starts it
			},
			apiAutoStarts: true,
			expectedState: pipelinemodel.StateStopped,
		},
		{
			name:      "update_read_consistency",
			operation: "update",
			existingState: &pipelineExistingState{
				id:          "pipeline-update",
				displayName: "old-name",
				description: "old description",
				configYaml:  testConfigYaml,
				state:       pipelinemodel.StateStopped,
			},
			inputPipeline: pipelineInput{
				displayName:  "new-name",
				description:  "new description",
				configYaml:   "input:\n  generate: {}\noutput:\n  drop: {}",
				desiredState: pipelinemodel.StateStopped,
			},
			apiReturns: pipelineAPIResponse{
				id:    "pipeline-update",
				state: dataplanev1.Pipeline_STATE_STOPPED,
			},
			expectedState: pipelinemodel.StateStopped,
		},
		{
			name:      "all_fields_with_resources_and_tags",
			operation: "create",
			inputPipeline: pipelineInput{
				displayName:  "full-pipeline",
				description:  "A fully configured pipeline",
				configYaml:   "input:\n  generate:\n    interval: 1s\noutput:\n  stdout: {}",
				desiredState: pipelinemodel.StateStopped,
				cpuShares:    testCPUShares,
				memoryShares: testMemoryShares,
				tags:         map[string]string{"environment": "test", "team": "data"},
			},
			apiReturns: pipelineAPIResponse{
				id:           "pipeline-full",
				state:        dataplanev1.Pipeline_STATE_STOPPED,
				url:          "https://pipeline.example.com/full",
				cpuShares:    testCPUShares,
				memoryShares: testMemoryShares,
				tags:         map[string]string{"environment": "test", "team": "data"},
			},
			expectedState: pipelinemodel.StateStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockClient := mocks.NewMockPipelineServiceClient(ctrl)

			apiPipeline := createMockPipeline(
				tt.apiReturns.id,
				tt.inputPipeline.displayName,
				tt.inputPipeline.description,
				tt.inputPipeline.configYaml,
				tt.apiReturns.url,
				tt.apiReturns.state,
				createMockPipelineResources(tt.apiReturns.cpuShares, tt.apiReturns.memoryShares),
				tt.apiReturns.tags,
			)

			switch tt.operation {
			case "create":
				setupCreateMocks(mockClient, apiPipeline, tt.apiAutoStarts)
			case "update":
				beforePipeline := createMockPipeline(
					tt.existingState.id,
					tt.existingState.displayName,
					tt.existingState.description,
					tt.existingState.configYaml,
					"",
					dataplanev1.Pipeline_STATE_STOPPED,
					nil, nil,
				)
				setupUpdateMocks(mockClient, beforePipeline, apiPipeline)
			}

			r := setupPipelineResource(mockClient)

			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			require.False(t, schemaResp.Diagnostics.HasError())

			resourcesObj := createResourcesObject(tt.inputPipeline.cpuShares, tt.inputPipeline.memoryShares)
			tagsMap := createTagsMap(tt.inputPipeline.tags)

			var stateAfterOp, stateAfterRead pipelinemodel.ResourceModel

			switch tt.operation {
			case "create":
				input := pipelinemodel.ResourceModel{
					ID:            types.StringUnknown(),
					ClusterAPIURL: types.StringValue(testClusterAPIURL),
					DisplayName:   types.StringValue(tt.inputPipeline.displayName),
					Description:   types.StringValue(tt.inputPipeline.description),
					ConfigYaml:    types.StringValue(tt.inputPipeline.configYaml),
					State:         types.StringValue(tt.inputPipeline.desiredState),
					URL:           types.StringUnknown(),
					Resources:     resourcesObj,
					Tags:          tagsMap,
					Timeouts:      createTestTimeouts(),
				}

				createReq := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
				diags := createReq.Plan.Set(ctx, &input)
				require.False(t, diags.HasError(), "Plan.Set error: %v", diags)

				createResp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
				r.Create(ctx, createReq, &createResp)
				require.False(t, createResp.Diagnostics.HasError(), "Create error: %v", createResp.Diagnostics)

				diags = createResp.State.Get(ctx, &stateAfterOp)
				require.False(t, diags.HasError())

				if tt.apiAutoStarts {
					assert.Equal(t, tt.expectedState, stateAfterOp.State.ValueString(),
						"State should match planned value, not API auto-started state")
					return
				}

				r.PipelineClient = nil
				r.dataplaneConn = nil

				readReq := resource.ReadRequest{State: createResp.State}
				readResp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
				r.Read(ctx, readReq, &readResp)
				require.False(t, readResp.Diagnostics.HasError(), "Read error: %v", readResp.Diagnostics)

				diags = readResp.State.Get(ctx, &stateAfterRead)
				require.False(t, diags.HasError())

			case "update":
				currentState := pipelinemodel.ResourceModel{
					ID:            types.StringValue(tt.existingState.id),
					ClusterAPIURL: types.StringValue(testClusterAPIURL),
					DisplayName:   types.StringValue(tt.existingState.displayName),
					Description:   types.StringValue(tt.existingState.description),
					ConfigYaml:    types.StringValue(tt.existingState.configYaml),
					State:         types.StringValue(tt.existingState.state),
					URL:           types.StringValue(""),
					Resources:     createResourcesObject("", ""),
					Tags:          createTagsMap(nil),
					Timeouts:      createTestTimeouts(),
				}

				planModel := pipelinemodel.ResourceModel{
					ID:            types.StringValue(tt.existingState.id),
					ClusterAPIURL: types.StringValue(testClusterAPIURL),
					DisplayName:   types.StringValue(tt.inputPipeline.displayName),
					Description:   types.StringValue(tt.inputPipeline.description),
					ConfigYaml:    types.StringValue(tt.inputPipeline.configYaml),
					State:         types.StringValue(tt.inputPipeline.desiredState),
					URL:           types.StringUnknown(),
					Resources:     resourcesObj,
					Tags:          tagsMap,
					Timeouts:      createTestTimeouts(),
				}

				updateReq := resource.UpdateRequest{
					State: tfsdk.State{Schema: schemaResp.Schema},
					Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
				}
				diags := updateReq.State.Set(ctx, &currentState)
				require.False(t, diags.HasError())
				diags = updateReq.Plan.Set(ctx, &planModel)
				require.False(t, diags.HasError())

				updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
				r.Update(ctx, updateReq, &updateResp)
				require.False(t, updateResp.Diagnostics.HasError(), "Update error: %v", updateResp.Diagnostics)

				diags = updateResp.State.Get(ctx, &stateAfterOp)
				require.False(t, diags.HasError())

				r.PipelineClient = nil
				r.dataplaneConn = nil

				readReq := resource.ReadRequest{State: updateResp.State}
				readResp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
				r.Read(ctx, readReq, &readResp)
				require.False(t, readResp.Diagnostics.HasError(), "Read error: %v", readResp.Diagnostics)

				diags = readResp.State.Get(ctx, &stateAfterRead)
				require.False(t, diags.HasError())
			}

			t.Run("field_consistency", func(t *testing.T) {
				fields := []struct {
					name     string
					opValue  string
					readVal  string
					nullable bool
				}{
					{"ID", stateAfterOp.ID.ValueString(), stateAfterRead.ID.ValueString(), false},
					{"ClusterAPIURL", stateAfterOp.ClusterAPIURL.ValueString(), stateAfterRead.ClusterAPIURL.ValueString(), false},
					{"DisplayName", stateAfterOp.DisplayName.ValueString(), stateAfterRead.DisplayName.ValueString(), false},
					{"Description", stateAfterOp.Description.ValueString(), stateAfterRead.Description.ValueString(), true},
					{"ConfigYaml", stateAfterOp.ConfigYaml.ValueString(), stateAfterRead.ConfigYaml.ValueString(), false},
					{"State", stateAfterOp.State.ValueString(), stateAfterRead.State.ValueString(), false},
					{"URL", stateAfterOp.URL.ValueString(), stateAfterRead.URL.ValueString(), true},
				}

				for _, f := range fields {
					assert.Equal(t, f.opValue, f.readVal, "%s mismatch: op=%q read=%q", f.name, f.opValue, f.readVal)
				}

				assert.Equal(t, stateAfterOp.Resources.IsNull(), stateAfterRead.Resources.IsNull(),
					"Resources null mismatch: op=%v read=%v", stateAfterOp.Resources.IsNull(), stateAfterRead.Resources.IsNull())
				if !stateAfterOp.Resources.IsNull() {
					assert.Equal(t, stateAfterOp.Resources, stateAfterRead.Resources, "Resources values mismatch")
				}

				assert.Equal(t, stateAfterOp.Tags.IsNull(), stateAfterRead.Tags.IsNull(),
					"Tags null mismatch: op=%v read=%v", stateAfterOp.Tags.IsNull(), stateAfterRead.Tags.IsNull())
				if !stateAfterOp.Tags.IsNull() {
					assert.Equal(t, stateAfterOp.Tags, stateAfterRead.Tags, "Tags values mismatch")
				}
			})

			assert.Equal(t, tt.expectedState, stateAfterOp.State.ValueString(),
				"Final state should be %q", tt.expectedState)
		})
	}
}

func TestPipeline_CreateErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockPipelineServiceClient(ctrl)

	// Simulate API error on create
	mockClient.EXPECT().
		CreatePipeline(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("API unavailable: service temporarily down"))

	r := setupPipelineResource(mockClient)

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	input := pipelinemodel.ResourceModel{
		ID:            types.StringUnknown(),
		ClusterAPIURL: types.StringValue(testClusterAPIURL),
		DisplayName:   types.StringValue(testDisplayName),
		Description:   types.StringValue(testDescription),
		ConfigYaml:    types.StringValue(testConfigYaml),
		State:         types.StringValue(pipelinemodel.StateStopped),
		URL:           types.StringUnknown(),
		Resources:     createResourcesObject("", ""),
		Tags:          createTagsMap(nil),
		Timeouts:      createTestTimeouts(),
	}

	createReq := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	diags := createReq.Plan.Set(ctx, &input)
	require.False(t, diags.HasError())

	createResp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, createReq, &createResp)

	// Verify that Create returned an error
	assert.True(t, createResp.Diagnostics.HasError(), "Expected error from Create when API fails")
	assert.Contains(t, createResp.Diagnostics.Errors()[0].Detail(), "API unavailable",
		"Error message should contain API error details")
}

func TestPipeline_DeleteBlockedByAllowDeletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockPipelineServiceClient(ctrl)

	// No API calls expected since deletion should be blocked before any API interaction

	r := setupPipelineResource(mockClient)

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	// State with allow_deletion = false (default)
	currentState := pipelinemodel.ResourceModel{
		ID:            types.StringValue(testPipelineID),
		ClusterAPIURL: types.StringValue(testClusterAPIURL),
		DisplayName:   types.StringValue(testDisplayName),
		Description:   types.StringValue(testDescription),
		ConfigYaml:    types.StringValue(testConfigYaml),
		State:         types.StringValue(pipelinemodel.StateStopped),
		URL:           types.StringValue(testPipelineURL),
		Resources:     createResourcesObject("", ""),
		Tags:          createTagsMap(nil),
		AllowDeletion: types.BoolValue(false), // Deletion not allowed
		Timeouts:      createTestTimeouts(),
	}

	deleteReq := resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	diags := deleteReq.State.Set(ctx, &currentState)
	require.False(t, diags.HasError())

	deleteResp := resource.DeleteResponse{}
	r.Delete(ctx, deleteReq, &deleteResp)

	// Verify that Delete returned an error and didn't proceed
	assert.True(t, deleteResp.Diagnostics.HasError(), "Expected error when allow_deletion is false")
	foundAllowDeletionError := false
	for _, err := range deleteResp.Diagnostics.Errors() {
		if strings.Contains(err.Detail(), "allow_deletion") || strings.Contains(err.Summary(), "allow_deletion") {
			foundAllowDeletionError = true
			break
		}
	}
	assert.True(t, foundAllowDeletionError, "Error should mention allow_deletion attribute")
}

func TestPipeline_StartPipelineError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockClient := mocks.NewMockPipelineServiceClient(ctrl)

	// Create succeeds
	createdPipeline := createMockPipeline(
		testPipelineID, testDisplayName, testDescription, testConfigYaml, testPipelineURL,
		dataplanev1.Pipeline_STATE_STOPPED, nil, nil,
	)
	mockClient.EXPECT().
		CreatePipeline(gomock.Any(), gomock.Any()).
		Return(&dataplanev1.CreatePipelineResponse{Pipeline: createdPipeline}, nil)

	// Start fails
	mockClient.EXPECT().
		StartPipeline(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("failed to start: insufficient resources"))

	r := setupPipelineResource(mockClient)

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	input := pipelinemodel.ResourceModel{
		ID:            types.StringUnknown(),
		ClusterAPIURL: types.StringValue(testClusterAPIURL),
		DisplayName:   types.StringValue(testDisplayName),
		Description:   types.StringValue(testDescription),
		ConfigYaml:    types.StringValue(testConfigYaml),
		State:         types.StringValue(pipelinemodel.StateRunning), // User wants running state
		URL:           types.StringUnknown(),
		Resources:     createResourcesObject("", ""),
		Tags:          createTagsMap(nil),
		Timeouts:      createTestTimeouts(),
	}

	createReq := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	diags := createReq.Plan.Set(ctx, &input)
	require.False(t, diags.HasError())

	createResp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, createReq, &createResp)

	// Create should succeed but with a warning about the start failure
	// The pipeline is created, so state should be set, but we should have a warning
	assert.True(t, createResp.Diagnostics.HasError() || len(createResp.Diagnostics.Warnings()) > 0,
		"Expected error or warning when start fails after create")
}
