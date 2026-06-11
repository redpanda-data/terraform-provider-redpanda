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
		parts = append(parts, numericRuleSummary(int32SummaryAdapter{t.Int32})...)
	case *bufvalidate.FieldRules_Int64:
		parts = append(parts, numericRuleSummary(int64SummaryAdapter{t.Int64})...)
	case *bufvalidate.FieldRules_Uint32:
		parts = append(parts, numericRuleSummary(uint32SummaryAdapter{t.Uint32})...)
	case *bufvalidate.FieldRules_Uint64:
		parts = append(parts, numericRuleSummary(uint64SummaryAdapter{t.Uint64})...)
	case *bufvalidate.FieldRules_Float:
		parts = append(parts, numericRuleSummary(float32SummaryAdapter{t.Float})...)
	case *bufvalidate.FieldRules_Double:
		parts = append(parts, numericRuleSummary(float64SummaryAdapter{t.Double})...)
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

func stringRuleSummary(r *bufvalidate.StringRules) []string {
	var parts []string
	switch {
	case r.MinLen != nil && r.MaxLen != nil && *r.MinLen == *r.MaxLen:
		parts = append(parts, fmt.Sprintf("Length must be exactly %d.", *r.MinLen))
	case r.MinLen != nil && r.MaxLen != nil:
		parts = append(parts, fmt.Sprintf("Length must be between %d and %d.", *r.MinLen, *r.MaxLen))
	case r.MinLen != nil:
		parts = append(parts, fmt.Sprintf("Length must be at least %d.", *r.MinLen))
	case r.MaxLen != nil:
		parts = append(parts, fmt.Sprintf("Length must be at most %d.", *r.MaxLen))
	default:
		// no length constraint
	}
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
	var parts []string
	switch {
	case r.MinLen != nil && r.MaxLen != nil && *r.MinLen == *r.MaxLen:
		parts = append(parts, fmt.Sprintf("Byte length must be exactly %d.", *r.MinLen))
	case r.MinLen != nil && r.MaxLen != nil:
		parts = append(parts, fmt.Sprintf("Byte length must be between %d and %d.", *r.MinLen, *r.MaxLen))
	case r.MinLen != nil:
		parts = append(parts, fmt.Sprintf("Byte length must be at least %d.", *r.MinLen))
	case r.MaxLen != nil:
		parts = append(parts, fmt.Sprintf("Byte length must be at most %d.", *r.MaxLen))
	default:
		// no byte length constraint
	}
	return parts
}

func repeatedRuleSummary(r *bufvalidate.RepeatedRules) []string {
	var parts []string
	switch {
	case r.MinItems != nil && r.MaxItems != nil && *r.MinItems == *r.MaxItems:
		parts = append(parts, fmt.Sprintf("Must have exactly %d items.", *r.MinItems))
	case r.MinItems != nil && r.MaxItems != nil:
		parts = append(parts, fmt.Sprintf("Must have between %d and %d items.", *r.MinItems, *r.MaxItems))
	case r.MinItems != nil:
		parts = append(parts, fmt.Sprintf("Must have at least %d items.", *r.MinItems))
	case r.MaxItems != nil:
		parts = append(parts, fmt.Sprintf("Must have at most %d items.", *r.MaxItems))
	default:
		// no item count constraint
	}
	if r.GetUnique() {
		parts = append(parts, "Items must be unique.")
	}
	return parts
}

func mapRuleSummary(r *bufvalidate.MapRules) []string {
	var parts []string
	switch {
	case r.MinPairs != nil && r.MaxPairs != nil && *r.MinPairs == *r.MaxPairs:
		parts = append(parts, fmt.Sprintf("Must have exactly %d entries.", *r.MinPairs))
	case r.MinPairs != nil && r.MaxPairs != nil:
		parts = append(parts, fmt.Sprintf("Must have between %d and %d entries.", *r.MinPairs, *r.MaxPairs))
	case r.MinPairs != nil:
		parts = append(parts, fmt.Sprintf("Must have at least %d entries.", *r.MinPairs))
	case r.MaxPairs != nil:
		parts = append(parts, fmt.Sprintf("Must have at most %d entries.", *r.MaxPairs))
	default:
		// no entry count constraint
	}
	return parts
}

type numericSummary interface {
	gte() (string, bool)
	gt() (string, bool)
	lte() (string, bool)
	lt() (string, bool)
	in() []string
	notIn() []string
}

