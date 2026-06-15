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
	"fmt"
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

// TestMerge_PlanModifiers_SubsumesStateNullAxis — a registry modifier flagged
// subsumesStateNullAxis suppresses the classifier's auto state modifier and
// emits exactly the registered expression.
func TestMerge_PlanModifiers_SubsumesStateNullAxis(t *testing.T) {
	tru := true
	proto := &ProtoMessage{
		Name: "Thing",
		Fields: []ProtoField{
			{Name: "zones", Kind: KindString, Cardinality: KindRepeated},
			{Name: "url", Kind: KindString, Cardinality: "singular"},
		},
	}
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"zones": {Optional: &tru, Computed: &tru, PlanModifiers: []string{"PinStateUnlessRpsqlEnables"}},
			"url":   {Computed: &tru, PlanModifiers: []string{"PinStateUnlessRpsqlEnables"}},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	got := map[string]string{}
	for _, a := range attrs {
		got[a.Name] = a.PlanModifiers
	}
	want := map[string]string{
		"zones": "[]planmodifier.List{rpsqlZonesStatePin()}",
		"url":   "[]planmodifier.String{rpsqlStringStatePin()}",
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s plan modifiers: got %q, want %q", name, got[name], w)
		}
	}
}

func maskContractProto() *ProtoMessage {
	return &ProtoMessage{
		Name: "Thing",
		Fields: []ProtoField{
			{Name: "region", Kind: KindString, Cardinality: "singular"},
			{Name: "name", Kind: KindString, Cardinality: "singular"},
			{Name: "rpsql", Kind: KindMessage, Cardinality: "singular", Nested: &ProtoMessage{
				Name:   "RPSql",
				Fields: []ProtoField{{Name: "enabled", Kind: KindBool, Cardinality: "singular"}},
			}},
			{Name: "state", Kind: KindString, Cardinality: "singular"},
			{Name: "cmr", Kind: KindMessage, Cardinality: "singular", Nested: &ProtoMessage{
				Name:   "CMR",
				Fields: []ProtoField{{Name: "aws", Kind: KindString, Cardinality: "singular"}},
			}},
			{Name: "cloud_provider_tags", Kind: KindMap, MapValKind: KindString, Cardinality: KindMap},
			{Name: "partition_count", Kind: KindInt32, Cardinality: "singular"},
		},
	}
}

func maskContractFor() *MaskContract {
	return &MaskContract{
		TopLevel: map[string]bool{"name": true, "cloud_provider_tags": true},
		Leaf:     map[string]bool{"rpsql": true},
	}
}

// TestMerge_MaskContract_DerivesRequiresReplace — fields absent from the
// update-mask contract gain RequiresReplace; in-contract and leaf-expanded
// fields stay updatable; computed-only fields are untouched.
func TestMerge_MaskContract_DerivesRequiresReplace(t *testing.T) {
	tru, fls := true, false
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"state": {ComputedOnly: true},
			"cmr":   {Optional: &tru, Computed: &fls, Fields: map[string]FieldConfig{"aws": {PlanModifiers: []string{"RequiresReplace"}}}},
		},
	}
	cfg.SetMaskContract(maskContractFor())
	attrs, _, _, errs := Merge(maskContractProto(), cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	byName := map[string]*SchemaAttr{}
	for i := range attrs {
		byName[attrs[i].Name] = &attrs[i]
	}
	if got := byName["region"].PlanModifiers; !strings.Contains(got, "RequiresReplace()") {
		t.Errorf("region (out of contract) should derive RequiresReplace; got %q", got)
	}
	if got := byName["name"].PlanModifiers; strings.Contains(got, "RequiresReplace") {
		t.Errorf("name (in contract) must not gain RequiresReplace; got %q", got)
	}
	if got := byName["rpsql"].PlanModifiers; strings.Contains(got, "RequiresReplace") {
		t.Errorf("rpsql (leaf-expanded) must not gain RequiresReplace; got %q", got)
	}
	if got := byName["state"].PlanModifiers; strings.Contains(got, "RequiresReplace") {
		t.Errorf("state (computed-only) must not gain RequiresReplace; got %q", got)
	}
	if got := byName["cloud_provider_tags"].PlanModifiers; strings.Contains(got, "RequiresReplace") {
		t.Errorf("cloud_provider_tags (in contract, map) must not gain RequiresReplace; got %q", got)
	}
	// Nested attrs are yaml-owned: the cmr.aws RequiresReplace stays, the cmr
	// top-level (out of contract) derives its own.
	if got := byName["cmr"].PlanModifiers; !strings.Contains(got, "RequiresReplace()") {
		t.Errorf("cmr (out of contract) should derive RequiresReplace; got %q", got)
	}
}

