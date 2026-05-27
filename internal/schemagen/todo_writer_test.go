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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
)

func TestWriteTodos_SynthesizesMissingSegmentAndFields(t *testing.T) {
	in := `customer_managed_resources:
  fields:
    azure:
      todo: true
    gcp:
      fields:
        psc_v2_nat_subnet_name:
          todo: true
`
	path := writeTempYAML(t, in)

	err := WriteTodos(path, []UncoveredField{
		{Path: "customer_managed_resources.aws.oxla_node_group_instance_profile"},
		{Path: "customer_managed_resources.aws.oxla_security_group"},
	})
	if err != nil {
		t.Fatalf("WriteTodos: %v", err)
	}

	got := readFile(t, path)
	awsScaffold := "    aws:\n      fields:\n        oxla_node_group_instance_profile:\n          todo: true\n        oxla_security_group:\n          todo: true"
	if !strings.Contains(got, awsScaffold) {
		t.Errorf("expected aws scaffold with both oxla entries; got:\n%s", got)
	}
	if !strings.Contains(got, "azure:\n      todo: true") || !strings.Contains(got, "gcp:") {
		t.Errorf("expected original siblings intact; got:\n%s", got)
	}
}

func TestWriteTodos_InsertsIntoExistingFieldsBlock(t *testing.T) {
	in := `aws_private_link:
  fields:
    kafka_api_auth_mode:
      todo: true
`
	path := writeTempYAML(t, in)

	err := WriteTodos(path, []UncoveredField{
		{Path: "aws_private_link.http_proxy_auth_mode"},
	})
	if err != nil {
		t.Fatalf("WriteTodos: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "    http_proxy_auth_mode:\n      todo: true") {
		t.Errorf("expected http_proxy_auth_mode at indent 4 inside fields block; got:\n%s", got)
	}
	if !strings.Contains(got, "    kafka_api_auth_mode:\n      todo: true") {
		t.Errorf("expected original kafka_api_auth_mode entry intact; got:\n%s", got)
	}
}

func TestWriteTodos_AppendsTopLevel(t *testing.T) {
	in := `foo:
  optional: true
`
	path := writeTempYAML(t, in)

	err := WriteTodos(path, []UncoveredField{{Path: "sandbox"}})
	if err != nil {
		t.Fatalf("WriteTodos: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "sandbox:\n  todo: true\n") {
		t.Errorf("expected sandbox appended at top level; got:\n%s", got)
	}
}

func TestWriteTodos_SynthesizesFieldsBlockWhenSegmentLacksOne(t *testing.T) {
	in := `parent:
  optional: true
`
	path := writeTempYAML(t, in)

	err := WriteTodos(path, []UncoveredField{{Path: "parent.child.grandchild"}})
	if err != nil {
		t.Fatalf("WriteTodos: %v", err)
	}

	got := readFile(t, path)
	scaffold := "  fields:\n    child:\n      fields:\n        grandchild:\n          todo: true"
	if !strings.Contains(got, scaffold) {
		t.Errorf("expected nested fields scaffold; got:\n%s", got)
	}
	if !strings.Contains(got, "parent:\n  optional: true") {
		t.Errorf("expected parent.optional intact; got:\n%s", got)
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := fileutil.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp yaml: %v", err)
	}
	return string(b)
}
