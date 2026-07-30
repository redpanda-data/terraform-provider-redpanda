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

package resources_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// perClusterClients are the client fields whose RPCs cross a per-cluster
// endpoint. Those endpoints are not serving the moment a cluster reports Ready,
// so every call through them needs the shared retry policy.
var perClusterClients = map[string]bool{
	"SecretClient":   true,
	"UserClient":     true,
	"ACLClient":      true,
	"TopicClient":    true,
	"PipelineClient": true,
	"SecurityClient": true,
}

// notYetMigrated records call sites still outside utils.DataplaneCall, so the
// guard can be enforced now and the list shrunk deliberately rather than the
// whole check waiting on a full migration. Removing an entry is the point;
// adding one needs a reason in review.
var notYetMigrated = map[string]bool{
	"topic":  true, // classifies via isTransientBrokerError, which delegates
	"secret": true, // hand-rolled probe-and-adopt, predates the helper
	"user":   true,
	"acl":    true,
}

// TestDataplaneRPCsGoThroughDataplaneCall fails when a resource issues an RPC on
// a per-cluster client outside utils.DataplaneCall or utils.DataplaneCallOnce.
//
// Every retry defect found so far was a call site deciding policy for itself:
// redpanda_pipeline's create had no retry and failed on a warm-up window;
// redpanda_role had none and blocked three acceptance suites; redpanda_secret
// adopted AlreadyExists unconditionally and took over another stack's secret.
// Each was caught by a live failure. This catches the next one at unit tier.
func TestDataplaneRPCsGoThroughDataplaneCall(t *testing.T) {
	offenders := map[string][]string{}

	// Walk this package's own tree; a relative path never contains the repo
	// prefix, so filtering on one silently matches nothing.
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		resource := filepath.Base(filepath.Dir(path))
		if notYetMigrated[resource] {
			return nil
		}

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}

		// Walk with a stack so each RPC call can be tested for an enclosing
		// DataplaneCall argument.
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)

			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			field, ok := sel.X.(*ast.SelectorExpr)
			if !ok || !perClusterClients[field.Sel.Name] {
				return true
			}
			if enclosedByDataplaneCall(stack) {
				return true
			}
			pos := fset.Position(call.Pos())
			offenders[resource] = append(offenders[resource],
				fmt.Sprintf("%s:%d %s", filepath.Base(path), pos.Line, sel.Sel.Name))
			return true
		})
		return nil
	})
	require.NoError(t, err)

	if len(offenders) == 0 {
		return
	}
	keys := make([]string, 0, len(offenders))
	for k := range offenders {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var report []string
	for _, k := range keys {
		report = append(report, k+": "+strings.Join(offenders[k], ", "))
	}
	t.Fatalf("per-cluster RPCs outside utils.DataplaneCall — wrap them, or use "+
		"utils.DataplaneCallOnce if the call must not retry:\n  %s",
		strings.Join(report, "\n  "))
}

// enclosedByDataplaneCall reports whether any ancestor is a utils.DataplaneCall
// or utils.DataplaneCallOnce invocation.
func enclosedByDataplaneCall(stack []ast.Node) bool {
	for _, n := range stack {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if sel.Sel.Name == "DataplaneCall" || sel.Sel.Name == "DataplaneCallOnce" {
			return true
		}
		// Generic instantiation renders as DataplaneCall[T](...).
		if idx, ok := call.Fun.(*ast.IndexExpr); ok {
			if s, ok := idx.X.(*ast.SelectorExpr); ok &&
				(s.Sel.Name == "DataplaneCall" || s.Sel.Name == "DataplaneCallOnce") {
				return true
			}
		}
	}
	return false
}
