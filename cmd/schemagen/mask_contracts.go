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
		Leaf:     leafKeys(clustermask.LeafExpansions),
	},
}

func leafKeys(m map[string][]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
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
