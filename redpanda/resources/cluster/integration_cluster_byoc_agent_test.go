//go:build integration

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

package cluster_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock/fakes"
)

// TestIntegration_Cluster_BYOC_AgentApplyAndDestroy pins the agent phases of
// a BYOC cluster's lifecycle: Create runs the byoc plugin's apply exactly once
// while the cluster is STATE_CREATING_AGENT, and the end-of-case destroy runs
// the plugin's destroy exactly once while it is STATE_DELETING_AGENT. The
// runner fake refuses either verb in any other state, so a call at the wrong
// moment fails the case rather than passing silently.
func TestIntegration_Cluster_BYOC_AgentApplyAndDestroy(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b1"
	cfg := awsBYOCConfig(name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, nil),
		},
	})

	calls := srv.Byoc.Calls()
	if len(calls) != 2 {
		t.Fatalf("byoc runner calls = %v, want exactly [apply destroy]", calls)
	}
	if calls[0].Verb != "apply" || calls[1].Verb != "destroy" {
		t.Fatalf("byoc runner verbs = [%s %s], want [apply destroy]", calls[0].Verb, calls[1].Verb)
	}
	if calls[0].ClusterID == "" || calls[0].ClusterID != calls[1].ClusterID {
		t.Fatalf("byoc runner cluster ids = %q and %q, want the same non-empty id", calls[0].ClusterID, calls[1].ClusterID)
	}
	if calls[0].StateAtCall != fakes.AgentStateCreating || calls[1].StateAtCall != fakes.AgentStateDeleting {
		t.Fatalf("byoc runner states at call = %v, want apply during CREATING_AGENT and destroy during DELETING_AGENT", calls)
	}
}
