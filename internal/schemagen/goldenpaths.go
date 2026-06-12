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
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
	"gopkg.in/yaml.v3"
)

type goldenAttr struct {
	Name       string       `yaml:"name"`
	Attributes []goldenAttr `yaml:"attributes"`
}

type goldenSchema struct {
	Attributes []goldenAttr `yaml:"attributes"`
}

// ParseGoldenPaths loads the golden YAML at path and returns the set of
// dotted attribute paths it contains (e.g. "aws_private_link.allowed_principals").
// Returns (nil, nil) if the file does not exist — callers should treat that
// as "no baseline available" and fall back to marking all uncovered fields.
func ParseGoldenPaths(path string) (map[string]struct{}, error) {
	data, err := fileutil.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read golden %s: %w", path, err)
	}

	var g goldenSchema
	if err := yaml.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse golden %s: %w", path, err)
	}

	paths := make(map[string]struct{})
	collectGoldenPaths(g.Attributes, "", paths)
	return paths, nil
}

func collectGoldenPaths(attrs []goldenAttr, prefix string, out map[string]struct{}) {
	for _, a := range attrs {
		if a.Name == "" {
			continue
		}
		p := joinPath(prefix, a.Name)
		out[p] = struct{}{}
		if len(a.Attributes) > 0 {
			collectGoldenPaths(a.Attributes, p, out)
		}
	}
}

// FilterUncoveredByGolden returns the subset of uncovered fields whose paths
// are NOT present in accepted AND whose ancestors are also not kept (so that
// marking a truly-new parent as todo implicitly handles its descendants).
// If accepted is nil (no golden available), the original slice is returned
// unchanged.
func FilterUncoveredByGolden(uncovered []UncoveredField, accepted map[string]struct{}) []UncoveredField {
	if accepted == nil {
		return uncovered
	}
	byLen := make([]UncoveredField, 0, len(uncovered))
	for _, u := range uncovered {
		if _, ok := accepted[u.Path]; ok {
			continue
		}
		byLen = append(byLen, u)
	}
	sort.SliceStable(byLen, func(i, j int) bool {
		return strings.Count(byLen[i].Path, ".") < strings.Count(byLen[j].Path, ".")
	})
	kept := make(map[string]struct{}, len(byLen))
	out := make([]UncoveredField, 0, len(byLen))
	for _, u := range byLen {
		if hasKeptAncestor(u.Path, kept) {
			continue
		}
		kept[u.Path] = struct{}{}
		out = append(out, u)
	}
	return out
}

func hasKeptAncestor(path string, kept map[string]struct{}) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if _, ok := kept[path[:i]]; ok {
				return true
			}
		}
	}
	return false
}
