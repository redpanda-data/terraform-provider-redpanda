//go:build live_test && (all || cluster_aws_dual)

// Copyright 2026 Redpanda Data, Inc.
//
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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

// TestAcc_Cluster_AWS_DualListeners runs the AWS BYOC stack (the certified
// envelope for dual listener mode) with public+private SASL connections on
// all three services and no connection_type. Requires the
// enable-public-private-listeners feature flag on the test org (preview) —
// without it the create fails PermissionDenied by design.
func TestAcc_Cluster_AWS_DualListeners(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + "aws-dual")
	rename := acc.RandomName(acc.NamePrefix + "aws-dual-r")

	conn := func(connType string) config.Variable {
		return config.ObjectVariable(map[string]config.Variable{
			"type": config.StringVariable(connType),
			"auth": config.ObjectVariable(map[string]config.Variable{
				"mode": config.StringVariable("sasl"),
			}),
		})
	}
	customVars := map[string]config.Variable{
		"dual_listener_connections": config.ListVariable(conn("public"), conn("private")),
		"cluster_create_timeout":    config.StringVariable("150m"),
	}
	// No upgrade entry: the released provider does not know the connections
	// schema yet. Remove once a release ships dual listener mode.
	testRunner(ctx, name, rename, acc.RedpandaVersion, acc.AwsByocClusterDir, customVars, t, withoutUpgradeEntry())
}
