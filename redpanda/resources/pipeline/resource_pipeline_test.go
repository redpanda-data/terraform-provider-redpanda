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
	"testing"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineStateToString(t *testing.T) {
	tests := []struct {
		name     string
		input    dataplanev1.Pipeline_State
		expected string
	}{
		{"starting", dataplanev1.Pipeline_STATE_STARTING, "starting"},
		{"running", dataplanev1.Pipeline_STATE_RUNNING, "running"},
		{"stopping", dataplanev1.Pipeline_STATE_STOPPING, "stopping"},
		{"stopped", dataplanev1.Pipeline_STATE_STOPPED, "stopped"},
		{"error", dataplanev1.Pipeline_STATE_ERROR, "error"},
		{"completed", dataplanev1.Pipeline_STATE_COMPLETED, "completed"},
		{"unspecified", dataplanev1.Pipeline_STATE_UNSPECIFIED, "unknown"},
		{"invalid", dataplanev1.Pipeline_State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pipelineStateToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipelineToModel(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		pipeline         *dataplanev1.Pipeline
		clusterAPIURL    types.String
		startAfterCreate types.Bool
		wantID           string
		wantDisplayName  string
		wantState        string
	}{
		{
			name: "basic pipeline",
			pipeline: &dataplanev1.Pipeline{
				Id:          "pipeline-123",
				DisplayName: "test-pipeline",
				Description: "A test pipeline",
				ConfigYaml:  "input:\n  generate: {}\noutput:\n  stdout: {}",
				State:       dataplanev1.Pipeline_STATE_STOPPED,
				Url:         "https://pipeline.example.com",
			},
			clusterAPIURL:    types.StringValue("https://api.cluster.example.com"),
			startAfterCreate: types.BoolValue(false),
			wantID:           "pipeline-123",
			wantDisplayName:  "test-pipeline",
			wantState:        "stopped",
		},
		{
			name: "running pipeline",
			pipeline: &dataplanev1.Pipeline{
				Id:          "pipeline-456",
				DisplayName: "running-pipeline",
				State:       dataplanev1.Pipeline_STATE_RUNNING,
			},
			clusterAPIURL:    types.StringValue("https://api.cluster.example.com"),
			startAfterCreate: types.BoolValue(true),
			wantID:           "pipeline-456",
			wantDisplayName:  "running-pipeline",
			wantState:        "running",
		},
		{
			name: "pipeline with resources",
			pipeline: &dataplanev1.Pipeline{
				Id:          "pipeline-789",
				DisplayName: "resourced-pipeline",
				State:       dataplanev1.Pipeline_STATE_STOPPED,
				Resources: &dataplanev1.Pipeline_Resources{
					MemoryShares: "512Mi",
					CpuShares:    "500m",
				},
			},
			clusterAPIURL:    types.StringValue("https://api.cluster.example.com"),
			startAfterCreate: types.BoolValue(false),
			wantID:           "pipeline-789",
			wantDisplayName:  "resourced-pipeline",
			wantState:        "stopped",
		},
		{
			name: "pipeline with tags",
			pipeline: &dataplanev1.Pipeline{
				Id:          "pipeline-tags",
				DisplayName: "tagged-pipeline",
				State:       dataplanev1.Pipeline_STATE_STOPPED,
				Tags: map[string]string{
					"environment": "test",
					"team":        "data",
				},
			},
			clusterAPIURL:    types.StringValue("https://api.cluster.example.com"),
			startAfterCreate: types.BoolValue(false),
			wantID:           "pipeline-tags",
			wantDisplayName:  "tagged-pipeline",
			wantState:        "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, diags := pipelineToModel(ctx, tt.pipeline, tt.clusterAPIURL, tt.startAfterCreate)
			require.False(t, diags.HasError(), "pipelineToModel should not return errors")

			assert.Equal(t, tt.wantID, model.ID.ValueString())
			assert.Equal(t, tt.wantDisplayName, model.DisplayName.ValueString())
			assert.Equal(t, tt.wantState, model.State.ValueString())
			assert.Equal(t, tt.clusterAPIURL.ValueString(), model.ClusterAPIURL.ValueString())
			assert.Equal(t, tt.startAfterCreate.ValueBool(), model.StartAfterCreate.ValueBool())

			// Verify resources if present
			if tt.pipeline.HasResources() {
				assert.False(t, model.Resources.IsNull(), "Resources should not be null when pipeline has resources")
			}

			// Verify tags if present
			if len(tt.pipeline.GetTags()) > 0 {
				assert.False(t, model.Tags.IsNull(), "Tags should not be null when pipeline has tags")
			}
		})
	}
}

func TestPipelineSchema(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{}

	schemaResp := resource.SchemaResponse{}
	p.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	require.False(t, schemaResp.Diagnostics.HasError(), "Schema should not return errors")

	schema := schemaResp.Schema

	// Verify required attributes
	requiredAttrs := []string{"cluster_api_url", "display_name", "config_yaml"}
	for _, attr := range requiredAttrs {
		schemaAttr, ok := schema.Attributes[attr]
		require.True(t, ok, "Schema should have %s attribute", attr)
		assert.True(t, schemaAttr.IsRequired(), "%s should be required", attr)
	}

	// Verify computed attributes
	computedAttrs := []string{"id", "state", "url"}
	for _, attr := range computedAttrs {
		schemaAttr, ok := schema.Attributes[attr]
		require.True(t, ok, "Schema should have %s attribute", attr)
		assert.True(t, schemaAttr.IsComputed(), "%s should be computed", attr)
	}

	// Verify optional attributes
	optionalAttrs := []string{"description", "start_after_create", "tags", "resources"}
	for _, attr := range optionalAttrs {
		schemaAttr, ok := schema.Attributes[attr]
		require.True(t, ok, "Schema should have %s attribute", attr)
		assert.True(t, schemaAttr.IsOptional(), "%s should be optional", attr)
	}
}

func TestPipelineMetadata(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{}

	metaResp := resource.MetadataResponse{}
	p.Metadata(ctx, resource.MetadataRequest{}, &metaResp)

	assert.Equal(t, "redpanda_pipeline", metaResp.TypeName)
}
