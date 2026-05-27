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

import "testing"

func TestAvroBodiesEquivalent(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "byte-identical",
			a:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			b:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			expected: true,
		},
		{
			name: "fqn type reference vs namespace-relative — the dataplane-v7 trigger case",
			a: `{
				"type":"record","name":"Account","namespace":"tfrp.v7",
				"fields":[{"name":"user","type":"tfrp.v7.User"},{"name":"id","type":"string"}]
			}`,
			b: `{
				"type":"record","name":"Account","namespace":"tfrp.v7",
				"fields":[{"name":"user","type":"User"},{"name":"id","type":"string"}]
			}`,

			expected: true,
		},
		{
			name: "whitespace and key order differences",
			a:    `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			b: `{
				"namespace": "tfrp.v7",
				"name":      "User",
				"type":      "record",
				"fields":    [{ "type": "string", "name": "id" }]
			}`,
			expected: true,
		},
		{
			name:     "non-essential metadata (doc) stripped by canonical form",
			a:        `{"type":"record","name":"User","namespace":"tfrp.v7","doc":"a user","fields":[{"name":"id","type":"string"}]}`,
			b:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			expected: true,
		},
		{
			name:     "different field set — not equivalent",
			a:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			b:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"int"}]}`,
			expected: false,
		},
		{
			name:     "different record name — not equivalent",
			a:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			b:        `{"type":"record","name":"Account","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			expected: false,
		},
		{
			name:     "malformed JSON on one side — false, no panic",
			a:        `{"type":"record","name":"User"`,
			b:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[]}`,
			expected: false,
		},
		{
			name:     "valid JSON but not Avro on one side",
			a:        `{"some":"random","object":42}`,
			b:        `{"type":"record","name":"User","namespace":"tfrp.v7","fields":[{"name":"id","type":"string"}]}`,
			expected: false,
		},
		{
			name:     "both sides empty string",
			a:        "",
			b:        "",
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AvroBodiesEquivalent(tt.a, tt.b); got != tt.expected {
				t.Errorf("AvroBodiesEquivalent() = %v, want %v\n--- a ---\n%s\n--- b ---\n%s",
					got, tt.expected, tt.a, tt.b)
			}
		})
	}
}
