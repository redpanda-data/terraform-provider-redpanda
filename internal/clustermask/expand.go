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

// Package clustermask holds the cluster field-mask path contract mirrored from
// cloudv2's control-plane path maps (see paths.go). It lets the provider send
// masks the control plane accepts and lets the mock drop what the control plane
// drops, from one hand-maintained source.
package clustermask

import "google.golang.org/protobuf/types/known/fieldmaskpb"

// ExpandLeafPaths rewrites, in place, any top-level mask path listed in
// LeafExpansions into its leaf paths. The control plane maps those fields
// (kafka_connect, redpanda_connect, rpsql) only at leaf granularity, so sending the bare object
// path silently drops the update. All other paths pass through unchanged.
func ExpandLeafPaths(fm *fieldmaskpb.FieldMask) {
	if fm == nil {
		return
	}
	out := make([]string, 0, len(fm.Paths)+2)
	for _, p := range fm.Paths {
		if leaves, ok := LeafExpansions[p]; ok {
			out = append(out, leaves...)
			continue
		}
		out = append(out, p)
	}
	fm.Paths = out
}

// ExpandConnectionLeaves rewrites, in place, a bare listener-service mask path
// ("kafka_api", "http_proxy", "schema_registry") into leaf granularity
// ("<svc>.connections", "<svc>.mtls") when the update payload carries
// connections for that service. GET always projects a non-nil sasl block, so a
// read-modify-write update under a bare service mask would be rejected by the
// control plane's "sasl cannot be set together with connections" guard
// (serviceSASLInMask matches the bare path); the leaf mask scopes the request
// to what the update actually changes. mtls rides along so the mask-driven
// merge takes the request's CA/rules whenever connections are in play.
// Services without connections in the payload keep their bare path (legacy
// clusters are updated object-level, unchanged).
func ExpandConnectionLeaves(fm *fieldmaskpb.FieldMask, hasConnections map[string]bool) {
	if fm == nil {
		return
	}
	out := make([]string, 0, len(fm.Paths)+3)
	for _, p := range fm.Paths {
		if hasConnections[p] {
			out = append(out, p+".connections", p+".mtls")
			continue
		}
		out = append(out, p)
	}
	fm.Paths = out
}
