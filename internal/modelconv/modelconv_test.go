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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

// TestListFromSliceWithDiags exercises the generic flatten direction for
// every TFScalar type, mirroring the list_scalar_kinds_conv golden case
// (tags/string, ids/int32, counts/int64, flags/bool, weights/double).
func TestListFromSliceWithDiags(t *testing.T) {
	ctx := context.Background()

	t.Run("String nil yields ListNull(StringType)", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags[string](ctx, nil, types.StringType, &d)
		require.True(t, got.IsNull())
		require.Equal(t, types.StringType, got.ElementType(ctx))
		require.False(t, d.HasError())
	})
	t.Run("String empty slice yields empty list (not null)", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags(ctx, []string{}, types.StringType, &d)
		require.False(t, got.IsNull())
		require.Equal(t, 0, len(got.Elements()))
		require.False(t, d.HasError())
	})
	t.Run("String populated round-trips through ListToSlice", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags(ctx, []string{"a", "b"}, types.StringType, &d)
		require.False(t, got.IsNull())
		round := ListToSliceWithDiags[string](ctx, got, &d)
		require.Equal(t, []string{"a", "b"}, round)
		require.False(t, d.HasError())
	})

	t.Run("Bool round-trip", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags(ctx, []bool{true, false, true}, types.BoolType, &d)
		require.False(t, got.IsNull())
		round := ListToSliceWithDiags[bool](ctx, got, &d)
		require.Equal(t, []bool{true, false, true}, round)
		require.False(t, d.HasError())
	})

	t.Run("Int32 round-trip", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags(ctx, []int32{1, 2, 3}, types.Int32Type, &d)
		require.False(t, got.IsNull())
		round := ListToSliceWithDiags[int32](ctx, got, &d)
		require.Equal(t, []int32{1, 2, 3}, round)
		require.False(t, d.HasError())
	})

	t.Run("Int64 round-trip", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags(ctx, []int64{10, 20, 30}, types.Int64Type, &d)
		require.False(t, got.IsNull())
		round := ListToSliceWithDiags[int64](ctx, got, &d)
		require.Equal(t, []int64{10, 20, 30}, round)
		require.False(t, d.HasError())
	})

	t.Run("Float64 round-trip", func(t *testing.T) {
		var d diag.Diagnostics
		got := ListFromSliceWithDiags(ctx, []float64{1.5, 2.5}, types.Float64Type, &d)
		require.False(t, got.IsNull())
		round := ListToSliceWithDiags[float64](ctx, got, &d)
		require.Equal(t, []float64{1.5, 2.5}, round)
		require.False(t, d.HasError())
	})

	t.Run("nil diags pointer is safe", func(t *testing.T) {
		got := ListFromSliceWithDiags(ctx, []string{"x"}, types.StringType, nil)
		require.False(t, got.IsNull())
		round := ListToSliceWithDiags[string](ctx, got, nil)
		require.Equal(t, []string{"x"}, round)
	})
}

func TestListToSliceWithDiags(t *testing.T) {
	ctx := context.Background()

	t.Run("null input yields nil slice", func(t *testing.T) {
		var d diag.Diagnostics
		require.Nil(t, ListToSliceWithDiags[string](ctx, types.ListNull(types.StringType), &d))
		require.False(t, d.HasError())
	})
	t.Run("unknown input yields nil slice", func(t *testing.T) {
		var d diag.Diagnostics
		require.Nil(t, ListToSliceWithDiags[int32](ctx, types.ListUnknown(types.Int32Type), &d))
		require.False(t, d.HasError())
	})
	t.Run("empty list yields empty slice (not nil)", func(t *testing.T) {
		empty, ld := types.ListValueFrom(ctx, types.StringType, []string{})
		require.False(t, ld.HasError())
		var d diag.Diagnostics
		got := ListToSliceWithDiags[string](ctx, empty, &d)
		require.NotNil(t, got)
		require.Equal(t, 0, len(got))
	})
}

// Compile-time assertion that types.StringType satisfies attr.Type so that
// the ListFromSliceWithDiags(ctx, v, types.StringType, ...) shape generated
// by schemagen always compiles.
var _ attr.Type = types.StringType

func TestListCarryKnownEmpty(t *testing.T) {
	ctx := context.Background()
	null := types.ListNull(types.StringType)
	unknown := types.ListUnknown(types.StringType)
	empty, d := types.ListValueFrom(ctx, types.StringType, []string{})
	require.False(t, d.HasError())
	populated, d := types.ListValueFrom(ctx, types.StringType, []string{"a"})
	require.False(t, d.HasError())

	t.Run("null cur carries known-empty prev", func(t *testing.T) {
		require.Equal(t, empty, ListCarryKnownEmpty(null, empty))
	})
	t.Run("null cur keeps null when prev null", func(t *testing.T) {
		require.Equal(t, null, ListCarryKnownEmpty(null, null))
	})
	t.Run("null cur keeps null when prev unknown", func(t *testing.T) {
		require.Equal(t, null, ListCarryKnownEmpty(null, unknown))
	})
	t.Run("null cur keeps null when prev populated", func(t *testing.T) {
		require.Equal(t, null, ListCarryKnownEmpty(null, populated))
	})
	t.Run("populated cur wins over empty prev", func(t *testing.T) {
		require.Equal(t, populated, ListCarryKnownEmpty(populated, empty))
	})
	t.Run("empty cur unchanged", func(t *testing.T) {
		require.Equal(t, empty, ListCarryKnownEmpty(empty, populated))
	})
}
