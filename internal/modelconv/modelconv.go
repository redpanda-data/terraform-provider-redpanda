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

package modelconv

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TFScalar constrains element types that map 1:1 to a Terraform plugin
// framework scalar type (StringType / BoolType / Int32Type / Int64Type /
// Float64Type). Used by the generic List/Map helpers below so a single
// implementation covers every scalar proto leaf.
type TFScalar interface {
	~string | ~bool | ~int32 | ~int64 | ~float64
}

// ListFromSliceWithDiags wraps a slice of any TFScalar T as a types.List of
// elemType (which must match T — caller passes types.StringType for []string,
// types.BoolType for []bool, etc.). Nil slices yield ListNull so "unset" vs
// "explicitly cleared" round-trips correctly. Encoding diagnostics are
// appended to diags.
func ListFromSliceWithDiags[T TFScalar](ctx context.Context, v []T, elemType attr.Type, diags *diag.Diagnostics) types.List {
	if v == nil {
		return types.ListNull(elemType)
	}
	out, d := types.ListValueFrom(ctx, elemType, v)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// ListCarryKnownEmpty returns prev when cur is null and prev is a known empty
// list, else cur. proto3 repeated fields erase empty-vs-absent on the wire, so
// a planned [] reads back as a nil slice that flattens to null; carrying the
// known-empty prev preserves the explicit empty. A populated prev never
// overrides cur — a genuinely cleared server value must surface as drift.
func ListCarryKnownEmpty(cur, prev types.List) types.List {
	if cur.IsNull() && !prev.IsNull() && !prev.IsUnknown() && len(prev.Elements()) == 0 {
		return prev
	}
	return cur
}

// ListToSliceWithDiags unwraps a types.List into []T for any TFScalar T.
// Null and unknown both yield nil so callers don't accidentally send empty
// slices upstream. Decoding diagnostics are appended to diags.
func ListToSliceWithDiags[T TFScalar](ctx context.Context, v types.List, diags *diag.Diagnostics) []T {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	out := make([]T, 0, len(v.Elements()))
	d := v.ElementsAs(ctx, &out, false)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// ListFromObjects converts a slice of proto messages into a types.List of
// types.Object using the supplied per-element flattener. The flattener
// produces the typed nested model; we wrap it via ObjectValueFrom keyed
// off the supplied attr-types map.
//
// The flatten function takes a prev *Model argument for symmetry with
// ObjectFromMessageWithDiagsAndPrev — list elements have no natural prev
// mapping (the prev list may differ in length / order from the proto list),
// so this helper always passes nil. Per-element prev-state preservation
// across list reorderings is out of scope.
func ListFromObjects[Proto any, Model any](
	ctx context.Context,
	protos []Proto,
	attrTypes map[string]attr.Type,
	flatten func(context.Context, Proto, *Model) (Model, diag.Diagnostics),
) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: attrTypes}
	if protos == nil {
		return types.ListNull(elemType), nil
	}
	models := make([]Model, 0, len(protos))
	var diags diag.Diagnostics
	for _, p := range protos {
		m, d := flatten(ctx, p, nil)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(elemType), diags
		}
		models = append(models, m)
	}
	out, d := types.ListValueFrom(ctx, elemType, models)
	diags.Append(d...)
	return out, diags
}

// ListToObjects walks a types.List of types.Object, materializing each
// element into a typed Model via the framework's reflection (obj.As), then
// mapping to a proto slice via the per-element expander.
func ListToObjects[Model any, Proto any](
	ctx context.Context,
	list types.List,
	expand func(context.Context, *Model) (Proto, diag.Diagnostics),
) ([]Proto, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	elements := list.Elements()
	out := make([]Proto, 0, len(elements))
	for _, el := range elements {
		obj, ok := el.(basetypes.ObjectValue)
		if !ok {
			diags.AddError("expected types.Object element", "got non-object element in list")
			return nil, diags
		}
		var m Model
		if d := obj.As(ctx, &m, basetypes.ObjectAsOptions{}); d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		p, d := expand(ctx, &m)
		diags.Append(d...)
		if d.HasError() {
			return nil, diags
		}
		out = append(out, p)
	}
	return out, diags
}

