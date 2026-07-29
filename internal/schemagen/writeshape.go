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

import "strings"

// maxWriteShapeDepth backstops the payload walk; the seen set breaks true cycles.
const maxWriteShapeDepth = 12

// WriteShapeIndex records the dotted paths a resource's create and update
// payload messages expose.
//
// Settable and Updatable are not interchangeable: region and zones are on the
// create shape but not the update shape — user input that requires replacement
// to change.
type WriteShapeIndex struct {
	create map[string]bool
	update map[string]bool
	// sharedCreate/sharedUpdate hold paths whose message type is identical on
	// the read shape and that payload. Diffing says nothing there — every field
	// is trivially present. The two are tracked separately because a subtree can
	// be shared with create and still diverge on update: cluster's
	// customer_managed_resources is CustomerManagedResources on both read and
	// create, but CustomerManagedResourcesUpdate on update.
	sharedCreate map[string]bool
	sharedUpdate map[string]bool
	// hasCreate/hasUpdate record whether each payload resolved at all. Without
	// one, "absent from that shape" means unknown, not server-owned.
	hasCreate bool
	hasUpdate bool
}

// SettableAuthoritative reports whether diffing can decide Settable for path.
// It cannot when the create payload shares the read type: Settable is then
// trivially true for every field.
func (i *WriteShapeIndex) SettableAuthoritative(path string) bool {
	return i != nil && i.hasCreate && !sharedAtOrAbove(i.sharedCreate, path)
}

// UpdatableAuthoritative reports whether diffing can decide Updatable for path.
func (i *WriteShapeIndex) UpdatableAuthoritative(path string) bool {
	return i != nil && i.hasUpdate && !sharedAtOrAbove(i.sharedUpdate, path)
}

func sharedAtOrAbove(shared map[string]bool, path string) bool {
	for p := path; p != ""; {
		if shared[p] {
			return true
		}
		idx := strings.LastIndex(p, ".")
		if idx < 0 {
			break
		}
		p = p[:idx]
	}
	return false
}

// Settable reports whether path appears on either write shape. Drives
// Optional vs Computed.
func (i *WriteShapeIndex) Settable(path string) bool {
	if i == nil {
		return false
	}
	return i.create[path] || i.update[path]
}

// Updatable reports whether path appears on the update shape. Drives
// RequiresReplace.
func (i *WriteShapeIndex) Updatable(path string) bool {
	if i == nil {
		return false
	}
	return i.update[path]
}

// Known reports whether any write shape resolved. Callers must not read
// "absent from the index" as "server-owned" when it did not.
func (i *WriteShapeIndex) Known() bool {
	return i != nil && (len(i.create) > 0 || len(i.update) > 0)
}

// HasCreate reports whether a create payload resolved.
func (i *WriteShapeIndex) HasCreate() bool { return i != nil && i.hasCreate }

// HasUpdate reports whether an update payload resolved.
func (i *WriteShapeIndex) HasUpdate() bool { return i != nil && i.hasUpdate }

// TopLevelUpdatePaths returns the update-shape paths with no parent — the
// granularity the update-mask contract works at.
func (i *WriteShapeIndex) TopLevelUpdatePaths() map[string]bool {
	out := map[string]bool{}
	if i == nil {
		return out
	}
	for p := range i.update {
		if !strings.Contains(p, ".") {
			out[p] = true
		}
	}
	return out
}

// UpdatePaths returns the update-shape path set, for the mask contract.
func (i *WriteShapeIndex) UpdatePaths() map[string]bool {
	if i == nil {
		return nil
	}
	out := make(map[string]bool, len(i.update))
	for k, v := range i.update {
		out[k] = v
	}
	return out
}

