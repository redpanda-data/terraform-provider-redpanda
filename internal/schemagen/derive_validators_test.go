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

	bufvalidate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"google.golang.org/protobuf/proto"
)

func fieldRulesRequired() *bufvalidate.FieldRules {
	return &bufvalidate.FieldRules{Required: proto.Bool(true)}
}

func fieldRulesStringLen(lo, hi uint64) *bufvalidate.FieldRules {
	return &bufvalidate.FieldRules{
		Type: &bufvalidate.FieldRules_String_{
			String_: &bufvalidate.StringRules{
				MinLen: proto.Uint64(lo),
				MaxLen: proto.Uint64(hi),
			},
		},
	}
}

func protoWithRules() *ProtoMessage {
	return &ProtoMessage{
		Name: "Foo",
		Fields: []ProtoField{
			{Name: "must_have", Kind: KindString, Cardinality: "singular", ValidateRules: fieldRulesRequired()},
			{Name: "bounded", Kind: KindString, Cardinality: "singular", ValidateRules: fieldRulesStringLen(1, 64)},
			{Name: "freeform", Kind: KindString, Cardinality: "singular"},
		},
	}
}

// TestProtoValidators_SkipProtoValidationField — per-field opt-out from
// proto-validate cross-check. Yaml sets optional: true on a proto-required
// field WITH skip_proto_validation: true; the field stays Optional and no
// drift error is raised. Without the skip, the merge errors (covered by
// TestDeriveValidators_OptionalOverrideErrors below).
func TestProtoValidators_SkipProtoValidationField(t *testing.T) {
	tru := true
	cfg := &Config{
		API: &APIConfig{},
		Fields: map[string]FieldConfig{
			"must_have": {Optional: &tru, SkipProtoValidation: true},
		},
	}
	attrs, _, _, errs := Merge(protoWithRules(), cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("skip_proto_validation should suppress drift error; got %v", errs)
	}
	mh := findAttrNamed(attrs, "must_have")
	if mh == nil {
		t.Fatal("must_have missing")
	}
	if mh.Required {
		t.Error("must_have should NOT be Required when yaml explicitly opts out via skip_proto_validation")
	}
	if !mh.Optional {
		t.Error("must_have should be Optional per yaml override")
	}
}

