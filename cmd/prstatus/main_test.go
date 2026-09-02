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

package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func pageJSON(t *testing.T, threads []map[string]any, next string) []byte {
	t.Helper()
	page := map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
		"reviewThreads": map[string]any{
			"nodes":    threads,
			"pageInfo": map[string]any{"hasNextPage": next != "", "endCursor": next},
		},
	}}}}
	b, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func threadNode(path string, resolved bool, comments int) map[string]any {
	nodes := make([]map[string]any, comments)
	for i := range nodes {
		nodes[i] = map[string]any{"author": map[string]any{"login": "rev"}, "body": fmt.Sprintf("c%d", i), "url": "u"}
	}
	return map[string]any{"isResolved": resolved, "isOutdated": false, "path": path, "line": 1, "comments": map[string]any{"nodes": nodes}}
}

func TestCollectThreads_Paginates(t *testing.T) {
	var cursors []string
	page := func(cursor string) ([]byte, error) {
		cursors = append(cursors, cursor)
		switch cursor {
		case "":
			return pageJSON(t, []map[string]any{threadNode("a.go", false, 1), threadNode("done.go", true, 1)}, "c1"), nil
		case "c1":
			return pageJSON(t, []map[string]any{threadNode("b.go", false, commentsPerThread)}, ""), nil
		default:
			return nil, fmt.Errorf("unexpected cursor %q", cursor)
		}
	}
	threads, err := collectThreads(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(cursors) != 2 || cursors[1] != "c1" {
		t.Fatalf("expected two pages driven by the end cursor, got %v", cursors)
	}
	if len(threads) != 2 || threads[0].Path != "a.go" || threads[1].Path != "b.go" {
		t.Fatalf("expected the unresolved thread from each page, got %+v", threads)
	}
	if threads[0].CommentsTruncated {
		t.Error("a one-comment thread must not be flagged as truncated")
	}
	if !threads[1].CommentsTruncated {
		t.Error("a thread at the comment cap must be flagged as truncated")
	}
}
