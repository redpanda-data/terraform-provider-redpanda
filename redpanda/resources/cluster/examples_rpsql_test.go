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

package cluster

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// rpsqlExample is the BYOVPC example rendered into docs/resources/cluster.md
// via the cluster doc template. It is the only copy-pasteable Redpanda SQL
// config we publish.
const rpsqlExample = "examples/byovpc/gcp/main.tf"

// TestRpsqlExampleMatchesSchema guards the published Redpanda SQL example
// against schema drift. terraform validate cannot run on this example — the
// upstream BYOVPC module declares the wrong provider namespace
// (hashicorp/redpanda), so init never completes — which leaves the example
// otherwise unchecked. Renaming or removing an rpsql attribute would silently
// leave users copying config the provider rejects.
func TestRpsqlExampleMatchesSchema(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), rpsqlExample))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}

	known := rpsqlSchemaAttrs(t)
	if len(known) == 0 {
		t.Fatal("no rpsql attributes found in the cluster schema — the walk is wrong, not the example")
	}

	// Attribute assignments only. A bare identifier match would also pick up
	// module outputs (module.redpanda_gcp.rpsql_..._email) and comment prose,
	// neither of which is a schema attribute.
	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*(rpsql[a-z_]*)\s*=`).FindAllStringSubmatch(string(body), -1) {
		used[m[1]] = true
	}
	if len(used) == 0 {
		t.Fatalf("%s references no rpsql attributes — the example lost its Redpanda SQL block", rpsqlExample)
	}

	for attr := range used {
		if !known[attr] {
			t.Errorf("%s uses %q, which is not an attribute on redpanda_cluster — the example has drifted from the schema", rpsqlExample, attr)
		}
	}
}

// rpsqlSchemaAttrs collects every attribute name beginning with "rpsql" from
// the cluster resource schema, at any nesting depth.
func rpsqlSchemaAttrs(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	var walk func(attrs map[string]schema.Attribute)
	walk = func(attrs map[string]schema.Attribute) {
		for name, a := range attrs {
			if len(name) >= 5 && name[:5] == "rpsql" {
				out[name] = true
			}
			if nested, ok := a.(schema.SingleNestedAttribute); ok {
				walk(nested.Attributes)
			}
		}
	}
	walk(ResourceClusterSchema(context.Background()).Attributes)
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for repo root")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}
