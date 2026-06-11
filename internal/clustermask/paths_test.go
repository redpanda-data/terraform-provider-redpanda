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

package clustermask

import (
	"reflect"
	"testing"
)

// TestFixtureContract pins the hand-maintained cluster field-mask maps to the
// shape the provider + mock depend on. AcceptedTopLevel is the union of no-dot
// keys from cloudv2's pathMap and multiListenersPathMap (the latter contributes
// kafka_api); LeafExpansions is the curated {rpsql, kafka_connect} set. An
// accidental edit to paths.go fails here.
func TestFixtureContract(t *testing.T) {
	wantTop := []string{
		"api_gateway_access", "aws_private_link", "azure_private_link",
		"cloud_provider_tags", "cloud_storage", "cluster_configuration",
		"gcp_enable_global_access_api_gateway", "gcp_private_service_connect",
		"http_proxy", "kafka_api", "maintenance_window_config", "name",
		"read_replica_cluster_ids", "redpanda_node_count", "schema_registry",
		"throughput_tier",
	}
	if len(AcceptedTopLevel) != len(wantTop) {
		t.Errorf("AcceptedTopLevel has %d keys, want %d", len(AcceptedTopLevel), len(wantTop))
	}
	for _, k := range wantTop {
		if !AcceptedTopLevel[k] {
			t.Errorf("AcceptedTopLevel missing %q", k)
		}
	}

	wantLeaf := map[string][]string{
		"kafka_connect": {"kafka_connect.enabled"},
		"rpsql":         {"rpsql.enabled", "rpsql.replicas", "rpsql.zones"},
	}
	if !reflect.DeepEqual(LeafExpansions, wantLeaf) {
		t.Errorf("LeafExpansions = %v, want %v", LeafExpansions, wantLeaf)
	}
}
