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
	"strings"
)

type maskContractVerdict int

const (
	maskVerdictAgree maskContractVerdict = iota
	maskVerdictDerive
	maskVerdictRedundant
	maskVerdictConflict
)

// maskContractVerdictFor classifies a top-level attr against the update-mask
// contract: out-of-contract fields without RequiresReplace derive it; an
// explicit RequiresReplace on an out-of-contract field is redundant (the
// derivation covers it); RequiresReplace on an in-contract field contradicts
// the contract and is kept but warned about.
func maskContractVerdictFor(inContract, hasRequiresReplace bool) maskContractVerdict {
	switch {
	case !inContract && !hasRequiresReplace:
		return maskVerdictDerive
	case !inContract && hasRequiresReplace:
		return maskVerdictRedundant
	case inContract && hasRequiresReplace:
		return maskVerdictConflict
	default:
		return maskVerdictAgree
	}
}

// deriveMaskContractRequiresReplace applies the update-mask contract to the
// TOP-LEVEL attrs only: a user-settable field whose proto name is in neither
// contract set cannot be updated in place, so it requires replace. Nested
// attrs stay yaml-owned. Pure synthetics (no proto backing and no from_proto)
// are invisible to the contract and skipped.
//
// A hand-maintained (non-WarnOnly) contract auto-derives RequiresReplace for
// out-of-contract fields and only flags genuine contradictions. A WarnOnly
// contract — derived from the update payload proto descriptor — never mutates
// plan modifiers; it warns in both directions instead (Direction A: a payload
// field marked RequiresReplace; Direction B: a non-payload field missing it).
func deriveMaskContractRequiresReplace(attrs []SchemaAttr, fields map[string]FieldConfig, contract *MaskContract, mc *mergeCtx) {
	for i := range attrs {
		a := &attrs[i]
		if !a.Required && !a.Optional {
			continue
		}
		key := a.ProtoName
		if key == "" {
			key = fields[a.Name].FromProto
		}
		if key == "" {
			continue
		}
		inContract := contract.TopLevel[key] || contract.Leaf[key]
		hasRR := false
		for _, m := range a.PlanModifierNames {
			if strings.HasPrefix(m, "RequiresReplace") {
				hasRR = true
				break
			}
		}
		verdict := maskContractVerdictFor(inContract, hasRR)
		if contract.WarnOnly {
			switch verdict {
			case maskVerdictDerive:
				mc.warn(
					"WARN mask-contract %s.%s: %q is not in the update payload but is missing RequiresReplace — add it (the control plane cannot update it in place)\n",
					mc.resourceLabel, a.Name, key)
			case maskVerdictConflict:
				mc.warn(
					"WARN mask-contract %s.%s: plan_modifiers lists RequiresReplace but %q is in the update payload — remove it so the field updates in place\n",
					mc.resourceLabel, a.Name, key)
			case maskVerdictRedundant, maskVerdictAgree:
			default:
			}
			continue
		}
		switch verdict {
		case maskVerdictDerive:
			a.PlanModifierNames = append([]string{"RequiresReplace"}, a.PlanModifierNames...)
		case maskVerdictRedundant:
			mc.warn(
				"INFO mask-contract %s.%s: yaml RequiresReplace is redundant — derived from the update-mask contract; the override can be removed\n",
				mc.resourceLabel, a.Name)
		case maskVerdictConflict:
			mc.warn(
				"WARN mask-contract %s.%s: plan_modifiers lists RequiresReplace but %q is updatable per the update-mask contract — keep only if intentional\n",
				mc.resourceLabel, a.Name, key)
		case maskVerdictAgree:
		default:
		}
	}
}
