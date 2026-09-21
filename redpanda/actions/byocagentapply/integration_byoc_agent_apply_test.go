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

package byocagentapply_test

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock/fakes"
)

const clusterAddr = "redpanda_cluster.test"

// baseConfig is a BYOC cluster (or a dedicated one when byoc is false) with
// the supporting resource group and network.
func baseConfig(name string, byoc bool) string {
	clusterType, connectionType := "dedicated", "public"
	if byoc {
		clusterType, connectionType = "byoc", "private"
	}
	return fmt.Sprintf(`
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = "tfrp-mock-act-rg"
}

resource "redpanda_network" "test" {
  name              = "tfrp-mock-act-net"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  cluster_type      = %q
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = %q
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "aws"
  region            = "us-east-1"
  zones             = ["use1-az1"]
  throughput_tier   = "tier-1-aws-v3-arm"
  cluster_type      = %q
  connection_type   = %q
  allow_deletion    = true
}
`, clusterType, name, clusterType, connectionType)
}

// withAction appends the action and a terraform_data resource whose
// after_create event triggers it. extra is spliced into the action config.
func withAction(base, extra string) string {
	return base + fmt.Sprintf(`
action "redpanda_byoc_agent_apply" "test" {
  config {
    cluster_id = redpanda_cluster.test.id
    %s
  }
}

resource "terraform_data" "agent_reconcile" {
  input = "1"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.redpanda_byoc_agent_apply.test]
    }
  }
}
`, extra)
}

// TestIntegration_ByocAgentApply_TriggeredApply pins the happy path: on a
// READY BYOC cluster the trigger runs the plugin's apply exactly once, the
// plugin's progress lines reach the action's sink, and the cluster itself
// plans clean afterwards. Create's own apply and the end-of-case destroy
// bracket it.
func TestIntegration_ByocAgentApply_TriggeredApply(t *testing.T) {
	srv, factories := integration.Setup(t)
	srv.Byoc.Lines = []string{"Reconciling agent infrastructure...", "Updates have been successfully applied."}

	const name = "tfrp-mock-act-1"
	cfg := withAction(baseConfig(name, true), "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, nil),
			integration.NoopReapplyStep(clusterAddr, cfg, nil),
		},
	})

	calls := srv.Byoc.Calls()
	want := []fakes.ByocCall{
		{Verb: "apply", StateAtCall: fakes.AgentStateCreating, SinkLines: 0},
		{Verb: "apply", StateAtCall: fakes.AgentStateReady, SinkLines: 2},
		{Verb: "destroy", StateAtCall: fakes.AgentStateDeleting, SinkLines: 0},
	}
	if len(calls) != len(want) {
		t.Fatalf("byoc runner calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i].Verb != want[i].Verb || calls[i].StateAtCall != want[i].StateAtCall || calls[i].SinkLines != want[i].SinkLines || calls[i].ClusterID != calls[0].ClusterID {
			t.Fatalf("byoc runner call %d = %+v, want %+v on cluster %q", i, calls[i], want[i], calls[0].ClusterID)
		}
	}
}

// TestIntegration_ByocAgentApply_RetriesRetryable pins that a "please retry
// later" failure from the plugin is retried within the action's timeout, the
// same classification Create uses, while the retry still lands on READY.
func TestIntegration_ByocAgentApply_RetriesRetryable(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-act-2"
	base := baseConfig(name, true)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, base, nil),
			{
				PreConfig: func() { srv.Byoc.FailNextWith(errors.New("agent apply: please retry later")) },
				Config:    withAction(base, ""),
			},
		},
	})

	verbs := ""
	for _, c := range srv.Byoc.Calls() {
		verbs += fmt.Sprintf("%s/%d ", c.Verb, c.StateAtCall)
	}
	const want = "apply/1 apply/0 apply/2 destroy/3 "
	if verbs != want {
		t.Fatalf("byoc runner calls = %q, want %q", verbs, want)
	}
}

// TestIntegration_ByocAgentApply_RejectsDedicatedAtPlan pins that a
// non-BYOC cluster is refused while planning, before any plugin run.
func TestIntegration_ByocAgentApply_RejectsDedicatedAtPlan(t *testing.T) {
	srv, factories := integration.Setup(t)

	const name = "tfrp-mock-act-3"
	base := baseConfig(name, false)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, base, nil),
			{
				Config:      withAction(base, ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`not a BYOC cluster`),
			},
		},
	})
	if calls := srv.Byoc.Calls(); len(calls) != 0 {
		t.Fatalf("byoc runner calls = %v, want none for a dedicated cluster", calls)
	}
}

