// Copyright 2026 Redpanda Data, Inc.
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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

// TestGetUpdatedModel_EmptyURLYieldsNull is the direct regression for the
// proto3-empty-string trap: an unset `url` field arrives from the wire as
// `""`. Mapping that to `types.StringValue("")` yields a non-null state that
// diverges from a config that simply omits the attribute (which is `null`),
// producing spurious diffs on every replan.
func TestGetUpdatedModel_EmptyURLYieldsNull(t *testing.T) {
	r := &ResourceModel{}
	pipeline := &dataplanev1.Pipeline{
		Id:          "pipeline-abc",
		DisplayName: "test",
		State:       dataplanev1.Pipeline_STATE_RUNNING,
		// Url intentionally omitted -> empty string on the wire.
	}
	result, diags := r.GetUpdatedModel(context.Background(), pipeline, ContingentFields{
		State: types.StringNull(),
	})
	require.False(t, diags.HasError(), "unexpected diags: %v", diags)
	require.True(t, result.URL.IsNull(), "empty proto URL must map to StringNull, not StringValue(\"\")")
}

// TestGenerateModelStatus_EmptyErrorYieldsNull covers the same empty-proto
// trap on the pipeline status.error field: a pipeline in a healthy state
// returns an empty Error, which must become null in Terraform state.
func TestGenerateModelStatus_EmptyErrorYieldsNull(t *testing.T) {
	r := &ResourceModel{}
	pipeline := &dataplanev1.Pipeline{
		Status: &dataplanev1.Pipeline_Status{
			// Error intentionally empty.
		},
	}
	result, diags := r.generateModelStatus(pipeline)
	require.False(t, diags.HasError())
	require.False(t, result.IsNull(), "status object should be present when pipeline.HasStatus() is true")

	errVal, ok := result.Attributes()[FieldError].(types.String)
	require.True(t, ok, "error attribute should be a types.String")
	require.True(t, errVal.IsNull(), "empty status.error must map to StringNull")
}
