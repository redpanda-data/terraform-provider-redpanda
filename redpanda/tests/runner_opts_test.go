//go:build live_test

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

package tests

import (
	"context"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

// runnerOpt tunes runner behavior.
type runnerOpt func(*runnerCfg)

type runnerCfg struct {
	skipUpgradeEntry bool
	// networkPublicSubnetARNs, when set, adds a step after create that
	// registers public subnets on the network in place.
	networkPublicSubnetARNs []string
	// byocAgentApply adds a step that re-runs the byoc agent apply on the
	// READY cluster through the redpanda_byoc_agent_apply action.
	byocAgentApply bool
	// inPlaceAdd, when set, adds a step after rename that layers these
	// variables onto the rename step's as an in-place cluster update and,
	// after the agent apply, one that leaves them out again and requires
	// the cluster to drop them.
	inPlaceAdd *inPlaceAddStep
	// outOfBandAdd, when set, adds a step after rename that changes the
	// cluster through the control plane, the way the UI or API does, then
	// applies the unchanged config and requires the change undone.
	outOfBandAdd *outOfBandAddStep
}

type inPlaceAddStep struct {
	vars          map[string]config.Variable
	checks        []resource.TestCheckFunc
	removedChecks []resource.TestCheckFunc
}

type outOfBandAddStep struct {
	apply  func(ctx context.Context, cp *cloud.ControlPlaneClientSet, clusterID string) error
	checks []resource.TestCheckFunc
}

// withoutUpgradeEntry skips the provider-upgrade entry for configs the
// released provider cannot parse yet (unreleased schema features). Remove the
// caller's use once a release ships the feature.
func withoutUpgradeEntry() runnerOpt { return func(c *runnerCfg) { c.skipUpgradeEntry = true } }

// withByocAgentApply is for BYOC and BYOVPC lanes only: dedicated clusters
// have no agent and the action refuses them at plan time.
func withByocAgentApply() runnerOpt { return func(c *runnerCfg) { c.byocAgentApply = true } }

// withOutOfBandDrift is for plain Optional attributes a cluster can gain
// outside Terraform: the value is drift, so the silent config plans an update
// that removes it, and the checks run on the state after that apply.
func withOutOfBandDrift(apply func(ctx context.Context, cp *cloud.ControlPlaneClientSet, clusterID string) error, checks ...resource.TestCheckFunc) runnerOpt {
	return func(c *runnerCfg) { c.outOfBandAdd = &outOfBandAddStep{apply: apply, checks: checks} }
}

// withInPlaceAddThenRemove is for plain Optional attributes the released
// provider cannot express: the upgrade entry and the create step run without
// them, so the cluster takes them as an update, which is also how an existing
// cluster gains them. After the agent apply a further step leaves them out
// of the config again and requires them cleared, which is how a
// configuration turns the feature they carry off.
func withInPlaceAddThenRemove(vars map[string]config.Variable, checks []resource.TestCheckFunc, removedChecks ...resource.TestCheckFunc) runnerOpt {
	return func(c *runnerCfg) {
		c.inPlaceAdd = &inPlaceAddStep{vars: vars, checks: checks, removedChecks: removedChecks}
	}
}

func withNetworkPublicSubnets(arns []string) runnerOpt {
	return func(c *runnerCfg) { c.networkPublicSubnetARNs = arns }
}

func resolveRunnerOpts(opts []runnerOpt) runnerCfg {
	cfg := runnerCfg{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}
