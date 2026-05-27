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

import "testing"

func TestChooseStateModifier(t *testing.T) {
	required := ancestorFrame{Name: "req", NullablePostCreate: false}
	computedOnly := ancestorFrame{Name: "co", NullablePostCreate: false}
	optional := ancestorFrame{Name: "opt", NullablePostCreate: true}

	tests := []struct {
		name            string
		ancestors       []ancestorFrame
		leafIsUserInput bool
		want            string
	}{
		{
			name:      "top-level computed_only leaf",
			ancestors: nil,
			want:      modUseStateForUnknown,
		},
		{
			name:            "top-level optional+computed leaf",
			ancestors:       nil,
			leafIsUserInput: true,
			want:            modUseStateForUnknown,
		},
		{
			name:      "single required ancestor, computed_only leaf",
			ancestors: []ancestorFrame{required},
			want:      modUseStateForUnknown,
		},
		{
			name:      "single computed_only ancestor, computed_only leaf",
			ancestors: []ancestorFrame{computedOnly},
			want:      modUseStateForUnknown,
		},
		{
			name:      "single optional ancestor, computed_only leaf — framework niche",
			ancestors: []ancestorFrame{optional},
			want:      modUseNonNullStateForUnknown,
		},
		{
			name:            "single optional ancestor, optional+computed leaf — terminal-null possible",
			ancestors:       []ancestorFrame{optional},
			leafIsUserInput: true,
			want:            modUseStateForUnknown,
		},
		{
			name:      "deep chain all always-set, computed_only leaf",
			ancestors: []ancestorFrame{required, computedOnly, required},
			want:      modUseStateForUnknown,
		},
		{
			name:      "nullable ancestor at root, computed_only leaf",
			ancestors: []ancestorFrame{optional, required, computedOnly},
			want:      modUseNonNullStateForUnknown,
		},
		{
			name:            "nullable ancestor at root, optional+computed leaf",
			ancestors:       []ancestorFrame{optional, required, computedOnly},
			leafIsUserInput: true,
			want:            modUseStateForUnknown,
		},
		{
			name:      "nullable ancestor in middle, computed_only leaf",
			ancestors: []ancestorFrame{required, optional, computedOnly},
			want:      modUseNonNullStateForUnknown,
		},
		{
			name:      "nullable ancestor at leaf parent, computed_only leaf",
			ancestors: []ancestorFrame{required, computedOnly, optional},
			want:      modUseNonNullStateForUnknown,
		},
		{
			name:            "nullable ancestor at leaf parent, optional+computed leaf",
			ancestors:       []ancestorFrame{required, computedOnly, optional},
			leafIsUserInput: true,
			want:            modUseStateForUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := chooseStateModifier(tc.ancestors, tc.leafIsUserInput)
			if got != tc.want {
				t.Fatalf("chooseStateModifier(%v, leafIsUserInput=%v) = %q, want %q",
					tc.ancestors, tc.leafIsUserInput, got, tc.want)
			}
		})
	}
}

func TestFrameForAttr(t *testing.T) {
	tests := []struct {
		name     string
		attr     SchemaAttr
		wantNull bool
	}{
		{
			name:     "required is always-set",
			attr:     SchemaAttr{Required: true},
			wantNull: false,
		},
		{
			name:     "computed_only is always-set",
			attr:     SchemaAttr{Computed: true},
			wantNull: false,
		},
		{
			name:     "optional+computed is nullable",
			attr:     SchemaAttr{Optional: true, Computed: true},
			wantNull: true,
		},
		{
			name:     "optional only is nullable",
			attr:     SchemaAttr{Optional: true},
			wantNull: true,
		},
		{
			name:     "no flags is nullable (defensive default)",
			attr:     SchemaAttr{},
			wantNull: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := frameForAttr(&tc.attr)
			if got.NullablePostCreate != tc.wantNull {
				t.Fatalf("frameForAttr(%+v).NullablePostCreate = %v, want %v",
					tc.attr, got.NullablePostCreate, tc.wantNull)
			}
		})
	}
}

func TestClassifierResourceLabel(t *testing.T) {
	if got := classifierResourceLabel(nil); got != classifierUnknownLabel {
		t.Errorf("nil cfg → %q, want %s", got, classifierUnknownLabel)
	}
	if got := classifierResourceLabel(&Config{}); got != classifierUnknownLabel {
		t.Errorf("empty cfg → %q, want %s", got, classifierUnknownLabel)
	}
	if got := classifierResourceLabel(&Config{APISchema: "Cluster"}); got != "Cluster" {
		t.Errorf("APISchema=Cluster → %q, want Cluster", got)
	}
}
