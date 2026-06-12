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
	"log"
	"strings"

	bufvalidate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
)

func constraintSummary(rules *bufvalidate.FieldRules, fieldPath string) string {
	if rules == nil {
		return ""
	}
	var parts []string

	switch t := rules.GetType().(type) {
	case *bufvalidate.FieldRules_String_:
		parts = append(parts, stringRuleSummary(t.String_)...)
	case *bufvalidate.FieldRules_Int32:
		parts = append(parts, numericRuleSummary(int32Bounds(t.Int32))...)
	case *bufvalidate.FieldRules_Int64:
		parts = append(parts, numericRuleSummary(int64Bounds(t.Int64))...)
	case *bufvalidate.FieldRules_Uint32:
		parts = append(parts, numericRuleSummary(uint32Bounds(t.Uint32))...)
	case *bufvalidate.FieldRules_Uint64:
		parts = append(parts, numericRuleSummary(uint64Bounds(t.Uint64))...)
	case *bufvalidate.FieldRules_Float:
		parts = append(parts, numericRuleSummary(float32Bounds(t.Float))...)
	case *bufvalidate.FieldRules_Double:
		parts = append(parts, numericRuleSummary(float64Bounds(t.Double))...)
	case *bufvalidate.FieldRules_Enum:
		parts = append(parts, enumRuleSummary(t.Enum)...)
	case *bufvalidate.FieldRules_Bytes:
		parts = append(parts, bytesRuleSummary(t.Bytes)...)
	case *bufvalidate.FieldRules_Bool, *bufvalidate.FieldRules_Duration,
		*bufvalidate.FieldRules_Timestamp, *bufvalidate.FieldRules_Any,
		*bufvalidate.FieldRules_Repeated, *bufvalidate.FieldRules_Map:
		// repeated / map rules are summarized below via GetRepeated/GetMap.

	default:
		if t != nil {
			log.Printf("[schemagen warning] %s: unrecognized buf.validate.field type %T", fieldPath, t)
		}
	}

	if r := rules.GetRepeated(); r != nil {
		parts = append(parts, repeatedRuleSummary(r)...)
	}
	if m := rules.GetMap(); m != nil {
		parts = append(parts, mapRuleSummary(m)...)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// rangePhrases holds the four phrasings of a min/max range constraint: an
// exact match (one operand), a both-bounds range (two operands), a lower bound
// only, and an upper bound only.
type rangePhrases struct {
	exactly, between, atLeast, atMost string
}

// describeRange renders a min/max constraint into at most one sentence. nil
// bounds are treated as unset.
func describeRange[T comparable](minVal, maxVal *T, p rangePhrases) []string {
	switch {
	case minVal != nil && maxVal != nil && *minVal == *maxVal:
		return []string{fmt.Sprintf(p.exactly, *minVal)}
	case minVal != nil && maxVal != nil:
		return []string{fmt.Sprintf(p.between, *minVal, *maxVal)}
	case minVal != nil:
		return []string{fmt.Sprintf(p.atLeast, *minVal)}
	case maxVal != nil:
		return []string{fmt.Sprintf(p.atMost, *maxVal)}
	default:
		return nil
	}
}

func stringRuleSummary(r *bufvalidate.StringRules) []string {
	parts := describeRange(r.MinLen, r.MaxLen, rangePhrases{
		exactly: "Length must be exactly %d.",
		between: "Length must be between %d and %d.",
		atLeast: "Length must be at least %d.",
		atMost:  "Length must be at most %d.",
	})
	if r.Pattern != nil && *r.Pattern != "" {
		parts = append(parts, fmt.Sprintf("Must match pattern `%s`.", *r.Pattern))
	}
	switch {
	case len(r.GetIn()) > 0:
		parts = append(parts, fmt.Sprintf("Must be one of: %s.", strings.Join(r.GetIn(), ", ")))
	case len(r.GetNotIn()) > 0:
		parts = append(parts, fmt.Sprintf("Must not be one of: %s.", strings.Join(r.GetNotIn(), ", ")))
	default:
		// no enum constraint
	}
	if r.GetEmail() {
		parts = append(parts, "Must be a valid email address.")
	}
	if r.GetUuid() {
		parts = append(parts, "Must be a valid UUID.")
	}
	if r.GetUri() {
		parts = append(parts, "Must be a valid URI.")
	}
	if r.GetHostname() {
		parts = append(parts, "Must be a valid hostname.")
	}
	if r.GetIp() || r.GetIpv4() || r.GetIpv6() {
		parts = append(parts, "Must be a valid IP address.")
	}
	return parts
}

func enumRuleSummary(r *bufvalidate.EnumRules) []string {
	var parts []string
	if len(r.GetIn()) > 0 {
		nums := make([]string, len(r.GetIn()))
		for i, n := range r.GetIn() {
			nums[i] = fmt.Sprintf("%d", n)
		}
		parts = append(parts, fmt.Sprintf("Must be one of (enum values): %s.", strings.Join(nums, ", ")))
	}

	return parts
}

func bytesRuleSummary(r *bufvalidate.BytesRules) []string {
	return describeRange(r.MinLen, r.MaxLen, rangePhrases{
		exactly: "Byte length must be exactly %d.",
		between: "Byte length must be between %d and %d.",
		atLeast: "Byte length must be at least %d.",
		atMost:  "Byte length must be at most %d.",
	})
}

func repeatedRuleSummary(r *bufvalidate.RepeatedRules) []string {
	parts := describeRange(r.MinItems, r.MaxItems, rangePhrases{
		exactly: "Must have exactly %d items.",
		between: "Must have between %d and %d items.",
		atLeast: "Must have at least %d items.",
		atMost:  "Must have at most %d items.",
	})
	if r.GetUnique() {
		parts = append(parts, "Items must be unique.")
	}
	return parts
}

func mapRuleSummary(r *bufvalidate.MapRules) []string {
	return describeRange(r.MinPairs, r.MaxPairs, rangePhrases{
		exactly: "Must have exactly %d entries.",
		between: "Must have between %d and %d entries.",
		atLeast: "Must have at least %d entries.",
		atMost:  "Must have at most %d entries.",
	})
}

// numericBounds holds the pre-formatted bound strings extracted from a numeric
// buf.validate rule. A nil pointer means the bound is unset; gt/gte and lt/lte
// are mutually exclusive within their oneof.
type numericBounds struct {
	gt, gte, lt, lte *string
	in, notIn        []string
}

func numericRuleSummary(b numericBounds) []string {
	var parts []string
	switch {
	case b.gte != nil && b.lte != nil:
		parts = append(parts, fmt.Sprintf("Must be between %s and %s (inclusive).", *b.gte, *b.lte))
	case b.gt != nil && b.lt != nil:
		parts = append(parts, fmt.Sprintf("Must be greater than %s and less than %s.", *b.gt, *b.lt))
	case b.gte != nil && b.lt != nil:
		parts = append(parts, fmt.Sprintf("Must be at least %s and less than %s.", *b.gte, *b.lt))
	case b.gt != nil && b.lte != nil:
		parts = append(parts, fmt.Sprintf("Must be greater than %s and at most %s.", *b.gt, *b.lte))
	case b.gte != nil:
		parts = append(parts, fmt.Sprintf("Must be at least %s.", *b.gte))
	case b.gt != nil:
		parts = append(parts, fmt.Sprintf("Must be greater than %s.", *b.gt))
	case b.lte != nil:
		parts = append(parts, fmt.Sprintf("Must be at most %s.", *b.lte))
	case b.lt != nil:
		parts = append(parts, fmt.Sprintf("Must be less than %s.", *b.lt))
	default:
		// no numeric bound
	}
	if len(b.in) > 0 {
		parts = append(parts, fmt.Sprintf("Must be one of: %s.", strings.Join(b.in, ", ")))
	}
	if len(b.notIn) > 0 {
		parts = append(parts, fmt.Sprintf("Must not be one of: %s.", strings.Join(b.notIn, ", ")))
	}
	return parts
}

func formatNumericSlice[T any](vals []T, verb string) []string {
	if len(vals) == 0 {
		return nil
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprintf(verb, v)
	}
	return out
}

func ptrString(s string) *string { return &s }

func int32Bounds(r *bufvalidate.Int32Rules) numericBounds {
	b := numericBounds{in: formatNumericSlice(r.GetIn(), "%d"), notIn: formatNumericSlice(r.GetNotIn(), "%d")}
	switch g := r.GetGreaterThan().(type) {
	case *bufvalidate.Int32Rules_Gte:
		b.gte = ptrString(fmt.Sprintf("%d", g.Gte))
	case *bufvalidate.Int32Rules_Gt:
		b.gt = ptrString(fmt.Sprintf("%d", g.Gt))
	}
	switch l := r.GetLessThan().(type) {
	case *bufvalidate.Int32Rules_Lte:
		b.lte = ptrString(fmt.Sprintf("%d", l.Lte))
	case *bufvalidate.Int32Rules_Lt:
		b.lt = ptrString(fmt.Sprintf("%d", l.Lt))
	}
	return b
}

func int64Bounds(r *bufvalidate.Int64Rules) numericBounds {
	b := numericBounds{in: formatNumericSlice(r.GetIn(), "%d"), notIn: formatNumericSlice(r.GetNotIn(), "%d")}
	switch g := r.GetGreaterThan().(type) {
	case *bufvalidate.Int64Rules_Gte:
		b.gte = ptrString(fmt.Sprintf("%d", g.Gte))
	case *bufvalidate.Int64Rules_Gt:
		b.gt = ptrString(fmt.Sprintf("%d", g.Gt))
	}
	switch l := r.GetLessThan().(type) {
	case *bufvalidate.Int64Rules_Lte:
		b.lte = ptrString(fmt.Sprintf("%d", l.Lte))
	case *bufvalidate.Int64Rules_Lt:
		b.lt = ptrString(fmt.Sprintf("%d", l.Lt))
	}
	return b
}

func uint32Bounds(r *bufvalidate.UInt32Rules) numericBounds {
	b := numericBounds{in: formatNumericSlice(r.GetIn(), "%d"), notIn: formatNumericSlice(r.GetNotIn(), "%d")}
	switch g := r.GetGreaterThan().(type) {
	case *bufvalidate.UInt32Rules_Gte:
		b.gte = ptrString(fmt.Sprintf("%d", g.Gte))
	case *bufvalidate.UInt32Rules_Gt:
		b.gt = ptrString(fmt.Sprintf("%d", g.Gt))
	}
	switch l := r.GetLessThan().(type) {
	case *bufvalidate.UInt32Rules_Lte:
		b.lte = ptrString(fmt.Sprintf("%d", l.Lte))
	case *bufvalidate.UInt32Rules_Lt:
		b.lt = ptrString(fmt.Sprintf("%d", l.Lt))
	}
	return b
}

func uint64Bounds(r *bufvalidate.UInt64Rules) numericBounds {
	b := numericBounds{in: formatNumericSlice(r.GetIn(), "%d"), notIn: formatNumericSlice(r.GetNotIn(), "%d")}
	switch g := r.GetGreaterThan().(type) {
	case *bufvalidate.UInt64Rules_Gte:
		b.gte = ptrString(fmt.Sprintf("%d", g.Gte))
	case *bufvalidate.UInt64Rules_Gt:
		b.gt = ptrString(fmt.Sprintf("%d", g.Gt))
	}
	switch l := r.GetLessThan().(type) {
	case *bufvalidate.UInt64Rules_Lte:
		b.lte = ptrString(fmt.Sprintf("%d", l.Lte))
	case *bufvalidate.UInt64Rules_Lt:
		b.lt = ptrString(fmt.Sprintf("%d", l.Lt))
	}
	return b
}

func float32Bounds(r *bufvalidate.FloatRules) numericBounds {
	b := numericBounds{in: formatNumericSlice(r.GetIn(), "%g"), notIn: formatNumericSlice(r.GetNotIn(), "%g")}
	switch g := r.GetGreaterThan().(type) {
	case *bufvalidate.FloatRules_Gte:
		b.gte = ptrString(fmt.Sprintf("%g", g.Gte))
	case *bufvalidate.FloatRules_Gt:
		b.gt = ptrString(fmt.Sprintf("%g", g.Gt))
	}
	switch l := r.GetLessThan().(type) {
	case *bufvalidate.FloatRules_Lte:
		b.lte = ptrString(fmt.Sprintf("%g", l.Lte))
	case *bufvalidate.FloatRules_Lt:
		b.lt = ptrString(fmt.Sprintf("%g", l.Lt))
	}
	return b
}

func float64Bounds(r *bufvalidate.DoubleRules) numericBounds {
	b := numericBounds{in: formatNumericSlice(r.GetIn(), "%g"), notIn: formatNumericSlice(r.GetNotIn(), "%g")}
	switch g := r.GetGreaterThan().(type) {
	case *bufvalidate.DoubleRules_Gte:
		b.gte = ptrString(fmt.Sprintf("%g", g.Gte))
	case *bufvalidate.DoubleRules_Gt:
		b.gt = ptrString(fmt.Sprintf("%g", g.Gt))
	}
	switch l := r.GetLessThan().(type) {
	case *bufvalidate.DoubleRules_Lte:
		b.lte = ptrString(fmt.Sprintf("%g", l.Lte))
	case *bufvalidate.DoubleRules_Lt:
		b.lt = ptrString(fmt.Sprintf("%g", l.Lt))
	}
	return b
}
