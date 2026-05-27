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
	"reflect"
	"sort"
	"testing"
)

const goldenSample = `has_timeouts: true
attributes:
    - name: allow_deletion
      type: BoolAttribute
      computed: true
    - name: aws_private_link
      type: SingleNestedAttribute
      computed: true
      attributes:
        - name: allowed_principals
          type: ListAttribute
          computed: true
          element_type: basetypes.StringType
        - name: connect_console
          type: BoolAttribute
          computed: true
        - name: status
          type: SingleNestedAttribute
          computed: true
          attributes:
            - name: console_port
              type: Int32Attribute
              computed: true
    - name: customer_managed_resources
      type: SingleNestedAttribute
      computed: true
      attributes:
        - name: aws
          type: SingleNestedAttribute
          computed: true
          attributes:
            - name: cluster_security_group
              type: SingleNestedAttribute
              computed: true
`

func TestParseGoldenPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.golden")
	if err := os.WriteFile(path, []byte(goldenSample), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := ParseGoldenPaths(path)
	if err != nil {
		t.Fatalf("ParseGoldenPaths: %v", err)
	}

	want := []string{
		"allow_deletion",
		"aws_private_link",
		"aws_private_link.allowed_principals",
		"aws_private_link.connect_console",
		"aws_private_link.status",
		"aws_private_link.status.console_port",
		"customer_managed_resources",
		"customer_managed_resources.aws",
		"customer_managed_resources.aws.cluster_security_group",
	}

	gotPaths := make([]string, 0, len(got))
	for p := range got {
		gotPaths = append(gotPaths, p)
	}
	sort.Strings(gotPaths)

	if !reflect.DeepEqual(gotPaths, want) {
		t.Errorf("paths mismatch\nwant: %v\ngot:  %v", want, gotPaths)
	}
}

func TestParseGoldenPaths_MissingFile(t *testing.T) {
	paths, err := ParseGoldenPaths(filepath.Join(t.TempDir(), "does-not-exist.golden"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths for missing file, got: %v", paths)
	}
}

func TestParseGoldenPaths_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.golden")
	if err := os.WriteFile(path, []byte("not: [valid yaml"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := ParseGoldenPaths(path); err == nil {
		t.Error("expected parse error, got nil")
	}
}

func TestFilterUncoveredByGolden(t *testing.T) {
	accepted := map[string]struct{}{
		"aws_private_link.allowed_principals": {},
		"aws_private_link.connect_console":    {},
	}
	uncovered := []UncoveredField{
		{Path: "aws_private_link.allowed_principals"},
		{Path: "sandbox"},
		{Path: "aws_private_link.connect_console"},
		{Path: "oxla_security_group"},
	}

	got := FilterUncoveredByGolden(uncovered, accepted)

	wantPaths := []string{"sandbox", "oxla_security_group"}
	gotPaths := make([]string, len(got))
	for i, u := range got {
		gotPaths[i] = u.Path
	}
	sort.Strings(gotPaths)
	sort.Strings(wantPaths)
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("filter mismatch\nwant: %v\ngot:  %v", wantPaths, gotPaths)
	}
}

func TestFilterUncoveredByGolden_NilAccepted(t *testing.T) {
	uncovered := []UncoveredField{{Path: "sandbox"}, {Path: "oxla"}}
	got := FilterUncoveredByGolden(uncovered, nil)
	if !reflect.DeepEqual(got, uncovered) {
		t.Errorf("expected passthrough when accepted is nil; got %v", got)
	}
}

func TestFilterUncoveredByGolden_DropsNestedUnderKeptAncestor(t *testing.T) {
	accepted := map[string]struct{}{
		"customer_managed_resources.aws": {},
	}
	uncovered := []UncoveredField{
		{Path: "customer_managed_resources.aws.oxla_node_group_instance_profile"},
		{Path: "customer_managed_resources.aws.oxla_security_group"},
		{Path: "sandbox"},
		{Path: "sandbox.url"},
	}

	got := FilterUncoveredByGolden(uncovered, accepted)

	wantPaths := []string{
		"customer_managed_resources.aws.oxla_node_group_instance_profile",
		"customer_managed_resources.aws.oxla_security_group",
		"sandbox",
	}
	gotPaths := make([]string, len(got))
	for i, u := range got {
		gotPaths[i] = u.Path
	}
	sort.Strings(gotPaths)
	sort.Strings(wantPaths)
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("expected sandbox.url suppressed under new 'sandbox'; aws children kept (parent accepted).\nwant: %v\ngot:  %v", wantPaths, gotPaths)
	}
}
