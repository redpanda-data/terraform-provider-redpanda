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
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
	// variables onto the rename step's as an in-place cluster update; later
	// steps keep them.
	inPlaceAdd *inPlaceAddStep
}

type inPlaceAddStep struct {
	vars   map[string]config.Variable
	checks []resource.TestCheckFunc
}

// withoutUpgradeEntry skips the provider-upgrade entry for configs the
// released provider cannot parse yet (unreleased schema features). Remove the
// caller's use once a release ships the feature.
func withoutUpgradeEntry() runnerOpt { return func(c *runnerCfg) { c.skipUpgradeEntry = true } }

// withByocAgentApply is for BYOC and BYOVPC lanes only: dedicated clusters
// have no agent and the action refuses them at plan time.
func withByocAgentApply() runnerOpt { return func(c *runnerCfg) { c.byocAgentApply = true } }

// withInPlaceAdd is for attributes the released provider cannot express: the
// upgrade entry and the create step run without them, so the cluster has to
// take them as an update, which is also how an existing cluster adopts them.
func withInPlaceAdd(vars map[string]config.Variable, checks ...resource.TestCheckFunc) runnerOpt {
	return func(c *runnerCfg) { c.inPlaceAdd = &inPlaceAddStep{vars: vars, checks: checks} }
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