// BuildWriteShapeIndex records every dotted path the create and update
// payloads expose. Payload resolution reuses inferRPCPayload so this agrees
// with the Expand planner and the rule bridge.
func BuildWriteShapeIndex(read *ProtoMessage, cfg *Config, lookup ProtoLookup) *WriteShapeIndex {
	idx := &WriteShapeIndex{
		create:       map[string]bool{},
		update:       map[string]bool{},
		sharedCreate: map[string]bool{},
		sharedUpdate: map[string]bool{},
	}
	if cfg == nil || cfg.API == nil || lookup == nil {
		return idx
	}
	for _, rpc := range []*RPCConfig{cfg.API.Create, cfg.API.Update} {
		msg := resolvePayloadMessage(rpc, lookup)
		if msg == nil {
			continue
		}
		out, shared := idx.create, idx.sharedCreate
		if rpc == cfg.API.Update {
			out, shared = idx.update, idx.sharedUpdate
			idx.hasUpdate = true
		} else {
			idx.hasCreate = true
		}
		collectWritePaths(msg, "", out, map[string]bool{}, 0)
		markSharedSubtrees(read, msg, "", shared, map[string]bool{}, 0)
	}
	return idx
}

// markSharedSubtrees records paths where the read and write shapes reference
// the same message type, so the fields below carry no diffing signal.
func markSharedSubtrees(read, write *ProtoMessage, prefix string, out, seen map[string]bool, depth int) {
	if read == nil || write == nil || depth > maxWriteShapeDepth {
		return
	}
	key := read.identityKey() + "|" + write.identityKey()
	if seen[key] {
		return
	}
	seen[key] = true
	defer delete(seen, key)

	for i := range read.Fields {
		rf := &read.Fields[i]
		wf := write.FindField(rf.Name)
		if wf == nil || rf.Nested == nil || wf.Nested == nil {
			continue
		}
		path := joinPath(prefix, rf.Name)
		if k := rf.Nested.identityKey(); k != "" && k == wf.Nested.identityKey() {
			out[path] = true
			continue
		}
		markSharedSubtrees(rf.Nested, wf.Nested, path, out, seen, depth+1)
	}
}

// resolvePayloadMessage returns the nested payload when the request wraps one,
// else the request itself.
func resolvePayloadMessage(rpc *RPCConfig, lookup ProtoLookup) *ProtoMessage {
	if rpc == nil {
		return nil
	}
	// Probe a copy so inferred payload type does not leak into cfg's RPC.
	probe := *rpc
	if err := inferRPCPayload(&probe, lookup); err != nil {
		return nil
	}
	// Look the payload up by name first: a nested field pointer may be an
	// unpopulated stub, while the lookup returns the materialized message.
	if probe.PayloadType != "" {
		if msg, err := lookup(probe.PayloadType); err == nil && msg != nil && len(msg.Fields) > 0 {
			return msg
		}
	}
	if probe.PayloadField != "" && probe.Request != "" {
		if reqMsg, err := lookup(probe.Request); err == nil && reqMsg != nil {
			if pf := reqMsg.FindField(probe.PayloadField); pf != nil && pf.Nested != nil {
				if len(pf.Nested.Fields) > 0 {
					return pf.Nested
				}
				if msg, err := lookup(pf.Nested.Name); err == nil && msg != nil {
					return msg
				}
				return pf.Nested
			}
		}
	}
	name := probe.PayloadType
	if name == "" {
		name = probe.Request
	}
	if name == "" {
		return nil
	}
	msg, err := lookup(name)
	if err != nil {
		return nil
	}
	return msg
}

// applyOneofArmLifecycle drops Computed from oneof arms the user selects.
// Computed attaches UseStateForUnknown, which anchors the outgoing arm and
// fails apply with "inconsistent result after apply" on a switch; a
// server-reported arm keeps it, since planning that null fails the same check.
//
// Runs after yaml so force_type has collapsed presence-only arms to leaves,
// and before plan-modifier selection.
func applyOneofArmLifecycle(attrs []SchemaAttr, fields map[string]FieldConfig, idx *WriteShapeIndex, parentPath string) {
	for i := range attrs {
		a := &attrs[i]
		fc := fieldConfigFor(fields, a.Name)
		path := joinPath(parentPath, protoKey(a, fc))
		if a.IsOneofArm && a.Optional && attrHasSettableLeaf(a, path, idx) {
			a.Computed = false
		}
		applyOneofArmLifecycle(a.NestedAttrs, childFields(fc), idx, path)
	}
}

func fieldConfigFor(fields map[string]FieldConfig, name string) *FieldConfig {
	if fields == nil {
		return nil
	}
	if fc, ok := fields[name]; ok {
		return &fc
	}
	return nil
}

func childFields(fc *FieldConfig) map[string]FieldConfig {
	if fc == nil {
		return nil
	}
	return fc.Fields
}

