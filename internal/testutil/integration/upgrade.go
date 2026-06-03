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

package integration

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// UpgradeState drives a resource's UpgradeResourceState RPC through the real
// provider server: it decodes priorStateJSON at priorVersion, dispatches to
// the resource's registered StateUpgrader, and returns the upgraded state
// decoded against schemaType. This exercises the schema-version detection and
// upgrader plumbing end-to-end without a live backend. Fails the test on any
// transport error or error-severity diagnostic.
func UpgradeState(t *testing.T, factories map[string]func() (tfprotov6.ProviderServer, error), typeName string, priorVersion int64, priorStateJSON string, schemaType tftypes.Type) tftypes.Value {
	t.Helper()
	server, err := factories["redpanda"]()
	if err != nil {
		t.Fatalf("build provider server: %v", err)
	}
	resp, err := server.UpgradeResourceState(context.Background(), &tfprotov6.UpgradeResourceStateRequest{
		TypeName: typeName,
		Version:  priorVersion,
		RawState: &tfprotov6.RawState{JSON: []byte(priorStateJSON)},
	})
	if err != nil {
		t.Fatalf("UpgradeResourceState(%s): %v", typeName, err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("UpgradeResourceState(%s) diagnostic: %s: %s", typeName, d.Summary, d.Detail)
		}
	}
	val, err := resp.UpgradedState.Unmarshal(schemaType)
	if err != nil {
		t.Fatalf("unmarshal upgraded state for %s: %v", typeName, err)
	}
	return val
}
