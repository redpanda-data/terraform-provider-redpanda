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

import (
	"fmt"
	"sort"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
	"gopkg.in/yaml.v3"
)

const maxDepth = 12

const refPrefix = "#/components/schemas/"

// Schema is the subset of OpenAPI 3 we care about for description walking.
// Unmarshaled via yaml.v3 from components.schemas entries. Everything except
// description, title, properties, items, additionalProperties, $ref is
// ignored.
type Schema struct {
	Ref                  string             `yaml:"$ref,omitempty"`
	Type                 string             `yaml:"type,omitempty"`
	Description          string             `yaml:"description,omitempty"`
	Title                string             `yaml:"title,omitempty"`
	Properties           map[string]*Schema `yaml:"properties,omitempty"`
	Items                *Schema            `yaml:"items,omitempty"`
	AdditionalProperties *Schema            `yaml:"additionalProperties,omitempty"`
}

// Spec is the minimal OpenAPI 3 envelope. Only components.schemas is read.
type Spec struct {
	Components struct {
		Schemas map[string]*Schema `yaml:"schemas"`
	} `yaml:"components"`
}

// LoadSpec reads and parses an OpenAPI YAML file.
func LoadSpec(path string) (*Spec, error) {
	data, err := fileutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &spec, nil
}

// Node is one level in the nested description tree. Description is the
// description for this field itself (may be empty). Fields holds any child
// properties inlined during ref resolution.
type Node struct {
	Description string           `yaml:"description"`
	Fields      map[string]*Node `yaml:"fields,omitempty"`
}

var filePriority = map[string]int{
	"cloudv2/openapi.controlplane.prod.yaml":       0,
	"cloudv2/openapi.dataplane.prod.yaml":          1,
	"cloudv2/openapi.dataplane.v1alpha2.prod.yaml": 2,
	"console/openapi.yaml":                         3,
	"console/openapi.v1alpha2.yaml":                4,
	"console/openapi.v1alpha3.yaml":                5,
}

func priorityOf(file string) int {
	if p, ok := filePriority[file]; ok {
		return p
	}
	return 1000
}

// Flatten merges one or more specs and produces the nested description tree
// keyed by top-level schema name. When multiple specs define the same schema
// with different descriptions, the spec with higher precedence (see
// filePriority) wins and a warning is emitted for each conflict. Unresolved
// refs and cycle guards also produce warnings. The function only returns an
// error for fatal problems like malformed input.
func Flatten(specs map[string]*Spec) (tree map[string]*Node, warnings []string, err error) {
	files := make([]string, 0, len(specs))
	for f := range specs {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool {
		pi, pj := priorityOf(files[i]), priorityOf(files[j])
		if pi != pj {
			return pi < pj
		}
		return files[i] < files[j]
	})

	allSchemas := make(map[string]*Schema)
	schemaSource := make(map[string]string)
	for _, file := range files {
		spec := specs[file]
		for name, s := range spec.Components.Schemas {
			if _, exists := allSchemas[name]; !exists {
				allSchemas[name] = s
				schemaSource[name] = file
			}
		}
	}

	tree = make(map[string]*Node)

	names := make([]string, 0, len(allSchemas))
	for n := range allSchemas {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		schema := allSchemas[name]
		root := walk(schema, allSchemas, map[string]bool{name: true}, 0, &warnings)
		tree[name] = root
	}

	for _, file := range files {
		spec := specs[file]
		fileNames := make([]string, 0, len(spec.Components.Schemas))
		for n := range spec.Components.Schemas {
			fileNames = append(fileNames, n)
		}
		sort.Strings(fileNames)
		for _, name := range fileNames {
			if schemaSource[name] == file {
				continue
			}
			s := spec.Components.Schemas[name]
			fileRoot := walk(s, spec.Components.Schemas, map[string]bool{name: true}, 0, &warnings)
			diffs := compareNodes(fileRoot, tree[name], name)
			for _, d := range diffs {
				warnings = append(warnings, fmt.Sprintf(
					"collision at %s: %s defines %q, using %q from %s",
					d.path, file, truncate(d.a, 80), truncate(d.b, 80), schemaSource[name],
				))
			}
		}
	}

	return tree, warnings, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func walk(s *Schema, allSchemas map[string]*Schema, visiting map[string]bool, depth int, warnings *[]string) *Node {
	if s == nil || depth > maxDepth {
		return &Node{}
	}

	desc := pickDescription(s)
	target := s
	if s.Ref != "" {
		refName := strings.TrimPrefix(s.Ref, refPrefix)
		if refName == s.Ref {
			*warnings = append(*warnings, fmt.Sprintf("unrecognized $ref: %q", s.Ref))
			return &Node{Description: desc}
		}
		if visiting[refName] {
			return &Node{Description: desc}
		}
		refSchema, ok := allSchemas[refName]
		if !ok {
			*warnings = append(*warnings, fmt.Sprintf("unresolved $ref: %q", refName))
			return &Node{Description: desc}
		}

		if desc == "" {
			desc = pickDescription(refSchema)
		}
		target = refSchema

		visiting[refName] = true
		defer delete(visiting, refName)
	}

	node := &Node{Description: desc}

	if len(target.Properties) > 0 {
		node.Fields = make(map[string]*Node)
		for propName, propSchema := range target.Properties {
			child := walkProperty(propSchema, allSchemas, visiting, depth+1, warnings)
			node.Fields[propName] = child
		}
	}

	return node
}

func walkProperty(p *Schema, allSchemas map[string]*Schema, visiting map[string]bool, depth int, warnings *[]string) *Node {
	if p == nil || depth > maxDepth {
		return &Node{}
	}

	if p.Ref != "" {
		return walk(p, allSchemas, visiting, depth, warnings)
	}

	desc := pickDescription(p)
	node := &Node{Description: desc}

	if p.Items != nil {
		itemsNode := walkProperty(p.Items, allSchemas, visiting, depth+1, warnings)

		if node.Description == "" {
			node.Description = itemsNode.Description
		}
		if len(itemsNode.Fields) > 0 {
			node.Fields = itemsNode.Fields
		}
	}

	if p.AdditionalProperties != nil {
		apNode := walkProperty(p.AdditionalProperties, allSchemas, visiting, depth+1, warnings)
		if node.Description == "" {
			node.Description = apNode.Description
		}
		if len(apNode.Fields) > 0 && node.Fields == nil {
			node.Fields = apNode.Fields
		}
	}

	if len(p.Properties) > 0 {
		if node.Fields == nil {
			node.Fields = make(map[string]*Node)
		}
		for propName, propSchema := range p.Properties {
			node.Fields[propName] = walkProperty(propSchema, allSchemas, visiting, depth+1, warnings)
		}
	}

	return node
}

func pickDescription(s *Schema) string {
	if s == nil {
		return ""
	}
	raw := s.Description
	if raw == "" {
		raw = s.Title
	}
	return normalize(raw)
}

func normalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

type diffEntry struct {
	path string
	a    string
	b    string
}

func compareNodes(a, b *Node, path string) []diffEntry {
	if a == nil || b == nil {
		return nil
	}
	var diffs []diffEntry
	if a.Description != "" && b.Description != "" && a.Description != b.Description {
		diffs = append(diffs, diffEntry{path: path, a: a.Description, b: b.Description})
	}
	for name, aChild := range a.Fields {
		bChild, ok := b.Fields[name]
		if !ok {
			continue
		}
		diffs = append(diffs, compareNodes(aChild, bChild, path+"."+name)...)
	}
	return diffs
}