// checkOneofArmOverrides reports yaml overrides that re-introduce the anchor
// applyOneofArmLifecycle removes from an arm the user selects. Runs post-merge
// so the subtree it inspects has already had exclude:/todo: pruned.
//
// computed: true warns — the lifecycle pass clears it, so the outcome is still
// correct and the override is merely redundant. An explicit state-pin plan
// modifier is an error: nothing downstream clears it, so it survives onto an
// Optional-only attribute and anchors the outgoing arm anyway. Silently
// stripping it would discard something the author asked for with no feedback.
func checkOneofArmOverrides(attrs []SchemaAttr, fields map[string]FieldConfig, idx *WriteShapeIndex, mc *mergeCtx, parentPath string) {
	for i := range attrs {
		a := &attrs[i]
		fc := fieldConfigFor(fields, a.Name)
		path := joinPath(parentPath, protoKey(a, fc))
		// a.Optional mirrors applyOneofArmLifecycle's own condition: it only
		// clears Computed from arms the user selects, so only those can end up
		// with an orphaned pin. A read-only arm keeps its anchor legitimately,
		// even when its own path appears on a write payload — an empty
		// presence-only arm always does.
		if a.IsOneofArm && a.Optional && fc != nil && attrHasSettableLeaf(a, path, idx) {
			if fc.Computed != nil && *fc.Computed {
				mc.warn("WARN oneof %s.%s: computed: true on a selectable arm — UseStateForUnknown will anchor it, and switching arms then fails with \"inconsistent result after apply\"; drop the override\n",
					mc.resourceLabel, path)
			}
			if pin := statePinModifier(fc.PlanModifiers); pin != "" {
				mc.errorf("yaml %s: plan_modifiers lists %s on a selectable oneof arm — it anchors the outgoing arm and switching arms then fails with \"inconsistent result after apply\"; remove it", path, pin)
			}
		}
		checkOneofArmOverrides(a.NestedAttrs, childFields(fc), idx, mc, path)
	}
}

// statePinModifier returns the first state-pinning modifier in names, or "".
// modNone is not a pin — it suppresses the automatic one.
func statePinModifier(names []string) string {
	for _, n := range names {
		if n == modUseStateForUnknown || n == modUseNonNullStateForUnknown {
			return n
		}
	}
	return ""
}

// warnWriteShapeDisagreements reports attributes whose lifecycle contradicts
// the write shape: a read-only attribute the payload accepts, or a user-settable
// one no payload carries. Diagnostic only — the yaml stays authoritative.
//
// Synthetic attributes have no proto path to check. The identity field rides
// the update payload to address the row, not as a mutable field.
func warnWriteShapeDisagreements(attrs []SchemaAttr, fields map[string]FieldConfig, idx *WriteShapeIndex, mc *mergeCtx, parentPath, identity string) {
	for i := range attrs {
		a := &attrs[i]
		fc := fieldConfigFor(fields, a.Name)
		if !protoBacked(a, fc) {
			continue
		}
		path := joinPath(parentPath, protoKey(a, fc))
		if parentPath == "" && protoKey(a, fc) == identity {
			continue
		}
		if !idx.SettableAuthoritative(path) || (fc != nil && fc.UpdatableOutOfBand) {
			warnWriteShapeDisagreements(a.NestedAttrs, childFields(fc), idx, mc, path, identity)
			continue
		}
		settable := idx.Settable(path)
		switch {
		case a.Computed && !a.Optional && !a.Required && settable:
			mc.warn("WARN write-shape %s.%s: read-only but %q is on a write payload — the annotation may be unnecessary\n",
				mc.resourceLabel, path, path)
		case (a.Optional || a.Required) && !settable:
			mc.warn("WARN write-shape %s.%s: user-settable but %q is on no write payload — it cannot be sent\n",
				mc.resourceLabel, path, path)
		default:
		}
		warnWriteShapeDisagreements(a.NestedAttrs, childFields(fc), idx, mc, path, identity)
	}
}

