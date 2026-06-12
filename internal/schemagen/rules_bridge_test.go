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

package schemagen

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

// bridgeShapes builds a read/write pair mirroring the cluster divergence:
// the read shape (Thing) is rule-less where the write shape (ThingCreate)
// carries buf.validate rules, including a divergently-typed nested message.
func bridgeShapes() (read, write *ProtoMessage) {
	read = &ProtoMessage{
		Name:   "Thing",
		GoName: "Thing",
		Fields: []ProtoField{
			{Name: "name", Kind: KindString, Cardinality: "singular"},
			{Name: "state", Kind: KindString, Cardinality: "singular"},
			{Name: "mode", Kind: KindString, Cardinality: "singular"},
			{
				Name: "labels", Kind: KindString, Cardinality: KindRepeated,
				ValidateRules: fieldRulesRepeated(0, 5, false),
			},
			{
				Name: "private_link", Kind: KindMessage, Cardinality: "singular",
				Nested: &ProtoMessage{
					Name:   "Thing_PrivateLink",
					GoName: "Thing_PrivateLink",
					Fields: []ProtoField{
						{Name: "supported_regions", Kind: KindString, Cardinality: KindRepeated},
						{Name: "status", Kind: KindString, Cardinality: "singular"},
					},
				},
			},
		},
	}
	write = &ProtoMessage{
		Name:   "ThingCreate",
		GoName: "ThingCreate",
		Fields: []ProtoField{
			{Name: "name", Kind: KindString, Cardinality: "singular", ValidateRules: fieldRulesRequired()},
			{Name: "mode", Kind: KindString, Cardinality: KindRepeated, ValidateRules: fieldRulesRepeated(0, 3, false)},
			{Name: "labels", Kind: KindString, Cardinality: KindRepeated, ValidateRules: fieldRulesRepeated(0, 99, false)},
			{
				Name: "private_link", Kind: KindMessage, Cardinality: "singular",
				Nested: &ProtoMessage{
					Name:   "PrivateLinkSpec",
					GoName: "PrivateLinkSpec",
					Fields: []ProtoField{
						{Name: "supported_regions", Kind: KindString, Cardinality: KindRepeated, ValidateRules: fieldRulesRepeated(0, 50, true)},
						{Name: "write_only_extra", Kind: KindString, Cardinality: "singular", ValidateRules: fieldRulesStringLen(1, 8)},
					},
				},
			},
		},
	}
	return read, write
}

func bridgeConfigAndLookup(read, write *ProtoMessage) (*Config, ProtoLookup) {
	req := &ProtoMessage{
		Name:   "CreateThingRequest",
		GoName: "CreateThingRequest",
		Fields: []ProtoField{
			{Name: "thing", Kind: KindMessage, Cardinality: "singular", Nested: write},
		},
	}
	cfg := &Config{
		TFName: "Thing",
		API:    &APIConfig{Create: &RPCConfig{RPC: "CreateThing", Request: "CreateThingRequest"}},
	}
	lookup := func(name string) (*ProtoMessage, error) {
		switch name {
		case "CreateThingRequest":
			return req, nil
		case "Thing":
			return read, nil
		case "ThingCreate":
			return write, nil
		default:
			return nil, fmt.Errorf("unknown message %q", name)
		}
	}
	return cfg, lookup
}

// TestBridgeWriteShapeRules_CopiesWriteRules — write-shape repeated rules land
// on rule-less read fields (recursing through divergently-typed nested
// messages); required-only rules strip to nothing; read-only fields with no
// write counterpart stay untouched.
func TestBridgeWriteShapeRules_CopiesWriteRules(t *testing.T) {
	read, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(read, write)

	if err := BridgeWriteShapeRules(read, cfg, "resource", lookup); err != nil {
		t.Fatalf("BridgeWriteShapeRules: %v", err)
	}

	sr := read.FindField("private_link").Nested.FindField("supported_regions")
	r := sr.ValidateRules.GetRepeated()
	if r == nil {
		t.Fatal("supported_regions: write-shape repeated rules not bridged")
	}
	if r.GetMaxItems() != 50 || !r.GetUnique() {
		t.Errorf("supported_regions: got max_items=%d unique=%v, want 50/true", r.GetMaxItems(), r.GetUnique())
	}
	if sr.ValidateRules.GetRequired() {
		t.Error("bridged rules must never carry required")
	}
	if name := read.FindField("name"); name.ValidateRules != nil {
		t.Errorf("name: required-only write rules must strip to nothing; got %v", name.ValidateRules)
	}
	if state := read.FindField("state"); state.ValidateRules != nil {
		t.Errorf("state: read-only field must stay rule-less; got %v", state.ValidateRules)
	}
}

// TestBridgeWriteShapeRules_ReadRulesUntouched — a read field carrying its own
// rules keeps them verbatim (no piecemeal merge with the write shape).
func TestBridgeWriteShapeRules_ReadRulesUntouched(t *testing.T) {
	read, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(read, write)
	orig := read.FindField("labels").ValidateRules

	if err := BridgeWriteShapeRules(read, cfg, "resource", lookup); err != nil {
		t.Fatalf("BridgeWriteShapeRules: %v", err)
	}

	labels := read.FindField("labels")
	if labels.ValidateRules != orig {
		t.Error("labels: read-shape rules must keep the original pointer")
	}
	if got := labels.ValidateRules.GetRepeated().GetMaxItems(); got != 5 {
		t.Errorf("labels: read rules mutated, max_items = %d, want 5", got)
	}
}