// ListFromObjectsWithDiags is the assignment-friendly variant of ListFromObjects.
func ListFromObjectsWithDiags[Proto any, Model any](
	ctx context.Context,
	protos []Proto,
	attrTypes map[string]attr.Type,
	flatten func(context.Context, Proto, *Model) (Model, diag.Diagnostics),
	diags *diag.Diagnostics,
) types.List {
	out, d := ListFromObjects(ctx, protos, attrTypes, flatten)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// ListToObjectsWithDiags is the struct-literal-friendly variant of ListToObjects.
func ListToObjectsWithDiags[Model any, Proto any](
	ctx context.Context,
	list types.List,
	expand func(context.Context, *Model) (Proto, diag.Diagnostics),
	diags *diag.Diagnostics,
) []Proto {
	out, d := ListToObjects(ctx, list, expand)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// ObjectFromMessage converts a single proto message into types.Object using
// the supplied per-element flattener. Returns ObjectNull when proto is nil.
func ObjectFromMessage[Proto any, Model any](
	ctx context.Context,
	proto *Proto,
	attrTypes map[string]attr.Type,
	flatten func(context.Context, *Proto) (Model, diag.Diagnostics),
) (types.Object, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: attrTypes}
	if proto == nil {
		return types.ObjectNull(attrTypes), nil
	}
	m, diags := flatten(ctx, proto)
	if diags.HasError() {
		return types.ObjectNull(attrTypes), diags
	}
	out, d := types.ObjectValueFrom(ctx, objType.AttrTypes, m)
	diags.Append(d...)
	return out, diags
}

// ObjectToMessage decodes a types.Object into a typed model via obj.As, then
// maps to a proto pointer via the supplied expander. Returns nil for null /
// unknown objects so callers don't accidentally send zero protos upstream.
func ObjectToMessage[Model any, Proto any](
	ctx context.Context,
	obj types.Object,
	expand func(context.Context, *Model) (*Proto, diag.Diagnostics),
) (*Proto, diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var m Model
	var diags diag.Diagnostics
	if d := obj.As(ctx, &m, basetypes.ObjectAsOptions{}); d.HasError() {
		diags.Append(d...)
		return nil, diags
	}
	out, d := expand(ctx, &m)
	diags.Append(d...)
	if d.HasError() {
		return nil, diags
	}
	return out, diags
}

// ObjectFromMessageWithDiags is the assignment-friendly variant of ObjectFromMessage.
func ObjectFromMessageWithDiags[Proto any, Model any](
	ctx context.Context,
	proto *Proto,
	attrTypes map[string]attr.Type,
	flatten func(context.Context, *Proto) (Model, diag.Diagnostics),
	diags *diag.Diagnostics,
) types.Object {
	out, d := ObjectFromMessage(ctx, proto, attrTypes, flatten)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// ObjectFromMessageWithDiagsAndPrev is the prev-aware variant of
// ObjectFromMessageWithDiags. The flatten function receives the prior
// per-resource Model state so Flatten can preserve user-supplied
// null-vs-empty distinctions for fields whose proto3 wire shape is
// ambiguous (e.g. Optional-only strings backed by non-optional proto3
// strings — see the schemagen flatten emitter's prev-state-preservation
// block). prev may be nil; the flatten function must handle that.
func ObjectFromMessageWithDiagsAndPrev[Proto any, Model any](
	ctx context.Context,
	proto *Proto,
	prev *Model,
	attrTypes map[string]attr.Type,
	flatten func(context.Context, *Proto, *Model) (Model, diag.Diagnostics),
	diags *diag.Diagnostics,
) types.Object {
	objType := types.ObjectType{AttrTypes: attrTypes}
	if proto == nil {
		return types.ObjectNull(attrTypes)
	}
	m, d := flatten(ctx, proto, prev)
	if d.HasError() {
		if diags != nil {
			diags.Append(d...)
		}
		return types.ObjectNull(attrTypes)
	}
	out, d2 := types.ObjectValueFrom(ctx, objType.AttrTypes, m)
	d.Append(d2...)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// ObjectToMessageWithDiags is the struct-literal-friendly variant of ObjectToMessage.
func ObjectToMessageWithDiags[Model any, Proto any](
	ctx context.Context,
	obj types.Object,
	expand func(context.Context, *Model) (*Proto, diag.Diagnostics),
	diags *diag.Diagnostics,
) *Proto {
	out, d := ObjectToMessage(ctx, obj, expand)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// MapFromStrings wraps a map[string]string proto field as types.Map of strings.
func MapFromStrings(ctx context.Context, v map[string]string) (types.Map, diag.Diagnostics) {
	if v == nil {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(ctx, types.StringType, v)
}

// MapToStrings unwraps a types.Map into map[string]string. Null/unknown → nil.
func MapToStrings(ctx context.Context, v types.Map) (map[string]string, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	out := make(map[string]string, len(v.Elements()))
	d := v.ElementsAs(ctx, &out, false)
	return out, d
}

// MapFromStringsWithDiags is the assignment-friendly variant of MapFromStrings.
func MapFromStringsWithDiags(ctx context.Context, v map[string]string, diags *diag.Diagnostics) types.Map {
	out, d := MapFromStrings(ctx, v)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// MapToStringsWithDiags is the struct-literal-friendly variant of MapToStrings.
func MapToStringsWithDiags(ctx context.Context, v types.Map, diags *diag.Diagnostics) map[string]string {
	out, d := MapToStrings(ctx, v)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// BoolFromOneofPresence converts the result of proto.HasX() for an
// empty-message oneof variant into a types.Bool. Returns BoolValue(true)
// when present, BoolNull() when absent — emitting BoolValue(false) would
// cause "Provider produced inconsistent result after apply" when the user's
// config omits the unselected variant (config-null vs state-false diverge).
func BoolFromOneofPresence(present bool) types.Bool {
	if present {
		return types.BoolValue(true)
	}
	return types.BoolNull()
}

// StringFromTimestamp formats a *timestamppb.Timestamp as an RFC3339 string
// wrapped in types.String. A nil timestamp yields types.StringNull.
func StringFromTimestamp(t *timestamppb.Timestamp) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.AsTime().Format(time.RFC3339))
}

// TimestampFromString parses an RFC3339-formatted types.String into a
// *timestamppb.Timestamp. Null / unknown / empty yield nil. Parse errors
// surface as diagnostics rather than silent zero values.
func TimestampFromString(v types.String) (*timestamppb.Timestamp, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v.ValueString())
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("invalid timestamp", err.Error())
		return nil, diags
	}
	return timestamppb.New(t), nil
}

// TimestampFromStringWithDiags is the struct-literal-friendly variant of
// TimestampFromString. Parse errors are appended to the supplied collector.
func TimestampFromStringWithDiags(v types.String, diags *diag.Diagnostics) *timestamppb.Timestamp {
	out, d := TimestampFromString(v)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// StringFromDuration formats a *durationpb.Duration as a Go duration string.
func StringFromDuration(d *durationpb.Duration) types.String {
	if d == nil {
		return types.StringNull()
	}
	return types.StringValue(d.AsDuration().String())
}

// DurationFromString parses a Go duration string into a *durationpb.Duration.
func DurationFromString(v types.String) (*durationpb.Duration, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(v.ValueString())
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("invalid duration", err.Error())
		return nil, diags
	}
	return durationpb.New(d), nil
}

// DurationFromStringWithDiags is the struct-literal-friendly variant of
// DurationFromString. Parse errors are appended to the supplied collector.
func DurationFromStringWithDiags(v types.String, diags *diag.Diagnostics) *durationpb.Duration {
	out, d := DurationFromString(v)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}

// StringFromProtoStruct serializes a *structpb.Struct as a JSON string in
// types.String. A nil input yields types.StringNull. Used by schemagen for
// proto fields of type google.protobuf.Struct, which surface as TF strings
// holding the JSON object.
func StringFromProtoStruct(s *structpb.Struct) types.String {
	if s == nil {
		return types.StringNull()
	}
	b, err := json.Marshal(s.AsMap())
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(string(b))
}

// ProtoStructFromString parses a JSON-encoded types.String into a
// *structpb.Struct. Null / unknown / empty yield nil. Parse errors and
// structpb conversion errors surface as diagnostics.
func ProtoStructFromString(v types.String) (*structpb.Struct, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, nil
	}
	var diags diag.Diagnostics
	var m map[string]any
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		diags.AddError("invalid JSON struct", err.Error())
		return nil, diags
	}
	pb, err := structpb.NewStruct(m)
	if err != nil {
		diags.AddError("failed to convert JSON to protobuf Struct", err.Error())
		return nil, diags
	}
	return pb, nil
}

// ProtoStructFromStringWithDiags is the struct-literal-friendly variant of
// ProtoStructFromString. Parse errors are appended to the supplied collector.
func ProtoStructFromStringWithDiags(v types.String, diags *diag.Diagnostics) *structpb.Struct {
	out, d := ProtoStructFromString(v)
	if diags != nil {
		diags.Append(d...)
	}
	return out
}
