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
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/compare"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
	const (
		getNetwork = controlplanev1grpc.NetworkService_GetNetwork_FullMethodName
		getRG      = controlplanev1grpc.ResourceGroupService_GetResourceGroup_FullMethodName
	)
	var networkBefore, rgBefore int
	noop := integration.NoopReapplyStep(clusterAddr, cfg, nil)
	noop.PreConfig = func() { networkBefore, rgBefore = srv.CallCount(getNetwork), srv.CallCount(getRG) }

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
			}),
			noop,
		},
	})

	// Every refresh in the no-op step reads the resource group and the
	// network once each, for their own resources; a cluster Read that also
	// read the network would make the network reads outnumber the resource
	// group reads: a cluster not on a cloud provider access network reads no
	// network on refresh once its agent owner is recorded.
	networkReads, rgReads := srv.CallCount(getNetwork)-networkBefore, srv.CallCount(getRG)-rgBefore
	if networkReads != rgReads || rgReads == 0 {
		t.Fatalf("no-op step: GetNetwork calls = %d, GetResourceGroup calls = %d, want equal and non-zero (no network read from the cluster's refresh)", networkReads, rgReads)
	}

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

// TestIntegration_Cluster_BYOC_RetriedApplyReadsNetworkOnce pins that a BYOC
// cluster not on a cloud provider access network reads its network once per
// create to decide who runs the agent, however often the byoc plugin's apply
// is retried: the create with retried applies makes as many GetNetwork calls
// as the one without.
func TestIntegration_Cluster_BYOC_RetriedApplyReadsNetworkOnce(t *testing.T) {
	const getNetwork = controlplanev1grpc.NetworkService_GetNetwork_FullMethodName
	reads := map[int]int{}
	for _, retries := range []int{0, 2} {
		srv, factories := clusterSetup(t)
		for range retries {
			srv.Byoc.FailNextWith(errors.New("agent busy, please retry later"))
		}
		var before int
		create := integration.CreateStep(clusterAddr, awsBYOCConfig(fmt.Sprintf("tfrp-mock-cl-retry-%d", retries)), nil)
		create.PreConfig = func() { before = srv.CallCount(getNetwork) }
		create.Check = func(*terraform.State) error {
			reads[retries] = srv.CallCount(getNetwork) - before
			return nil
		}
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps:                    []resource.TestStep{create},
		})
		if got := len(srv.Byoc.Calls()); got != retries+2 {
			t.Fatalf("retries=%d: byoc runner calls = %d, want %d applies and a destroy", retries, got, retries+1)
		}
	}
	if reads[2] != reads[0] {
		t.Fatalf("GetNetwork calls during create = %d with two retried applies, %d with none; want equal", reads[2], reads[0])
	}
}

// TestIntegration_Cluster_BYOC_CloudProviderAccessAgent pins that a BYOC
// cluster on a network provisioned through a cloud provider access never runs
// the byoc plugin: the control plane applies and destroys its agent with the
// role it assumes. The cluster fake reports each agent phase once and refuses
// a plugin run, so a provider that shells out fails the case. Re-pointing the
// network at another access updates the network in place and leaves the
// cluster untouched, and Delete decides from the owner recorded in private
// state rather than reading the network.
func TestIntegration_Cluster_BYOC_CloudProviderAccessAgent(t *testing.T) {
	srv, factories := clusterSetup(t)

	const name = "tfrp-mock-cl-cpa"
	withAccess := func(label string) string {
		return strings.Replace(awsBYOCConfig(name), `  cidr_block        = "10.0.0.0/20"
}`, `  cidr_block        = "10.0.0.0/20"
  cloud_provider_access_id = redpanda_cloud_provider_access.`+label+`.id
}

resource "redpanda_cloud_provider_access" "a" {
  name           = "tfrp-mock-cl-cpa-a"
  cloud_provider = "aws"
  aws = {
    role_arn = "arn:aws:iam::123456789012:role/tfrp-mock-cl-a"
  }
}

resource "redpanda_cloud_provider_access" "b" {
  name           = "tfrp-mock-cl-cpa-b"
  cloud_provider = "aws"
  aws = {
    role_arn = "arn:aws:iam::123456789012:role/tfrp-mock-cl-b"
  }
}`, 1)
	}
	cfg := withAccess("a")
	const getNetwork = controlplanev1grpc.NetworkService_GetNetwork_FullMethodName
	var networkReadsBeforeDestroy int
	if !strings.Contains(cfg, "cloud_provider_access_id") {
		t.Fatal("awsBYOCConfig no longer carries the network block this case extends")
	}
	clusterStable := statecheck.CompareValue(compare.ValuesSame())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, cfg, []statecheck.StateCheck{
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("cluster_type"), knownvalue.StringExact("byoc")),
				statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("state"), knownvalue.StringExact("STATE_READY")),
				clusterStable.AddStateValue(clusterAddr, tfjsonpath.New("id")),
			}),
			integration.NoopReapplyStep(clusterAddr, cfg, nil),
			{
				Config: withAccess("b"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("redpanda_network.test", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					clusterStable.AddStateValue(clusterAddr, tfjsonpath.New("id")),
				},
			},
			{
				PreConfig: func() { networkReadsBeforeDestroy = srv.CallCount(getNetwork) },
				Config:    withAccess("b"),
				Destroy:   true,
			},
		},
	})

	// Destroy's refresh reads the network once for the network resource; the
	// cluster's Read keeps its recorded agent owner and its Delete uses it, so
	// neither reads the network.
	if got := srv.CallCount(getNetwork) - networkReadsBeforeDestroy; got != 1 {
		t.Fatalf("GetNetwork calls during destroy = %d, want 1 (the network's own refresh)", got)
	}

	if calls := srv.Byoc.Calls(); len(calls) != 0 {
		t.Fatalf("byoc runner calls = %v, want none for a cluster whose agent Redpanda manages", calls)
	}
	want := []controlplanev1.Cluster_State{controlplanev1.Cluster_STATE_CREATING_AGENT, controlplanev1.Cluster_STATE_DELETING_AGENT}
	if got := srv.Cluster.ManagedAgentPhasesSeen(); !slices.Equal(got, want) {
		t.Fatalf("agent phases the provider polled through = %v, want %v", got, want)
	}
}
