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

import "testing"

func TestFilterByRoots(t *testing.T) {
	tree := map[string]*Node{
		"Cluster": {
			Description: "A cluster.",
			Fields: map[string]*Node{
				"aws_private_link": {
					Description: "APL config.",
					Fields: map[string]*Node{
						"enabled": {Description: "Whether enabled."},
					},
				},
			},
		},
		"AWSPrivateLink": {Description: "Standalone APL."},
		"AIAgent":        {Description: "Unreleased."},
	}

	got := FilterByRoots(tree, []string{"Cluster"})

	if len(got) != 1 {
		t.Fatalf("got %d roots, want 1", len(got))
	}
	if _, ok := got["Cluster"]; !ok {
		t.Error("Cluster missing from filtered tree")
	}
	if _, ok := got["AWSPrivateLink"]; ok {
		t.Error("AWSPrivateLink should have been filtered out")
	}
	if _, ok := got["AIAgent"]; ok {
		t.Error("AIAgent should have been filtered out")
	}

	apl := got["Cluster"].Fields["aws_private_link"]
	if apl == nil || apl.Fields["enabled"].Description != "Whether enabled." {
		t.Error("subtree under kept root lost content")
	}
}

func TestFilterByRoots_Empty(t *testing.T) {
	tree := map[string]*Node{"A": {}, "B": {}}
	got := FilterByRoots(tree, nil)
	if len(got) != 0 {
		t.Errorf("empty roots should produce empty tree, got %d entries", len(got))
	}
}

func TestFilterByRoots_MissingRootIgnored(t *testing.T) {
	tree := map[string]*Node{"A": {Description: "a"}}
	got := FilterByRoots(tree, []string{"A", "DoesNotExist"})
	if len(got) != 1 {
		t.Errorf("want 1 entry, got %d", len(got))
	}
	if got["A"].Description != "a" {
		t.Error("A content lost")
	}
}
