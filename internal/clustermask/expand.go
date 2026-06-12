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
// (rpsql, kafka_connect) only at leaf granularity, so sending the bare object
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
