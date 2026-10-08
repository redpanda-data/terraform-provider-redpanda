// Copyright 2023 Redpanda Data, Inc.
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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeTagsProto struct{ m map[string]string }

func (f fakeTagsProto) GetCloudProviderTags() map[string]string { return f.m }

func TestTagsFromProto_StripsServerKeys(t *testing.T) {
	got := tagsFromProto(fakeTagsProto{m: map[string]string{
		"env":                     "prod",
		"redpanda-managed":        "true",
		"redpanda-cluster":        "abc",
		"team":                    "platform",
		"aws-apn-id-123456789012": "pub-abc",
	}})
	if got.IsNull() {
		t.Fatal("expected non-null types.Map")
	}
	want := map[string]string{"env": "prod", "team": "platform", "aws-apn-id-123456789012": "pub-abc"}
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

func TestDatasourceTagsFromProto_DropsControlPlaneKeys(t *testing.T) {
	got := datasourceTagsFromProto(fakeTagsProto{m: map[string]string{
		"env":                     "prod",
		"redpanda-managed":        "true",
		"aws-apn-id-123456789012": "pub-abc",
	}})
	if len(got.Elements()) != 1 {
		t.Fatalf("got %v, want only env", got.Elements())
	}
	if _, ok := got.Elements()["env"]; !ok {
		t.Fatalf("got %v, want env", got.Elements())
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

func TestRetainConfiguredTags(t *testing.T) {
	tags := func(kv map[string]string) types.Map {
		elements := make(map[string]attr.Value, len(kv))
		for k, v := range kv {
			elements[k] = types.StringValue(v)
		}
		return types.MapValueMust(types.StringType, elements)
	}
	const apn = "aws-apn-id-123456789012"
	cases := []struct {
		name               string
		server, configured types.Map
		want               types.Map
	}{
		{
			name:       "key the control plane added is kept out of state",
			server:     tags(map[string]string{"env": "dev", apn: "pub-abc"}),
			configured: tags(map[string]string{"env": "dev"}),
			want:       tags(map[string]string{"env": "dev"}),
		},
		{
			name:       "null configuration keeps the echo minus control-plane keys so import sees the cluster's tags",
			server:     tags(map[string]string{"env": "dev", apn: "pub-abc"}),
			configured: types.MapNull(types.StringType),
			want:       tags(map[string]string{"env": "dev"}),
		},
		{
			name:       "null configuration and only control-plane keys is null",
			server:     tags(map[string]string{apn: "pub-abc"}),
			configured: types.MapNull(types.StringType),
			want:       types.MapNull(types.StringType),
		},
		{
			name:       "a configured key that shares a control-plane prefix is the user's",
			server:     tags(map[string]string{"aws-apn-id-999999999999": "mine", apn: "pub-abc"}),
			configured: tags(map[string]string{"aws-apn-id-999999999999": "mine"}),
			want:       tags(map[string]string{"aws-apn-id-999999999999": "mine"}),
		},
		{
			name:       "unknown configured tags keep the echo minus control-plane keys",
			server:     tags(map[string]string{"env": "dev", apn: "pub-abc"}),
			configured: types.MapUnknown(types.StringType),
			want:       tags(map[string]string{"env": "dev"}),
		},
		{
			name:       "value drift on a configured key is visible",
			server:     tags(map[string]string{"env": "prod", apn: "pub-abc"}),
			configured: tags(map[string]string{"env": "dev"}),
			want:       tags(map[string]string{"env": "prod"}),
		},
		{
			name:       "configured key removed out of band is visible as an empty map",
			server:     tags(map[string]string{apn: "pub-abc"}),
			configured: tags(map[string]string{"env": "dev"}),
			want:       tags(map[string]string{}),
		},
		{
			name:       "an empty configured map stays an empty map",
			server:     tags(map[string]string{apn: "pub-abc"}),
			configured: tags(map[string]string{}),
			want:       tags(map[string]string{}),
		},
		{
			name:       "an empty configured map with a null echo stays an empty map",
			server:     types.MapNull(types.StringType),
			configured: tags(map[string]string{}),
			want:       tags(map[string]string{}),
		},
		{
			name:       "null server echo under a known configuration is an empty map",
			server:     types.MapNull(types.StringType),
			configured: tags(map[string]string{"env": "dev"}),
			want:       tags(map[string]string{}),
		},
		{
			name:       "null server echo under a null configuration stays null",
			server:     types.MapNull(types.StringType),
			configured: types.MapNull(types.StringType),
			want:       types.MapNull(types.StringType),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RetainConfiguredTags(tc.server, tc.configured)
			if !got.Equal(tc.want) {
				t.Fatalf("RetainConfiguredTags(%v, %v) = %v, want %v", tc.server, tc.configured, got, tc.want)
			}
		})
	}
}
