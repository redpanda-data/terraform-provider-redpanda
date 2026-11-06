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

package schemagen

import (
	"fmt"
	"log"
	"strings"
)

// deriveRepeatedRuleValidators turns buf.validate repeated rules into
// plan-time list validators (min_items → SizeAtLeast, max_items → SizeAtMost,
// unique → UniqueValues). The rpvalidate config validator also enforces these
// rules but defers whenever the config has unknowns, which real configs with
// cross-resource references always do; schema validators run regardless.
// Yaml-resolved validators compose via append. Computed-only attrs take no
// input and are skipped.
func deriveRepeatedRuleValidators(attrs []SchemaAttr, proto *ProtoMessage, parentPath string) {
	if proto == nil {
		return
	}
	for i := range attrs {
		a := &attrs[i]
		f := proto.FindField(a.ProtoName)
		if f == nil {
			continue
		}
		if len(a.NestedAttrs) > 0 && f.Nested != nil {
			deriveRepeatedRuleValidators(a.NestedAttrs, f.Nested, joinPath(parentPath, a.ProtoName))
		}
		r := f.ValidateRules.GetRepeated()
		if r == nil {
			continue
		}
		if !a.Optional && !a.Required {
			continue
		}
		var exprs []string
		if r.MinItems != nil {
			exprs = append(exprs, fmt.Sprintf("listvalidator.SizeAtLeast(%d)", r.GetMinItems()))
		}
		if r.MaxItems != nil {
			exprs = append(exprs, fmt.Sprintf("listvalidator.SizeAtMost(%d)", r.GetMaxItems()))
		}
		if r.GetUnique() {
			exprs = append(exprs, "listvalidator.UniqueValues()")
		}
		if len(exprs) == 0 {
			continue
		}
		if validatorAttrType(a.AttrType) != "List" {
			log.Printf("[schemagen warning] %s: repeated buf.validate rules on non-list attr type %s — validators skipped",
				joinPath(parentPath, a.ProtoName), a.AttrType)
			continue
		}
		if a.Validators == "" {
			a.Validators = wrapValidatorSlice("List", exprs)
		} else {
			a.Validators = fmt.Sprintf("append(%s, %s)", a.Validators, strings.Join(exprs, ", "))
		}
	}
}

// deriveFoldLeafValidators refuses "" on the required string leaves of a
// single-nested block whose all-empty echo folds to null. The fold reads an
// empty leaf as the block being gone, so a configured "" would plan a block
// and read back null, an inconsistent result after apply; the control plane
// rejects the value too, but only at apply. Yaml-resolved validators compose
// via append.
func deriveFoldLeafValidators(attrs []SchemaAttr, proto *ProtoMessage, fcs map[string]FieldConfig, parentPath string) {
	if proto == nil {
		return
	}
	for i := range attrs {
		a := &attrs[i]
		f := proto.FindField(lookupProtoName(a))
		if f == nil {
			continue
		}
		var fc *FieldConfig
		var nestedFcs map[string]FieldConfig
		if c, ok := fcs[a.Name]; ok {
			fc = &c
			nestedFcs = c.Fields
		}
		if len(a.NestedAttrs) > 0 && f.Nested != nil {
			deriveFoldLeafValidators(a.NestedAttrs, f.Nested, nestedFcs, joinPath(parentPath, a.ProtoName))
		}
		if a.AttrType != AttrTypeSingleNested {
			continue
		}
		if (fc == nil || !fc.EmptyIsNull) && !zeroFoldEligible(a, fc, f) {
			continue
		}
		for j := range a.NestedAttrs {
			leaf := &a.NestedAttrs[j]
			if !leaf.Required || leaf.AttrType != AttrTypeString || strings.Contains(leaf.Validators, "LengthAtLeast") {
				continue
			}
			const expr = "stringvalidator.LengthAtLeast(1)"
			if leaf.Validators == "" {
				leaf.Validators = wrapValidatorSlice("String", []string{expr})
			} else {
				leaf.Validators = fmt.Sprintf("append(%s, %s)", leaf.Validators, expr)
			}
		}
	}
}
