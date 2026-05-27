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

package cluster

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeTagsProto struct{ m map[string]string }

func (f fakeTagsProto) GetCloudProviderTags() map[string]string { return f.m }

func TestTagsFromProto_StripsServerKeys(t *testing.T) {
	got := tagsFromProto(fakeTagsProto{m: map[string]string{
		"env":              "prod",
		"redpanda-managed": "true",
		"redpanda-cluster": "abc",
		"team":             "platform",
	}})
	if got.IsNull() {
		t.Fatal("expected non-null types.Map")
	}
	want := map[string]string{"env": "prod", "team": "platform"}
	if len(got.Elements()) != len(want) {
		t.Fatalf("got %d elements, want %d: %v", len(got.Elements()), len(want), got.Elements())
	}
	for k, v := range want {
		gotVal, ok := got.Elements()[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		s, ok := gotVal.(types.String)
		if !ok || s.ValueString() != v {
			t.Errorf("key %q: got %v, want %s", k, gotVal, v)
		}
	}
	for _, k := range []string{"redpanda-managed", "redpanda-cluster"} {
		if _, ok := got.Elements()[k]; ok {
			t.Errorf("expected key %q to be filtered out", k)
		}
	}
}

func TestTagsFromProto_AllServerKeysReturnsNull(t *testing.T) {
	got := tagsFromProto(fakeTagsProto{m: map[string]string{
		"redpanda-managed": "true",
		"redpanda-cluster": "abc",
	}})
	if !got.IsNull() {
		t.Fatalf("expected null (all keys filtered), got %v", got.Elements())
	}
}

func TestTagsFromProto_EmptyInputReturnsNull(t *testing.T) {
	got := tagsFromProto(fakeTagsProto{m: nil})
	if !got.IsNull() {
		t.Fatalf("expected null for nil input, got %v", got.Elements())
	}
}
