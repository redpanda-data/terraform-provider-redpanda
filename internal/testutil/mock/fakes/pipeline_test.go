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

package fakes

import (
	"context"
	"testing"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestPipelineFake_ConflictAfterStateChange pins the fake to a dataplane whose
// UpdatePipeline does not retry the Kubernetes conflict (cloudv2
// apps/redpanda-connect-api/internal/pipeline maps it to Aborted): StartPipeline
// and StopPipeline patch spec.paused, the controller's status write for that
// change bumps the resourceVersion, and an UpdatePipeline that reads before
// that write lands fails with Aborted. The fake keeps the stale write armed
// until the next UpdatePipeline and reports RUNNING on Start at once, where
// the dataplane follows the controller; both are simplifications on the strict
// side. A fake that accepted the write would let a stop-then-write sequence
// pass.
func TestPipelineFake_ConflictAfterStateChange(t *testing.T) {
	ctx := context.Background()

	runningPipeline := func(t *testing.T, f *PipelineFake) string {
		t.Helper()
		created, err := f.CreatePipeline(ctx, &dataplanev1.CreatePipelineRequest{
			Pipeline: &dataplanev1.PipelineCreate{DisplayName: "p", ConfigYaml: "input: {}"},
		})
		require.NoError(t, err)
		id := created.GetPipeline().GetId()
		_, err = f.StartPipeline(ctx, &dataplanev1.StartPipelineRequest{Id: id})
		require.NoError(t, err)
		return id
	}
	stop := func(t *testing.T, f *PipelineFake, id string) {
		t.Helper()
		_, err := f.StopPipeline(ctx, &dataplanev1.StopPipelineRequest{Id: id})
		require.NoError(t, err)
	}
	write := func(f *PipelineFake, id string) error {
		_, err := f.UpdatePipeline(ctx, &dataplanev1.UpdatePipelineRequest{
			Id:       id,
			Pipeline: &dataplanev1.PipelineUpdate{DisplayName: "renamed", ConfigYaml: "input: {}"},
		})
		return err
	}

	t.Run("the first write after stopping a running pipeline is Aborted and the next lands", func(t *testing.T) {
		f := NewPipelineFake()
		id := runningPipeline(t, f)
		stop(t, f, id)

		err := write(f, id)
		require.Error(t, err)
		assert.Equal(t, codes.Aborted, status.Code(err))
		require.NoError(t, write(f, id), "the conflict is a single stale write, not a locked pipeline")
	})

	t.Run("stopping a pipeline that is not running arms nothing", func(t *testing.T) {
		f := NewPipelineFake()
		created, err := f.CreatePipeline(ctx, &dataplanev1.CreatePipelineRequest{
			Pipeline: &dataplanev1.PipelineCreate{DisplayName: "p", ConfigYaml: "input: {}"},
		})
		require.NoError(t, err)
		stop(t, f, created.GetPipeline().GetId())
		require.NoError(t, write(f, created.GetPipeline().GetId()))
	})

	t.Run("the first write after starting a stopped pipeline is Aborted too", func(t *testing.T) {
		f := NewPipelineFake()
		id := runningPipeline(t, f)

		err := write(f, id)
		require.Error(t, err)
		assert.Equal(t, codes.Aborted, status.Code(err))
		require.NoError(t, write(f, id))
	})

	t.Run("SetConflictAfterStateChange(false) models a dataplane that retries the conflict itself", func(t *testing.T) {
		f := NewPipelineFake()
		f.SetConflictAfterStateChange(false)
		id := runningPipeline(t, f)
		stop(t, f, id)
		require.NoError(t, write(f, id))
	})
}