// TestDeriveValidators_Propagation — opt-in flips a required field to TF
// Required, and appends string-length summary to the bounded field's
// description.
func TestDeriveValidators_Propagation(t *testing.T) {
	cfg := &Config{
		API: &APIConfig{},
	}
	attrs, _, _, errs := Merge(protoWithRules(), cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	mh := findAttrNamed(attrs, "must_have")
	if mh == nil {
		t.Fatal("must_have missing")
	}
	if !mh.Required {
		t.Error("must_have should be Required from proto (buf.validate).required")
	}
	if mh.Optional {
		t.Error("must_have should not be Optional when proto says required")
	}
	b := findAttrNamed(attrs, "bounded")
	if b == nil {
		t.Fatal("bounded missing")
	}
	if !strings.Contains(b.Description, "Length must be between 1 and 64.") {
		t.Errorf("bounded description should carry string-length summary; got: %q", b.Description)
	}
}

// TestDeriveValidators_DatasourceSkipsRequired — datasources have every field
// computed; the propagation logic must not flip them to Required, since the
// API doesn't accept input from a datasource.
func TestDeriveValidators_DatasourceSkipsRequired(t *testing.T) {
	cfg := &Config{
		ComputedDefault: true,
		API:             &APIConfig{},
	}
	attrs, _, _, errs := Merge(protoWithRules(), cfg, "datasource", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	mh := findAttrNamed(attrs, "must_have")
	if mh == nil {
		t.Fatal("must_have missing")
	}
	if mh.Required {
		t.Error("datasource: must_have must NOT be Required from proto")
	}
	if !mh.Computed {
		t.Error("datasource: must_have should remain Computed")
	}
}

// TestDeriveValidators_ComputedOverrideAllowed — when yaml explicitly marks a
// required field as computed_only (server populates it), no conflict; the
// override is the legitimate way to reconcile required + server-populated.
func TestDeriveValidators_ComputedOverrideAllowed(t *testing.T) {
	cfg := &Config{
		API: &APIConfig{},
		Fields: map[string]FieldConfig{
			"must_have": {ComputedOnly: true},
		},
	}
	_, _, _, errs := Merge(protoWithRules(), cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
}

// TestDeriveValidators_OptionalOverrideErrors — yaml that explicitly sets
// `optional: true` on a buf.validate-required field is a drift error: it
// hides the proto signal under a stale yaml override.
func TestDeriveValidators_OptionalOverrideErrors(t *testing.T) {
	tru := true
	cfg := &Config{
		API: &APIConfig{},
		Fields: map[string]FieldConfig{
			"must_have": {Optional: &tru},
		},
	}
	_, _, _, errs := Merge(protoWithRules(), cfg, "resource", nil)
	if len(errs) == 0 {
		t.Fatal("expected drift error for explicit optional: true override on a buf.validate-required field")
	}
	if !containsErr(errs, "must_have") {
		t.Errorf("error should name the conflicting field; got %v", errs)
	}
}

// TestConstraintSummary_StringPattern locks pattern + length rendering.
func TestConstraintSummary_StringPattern(t *testing.T) {
	rules := &bufvalidate.FieldRules{
		Type: &bufvalidate.FieldRules_String_{
			String_: &bufvalidate.StringRules{
				MinLen:  proto.Uint64(3),
				MaxLen:  proto.Uint64(3),
				Pattern: proto.String("^[a-z]{3}$"),
			},
		},
	}
	got := constraintSummary(rules, "foo.bar")
	if !strings.Contains(got, "Length must be exactly 3.") {
		t.Errorf("missing length-exactly summary; got %q", got)
	}
	if !strings.Contains(got, "Must match pattern `^[a-z]{3}$`.") {
		t.Errorf("missing pattern summary; got %q", got)
	}
}

// TestConstraintSummary_OnlyRequired returns empty because Required surfaces
// via the Required flag, not the description.
func TestConstraintSummary_OnlyRequired(t *testing.T) {
	if got := constraintSummary(fieldRulesRequired(), "foo.bar"); got != "" {
		t.Errorf("only-required rules should produce empty summary; got %q", got)
	}
}

// TestDeriveValidators_AppendsToExistingDescription pins the
// enrichment-vs-replacement contract: when an apidesc / yaml description
// is already present, the constraint summary must append. Without this
// guarantee the apidesc text ("GCP service account email.") gets replaced
// by the bare summary ("Must be a valid email address."), losing context.
func TestDeriveValidators_AppendsToExistingDescription(t *testing.T) {
	proto := &ProtoMessage{
		Name: "Foo",
		Fields: []ProtoField{
			{
				Name: "bounded", Kind: KindString, Cardinality: "singular",
				ValidateRules: fieldRulesStringLen(1, 64),
			},
		},
	}
	cfg := &Config{
		API: &APIConfig{},
		Fields: map[string]FieldConfig{
			"bounded": {Description: "The bounded field."},
		},
	}
	attrs, _, _, errs := Merge(proto, cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	b := findAttrNamed(attrs, "bounded")
	if b == nil {
		t.Fatal("bounded missing")
	}
	if !strings.Contains(b.Description, "The bounded field.") {
		t.Errorf("description should preserve original text; got %q", b.Description)
	}
	if !strings.Contains(b.Description, "Length must be between 1 and 64.") {
		t.Errorf("description should append constraint summary; got %q", b.Description)
	}
}

// TestConstraintSummary_Int32Range exercises the numeric adapter.
func TestConstraintSummary_Int32Range(t *testing.T) {
	rules := &bufvalidate.FieldRules{
		Type: &bufvalidate.FieldRules_Int32{
			Int32: &bufvalidate.Int32Rules{
				GreaterThan: &bufvalidate.Int32Rules_Gte{Gte: 1},
				LessThan:    &bufvalidate.Int32Rules_Lte{Lte: 99},
			},
		},
	}
	if got := constraintSummary(rules, "foo"); !strings.Contains(got, "Must be between 1 and 99 (inclusive).") {
		t.Errorf("int32 range summary: got %q", got)
	}
}

// TestConstraintSummary_MixedBounds exercises gte+lt and gt+lte combinations,
// which previously fell through to the bare-min case.
func TestConstraintSummary_MixedBounds(t *testing.T) {
	cases := []struct {
		name  string
		rules *bufvalidate.FieldRules
		want  string
	}{
		{
			name: "gte+lt",
			rules: &bufvalidate.FieldRules{
				Type: &bufvalidate.FieldRules_Int32{
					Int32: &bufvalidate.Int32Rules{
						GreaterThan: &bufvalidate.Int32Rules_Gte{Gte: 1},
						LessThan:    &bufvalidate.Int32Rules_Lt{Lt: 100},
					},
				},
			},
			want: "Must be at least 1 and less than 100.",
		},
		{
			name: "gt+lte",
			rules: &bufvalidate.FieldRules{
				Type: &bufvalidate.FieldRules_Int32{
					Int32: &bufvalidate.Int32Rules{
						GreaterThan: &bufvalidate.Int32Rules_Gt{Gt: 0},
						LessThan:    &bufvalidate.Int32Rules_Lte{Lte: 99},
					},
				},
			},
			want: "Must be greater than 0 and at most 99.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := constraintSummary(tc.rules, "foo")
			if !strings.Contains(got, tc.want) {
				t.Errorf("mixed bounds summary: want %q in %q", tc.want, got)
			}
		})
	}
}
