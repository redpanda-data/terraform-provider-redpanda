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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// State constants for pipeline.
const (
	StateRunning = "running"
	StateStopped = "stopped"
)

// State equivalence maps for pipeline lifecycle.
// Running states: pipeline is either starting up or actively running.
// Stopped states: pipeline is stopping, fully stopped, or completed its work.
var (
	runningStates = map[string]bool{"starting": true, StateRunning: true}
	stoppedStates = map[string]bool{"stopping": true, StateStopped: true, "completed": true}
)

// GetResourcesType returns the attribute types for the resources nested object.
func GetResourcesType() map[string]attr.Type {
	return map[string]attr.Type{
		"memory_shares": types.StringType,
		"cpu_shares":    types.StringType,
	}
}

// StateToString converts pipeline state enum to string.
func StateToString(state dataplanev1.Pipeline_State) string {
	switch state {
	case dataplanev1.Pipeline_STATE_STARTING:
		return "starting"
	case dataplanev1.Pipeline_STATE_RUNNING:
		return StateRunning
	case dataplanev1.Pipeline_STATE_STOPPING:
		return "stopping"
	case dataplanev1.Pipeline_STATE_STOPPED:
		return StateStopped
	case dataplanev1.Pipeline_STATE_ERROR:
		return "error"
	case dataplanev1.Pipeline_STATE_COMPLETED:
		return "completed"
	default:
		return "unknown"
	}
}

// StatesEquivalent checks if two pipeline states are functionally equivalent.
// Running states (starting, running) are considered equivalent to each other.
// Stopped states (stopping, stopped, completed) are considered equivalent to each other.
// Error states are never considered equivalent to prevent masking failures.
func StatesEquivalent(prior, current string) bool {
	if runningStates[prior] && runningStates[current] {
		return true
	}
	if stoppedStates[prior] && stoppedStates[current] {
		return true
	}
	return prior == current
}

// DesiredStateFromAPIState normalizes API state to desired state.
func DesiredStateFromAPIState(apiState string) string {
	switch apiState {
	case "starting", StateRunning:
		return StateRunning
	case "stopping", StateStopped, "completed":
		return StateStopped
	default:
		return StateStopped
	}
}
