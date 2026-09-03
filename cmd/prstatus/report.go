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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// PRInfo is the subset of PR facts the report prints.
type PRInfo struct {
	Number         int
	Title          string
	URL            string
	Branch         string
	Head           string
	ReviewDecision string
}

// JobFailure is one failing CI job with the log lines worth reading.
type JobFailure struct {
	Job   string
	State string
	URL   string
	Lines []string
}

// ReviewThread is an unresolved review comment thread.
type ReviewThread struct {
	Path   string
	Line   int
	Author string
	Body   string
	URL    string
}

// Report is everything Render needs.
type Report struct {
	PR       PRInfo
	Failures []JobFailure
	Threads  []ReviewThread
	Note     string
	LogDir   string
}

// TruncateBody keeps a thread body readable in a summary; the link carries the rest.
func TruncateBody(body string, maxRunes int) string {
	if utf8.RuneCountInString(body) <= maxRunes {
		return body
	}
	runes := []rune(body)
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// WriteLogs stores each job's full log as <dir>/<job>.log.cld so failures can be
// read selectively instead of dumped into context. The .cld suffix keeps them
// out of git.
func WriteLogs(dir string, logs map[string]string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	var paths []string
	for job, content := range logs {
		p := filepath.Join(dir, unsafeName.ReplaceAllString(job, "_")+".log.cld")
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

var (
	escapeRe  = regexp.MustCompile(`\x1b_bk;t=\d+\x07|\x1b\[[0-9;]*[A-Za-z]`)
	failureRe = regexp.MustCompile(strings.Join([]string{
		`^--- FAIL`, `^FAIL\b`, `panic:`, `^\s*Error:`, `\bError: `,
		`^\S+\.go:\d+:\d+: `,
		`\.go:\d+: .*(?i:error|fail|expected|unexpected|panic|timeout)`,
		`expected empty plan`, `level=error`, `\bERROR\b`, `exit status [1-9]`,
		`^diff --git `, `out of date`,
	}, "|"))
)

// ExtractFailures keeps the lines of a job log that name a failure, in order, capped at maxLines.
func ExtractFailures(log string, maxLines int) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(escapeRe.ReplaceAllString(log, ""), "\n") {
		line := strings.TrimRight(raw, " \r\t")
		key := strings.TrimSpace(line)
		if key == "" || seen[key] || !failureRe.MatchString(key) {
			continue
		}
		seen[key] = true
		out = append(out, line)
		if len(out) == maxLines {
			break
		}
	}
	return out
}

// IsFailedJob is true for states that mean the job ran and did not pass.
// "broken" is a job whose conditions excluded it, so it is not a failure.
func IsFailedJob(state string) bool {
	return state == "failed" || state == "timed_out"
}

var jobOrder = []string{"lint", "docs", "generate", "ready", "unit", "integration", "test_", "release"}

// JobPriority orders jobs so the cheapest, most blocking failures come first.
func JobPriority(job string) int {
	name := strings.ToLower(job)
	for i, prefix := range jobOrder {
		if strings.HasPrefix(name, prefix) {
			return i
		}
	}
	return len(jobOrder)
}

// Render formats the report as markdown: failures first, then review threads.
func Render(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# PR #%d: %s\n%s\n", r.PR.Number, r.PR.Title, r.PR.URL)
	if r.PR.Branch != "" || r.PR.Head != "" {
		fmt.Fprintf(&b, "branch %s at %s\n", r.PR.Branch, shortSHA(r.PR.Head))
	}
	if r.PR.ReviewDecision != "" {
		fmt.Fprintf(&b, "review: %s\n", r.PR.ReviewDecision)
	}

	b.WriteString("\n## CI failures\n")
	if len(r.Failures) == 0 {
		b.WriteString("no failing jobs\n")
	}
	sorted := append([]JobFailure(nil), r.Failures...)
	sort.SliceStable(sorted, func(i, j int) bool { return JobPriority(sorted[i].Job) < JobPriority(sorted[j].Job) })
	for _, f := range sorted {
		fmt.Fprintf(&b, "\n### %s", f.Job)
		if f.State != "" {
			fmt.Fprintf(&b, " (%s)", f.State)
		}
		b.WriteString("\n")
		if f.URL != "" {
			fmt.Fprintf(&b, "%s\n", f.URL)
		}
		if len(f.Lines) > 0 {
			b.WriteString("```\n")
			b.WriteString(strings.Join(f.Lines, "\n"))
			b.WriteString("\n```\n")
		}
	}
	if r.LogDir != "" && len(r.Failures) > 0 {
		fmt.Fprintf(&b, "\nFull job logs: %s/<job>.log.cld (grep them; do not cat them).\n", r.LogDir)
	}
	if r.Note != "" {
		fmt.Fprintf(&b, "\n%s\n", r.Note)
	}

	b.WriteString("\n## Unresolved review threads\n")
	if len(r.Threads) == 0 {
		b.WriteString("none\n")
	}
	for _, t := range r.Threads {
		loc := t.Path
		if t.Line > 0 {
			loc = fmt.Sprintf("%s:%d", t.Path, t.Line)
		}
		fmt.Fprintf(&b, "\n- %s (%s)\n  %s\n", loc, t.Author, strings.ReplaceAll(TruncateBody(strings.TrimSpace(t.Body), 600), "\n", "\n  "))
		if t.URL != "" {
			fmt.Fprintf(&b, "  %s\n", t.URL)
		}
	}

	b.WriteString("\nTriage: fix lint, docs, and generate before tests; treat every failure as real until reproduced locally with the same tier and env.\n")
	return b.String()
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
