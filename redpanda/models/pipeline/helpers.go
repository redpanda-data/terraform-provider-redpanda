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
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// User-visible pipeline state values. The proto enum carries 6 states
// (STARTING / RUNNING / STOPPING / STOPPED / ERROR / COMPLETED); the TF
// schema only exposes these two. FlattenDesiredState below collapses
// the API enum into one of these.
const (
	StateRunning = "running"
	StateStopped = "stopped"
)

// FlattenDesiredState collapses the proto Pipeline_State enum into the
// two user-visible desired states. Schemagen calls this via
// `flatten_via: FlattenDesiredState` on the `state` field.
func FlattenDesiredState(proto PipelineResponse) types.String {
	switch proto.GetState() {
	case dataplanev1.Pipeline_STATE_STARTING, dataplanev1.Pipeline_STATE_RUNNING:
		return types.StringValue(StateRunning)
	default:
		return types.StringValue(StateStopped)
	}
}
