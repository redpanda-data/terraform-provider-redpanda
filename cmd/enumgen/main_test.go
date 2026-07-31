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
	"testing"
)

// TestCarveoutParityAgainstRepo runs enumgen's carve-out parity check against
// the real codegen.yaml and handrolled.go, so a missing mapper direction fails
// `task test:unit` instead of surfacing only at the next `task generate:models`.
// A deadcode sweep once deleted the "unreachable" StringTo*State mappers and
// silently broke generation; this is the tripwire.
func TestCarveoutParityAgainstRepo(t *testing.T) {
	cfg, err := loadCodegenConfig(filepath.Join("..", "..", "redpanda", "resources", "codegen.yaml"))
	if err != nil {
		t.Fatalf("load codegen config: %v", err)
	}
	if len(cfg.EnumCarveouts) == 0 {
		t.Fatal("codegen.yaml lists no enum carve-outs; the parity guard would be vacuous")
	}

	carveoutSet := make(map[string]string, len(cfg.EnumCarveouts))
	for _, c := range cfg.EnumCarveouts {
		carveoutSet[funcNameRoot(c.Enum)] = c.Reason
	}

	if err := verifyCarveoutParity(filepath.Join("..", "..", "redpanda", "utils", "enums", "handrolled.go"), carveoutSet); err != nil {
		t.Fatalf("carve-out parity broken — task generate:models will fail: %v", err)
	}
}

func TestVerifyCarveoutParity_DetectsMissingDirection(t *testing.T) {
	testCases := []struct {
		name        string
		src         string
		expectError bool
	}{
		{
			name: "both directions present",
			src: `package enums
func FooToString(v int) string { return "" }
func StringToFoo(s string) int { return 0 }
`,
		},
		{
			name: "missing reverse direction",
			src: `package enums
func FooToString(v int) string { return "" }
`,
			expectError: true,
		},
		{
			name: "missing forward direction",
			src: `package enums
func StringToFoo(s string) int { return 0 }
`,
			expectError: true,
		},
		{
			name: "unexported mappers do not count",
			src: `package enums
func fooToString(v int) string { return "" }
func stringToFoo(s string) int { return 0 }
`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "handrolled.go")
			if err := os.WriteFile(p, []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}

			err := verifyCarveoutParity(p, map[string]string{"Foo": "test"})

			if tc.expectError && err == nil {
				t.Fatal("expected parity error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Fatalf("unexpected parity error: %v", err)
			}
		})
	}
}
