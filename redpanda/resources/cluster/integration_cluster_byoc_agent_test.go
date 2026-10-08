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
	"context"
	"errors"
	"regexp"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock/fakes"
)

// TestIntegration_Cluster_BYOC_AgentApplyAndDestroy pins the agent phases of
// a BYOC cluster's lifecycle: Create runs the byoc plugin's apply exactly once
// while the cluster is STATE_CREATING_AGENT, a destroy whose plugin run fails
// surfaces the failure and leaves the cluster in state and in
// STATE_DELETING_AGENT, and the next destroy runs the plugin again and
// succeeds. The plugin's failure text contains "not found", which the
// provider must not read as the cluster being gone. The runner fake refuses
// either verb in any other state, so a call at the wrong moment fails the
// case rather than passing silently.
func TestIntegration_Cluster_BYOC_AgentApplyAndDestroy(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-b1"
	cfg := awsBYOCConfig(name)
	pluginFailure := errors.New("byoc azure destroy: DefaultAzureCredential: failed to acquire a token.\n\tManagedIdentityCredential: Identity not found")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, nil),
			{
				PreConfig:   func() { srv.Byoc.FailNextWith(pluginFailure) },
				Config:      cfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Identity not found`),
			},
			{
				PreConfig: func() { requireClusterState(t, srv.Cluster, name, controlplanev1.Cluster_STATE_DELETING_AGENT) },
				Config:    cfg,
				Destroy:   true,
			},
		},
	})

	calls := srv.Byoc.Calls()
	if len(calls) != 3 {
		t.Fatalf("byoc runner calls = %v, want exactly [apply destroy destroy]", calls)
	}
	if calls[0].Verb != "apply" || calls[1].Verb != "destroy" || calls[2].Verb != "destroy" {
		t.Fatalf("byoc runner verbs = [%s %s %s], want [apply destroy destroy]", calls[0].Verb, calls[1].Verb, calls[2].Verb)
	}
	if calls[0].ClusterID == "" || calls[0].ClusterID != calls[1].ClusterID || calls[1].ClusterID != calls[2].ClusterID {
		t.Fatalf("byoc runner cluster ids = %v, want the same non-empty id on every call", calls)
	}
	if calls[0].StateAtCall != fakes.AgentStateCreating || calls[1].StateAtCall != fakes.AgentStateNone || calls[2].StateAtCall != fakes.AgentStateDeleting {
		t.Fatalf("byoc runner states at call = %v, want apply during CREATING_AGENT, a failed destroy, then destroy during DELETING_AGENT", calls)
	}
}

// requireClusterState fails the test unless the fake still holds a cluster
// with the given name in the given state.
func requireClusterState(t *testing.T, cl *fakes.ClusterFake, name string, want controlplanev1.Cluster_State) {
	t.Helper()
	resp, err := cl.ListClusters(context.Background(), &controlplanev1.ListClustersRequest{})
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	for _, c := range resp.GetClusters() {
		if c.GetName() == name {
			if c.GetState() != want {
				t.Fatalf("cluster %q is in %s, want %s", name, c.GetState(), want)
			}
			return
		}
	}
	t.Fatalf("cluster %q is gone from the control plane; a failed plugin destroy must leave it", name)
}
