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
)

// Diffing can only speak where the read and write shapes actually differ. A
// subtree reached through the same message type on both sides carries no
// signal, and the index must say so rather than report every field settable.
func TestWriteShapeIndex_Authoritative(t *testing.T) {
	// client_options is the same message on read and write; cloud_storage is not.
	shared := &ProtoMessage{Name: "ClientOptions", GoName: "ClientOptions", Fields: []ProtoField{
		{Name: "client_id", Kind: KindString, Cardinality: "singular"},
	}}
	read := &ProtoMessage{Name: "Thing", GoName: "Thing", Fields: []ProtoField{
		{Name: "client_options", Kind: KindMessage, Cardinality: "singular", Nested: shared},
		{
			Name: "cloud_storage", Kind: KindMessage, Cardinality: "singular",
			Nested: &ProtoMessage{Name: "CloudStorage", GoName: "Thing_CloudStorage", Fields: []ProtoField{
				{Name: "arn", Kind: KindString, Cardinality: "singular"},
			}},
		},
	}}
	write := &ProtoMessage{Name: "ThingCreate", GoName: "ThingCreate", Fields: []ProtoField{
		{Name: "client_options", Kind: KindMessage, Cardinality: "singular", Nested: shared},
		{
			Name: "cloud_storage", Kind: KindMessage, Cardinality: "singular",
			Nested: &ProtoMessage{Name: "CloudStorage", GoName: "ThingCreate_CloudStorage"},
		},
	}}

	idx := &WriteShapeIndex{create: map[string]bool{}, update: map[string]bool{}, sharedCreate: map[string]bool{}, sharedUpdate: map[string]bool{}, hasCreate: true}
	collectWritePaths(write, "", idx.create, map[string]bool{}, 0)
	markSharedSubtrees(read, write, "", idx.sharedCreate, map[string]bool{}, 0)

	if idx.SettableAuthoritative("client_options.client_id") {
		t.Error("a shared message carries no diffing signal — must not be authoritative")
	}
	if !idx.SettableAuthoritative("cloud_storage.arn") {
		t.Error("divergent shapes must stay authoritative")
	}
	if idx.Settable("cloud_storage.arn") {
		t.Error("arn is absent from the write shape")
	}
}

// Create and update authority are tracked separately. cluster's
// customer_managed_resources is the live case: CustomerManagedResources on both
// read and create, CustomerManagedResourcesUpdate on update. Sharing with
// create must not discard the update shape's signal.
func TestWriteShapeIndex_AuthoritySplitPerShape(t *testing.T) {
	cmr := &ProtoMessage{Name: "CMR", GoName: "CMR", Fields: []ProtoField{
		{Name: "vpc", Kind: KindString, Cardinality: "singular"},
	}}
	read := &ProtoMessage{Name: "Thing", GoName: "Thing", Fields: []ProtoField{
		{Name: "cmr", Kind: KindMessage, Cardinality: "singular", Nested: cmr},
	}}
	create := &ProtoMessage{Name: "ThingCreate", GoName: "ThingCreate", Fields: []ProtoField{
		{Name: "cmr", Kind: KindMessage, Cardinality: "singular", Nested: cmr},
	}}
	update := &ProtoMessage{Name: "ThingUpdate", GoName: "ThingUpdate", Fields: []ProtoField{
		{
			Name: "cmr", Kind: KindMessage, Cardinality: "singular",
			Nested: &ProtoMessage{Name: "CMRUpdate", GoName: "CMRUpdate"},
		},
	}}

	idx := &WriteShapeIndex{
		create: map[string]bool{}, update: map[string]bool{},
		sharedCreate: map[string]bool{}, sharedUpdate: map[string]bool{},
		hasCreate: true,
	}
	collectWritePaths(create, "", idx.create, map[string]bool{}, 0)
	markSharedSubtrees(read, create, "", idx.sharedCreate, map[string]bool{}, 0)
	collectWritePaths(update, "", idx.update, map[string]bool{}, 0)
	markSharedSubtrees(read, update, "", idx.sharedUpdate, map[string]bool{}, 0)
	idx.hasUpdate = true

	if idx.SettableAuthoritative("cmr.vpc") {
		t.Error("create shares the read type — Settable carries no signal there")
	}
	if !idx.UpdatableAuthoritative("cmr.vpc") {
		t.Error("update diverges — Updatable must stay authoritative")
	}
	if idx.Updatable("cmr.vpc") {
		t.Error("cmr.vpc is absent from the update payload")
	}
}

// Without an update payload, "absent from the update shape" means unknown, not
// immutable — otherwise every attribute on a create-only resource is flagged.
func TestWriteShapeIndex_NoUpdatePayload(t *testing.T) {
	idx := &WriteShapeIndex{
		create: map[string]bool{"name": true}, update: map[string]bool{},
		sharedCreate: map[string]bool{}, sharedUpdate: map[string]bool{},
		hasCreate: true,
	}
	if idx.UpdatableAuthoritative("name") {
		t.Error("no update payload resolved — nothing is decidable about updatability")
	}
}

// The diagnostics must be demonstrably capable of firing; a check never seen to
// warn is indistinguishable from one that cannot.
func TestMerge_WriteShapeDiagnostics_Fire(t *testing.T) {
	proto := &ProtoMessage{Name: "Thing", GoName: "Thing", Fields: []ProtoField{
		{Name: "server_owned", Kind: KindString, Cardinality: "singular"},
		{
			Name: "block", Kind: KindMessage, Cardinality: "singular",
			Nested: &ProtoMessage{Name: "Block", GoName: "Thing_Block", Fields: []ProtoField{
				{Name: "frozen", Kind: KindString, Cardinality: "singular"},
				{Name: "guarded", Kind: KindString, Cardinality: "singular"},
			}},
		},
	}}
	yes := true
	cfg := &Config{Fields: map[string]FieldConfig{
		"server_owned": {ComputedOnly: true},
		"block": {Fields: map[string]FieldConfig{
			"guarded": {Optional: &yes, PlanModifiers: []string{"RequiresReplaceIfConfigured"}},
		}},
	}}
	cfg.SetWriteShapeIndex(&WriteShapeIndex{
		create: map[string]bool{
			"server_owned": true, "block": true, "block.frozen": true, "block.guarded": true,
		},
		update:       map[string]bool{"server_owned": true, "block": true},
		sharedCreate: map[string]bool{},
		sharedUpdate: map[string]bool{},
		hasCreate:    true,
		hasUpdate:    true,
	})

	var warns []string
	Merge(proto, cfg, "resource", nil, func(f string, a ...any) { warns = append(warns, fmt.Sprintf(f, a...)) })
	joined := strings.Join(warns, "\n")

	// Marked read-only, but the payload accepts it.
	if !strings.Contains(joined, "write-shape") || !strings.Contains(joined, "server_owned") {
		t.Errorf("write-shape check did not fire; got:\n%s", joined)
	}
	// Settable, absent from the update payload, no RequiresReplace.
	if !strings.Contains(joined, "block.frozen") {
		t.Errorf("nested-rr did not fire for an uncovered leaf; got:\n%s", joined)
	}
	// A conditional RequiresReplaceIf* must not count as coverage.
	if !strings.Contains(joined, "block.guarded") {
		t.Errorf("conditional RequiresReplace must not count as covered; got:\n%s", joined)
	}
}
