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

package schemagen

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
)

// WriteTodos inserts todo: true entries into a YAML config file for
// uncovered proto fields. Walks the YAML structure to insert each entry
// at the correct nesting depth.
func WriteTodos(configPath string, uncovered []UncoveredField) error {
	if len(uncovered) == 0 {
		return nil
	}

	data, err := fileutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	var topLevel []string
	var nested []string
	for _, uf := range uncovered {
		if strings.Contains(uf.Path, ".") {
			nested = append(nested, uf.Path)
		} else {
			topLevel = append(topLevel, uf.Path)
		}
	}

	sort.Strings(nested)
	for _, path := range nested {
		lines = insertNestedTodo(lines, path)
	}

	if len(topLevel) > 0 {
		sort.Strings(topLevel)
		lines = append(lines,
			"",
			"# --- New proto fields (auto-added by schemagen -todo) ---")
		for _, name := range topLevel {
			lines = append(lines,
				fmt.Sprintf("%s:", name),
				"  todo: true")
		}
	}

	result := strings.Join(lines, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return os.WriteFile(configPath, []byte(result), 0o600)
}

func insertNestedTodo(lines []string, path string) []string {
	parts := strings.Split(path, ".")
	leaf := parts[len(parts)-1]
	parents := parts[:len(parts)-1]

	searchStart := 0
	expectedSegIndent := 0
	for i, segment := range parents {
		found, segIdx, _ := findYAMLKey(lines, searchStart, expectedSegIndent, segment)
		if !found {
			insertIdx := endOfBlock(lines, searchStart, expectedSegIndent)
			scaffold := buildSegmentScaffold(parents[i:], leaf, expectedSegIndent)
			return insertAt(lines, insertIdx, scaffold)
		}
		fieldsIndent := expectedSegIndent + 2

		fFound, fIdx, _ := findYAMLKey(lines, segIdx+1, fieldsIndent, "fields")
		if !fFound {
			insertIdx := endOfBlock(lines, segIdx+1, fieldsIndent)
			scaffold := buildFieldsScaffold(parents[i+1:], leaf, fieldsIndent)
			return insertAt(lines, insertIdx, scaffold)
		}
		searchStart = fIdx + 1
		expectedSegIndent = fieldsIndent + 2
	}

	childIndent := expectedSegIndent
	insertIdx := endOfBlock(lines, searchStart, childIndent)
	return insertAt(lines, insertIdx, buildLeafLines(leaf, childIndent))
}

func endOfBlock(lines []string, startIdx, blockIndent int) int {
	insertIdx := startIdx
	for insertIdx < len(lines) {
		line := lines[insertIdx]
		if line == "" {
			insertIdx++
			continue
		}
		if countIndent(line) < blockIndent {
			break
		}
		insertIdx++
	}
	return insertIdx
}

func buildSegmentScaffold(segments []string, leaf string, segIndent int) []string {
	var out []string
	indent := segIndent
	for _, s := range segments {
		pad := strings.Repeat(" ", indent)
		out = append(out,
			fmt.Sprintf("%s%s:", pad, s),
			fmt.Sprintf("%s  fields:", pad),
		)
		indent += 4
	}
	return append(out, buildLeafLines(leaf, indent)...)
}

func buildFieldsScaffold(remainingSegments []string, leaf string, fieldsIndent int) []string {
	out := []string{fmt.Sprintf("%sfields:", strings.Repeat(" ", fieldsIndent))}
	segIndent := fieldsIndent + 2
	for _, s := range remainingSegments {
		pad := strings.Repeat(" ", segIndent)
		out = append(out,
			fmt.Sprintf("%s%s:", pad, s),
			fmt.Sprintf("%s  fields:", pad),
		)
		segIndent += 4
	}
	return append(out, buildLeafLines(leaf, segIndent)...)
}

func buildLeafLines(leaf string, indent int) []string {
	prefix := strings.Repeat(" ", indent)
	return []string{
		fmt.Sprintf("%s%s:", prefix, leaf),
		fmt.Sprintf("%s  todo: true", prefix),
	}
}

func insertAt(lines []string, idx int, block []string) []string {
	result := make([]string, 0, len(lines)+len(block))
	result = append(result, lines[:idx]...)
	result = append(result, block...)
	return append(result, lines[idx:]...)
}

func findYAMLKey(lines []string, startIdx, minIndent int, key string) (found bool, lineIdx, keyIndent int) {
	target := key + ":"
	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		lineIndent := countIndent(line)
		trimmed := strings.TrimSpace(line)

		if lineIndent < minIndent && trimmed != "" {
			return false, 0, 0
		}

		if trimmed == target && lineIndent >= minIndent {
			return true, i, lineIndent
		}

		if strings.HasPrefix(trimmed, target) && lineIndent >= minIndent {
			return true, i, lineIndent
		}
	}
	return false, 0, 0
}

func countIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