func numericRuleSummary(r numericSummary) []string {
	var parts []string
	gte, hasGTE := r.gte()
	gt, hasGT := r.gt()
	lte, hasLTE := r.lte()
	lt, hasLT := r.lt()
	switch {
	case hasGTE && hasLTE:
		parts = append(parts, fmt.Sprintf("Must be between %s and %s (inclusive).", gte, lte))
	case hasGT && hasLT:
		parts = append(parts, fmt.Sprintf("Must be greater than %s and less than %s.", gt, lt))
	case hasGTE && hasLT:
		parts = append(parts, fmt.Sprintf("Must be at least %s and less than %s.", gte, lt))
	case hasGT && hasLTE:
		parts = append(parts, fmt.Sprintf("Must be greater than %s and at most %s.", gt, lte))
	case hasGTE:
		parts = append(parts, fmt.Sprintf("Must be at least %s.", gte))
	case hasGT:
		parts = append(parts, fmt.Sprintf("Must be greater than %s.", gt))
	case hasLTE:
		parts = append(parts, fmt.Sprintf("Must be at most %s.", lte))
	case hasLT:
		parts = append(parts, fmt.Sprintf("Must be less than %s.", lt))
	default:
		// no numeric bound
	}
	if vals := r.in(); len(vals) > 0 {
		parts = append(parts, fmt.Sprintf("Must be one of: %s.", strings.Join(vals, ", ")))
	}
	if vals := r.notIn(); len(vals) > 0 {
		parts = append(parts, fmt.Sprintf("Must not be one of: %s.", strings.Join(vals, ", ")))
	}
	return parts
}

type int32SummaryAdapter struct{ r *bufvalidate.Int32Rules }

func (a int32SummaryAdapter) gte() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.Int32Rules_Gte); ok {
		return fmt.Sprintf("%d", g.Gte), true
	}
	return "", false
}

func (a int32SummaryAdapter) gt() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.Int32Rules_Gt); ok {
		return fmt.Sprintf("%d", g.Gt), true
	}
	return "", false
}

func (a int32SummaryAdapter) lte() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.Int32Rules_Lte); ok {
		return fmt.Sprintf("%d", l.Lte), true
	}
	return "", false
}

func (a int32SummaryAdapter) lt() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.Int32Rules_Lt); ok {
		return fmt.Sprintf("%d", l.Lt), true
	}
	return "", false
}

func (a int32SummaryAdapter) in() []string {
	if len(a.r.GetIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetIn()))
	for i, v := range a.r.GetIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

func (a int32SummaryAdapter) notIn() []string {
	if len(a.r.GetNotIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetNotIn()))
	for i, v := range a.r.GetNotIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

type int64SummaryAdapter struct{ r *bufvalidate.Int64Rules }

func (a int64SummaryAdapter) gte() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.Int64Rules_Gte); ok {
		return fmt.Sprintf("%d", g.Gte), true
	}
	return "", false
}

func (a int64SummaryAdapter) gt() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.Int64Rules_Gt); ok {
		return fmt.Sprintf("%d", g.Gt), true
	}
	return "", false
}

func (a int64SummaryAdapter) lte() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.Int64Rules_Lte); ok {
		return fmt.Sprintf("%d", l.Lte), true
	}
	return "", false
}

func (a int64SummaryAdapter) lt() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.Int64Rules_Lt); ok {
		return fmt.Sprintf("%d", l.Lt), true
	}
	return "", false
}

func (a int64SummaryAdapter) in() []string {
	if len(a.r.GetIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetIn()))
	for i, v := range a.r.GetIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

func (a int64SummaryAdapter) notIn() []string {
	if len(a.r.GetNotIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetNotIn()))
	for i, v := range a.r.GetNotIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

type uint32SummaryAdapter struct{ r *bufvalidate.UInt32Rules }

func (a uint32SummaryAdapter) gte() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.UInt32Rules_Gte); ok {
		return fmt.Sprintf("%d", g.Gte), true
	}
	return "", false
}

func (a uint32SummaryAdapter) gt() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.UInt32Rules_Gt); ok {
		return fmt.Sprintf("%d", g.Gt), true
	}
	return "", false
}

func (a uint32SummaryAdapter) lte() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.UInt32Rules_Lte); ok {
		return fmt.Sprintf("%d", l.Lte), true
	}
	return "", false
}

