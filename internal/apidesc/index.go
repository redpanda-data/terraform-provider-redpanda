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
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// File is the on-disk structure of apidescriptions.yaml. Wrapping the schemas
// under a `schemas:` key leaves room for future top-level metadata without
// breaking the parser.
type File struct {
	Schemas map[string]*Node `yaml:"schemas"`
}

// Index is the runtime view of an apidescriptions.yaml file. It holds the
// nested tree for round-trip editing and a flat map for O(1) lookup by dotted
// path.
type Index struct {
	Source  string
	Schemas map[string]*Node
	flat    map[string]string
}

// Stats counts description lookups during schemagen merge. Reported per
// resource to give visibility into API coverage.
type Stats struct {
	Attempted int
	Matched   int
}

// Load reads and parses an apidescriptions.yaml file. Returns (nil, nil) if
// the file does not exist — callers should treat a missing index as "skip
// the API description pass" and fall back to mechanical defaults.
func Load(path string) (*Index, error) {
	data, err := fileutil.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	idx := &Index{
		Source:  path,
		Schemas: f.Schemas,
		flat:    make(map[string]string),
	}
	for name, node := range f.Schemas {
		idx.addToFlat(name, node)
	}
	return idx, nil
}

// Len returns the total entry count, including entries with empty descriptions.
func (i *Index) Len() int {
	if i == nil {
		return 0
	}
	return len(i.flat)
}

// Lookup returns the description for a dotted path like
// "Cluster.aws_private_link.status.console_port". Returns ("", false) if the
// path is absent or its description is empty.
func (i *Index) Lookup(path string) (string, bool) {
	if i == nil {
		return "", false
	}
	desc, ok := i.flat[path]
	if !ok || desc == "" {
		return "", false
	}
	return desc, true
}

func (i *Index) addToFlat(prefix string, node *Node) {
	if node == nil {
		return
	}
	i.flat[prefix] = node.Description
	for name, child := range node.Fields {
		i.addToFlat(prefix+"."+name, child)
	}
}

// Encode renders an Index to deterministic YAML bytes with sorted keys at
// every level.
func Encode(idx *File, headerComment string) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "schemas"},
		nodeToYAML(idx.Schemas),
	)

	var buf []byte
	if headerComment != "" {
		buf = append(buf, headerComment...)
		if headerComment[len(headerComment)-1] != '\n' {
			buf = append(buf, '\n')
		}
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("marshal index: %w", err)
	}
	buf = append(buf, out...)
	return buf, nil
}

func nodeToYAML(schemas map[string]*Node) *yaml.Node {
	names := make([]string, 0, len(schemas))
	for n := range schemas {
		names = append(names, n)
	}
	sort.Strings(names)

	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range names {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: name},
			encodeNode(schemas[name]),
		)
	}
	return m
}

func encodeNode(n *Node) *yaml.Node {
	if n == nil {
		return &yaml.Node{Kind: yaml.MappingNode}
	}
	m := &yaml.Node{Kind: yaml.MappingNode}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "description"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: n.Description, Style: yaml.DoubleQuotedStyle},
	)
	if len(n.Fields) > 0 {
		names := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			names = append(names, k)
		}
		sort.Strings(names)

		fieldsMap := &yaml.Node{Kind: yaml.MappingNode}
		for _, name := range names {
			fieldsMap.Content = append(fieldsMap.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: name},
				encodeNode(n.Fields[name]),
			)
		}
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "fields"},
			fieldsMap,
		)
	}
	return m
}
