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

package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runHook(t *testing.T, msg string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	hook, err := os.ReadFile("commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "commit-msg"), hook, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "COMMIT_EDITMSG"), []byte(msg), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", "commit-msg", "COMMIT_EDITMSG")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCommitMsgHook(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		ok   bool
		want string
	}{
		{"conventional title", "feat(cluster): add dual listener mode\n", true, ""},
		{"no scope", "chore: update gitignore\n", true, ""},
		{"breaking marker", "feat(topic)!: drop legacy config keys\n", true, ""},
		{"body allowed", "fix(schemagen): order the state modifier first\n\nThe framework nulls unknowns before plan.\n", true, ""},
		{"revert exempt", "Revert \"feat(cluster): add dual listener mode\"\n", true, ""},
		{"fixup exempt", "fixup! feat(cluster): add dual listener mode\n", true, ""},
		{"merge exempt", "Merge branch 'main' into feat/x\n", true, ""},
		{"comment lines ignored", "fix(acl): retry on unavailable\n# Please enter the commit message\n# ENG-1234 mentioned only in a comment\n", true, ""},
		{"unknown type", "refactor(cluster): split plan modifiers\n", false, "type"},
		{"capitalized description", "fix(cluster): Add retry\n", false, "lowercase"},
		{"trailing period", "fix(cluster): add retry.\n", false, "period"},
		{"missing space after colon", "fix(cluster):add retry\n", false, "type"},
		{"over 72 characters", "feat(cluster): " + "add a very long description that keeps going past the limit\n", false, "72"},
		{"jira id in body", "fix(cluster): add retry\n\nFixes ENG-1234.\n", false, "ticket"},
		{"jira id in title", "fix(cluster): address K8S-99 flake\n", false, "ticket"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runHook(t, tc.msg)
			if tc.ok && err != nil {
				t.Fatalf("expected accept, got reject: %s", out)
			}
			if !tc.ok {
				if err == nil {
					t.Fatal("expected reject, got accept")
				}
				if tc.want != "" && !strings.Contains(out, tc.want) {
					t.Fatalf("reject reason %q does not mention %q", out, tc.want)
				}
			}
		})
	}
}