// attrHasSettableLeaf reports whether an attribute carries any writable value:
// a leaf by its own path, an object by any descendant leaf.
func attrHasSettableLeaf(a *SchemaAttr, path string, idx *WriteShapeIndex) bool {
	if len(a.NestedAttrs) == 0 {
		return idx.Settable(path)
	}
	for i := range a.NestedAttrs {
		child := &a.NestedAttrs[i]
		if attrHasSettableLeaf(child, joinPath(path, protoKey(child, nil)), idx) {
			return true
		}
	}
	return false
}

// protoBacked reports whether an attribute has a proto field behind it. TF-only
// conveniences (allow_deletion, cluster_api_url, password_wo) never appear on a
// payload and must not be judged against one.
func protoBacked(a *SchemaAttr, fc *FieldConfig) bool {
	if a.ProtoName != "" {
		return true
	}
	return fc != nil && (fc.FromProto != "" || fc.ExpandProtoName != "")
}

// protoKey resolves the write-shape name for an attribute. The index is keyed
// by proto names; yaml can rename an attribute or back a synthetic one with
// from_proto / expand_proto_name. Mirrors deriveMaskContractRequiresReplace.
func protoKey(a *SchemaAttr, fc *FieldConfig) string {
	key := a.ProtoName
	if fc != nil {
		if key == "" {
			key = fc.FromProto
		}
		if fc.ExpandProtoName != "" {
			key = fc.ExpandProtoName
		}
	}
	if key == "" {
		key = a.Name
	}
	return key
}

// collectWritePaths records every dotted path reachable on msg. seen is
// unwound on exit so a message type may recur at sibling paths.
func collectWritePaths(msg *ProtoMessage, prefix string, out, seen map[string]bool, depth int) {
	if msg == nil || depth > maxWriteShapeDepth {
		return
	}
	key := msg.identityKey()
	if key == "" {
		key = msg.Name
	}
	if seen[key] {
		return
	}
	seen[key] = true
	defer delete(seen, key)

	for i := range msg.Fields {
		f := &msg.Fields[i]
		if f.Kind == KindMessage && f.Nested != nil && f.Nested.Name == fieldMaskMessage {
			continue
		}
		path := joinPath(prefix, f.Name)
		out[path] = true
		if f.Nested != nil {
			collectWritePaths(f.Nested, path, out, seen, depth+1)
		}
	}
}

// warnNestedRequiresReplace flags nested user-settable attributes that the
// update payload does not carry: changing one cannot round-trip in place, so it
// needs RequiresReplace. Top-level attrs are covered by the mask contract.
func warnNestedRequiresReplace(attrs []SchemaAttr, fields map[string]FieldConfig, idx *WriteShapeIndex, mc *mergeCtx, parentPath string) {
	for i := range attrs {
		a := &attrs[i]
		fc := fieldConfigFor(fields, a.Name)
		if !protoBacked(a, fc) {
			continue
		}
		path := joinPath(parentPath, protoKey(a, fc))
		// Leaves only: the update mask is applied per leaf, and a container whose
		// leaves are each covered needs no marker of its own.
		if parentPath != "" && len(a.NestedAttrs) == 0 && (a.Optional || a.Required) && idx.UpdatableAuthoritative(path) &&
			!idx.Updatable(path) && !hasRequiresReplace(a.PlanModifierNames) {
			mc.warn("WARN nested-rr %s.%s: settable but absent from the update payload and missing RequiresReplace\n",
				mc.resourceLabel, path)
		}
		warnNestedRequiresReplace(a.NestedAttrs, childFields(fc), idx, mc, path)
	}
}

func hasRequiresReplace(names []string) bool {
	for _, n := range names {
		if n == modRequiresReplace {
			return true
		}
	}
	return false
}

// AttrsWithoutLifecycle returns the paths of attributes stating no lifecycle.
// Terraform requires at least one of Required/Optional/Computed, and the
// generator supplies no default, so silence means a proto field arrived that
// nobody dispositioned.
func AttrsWithoutLifecycle(attrs []SchemaAttr) []string {
	var out []string
	var walk func(as []SchemaAttr, prefix string)
	walk = func(as []SchemaAttr, prefix string) {
		for i := range as {
			a := &as[i]
			path := joinPath(prefix, a.Name)
			if !a.Required && !a.Optional && !a.Computed {
				out = append(out, path)
			}
			walk(a.NestedAttrs, path)
		}
	}
	walk(attrs, "")
	return out
}
