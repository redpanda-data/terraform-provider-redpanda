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

// These maps mirror cloudv2's cluster field-mask path maps, hand-maintained.
// Source of truth: apps/public-api-go/internal/services/cluster/v1/mapper.go —
// the `pathMap` and `multiListenersPathMap` literals. The set is small and rarely
// changes; when bumping the cloudv2 pin, eyeball those two maps and update here.
// TestFixtureContract pins the expected shape so an accidental edit fails loudly.

// AcceptedTopLevel is the set of top-level ClusterUpdate field-mask paths the
// control plane accepts at the object level in either listener mode (the no-dot
// keys of pathMap and multiListenersPathMap; the latter contributes kafka_api).
// A path absent here is silently dropped by the control plane unless sent at leaf
// granularity — see LeafExpansions.
var AcceptedTopLevel = map[string]bool{
	"api_gateway_access":                   true,
	"aws_private_link":                     true,
	"azure_private_link":                   true,
	"cloud_provider_tags":                  true,
	"cloud_storage":                        true,
	"cluster_configuration":                true,
	"gcp_enable_global_access_api_gateway": true,
	"gcp_private_service_connect":          true,
	"http_proxy":                           true,
	"kafka_api":                            true,
	"maintenance_window_config":            true,
	"name":                                 true,
	"read_replica_cluster_ids":             true,
	"redpanda_node_count":                  true,
	"schema_registry":                      true,
	"throughput_tier":                      true,
}

// LeafExpansions maps a top-level field the control plane accepts ONLY at leaf
// granularity to the leaf paths to send instead. The diff emits the bare object
// path; ExpandLeafPaths rewrites it to these before the UpdateCluster request.
var LeafExpansions = map[string][]string{
	"kafka_connect": {"kafka_connect.enabled"},
	"rpsql":         {"rpsql.enabled", "rpsql.replicas", "rpsql.zones"},
}