func (a uint32SummaryAdapter) lt() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.UInt32Rules_Lt); ok {
		return fmt.Sprintf("%d", l.Lt), true
	}
	return "", false
}

func (a uint32SummaryAdapter) in() []string {
	if len(a.r.GetIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetIn()))
	for i, v := range a.r.GetIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

func (a uint32SummaryAdapter) notIn() []string {
	if len(a.r.GetNotIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetNotIn()))
	for i, v := range a.r.GetNotIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

type uint64SummaryAdapter struct{ r *bufvalidate.UInt64Rules }

func (a uint64SummaryAdapter) gte() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.UInt64Rules_Gte); ok {
		return fmt.Sprintf("%d", g.Gte), true
	}
	return "", false
}

func (a uint64SummaryAdapter) gt() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.UInt64Rules_Gt); ok {
		return fmt.Sprintf("%d", g.Gt), true
	}
	return "", false
}

func (a uint64SummaryAdapter) lte() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.UInt64Rules_Lte); ok {
		return fmt.Sprintf("%d", l.Lte), true
	}
	return "", false
}

func (a uint64SummaryAdapter) lt() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.UInt64Rules_Lt); ok {
		return fmt.Sprintf("%d", l.Lt), true
	}
	return "", false
}

func (a uint64SummaryAdapter) in() []string {
	if len(a.r.GetIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetIn()))
	for i, v := range a.r.GetIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

func (a uint64SummaryAdapter) notIn() []string {
	if len(a.r.GetNotIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetNotIn()))
	for i, v := range a.r.GetNotIn() {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

type float32SummaryAdapter struct{ r *bufvalidate.FloatRules }

func (a float32SummaryAdapter) gte() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.FloatRules_Gte); ok {
		return fmt.Sprintf("%g", g.Gte), true
	}
	return "", false
}

func (a float32SummaryAdapter) gt() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.FloatRules_Gt); ok {
		return fmt.Sprintf("%g", g.Gt), true
	}
	return "", false
}

func (a float32SummaryAdapter) lte() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.FloatRules_Lte); ok {
		return fmt.Sprintf("%g", l.Lte), true
	}
	return "", false
}

func (a float32SummaryAdapter) lt() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.FloatRules_Lt); ok {
		return fmt.Sprintf("%g", l.Lt), true
	}
	return "", false
}

func (a float32SummaryAdapter) in() []string {
	if len(a.r.GetIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetIn()))
	for i, v := range a.r.GetIn() {
		out[i] = fmt.Sprintf("%g", v)
	}
	return out
}

func (a float32SummaryAdapter) notIn() []string {
	if len(a.r.GetNotIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetNotIn()))
	for i, v := range a.r.GetNotIn() {
		out[i] = fmt.Sprintf("%g", v)
	}
	return out
}

type float64SummaryAdapter struct{ r *bufvalidate.DoubleRules }

func (a float64SummaryAdapter) gte() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.DoubleRules_Gte); ok {
		return fmt.Sprintf("%g", g.Gte), true
	}
	return "", false
}

func (a float64SummaryAdapter) gt() (string, bool) {
	if g, ok := a.r.GetGreaterThan().(*bufvalidate.DoubleRules_Gt); ok {
		return fmt.Sprintf("%g", g.Gt), true
	}
	return "", false
}

func (a float64SummaryAdapter) lte() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.DoubleRules_Lte); ok {
		return fmt.Sprintf("%g", l.Lte), true
	}
	return "", false
}

func (a float64SummaryAdapter) lt() (string, bool) {
	if l, ok := a.r.GetLessThan().(*bufvalidate.DoubleRules_Lt); ok {
		return fmt.Sprintf("%g", l.Lt), true
	}
	return "", false
}

func (a float64SummaryAdapter) in() []string {
	if len(a.r.GetIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetIn()))
	for i, v := range a.r.GetIn() {
		out[i] = fmt.Sprintf("%g", v)
	}
	return out
}

func (a float64SummaryAdapter) notIn() []string {
	if len(a.r.GetNotIn()) == 0 {
		return nil
	}
	out := make([]string, len(a.r.GetNotIn()))
	for i, v := range a.r.GetNotIn() {
		out[i] = fmt.Sprintf("%g", v)
	}
	return out
}
