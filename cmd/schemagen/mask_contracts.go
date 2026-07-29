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

package main

import (
	"fmt"

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
func verifyClusterMaskPaths(cfg *schemagen.Config, warn func(string, ...any)) {
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