// TestIntegration_ByocAgentApply_RejectsNotReadyAtPlan pins the READY-only
// rule and that a FAILED cluster's state_description is surfaced.
func TestIntegration_ByocAgentApply_RejectsNotReadyAtPlan(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state controlplanev1.Cluster_State
		desc  string
		want  string
	}{
		{"upgrading", controlplanev1.Cluster_STATE_UPGRADING, "", `(?s)state\s+STATE_UPGRADING`},
		{"failed", controlplanev1.Cluster_STATE_FAILED, "agent never registered", `agent never registered`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, factories := integration.Setup(t)
			clusterName := "tfrp-mock-act-4-" + tc.name
			base := baseConfig(clusterName, true)

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{
					integration.CreateStep(clusterAddr, base, nil),
					{
						PreConfig: func() {
							if !srv.Cluster.SetStateOutOfBand(clusterName, tc.state, tc.desc) {
								t.Fatalf("cluster %q not found in fake", clusterName)
							}
						},
						Config:      withAction(base, ""),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(tc.want),
					},
				},
			})
			// Create's apply and the end-of-case destroy are the only runs;
			// the refused action must not have added an apply on top.
			if calls := srv.Byoc.Calls(); len(calls) != 2 || calls[0].Verb != "apply" || calls[1].Verb != "destroy" {
				t.Fatalf("byoc runner calls = %v, want only Create's apply and the final destroy", calls)
			}
		})
	}
}

// TestIntegration_ByocAgentApply_RejectsBadTimeout pins config validation of
// the timeout attribute.
func TestIntegration_ByocAgentApply_RejectsBadTimeout(t *testing.T) {
	_, factories := integration.Setup(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      withAction(baseConfig("tfrp-mock-act-5", true), `timeout = "soon"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`timeout`),
			},
		},
	})
}

// withBeforeUpdateOnCluster is the documented "reconcile before every cluster
// change" pattern: the trigger sits on the cluster and the action takes the
// cluster id from a variable. Reading redpanda_cluster.test.id from the action
// config instead makes Terraform report a cycle, since a before-event trigger
// runs ahead of the resource it references.
func withBeforeUpdateOnCluster(base string) string {
	const tail = "  allow_deletion    = true\n}\n"
	return base[:len(base)-len(tail)] + `  allow_deletion    = true
  lifecycle {
    action_trigger {
      events  = [before_update]
      actions = [action.redpanda_byoc_agent_apply.test]
    }
  }
}

variable "cluster_id" {
  type = string
}

action "redpanda_byoc_agent_apply" "test" {
  config {
    cluster_id = var.cluster_id
  }
}
`
}

// TestIntegration_ByocAgentApply_BeforeUpdateOnCluster pins the documented
// before_update pattern: a rename of the cluster runs the agent apply on the
// READY cluster ahead of the update, and creating the cluster with the trigger
// in place runs nothing beyond Create's own apply.
func TestIntegration_ByocAgentApply_BeforeUpdateOnCluster(t *testing.T) {
	srv, factories := integration.Setup(t)

	vars := map[string]config.Variable{"cluster_id": config.StringVariable("00000008000000000001")}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: withBeforeUpdateOnCluster(baseConfig("tfrp-mock-act-6", true)), ConfigVariables: vars},
			{Config: withBeforeUpdateOnCluster(baseConfig("tfrp-mock-act-6-renamed", true)), ConfigVariables: vars},
		},
	})

	verbs := ""
	for _, c := range srv.Byoc.Calls() {
		verbs += fmt.Sprintf("%s/%d ", c.Verb, c.StateAtCall)
	}
	const want = "apply/1 apply/2 destroy/3 "
	if verbs != want {
		t.Fatalf("byoc runner calls = %q, want %q", verbs, want)
	}
}

// TestIntegration_ByocAgentApply_TimeoutBoundsThePluginRun pins that the
// timeout attribute cuts off a hung plugin run, not only the retry window.
func TestIntegration_ByocAgentApply_TimeoutBoundsThePluginRun(t *testing.T) {
	srv, factories := integration.Setup(t)

	base := baseConfig("tfrp-mock-act-7", true)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, base, nil),
			{
				PreConfig:   func() { srv.Byoc.BlockNextFor(5 * time.Second) },
				Config:      withAction(base, `timeout = "1s"`),
				ExpectError: regexp.MustCompile(`(?s)context deadline\s+exceeded`),
			},
		},
	})
}
