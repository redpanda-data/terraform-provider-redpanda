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
	"os"
	"strings"
)

type ancestorFrame struct {
	Name               string
	NullablePostCreate bool
}

func frameForAttr(attr *SchemaAttr) ancestorFrame {
	alwaysSet := attr.Required || (attr.Computed && !attr.Optional)
	return ancestorFrame{
		Name:               attr.Name,
		NullablePostCreate: !alwaysSet,
	}
}

const (
	modUseStateForUnknown        = "UseStateForUnknown"
	modUseNonNullStateForUnknown = "UseNonNullStateForUnknown"
	// modNone is a sentinel meaning "emit no plan modifier" — it suppresses the
	// auto-added state modifier. For a computed leaf whose value changes with a
	// sibling (e.g. a URL empty until `enabled` flips true), a state-pin modifier
	// would pin the stale value and trip inconsistent-result.
	modNone = "None"
)

func chooseStateModifier(ancestors []ancestorFrame, leafIsUserInput bool) string {
	for _, a := range ancestors {
		if a.NullablePostCreate {
			if leafIsUserInput {
				return modUseStateForUnknown
			}
			return modUseNonNullStateForUnknown
		}
	}
	return modUseStateForUnknown
}

func classifierReason(ancestors []ancestorFrame, leafIsUserInput bool, verdict string) string {
	if len(ancestors) == 0 {
		return "top-level computed field (no nullable parent)"
	}
	if verdict == modUseNonNullStateForUnknown {
		for _, a := range ancestors {
			if a.NullablePostCreate {
				return fmt.Sprintf("ancestor %q can be null after Create and leaf is computed_only (framework's child-of-nullable-parent niche)", a.Name)
			}
		}
	}
	if leafIsUserInput {
		for _, a := range ancestors {
			if a.NullablePostCreate {
				return fmt.Sprintf("ancestor %q can be null after Create but leaf is Optional+Computed — terminal-null is plausible, UseStateForUnknown avoids perpetual diff", a.Name)
			}
		}
	}
	return "all ancestors are always-set (Required or computed_only)"
}

func diagnoseOverride(resourceLabel, fieldPath string, modifiers []string, ancestors []ancestorFrame, leafIsUserInput bool) {
	var hasUseState, hasUseNonNull bool
	for _, m := range modifiers {
		switch m {
		case modUseStateForUnknown:
			hasUseState = true
		case modUseNonNullStateForUnknown:
			hasUseNonNull = true
		default:
			// other plan modifiers don't affect this classifier
		}
	}
	if !hasUseState && !hasUseNonNull {
		return
	}
	if hasUseState && hasUseNonNull {
		fmt.Fprintf(os.Stderr,
			"WARN classifier %s.%s: plan_modifiers lists both UseStateForUnknown and UseNonNullStateForUnknown — choose one\n",
			resourceLabel, fieldPath)
		return
	}
	expected := chooseStateModifier(ancestors, leafIsUserInput)
	got := modUseStateForUnknown
	if hasUseNonNull {
		got = modUseNonNullStateForUnknown
	}
	reason := classifierReason(ancestors, leafIsUserInput, expected)
	if got == expected {
		fmt.Fprintf(os.Stderr,
			"INFO classifier %s.%s: plan_modifiers=[%s] matches classifier (%s) — override is redundant, can be removed\n",
			resourceLabel, fieldPath, got, reason)
		return
	}
	fmt.Fprintf(os.Stderr,
		"WARN classifier %s.%s: plan_modifiers=[%s] conflicts with classifier (expected %s — %s); keep only if intentional\n",
		resourceLabel, fieldPath, got, expected, reason)
}

const classifierUnknownLabel = "<unknown>"

func classifierResourceLabel(cfg *Config) string {
	if cfg == nil {
		return classifierUnknownLabel
	}
	if cfg.APISchema != "" {
		return cfg.APISchema
	}
	return classifierUnknownLabel
}

func classifierDebug() bool {
	return strings.EqualFold(os.Getenv("SCHEMAGEN_CLASSIFIER_DEBUG"), "1") ||
		strings.EqualFold(os.Getenv("SCHEMAGEN_CLASSIFIER_DEBUG"), "true")
}

func debugClassify(resourceLabel, fieldPath, verdict string, ancestors []ancestorFrame, leafIsUserInput bool) {
	if !classifierDebug() {
		return
	}
	parts := make([]string, 0, len(ancestors))
	for _, a := range ancestors {
		nullable := "set"
		if a.NullablePostCreate {
			nullable = "nullable"
		}
		parts = append(parts, fmt.Sprintf("%s:%s", a.Name, nullable))
	}
	chain := strings.Join(parts, ",")
	if chain == "" {
		chain = "(top-level)"
	}
	leafKind := "computed_only"
	if leafIsUserInput {
		leafKind = "optional+computed"
	}
	fmt.Fprintf(os.Stderr, "DEBUG classifier %s.%s → %s [ancestors: %s] [leaf: %s]\n",
		resourceLabel, fieldPath, verdict, chain, leafKind)
}
