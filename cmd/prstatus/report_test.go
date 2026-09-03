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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFailures(t *testing.T) {
	log := strings.Join([]string{
		"\x1b_bk;t=1700000000000\x07--- running tests",
		"=== RUN   TestIntegration_Cluster_Create",
		"    integration_cluster_test.go:120: expected empty plan but resource has planned action(s)",
		"--- FAIL: TestIntegration_Cluster_Create (0.10s)",
		"ok   github.com/x/y/pkg 0.5s",
		"FAIL github.com/x/y/redpanda/resources/cluster 12.3s",
		"redpanda/utils/retry.go:29:2: time-naming: var retryWaitMin (revive)",
		"panic: runtime error: index out of range [3]",
		"\x1b[31mError: exit status 1\x1b[0m",
		"    upgrade_acl_test.go:22: KAFKA_CLUSTER_API_URL not set; set to a live cluster to run",
		"diff --git a/docs/resources/cluster.md b/docs/resources/cluster.md",
		"Documentation is out of date. Run './taskw docs' locally and commit the changes.",
	}, "\n")
	got := ExtractFailures(log, 10)
	for _, want := range []string{
		"--- FAIL: TestIntegration_Cluster_Create",
		"integration_cluster_test.go:120: expected empty plan",
		"FAIL github.com/x/y/redpanda/resources/cluster",
		"retry.go:29:2: time-naming",
		"panic: runtime error",
		"Error: exit status 1",
		"diff --git a/docs/resources/cluster.md",
		"Documentation is out of date",
	} {
		if !containsLine(got, want) {
			t.Errorf("missing %q in\n%s", want, strings.Join(got, "\n"))
		}
	}
	for _, l := range got {
		if strings.Contains(l, "\x1b") {
			t.Errorf("ANSI/Buildkite escapes not stripped: %q", l)
		}
		if strings.HasPrefix(l, "ok ") || strings.HasPrefix(l, "=== RUN") || strings.Contains(l, "KAFKA_CLUSTER_API_URL not set") {
			t.Errorf("noise line kept: %q", l)
		}
	}
	if n := len(ExtractFailures(log, 2)); n != 2 {
		t.Errorf("limit not applied: got %d lines", n)
	}
}

func TestJobPriority(t *testing.T) {
	order := []string{"lint", "docs", "generate", "ready", "unit", "integration", "test_network", "test_cluster_aws", "release:snapshot"}
	for i := 1; i < len(order); i++ {
		if JobPriority(order[i-1]) > JobPriority(order[i]) {
			t.Errorf("%s should sort before %s", order[i-1], order[i])
		}
	}
	if JobPriority("lint") == JobPriority("unit") {
		t.Error("lint and unit must not share a priority")
	}
}

func TestRender(t *testing.T) {
	r := Report{
		PR:       PRInfo{Number: 372, Title: "feat(cluster): dual listener mode", URL: "https://github.com/o/r/pull/372", Head: "abc123", ReviewDecision: "CHANGES_REQUESTED"},
		Failures: []JobFailure{{Job: "unit", URL: "https://buildkite.com/b/1#j", Lines: []string{"--- FAIL: TestX"}}, {Job: "lint", URL: "https://buildkite.com/b/1#k", Lines: []string{"a.go:1:1: issue"}}},
		Threads:  []ReviewThread{{Path: "redpanda/x.go", Line: 10, Author: "reviewer", Body: "please retry here"}},
		Note:     "",
	}
	out := Render(r)
	if !strings.Contains(out, "#372") || !strings.Contains(out, "CHANGES_REQUESTED") {
		t.Errorf("header missing PR facts:\n%s", out)
	}
	if strings.Index(out, "lint") > strings.Index(out, "unit") {
		t.Errorf("lint failure must be listed before unit:\n%s", out)
	}
	if strings.Index(out, "--- FAIL: TestX") > strings.Index(out, "please retry here") {
		t.Errorf("CI failures must come before review threads:\n%s", out)
	}
	if !strings.Contains(out, "redpanda/x.go:10") {
		t.Errorf("thread location missing:\n%s", out)
	}
	if !strings.Contains(Render(Report{PR: r.PR}), "no failing jobs") {
		t.Error("empty report should say there are no failing jobs")
	}
}

func containsLine(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func TestTruncateBody(t *testing.T) {
	long := strings.Repeat("word ", 300)
	got := TruncateBody(long, 200)
	if len(got) > 220 || !strings.HasSuffix(got, "…") {
		t.Errorf("not truncated with marker: len=%d tail=%q", len(got), got[max(0, len(got)-10):])
	}
	if TruncateBody("short", 200) != "short" {
		t.Error("short body must pass through")
	}
}

func TestWriteLogs(t *testing.T) {
	dir := t.TempDir()
	paths, err := WriteLogs(dir, map[string]string{"unit": "log a", "test_cluster_aws": "log b", "release:snapshot": "log c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("want 3 files, got %v", paths)
	}
	for job, want := range map[string]string{"unit": "log a", "release_snapshot": "log c"} {
		b, err := os.ReadFile(filepath.Clean(filepath.Join(dir, job+".log.cld")))
		if err != nil || string(b) != want {
			t.Errorf("%s: %v %q", job, err, b)
		}
	}
}

func TestIsFailedJob(t *testing.T) {
	for state, want := range map[string]bool{"failed": true, "timed_out": true, "broken": false, "passed": false, "skipped": false, "canceled": false} {
		if got := IsFailedJob(state); got != want {
			t.Errorf("IsFailedJob(%q) = %v, want %v", state, got, want)
		}
	}
}