// TestMerge_MaskContract_ComposesWithStateModifier — an Optional+Computed
// out-of-contract field renders RequiresReplace before the classifier's state
// modifier.
func TestMerge_MaskContract_ComposesWithStateModifier(t *testing.T) {
	tru := true
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"region": {Optional: &tru, Computed: &tru},
		},
	}
	cfg.SetMaskContract(maskContractFor())
	attrs, _, _, errs := Merge(maskContractProto(), cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, a := range attrs {
		if a.Name != "region" {
			continue
		}
		want := "[]planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}"
		if a.PlanModifiers != want {
			t.Errorf("region modifiers: got %q, want %q", a.PlanModifiers, want)
		}
	}
}

// TestMerge_MaskContract_NoDoubleAddAndAliases — existing RequiresReplace*
// names are not duplicated; extra attrs match the contract via from_proto;
// pure synthetics are skipped.
func TestMerge_MaskContract_NoDoubleAddAndAliases(t *testing.T) {
	tru := true
	proto := &ProtoMessage{
		Name: "Thing",
		Fields: []ProtoField{
			{Name: "partition_count", Kind: KindInt32, Cardinality: "singular"},
			{Name: "cloud_provider_tags", Kind: KindMap, MapValKind: KindString, Cardinality: KindMap},
		},
	}
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"partition_count":     {PlanModifiers: []string{"RequiresReplaceIfShrinking"}},
			"cloud_provider_tags": {ProtoOnly: true},
			"tags": {
				Extra: true, Type: "map", FromProto: "cloud_provider_tags",
				Optional: &tru, Computed: &tru,
			},
			"synthetic_only": {Extra: true, Type: "string", Optional: &tru},
		},
	}
	cfg.SetMaskContract(maskContractFor())
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, a := range attrs {
		switch a.Name {
		case "partition_count":
			if !strings.Contains(a.PlanModifiers, "RequiresReplaceIf(") {
				t.Errorf("partition_count must keep its RequiresReplaceIf; got %q", a.PlanModifiers)
			}
			if strings.Contains(a.PlanModifiers, "RequiresReplace()") {
				t.Errorf("partition_count must not gain a second plain RequiresReplace; got %q", a.PlanModifiers)
			}
		case "tags":
			if strings.Contains(a.PlanModifiers, "RequiresReplace") {
				t.Errorf("tags aliases in-contract cloud_provider_tags; got %q", a.PlanModifiers)
			}
		case "synthetic_only":
			if strings.Contains(a.PlanModifiers, "RequiresReplace") {
				t.Errorf("pure synthetic must be skipped; got %q", a.PlanModifiers)
			}
		default:
		}
	}
}

// TestMerge_MaskContract_NilIsNoop — resources without a contract are
// byte-identical to a run without the feature.
func TestMerge_MaskContract_NilIsNoop(t *testing.T) {
	attrsWith, _, _, errs1 := Merge(maskContractProto(), &Config{}, "resource", nil)
	attrsWithout, _, _, errs2 := Merge(maskContractProto(), &Config{}, "resource", nil)
	if len(errs1)+len(errs2) != 0 {
		t.Fatalf("unexpected errors: %v %v", errs1, errs2)
	}
	for i := range attrsWith {
		if attrsWith[i].PlanModifiers != attrsWithout[i].PlanModifiers {
			t.Errorf("nil contract must be a no-op: %s differs", attrsWith[i].Name)
		}
	}
}

// TestMaskContractVerdicts pins the disagree-diagnostics matrix.
func TestMaskContractVerdicts(t *testing.T) {
	cases := []struct {
		inContract, hasRR bool
		want              maskContractVerdict
	}{
		{false, false, maskVerdictDerive},
		{false, true, maskVerdictRedundant},
		{true, true, maskVerdictConflict},
		{true, false, maskVerdictAgree},
	}
	for _, tc := range cases {
		if got := maskContractVerdictFor(tc.inContract, tc.hasRR); got != tc.want {
			t.Errorf("verdict(in=%v, rr=%v) = %v, want %v", tc.inContract, tc.hasRR, got, tc.want)
		}
	}
}

const nameField = "name"