// TestBridgeWriteShapeRules_KindMismatchSkips — same field name with a
// different shape (singular string vs repeated string) must not copy rules.
func TestBridgeWriteShapeRules_KindMismatchSkips(t *testing.T) {
	read, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(read, write)

	if err := BridgeWriteShapeRules(read, cfg, "resource", lookup); err != nil {
		t.Fatalf("BridgeWriteShapeRules: %v", err)
	}

	if mode := read.FindField("mode"); mode.ValidateRules != nil {
		t.Errorf("mode: cardinality mismatch must skip; got %v", mode.ValidateRules)
	}
}

// TestBridgeWriteShapeRules_WalkedIsWriteShape — resources that already walk
// the create payload (user/topic style) must be a strict no-op.
func TestBridgeWriteShapeRules_WalkedIsWriteShape(t *testing.T) {
	_, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(write, write)
	before := proto.Clone(write.FindField("private_link").Nested.FindField("supported_regions").ValidateRules)

	if err := BridgeWriteShapeRules(write, cfg, "resource", lookup); err != nil {
		t.Fatalf("BridgeWriteShapeRules: %v", err)
	}

	after := write.FindField("private_link").Nested.FindField("supported_regions").ValidateRules
	if !proto.Equal(before, after) {
		t.Error("walked == write shape: tree must not be mutated")
	}
	if name := write.FindField("name"); !name.ValidateRules.GetRequired() {
		t.Error("walked == write shape: required rules must survive untouched")
	}
}

// TestBridgeWriteShapeRules_NoCreateOrDatasource — datasources and configs
// without a create RPC are no-ops with a nil error.
func TestBridgeWriteShapeRules_NoCreateOrDatasource(t *testing.T) {
	read, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(read, write)

	if err := BridgeWriteShapeRules(read, cfg, SchemaTypeDatasource, lookup); err != nil {
		t.Fatalf("datasource: %v", err)
	}
	if sr := read.FindField("private_link").Nested.FindField("supported_regions"); sr.ValidateRules != nil {
		t.Error("datasource: must not bridge")
	}

	for _, c := range []*Config{
		{TFName: "Thing"},
		{TFName: "Thing", API: &APIConfig{}},
		{TFName: "Thing", API: &APIConfig{Create: &RPCConfig{}}},
		{TFName: "Thing", ExcludeOperations: []string{"create"}},
	} {
		if err := BridgeWriteShapeRules(read, c, "resource", lookup); err != nil {
			t.Fatalf("no-create config %+v: %v", c, err)
		}
		if sr := read.FindField("private_link").Nested.FindField("supported_regions"); sr.ValidateRules != nil {
			t.Errorf("no-create config %+v: must not bridge", c)
		}
	}
}

// TestBridgeWriteShapeRules_LookupErrorPropagates — a broken create-request
// lookup is pin/yaml drift; the bridge hard-fails rather than masking it.
func TestBridgeWriteShapeRules_LookupErrorPropagates(t *testing.T) {
	read, write := bridgeShapes()
	cfg, _ := bridgeConfigAndLookup(read, write)
	lookup := func(name string) (*ProtoMessage, error) { return nil, fmt.Errorf("nope: %s", name) }

	if err := BridgeWriteShapeRules(read, cfg, "resource", lookup); err == nil {
		t.Fatal("expected lookup error to propagate")
	}
}

// TestBridgeWriteShapeRules_CloneIsolation — mutating bridged rules on the
// read tree must not write through to the write tree.
func TestBridgeWriteShapeRules_CloneIsolation(t *testing.T) {
	read, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(read, write)

	if err := BridgeWriteShapeRules(read, cfg, "resource", lookup); err != nil {
		t.Fatalf("BridgeWriteShapeRules: %v", err)
	}

	read.FindField("private_link").Nested.FindField("supported_regions").ValidateRules.GetRepeated().MaxItems = proto.Uint64(1)
	w := write.FindField("private_link").Nested.FindField("supported_regions").ValidateRules.GetRepeated()
	if w.GetMaxItems() != 50 {
		t.Errorf("write tree mutated through bridged clone: max_items = %d, want 50", w.GetMaxItems())
	}
}

// TestBridge_MergeEmitsValidators — end-to-end through Merge: bridged rules
// produce list validators and description suffixes, never Required flips, and
// a yaml optional override on a write-required field raises no drift error.
func TestBridge_MergeEmitsValidators(t *testing.T) {
	read, write := bridgeShapes()
	cfg, lookup := bridgeConfigAndLookup(read, write)
	tru := true
	cfg.Fields = map[string]FieldConfig{
		"name": {Optional: &tru},
	}

	if err := BridgeWriteShapeRules(read, cfg, "resource", lookup); err != nil {
		t.Fatalf("BridgeWriteShapeRules: %v", err)
	}
	attrs, _, _, errs := Merge(read, cfg, "resource", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected merge errs (required must not leak from write shape): %v", errs)
	}

	pl := findAttrNamed(attrs, "private_link")
	if pl == nil {
		t.Fatal("private_link missing")
	}
	var sr *SchemaAttr
	for i := range pl.NestedAttrs {
		if pl.NestedAttrs[i].Name == "supported_regions" {
			sr = &pl.NestedAttrs[i]
		}
	}
	if sr == nil {
		t.Fatal("supported_regions missing")
	}
	want := "[]validator.List{listvalidator.SizeAtMost(50), listvalidator.UniqueValues()}"
	if sr.Validators != want {
		t.Errorf("supported_regions validators: got %q, want %q", sr.Validators, want)
	}
	if !strings.Contains(sr.Description, "Must have at most 50 items.") {
		t.Errorf("supported_regions description missing rule suffix; got %q", sr.Description)
	}
	name := findAttrNamed(attrs, "name")
	if name == nil || name.Required {
		t.Errorf("name must not be flipped Required by write-shape rules; got %+v", name)
	}
}
