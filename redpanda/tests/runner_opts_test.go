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
}

// withoutUpgradeEntry skips the provider-upgrade entry for configs the
// released provider cannot parse yet (unreleased schema features). Remove the
// caller's use once a release ships the feature.
func withoutUpgradeEntry() runnerOpt { return func(c *runnerCfg) { c.skipUpgradeEntry = true } }

// withByocAgentApply is for BYOC and BYOVPC lanes only: dedicated clusters
// have no agent and the action refuses them at plan time.
func withByocAgentApply() runnerOpt { return func(c *runnerCfg) { c.byocAgentApply = true } }

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
