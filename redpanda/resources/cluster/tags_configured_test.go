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

package cluster

import (
	"context"
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
)

// TestConfiguredTagKeys_RoundTrip pins that the recorded key set is what Read
// gets back: a known map yields its keys, `tags = {}` yields an empty non-nil
// set, and a null configuration leaves whatever record exists in place.
func TestConfiguredTagKeys_RoundTrip(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		existing []byte
		tags     types.Map
		want     clustermodel.ConfiguredTagKeys
	}{
		{"keys of a known map", nil, types.MapValueMust(types.StringType, map[string]attr.Value{
			"env": types.StringValue("dev"), "team": types.StringValue("ingest"),
		}), clustermodel.ConfiguredTagKeys{"env": {}, "team": {}}},
		{"empty map is an empty set", nil, types.MapValueMust(types.StringType, map[string]attr.Value{}), clustermodel.ConfiguredTagKeys{}},
		{"null configuration with no record stays unrecorded", nil, types.MapNull(types.StringType), nil},
		{"null configuration keeps the existing record", []byte(`["env"]`), types.MapNull(types.StringType), clustermodel.ConfiguredTagKeys{"env": {}}},
		{"a known map replaces the existing record", []byte(`["env"]`), types.MapValueMust(types.StringType, map[string]attr.Value{
			"team": types.StringValue("ingest"),
		}), clustermodel.ConfiguredTagKeys{"team": {}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			private := fakePrivate{}
			if tc.existing != nil {
				private[configuredTagsKey] = tc.existing
			}
			var diags diag.Diagnostics
			stamped := stampConfiguredTags(ctx, configWithTags(ctx, t, tc.tags), private, private, &diags)
			if diags.HasError() {
				t.Fatalf("stamp: %v", diags)
			}
			got := configuredTagKeys(ctx, private, &diags)
			if diags.HasError() {
				t.Fatalf("read: %v", diags)
			}
			for label, set := range map[string]clustermodel.ConfiguredTagKeys{"stamped": stamped, "read back": got} {
				if (set == nil) != (tc.want == nil) || !maps.Equal(set, tc.want) {
					t.Errorf("%s = %v (nil=%t), want %v (nil=%t)", label, set, set == nil, tc.want, tc.want == nil)
				}
			}
		})
	}
}

// configWithTags builds a config whose only populated attribute is tags; the
// schema is the real resource schema so the path resolves as in production.
func configWithTags(ctx context.Context, t *testing.T, tags types.Map) tfsdk.Config {
	t.Helper()
	schema := ResourceClusterSchema(ctx)
	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, a := range schema.Attributes {
		ty := a.GetType().TerraformType(ctx)
		attrTypes[name] = ty
		values[name] = tftypes.NewValue(ty, nil)
	}
	tagsValue, err := tags.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("tags to terraform value: %v", err)
	}
	values["tags"] = tagsValue
	return tfsdk.Config{
		Schema: schema,
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values),
	}
}
