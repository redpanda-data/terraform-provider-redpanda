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

// Package compare provides helper functions for comparing Terraform model fields
// for plan/apply consistency testing and other comparison use cases.
package compare

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FieldDiff represents a single difference between two model instances
type FieldDiff struct {
	Field string // tfsdk field name (e.g., "id", "kafka_api.mtls.enabled")
	A     string // string representation of first value
	B     string // string representation of second value
	Type  string // "null_mismatch" | "value_mismatch" | "unknown_mismatch"
}

// Collect appends a non-nil diff to the slice
func Collect(diffs *[]FieldDiff, d *FieldDiff) {
	if d != nil {
		*diffs = append(*diffs, *d)
	}
}

// CollectAll appends all diffs to the slice
func CollectAll(diffs *[]FieldDiff, more []FieldDiff) {
	*diffs = append(*diffs, more...)
}

// String compares two types.String values
func String(field string, a, b types.String) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && a.ValueString() != b.ValueString() {
		return &FieldDiff{Field: field, A: a.ValueString(), B: b.ValueString(), Type: "value_mismatch"}
	}
	return nil
}

// Bool compares two types.Bool values
func Bool(field string, a, b types.Bool) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && a.ValueBool() != b.ValueBool() {
		return &FieldDiff{Field: field, A: boolStr(a.ValueBool()), B: boolStr(b.ValueBool()), Type: "value_mismatch"}
	}
	return nil
}

// Int64 compares two types.Int64 values
func Int64(field string, a, b types.Int64) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && a.ValueInt64() != b.ValueInt64() {
		return &FieldDiff{Field: field, A: fmt.Sprintf("%d", a.ValueInt64()), B: fmt.Sprintf("%d", b.ValueInt64()), Type: "value_mismatch"}
	}
	return nil
}

// Int32 compares two types.Int32 values
func Int32(field string, a, b types.Int32) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && a.ValueInt32() != b.ValueInt32() {
		return &FieldDiff{Field: field, A: fmt.Sprintf("%d", a.ValueInt32()), B: fmt.Sprintf("%d", b.ValueInt32()), Type: "value_mismatch"}
	}
	return nil
}

// Float64 compares two types.Float64 values
func Float64(field string, a, b types.Float64) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && a.ValueFloat64() != b.ValueFloat64() {
		return &FieldDiff{Field: field, A: fmt.Sprintf("%f", a.ValueFloat64()), B: fmt.Sprintf("%f", b.ValueFloat64()), Type: "value_mismatch"}
	}
	return nil
}

// List compares two types.List values (null status and length only)
// Deep element comparison should be done by generated code that knows the element type
func List(field string, a, b types.List) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && len(a.Elements()) != len(b.Elements()) {
		return &FieldDiff{
			Field: field,
			A:     fmt.Sprintf("len=%d", len(a.Elements())),
			B:     fmt.Sprintf("len=%d", len(b.Elements())),
			Type:  "value_mismatch",
		}
	}
	return nil
}

// Map compares two types.Map values (null status and key set only)
// Deep value comparison should be done by generated code that knows the value type
func Map(field string, a, b types.Map) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	if !a.IsNull() && !a.IsUnknown() && len(a.Elements()) != len(b.Elements()) {
		return &FieldDiff{
			Field: field,
			A:     fmt.Sprintf("len=%d", len(a.Elements())),
			B:     fmt.Sprintf("len=%d", len(b.Elements())),
			Type:  "value_mismatch",
		}
	}
	return nil
}

// Object compares two types.Object values (null status only)
// Deep attribute comparison should be done by generated code that knows the attribute types
func Object(field string, a, b types.Object) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	return nil
}

// ObjectNullCheck returns a diff if null status differs, nil otherwise
// Used by generated code before doing deep comparison
func ObjectNullCheck(field string, a, b types.Object) *FieldDiff {
	if a.IsNull() != b.IsNull() {
		return &FieldDiff{Field: field, A: nullStr(a.IsNull()), B: nullStr(b.IsNull()), Type: "null_mismatch"}
	}
	if a.IsUnknown() != b.IsUnknown() {
		return &FieldDiff{Field: field, A: unknownStr(a.IsUnknown()), B: unknownStr(b.IsUnknown()), Type: "unknown_mismatch"}
	}
	return nil
}

// Helper functions for string representation

func nullStr(isNull bool) string {
	if isNull {
		return "null"
	}
	return "set"
}

func unknownStr(isUnknown bool) string {
	if isUnknown {
		return "unknown"
	}
	return "known"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