// TestMerge_MaskContract_WarnOnly_BothDirections — a derived (WarnOnly) contract
// never mutates plan modifiers and warns in both directions: a payload field
// marked RequiresReplace (Direction A) and a non-payload field missing it
// (Direction B). In-contract-without-RR and out-of-contract-with-RR stay silent.
func TestMerge_MaskContract_WarnOnly_BothDirections(t *testing.T) {
	attrs := []SchemaAttr{
		{Name: nameField, ProtoName: nameField, Optional: true, PlanModifierNames: []string{"RequiresReplace"}},               // in-payload + RR → Direction A
		{Name: "region", ProtoName: "region", Optional: true},                                                                 // not in payload, no RR → Direction B
		{Name: "tags", ProtoName: "tags", Optional: true},                                                                     // in-payload, no RR → silent
		{Name: "cloud_provider", ProtoName: "cloud_provider", Required: true, PlanModifierNames: []string{"RequiresReplace"}}, // not in payload + RR → silent
		{Name: "id", ProtoName: "id", Computed: true},                                                                         // not optional/required → skipped
	}
	contract := &MaskContract{TopLevel: map[string]bool{nameField: true, "tags": true}, WarnOnly: true}
	var warns []string
	mc := &mergeCtx{resourceLabel: "Thing", warnf: func(f string, a ...any) { warns = append(warns, fmt.Sprintf(f, a...)) }}

	deriveMaskContractRequiresReplace(attrs, nil, contract, mc)

	// No mutation: existing modifiers untouched, no RequiresReplace added.
	if got := attrs[0].PlanModifierNames; len(got) != 1 || got[0] != "RequiresReplace" {
		t.Errorf("warn-only must not mutate name modifiers; got %v", got)
	}
	if len(attrs[1].PlanModifierNames) != 0 {
		t.Errorf("warn-only must not auto-add RequiresReplace to region; got %v", attrs[1].PlanModifierNames)
	}

	if len(warns) != 2 {
		t.Fatalf("expected exactly 2 warnings (Direction A + B), got %d: %v", len(warns), warns)
	}
	var sawA, sawB bool
	for _, w := range warns {
		if strings.Contains(w, "Thing.name") && strings.Contains(w, "is in the update payload") {
			sawA = true
		}
		if strings.Contains(w, "Thing.region") && strings.Contains(w, "missing RequiresReplace") {
			sawB = true
		}
	}
	if !sawA {
		t.Errorf("missing Direction-A warning for name; got %v", warns)
	}
	if !sawB {
		t.Errorf("missing Direction-B warning for region; got %v", warns)
	}
}

// TestResolveUpdateContractFields — the derived contract reads the nested
// payload message when one matches the request convention, otherwise the
// request message itself; FieldMask fields are excluded.
func TestResolveUpdateContractFields(t *testing.T) {
	fieldMask := ProtoField{Name: "update_mask", Kind: KindMessage, Nested: &ProtoMessage{Name: "FieldMask"}}

	nestedLookup := func(name string) (*ProtoMessage, error) {
		switch name {
		case "UpdateThingRequest":
			return &ProtoMessage{Name: "UpdateThingRequest", Fields: []ProtoField{
				{Name: "thing", Kind: KindMessage, Nested: &ProtoMessage{Name: "ThingUpdate", GoName: "ThingUpdate"}},
				fieldMask,
			}}, nil
		case "ThingUpdate":
			return &ProtoMessage{Name: "ThingUpdate", Fields: []ProtoField{
				{Name: "id", Kind: KindString}, {Name: nameField, Kind: KindString}, {Name: "foo", Kind: KindString},
			}}, nil
		default:
			return nil, nil
		}
	}
	cfg := &Config{API: &APIConfig{Update: &RPCConfig{Request: "UpdateThingRequest"}}}
	got, ok := ResolveUpdateContractFields(cfg, nestedLookup)
	if !ok {
		t.Fatal("expected nested payload to resolve")
	}
	for _, want := range []string{"id", nameField, "foo"} {
		if !got[want] {
			t.Errorf("nested payload field %q missing from contract %v", want, got)
		}
	}

	// Request-as-payload: no conventionally-named nested message, so the
	// request's own fields are the surface; FieldMask excluded.
	flatLookup := func(name string) (*ProtoMessage, error) {
		if name == "UpdateThingRequest" {
			return &ProtoMessage{Name: "UpdateThingRequest", Fields: []ProtoField{
				{Name: "id", Kind: KindString}, {Name: "tags", Kind: KindMap}, fieldMask,
			}}, nil
		}
		return nil, nil
	}
	got, ok = ResolveUpdateContractFields(cfg, flatLookup)
	if !ok {
		t.Fatal("expected request-as-payload to resolve")
	}
	if !got["id"] || !got["tags"] {
		t.Errorf("request-as-payload fields missing; got %v", got)
	}
	if got["update_mask"] {
		t.Errorf("FieldMask must be excluded from the contract; got %v", got)
	}
}

// TestUpdateContractIdentityProtoField — the identity proto field is the one
// backing the TF id attribute (from_proto), defaulting to "id".
func TestUpdateContractIdentityProtoField(t *testing.T) {
	keyedByName := &Config{Fields: map[string]FieldConfig{"id": {FromProto: nameField}}}
	if got := UpdateContractIdentityProtoField(keyedByName); got != nameField {
		t.Errorf("id.from_proto=%s should be preserved, got %q", nameField, got)
	}
	if got := UpdateContractIdentityProtoField(&Config{}); got != "id" {
		t.Errorf("default identity field should be %q, got %q", "id", got)
	}
}
