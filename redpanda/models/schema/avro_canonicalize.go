// Copyright 2025 Redpanda Data, Inc.
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

package schema

import (
	"bytes"
	"encoding/json"
	"strings"
)

var avroPrimitives = map[string]bool{
	"null": true, "boolean": true, "int": true, "long": true,
	"float": true, "double": true, "bytes": true, "string": true,
	"record": true, "enum": true, "array": true, "map": true,
	"union": true, "fixed": true, "error": true,
}

// AvroBodiesEquivalent reports whether two Avro JSON schema bodies represent
// the same schema after canonicalization (resolves namespace-relative refs,
// strips non-essential keys, normalizes JSON key order + whitespace).
func AvroBodiesEquivalent(a, b string) bool {
	if a == b {
		return true
	}
	ac, err := canonicalizeAvroBody(a)
	if err != nil {
		return false
	}
	bc, err := canonicalizeAvroBody(b)
	if err != nil {
		return false
	}
	return ac == bc
}

func canonicalizeAvroBody(body string) (string, error) {
	if body == "" {
		return "", nil
	}
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return "", err
	}
	resolveTypeRefs(root, "")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return "", err
	}

	return strings.TrimRight(buf.String(), "\n"), nil
}

var nonEssentialKeys = map[string]bool{
	"doc":     true,
	"aliases": true,
	"default": true,
	"order":   true,
}

func resolveTypeRefs(v any, ns string) {
	switch x := v.(type) {
	case map[string]any:
		current := ns

		if n, ok := x["namespace"].(string); ok && n != "" {
			current = n
		} else if name, ok := x["name"].(string); ok {
			if idx := strings.LastIndex(name, "."); idx > 0 {
				current = name[:idx]
			}
		}
		for k := range x {
			if nonEssentialKeys[k] {
				delete(x, k)
			}
		}
		for k, val := range x {
			switch k {
			case "type", "items", "values":
				if s, ok := val.(string); ok {
					x[k] = resolveTypeName(s, current)
				} else {
					resolveTypeRefs(val, current)
				}
			default:
				resolveTypeRefs(val, current)
			}
		}
	case []any:

		for i, val := range x {
			if s, ok := val.(string); ok {
				x[i] = resolveTypeName(s, ns)
			} else {
				resolveTypeRefs(val, ns)
			}
		}
	}
}

func resolveTypeName(ref, ns string) string {
	if avroPrimitives[ref] {
		return ref
	}
	if strings.Contains(ref, ".") || ns == "" {
		return ref
	}
	return ns + "." + ref
}
