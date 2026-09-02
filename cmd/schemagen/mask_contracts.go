// Copyright 2026 Redpanda Data, Inc.
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

package main

import (
	"fmt"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/clustermask"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/schemagen"
)

// maskContracts maps the yaml api.update.mask_contract name to the provider's
// hand-maintained mirror of the control plane's update path map. Only cmd
// wires schemagen to clustermask, keeping the packages decoupled.
var maskContracts = map[string]*schemagen.MaskContract{
	"cluster": {
		TopLevel: clustermask.AcceptedTopLevel,
		// customer_managed_resources is updatable only at leaf granularity, via
		// the data-dependent clustermask.ExpandCustomerManagedResourceLeaves (not
		// a static LeafExpansions entry, which would wrongly expand it
		// unconditionally). Listing it here tells the RequiresReplace derivation
		// the top-level object is updatable so it is not auto-marked
		// RequiresReplace; its immutable child leaves keep their yaml-owned
		// RequiresReplace.
		Leaf: leafKeys(clustermask.LeafExpansions, "customer_managed_resources"),
	},
}

func leafKeys(m map[string][]string, extra ...string) map[string]bool {
	out := make(map[string]bool, len(m)+len(extra))
	for k := range m {
		out[k] = true
	}
	for _, k := range extra {
		out[k] = true
	}
	return out
}

func resolveMaskContract(cfg *schemagen.Config) error {
	if cfg.API == nil || cfg.API.Update == nil || cfg.API.Update.MaskContract == "" {
		return nil
	}
	name := cfg.API.Update.MaskContract
	contract, ok := maskContracts[name]
	if !ok {
		return fmt.Errorf("unknown mask_contract %q — register it in cmd/schemagen/mask_contracts.go", name)
	}
	cfg.SetMaskContract(contract)
	return nil
}

// verifyClusterMaskPaths checks the hand-maintained clustermask maps against
// the resource's update payload. clustermask mirrors cloudv2's pathMap, which
// is server implementation and not derivable — but a key that no longer exists
// on the update message is stale, and that is derivable.
func verifyClusterMaskPaths(cfg *schemagen.Config, attrs []schemagen.SchemaAttr, warn func(string, ...any)) {
	if cfg.API == nil || cfg.API.Update == nil || cfg.API.Update.MaskContract != "cluster" {
		return
	}
	idx := cfg.WriteShapeIndex()
	if !idx.Known() {
		return
	}
	for key := range clustermask.AcceptedTopLevel {
		if !idx.Updatable(key) {
			warn("WARN clustermask: AcceptedTopLevel %q is not on the update payload — stale after a pin bump?\n", key)
		}
	}
	for _, leaf := range clustermask.CMRUpdatableLeafPaths() {
		if !idx.Updatable(leaf) {
			warn("WARN clustermask: CMR updatable leaf %q is not on the update payload — stale after a pin bump?\n", leaf)
		}
	}
	warnUnmappedCMRLeaves(attrs, idx, warn)
	for key, leaves := range clustermask.LeafExpansions {
		if !idx.Updatable(key) {
			warn("WARN clustermask: LeafExpansions key %q is not on the update payload — stale after a pin bump?\n", key)
		}
		for _, leaf := range leaves {
			if !idx.Updatable(leaf) {
				warn("WARN clustermask: LeafExpansions %q -> %q is not on the update payload — stale after a pin bump?\n", key, leaf)
			}
		}
	}
}

// warnUnmappedCMRLeaves reports customer_managed_resources leaves the update
// payload accepts and the schema exposes as mutable, but cmrUpdatableLeaves
// omits. ExpandCustomerManagedResourceLeaves cannot name such a leaf, so the
// mask never carries it: the user's edit applies cleanly and changes nothing.
//
// Leaves that force replacement are correctly absent — they never reach the
// update path.
func warnUnmappedCMRLeaves(attrs []schemagen.SchemaAttr, idx *schemagen.WriteShapeIndex, warn func(string, ...any)) {
	mapped := map[string]bool{}
	for _, p := range clustermask.CMRUpdatableLeafPaths() {
		mapped[p] = true
	}
	var walk func(as []schemagen.SchemaAttr, prefix string)
	walk = func(as []schemagen.SchemaAttr, prefix string) {
		for i := range as {
			a := &as[i]
			if a.ProtoName == "" {
				continue
			}
			path := prefix + a.ProtoName
			if len(a.NestedAttrs) > 0 {
				walk(a.NestedAttrs, path+".")
				continue
			}
			if !a.Optional && !a.Required {
				continue
			}
			if requiresReplace(a.PlanModifierNames) || mapped[path] || !idx.Updatable(path) {
				continue
			}
			warn("WARN clustermask: %q is updatable and mutable but absent from cmrUpdatableLeaves — the mask cannot carry it\n", path)
		}
	}
	for i := range attrs {
		if attrs[i].ProtoName == cmrAttrName {
			walk(attrs[i].NestedAttrs, cmrAttrName+".")
		}
	}
}

const cmrAttrName = "customer_managed_resources"

// requiresReplace matches any RequiresReplace form: a conditional one still
// keeps the leaf off the update path for the cases it triggers on, and the
// non-triggering cases are the mask's business, not this check's.
func requiresReplace(names []string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, "RequiresReplace") {
			return true
		}
	}
	return false
}
