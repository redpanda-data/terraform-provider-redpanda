// Copyright 2023 Redpanda Data, Inc.
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

package apidesc

// FilterByRoots returns a subset tree containing only the named top-level
// schemas. Child subtrees are unchanged — Flatten already inlined referenced
// schemas at every use site, so reachability from the kept roots is
// preserved without needing their standalone entries.
func FilterByRoots(tree map[string]*Node, roots []string) map[string]*Node {
	keep := make(map[string]bool, len(roots))
	for _, r := range roots {
		keep[r] = true
	}
	out := make(map[string]*Node, len(roots))
	for name, node := range tree {
		if keep[name] {
			out[name] = node
		}
	}
	return out
}
