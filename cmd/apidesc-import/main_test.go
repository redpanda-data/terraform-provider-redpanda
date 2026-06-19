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

package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeSchema(t *testing.T, dir, resource, body string) {
	t.Helper()
	resDir := filepath.Join(dir, resource)
	if err := os.MkdirAll(resDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resDir, "schema.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func refRoots(t *testing.T, dir string) []string {
	t.Helper()
	refs, err := collectAPISchemas(dir)
	if err != nil {
		t.Fatalf("collectAPISchemas: %v", err)
	}
	out := make([]string, 0, len(refs))
	for k := range refs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestCollectAPISchemas_WriteSchemasScoped — api_write_schemas roots are
// registered as additional scoped roots alongside the api_schema read root.
func TestCollectAPISchemas_WriteSchemasScoped(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, "cluster", "api_schema: Cluster\napi_write_schemas: [ClusterCreate, ClusterUpdate]\n")

	got := refRoots(t, dir)
	want := []string{"Cluster", "ClusterCreate", "ClusterUpdate"}
	if len(got) != len(want) {
		t.Fatalf("roots: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("roots: got %v, want %v", got, want)
			break
		}
	}
}

// TestCollectAPISchemas_NoWriteSchemas — a resource without api_write_schemas
// registers only its read root (the existing single-root behavior).
func TestCollectAPISchemas_NoWriteSchemas(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, "network", "api_schema: Network\n")

	got := refRoots(t, dir)
	if len(got) != 1 || got[0] != "Network" {
		t.Errorf("roots: got %v, want [Network]", got)
	}
}

// TestCollectAPISchemas_WriteSchemasInheritPrefix — strip_openapi_prefix applies
// to the write roots too, so they resolve against the prefixed openapi index.
func TestCollectAPISchemas_WriteSchemasInheritPrefix(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, "cluster", "api_schema: Cluster\napi_write_schemas: [ClusterCreate]\nstrip_openapi_prefix: v1.\n")

	refs, err := collectAPISchemas(dir)
	if err != nil {
		t.Fatalf("collectAPISchemas: %v", err)
	}
	for _, root := range []string{"Cluster", "ClusterCreate"} {
		ref, ok := refs[root]
		if !ok {
			t.Fatalf("missing root %q", root)
		}
		if ref.prefix != "v1." {
			t.Errorf("%s prefix: got %q, want %q", root, ref.prefix, "v1.")
		}
	}
}
