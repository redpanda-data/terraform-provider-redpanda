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
	"strings"
	"testing"
)

// Regression guard: yaml `plan_modifiers: [RequiresReplace]` on an
// Optional+Computed field must compose with the classifier's state-null
// verdict — not replace it. Without the compose path, the auto-emit pass
// skips any field with a yaml override, so RequiresReplace-only fields
// lose UseStateForUnknown and go `(known after apply)` on every plan,
// triggering destroy+recreate of the resource.
func TestMerge_PlanModifiers_ComposeWithStateNull(t *testing.T) {
	proto := &ProtoMessage{
		Name: "Thing",
		Fields: []ProtoField{
			{Name: "type", Kind: KindString, Cardinality: "singular"},
		},
	}
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"type": {PlanModifiers: []string{"RequiresReplace"}},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var got string
	for _, a := range attrs {
		if a.Name == "type" {
			got = a.PlanModifiers
		}
	}
	if got == "" {
		t.Fatal("type attr not found or PlanModifiers empty")
	}
	if !strings.Contains(got, "RequiresReplace") {
		t.Errorf("PlanModifiers missing RequiresReplace; got %q", got)
	}
	if !strings.Contains(got, "UseStateForUnknown") {
		t.Errorf("PlanModifiers missing UseStateForUnknown — classifier verdict was replaced instead of composed; got %q", got)
	}
}

// Optional+Computed fields with a Default must not get UseStateForUnknown:
// Default fills null at config-resolution time, before any plan modifier
// could fire.
func TestMerge_PlanModifiers_SkippedWhenDefaultSet(t *testing.T) {
	proto := &ProtoMessage{
		Name:   "Thing",
		Fields: []ProtoField{},
	}
	optTrue := true
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"allow_deletion": {
				Extra:    true,
				Type:     "bool",
				Optional: &optTrue,
				Computed: &optTrue,
				Default:  false,
			},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var got string
	var foundDefault string
	for _, a := range attrs {
		if a.Name == "allow_deletion" {
			got = a.PlanModifiers
			foundDefault = a.Default
		}
	}
	if foundDefault == "" {
		t.Fatal("allow_deletion attr not found or Default empty")
	}
	if got != "" {
		t.Errorf("expected no PlanModifiers when Default is set; got %q", got)
	}
}

// The None sentinel suppresses the auto-added state modifier and emits nothing,
// for a computed leaf that would otherwise get UseStateForUnknown.
func TestMerge_PlanModifiers_NoneSuppresses(t *testing.T) {
	proto := &ProtoMessage{Name: "Thing", Fields: []ProtoField{}}
	tru := true
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"endpoint": {
				Extra:         true,
				Type:          "string",
				Computed:      &tru,
				PlanModifiers: []string{modNone},
			},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var found bool
	for _, a := range attrs {
		if a.Name == "endpoint" {
			found = true
			if a.PlanModifiers != "" {
				t.Errorf("expected no PlanModifiers with [None]; got %q", a.PlanModifiers)
			}
		}
	}
	if !found {
		t.Fatal("endpoint attr not found")
	}
}
