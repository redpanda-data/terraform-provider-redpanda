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
	"encoding/json"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
)

// configuredTagsKey names the private-state record of the tags keys the
// configuration set at the last applied change. Read has no configuration to
// consult and the prior state is not a substitute: a tagless resource would
// freeze on whichever keys reached state first, and a resource upgraded from
// a provider that stored a control-plane key would keep it forever. Without
// the record (import, a tagless configuration, a resource last applied before
// the record existed) Read keeps every key outside the known control-plane
// keys.
const configuredTagsKey = "tags_configured"

// stampConfiguredTags records the tags keys cfg sets and returns them. A null
// configuration leaves the record as it is and returns it: tags is computed,
// so a configuration that stops setting it keeps the last applied keys in
// state, and the record has to keep matching that state.
func stampConfiguredTags(ctx context.Context, cfg tfsdk.Config, prev privateReader, next privateWriter, diags *diag.Diagnostics) clustermodel.ConfiguredTagKeys {
	var tags types.Map
	d := cfg.GetAttribute(ctx, path.Root("tags"), &tags)
	diags.Append(d...)
	if d.HasError() {
		return nil
	}
	keys := clustermodel.ConfiguredTagKeysOf(tags)
	if keys == nil {
		return configuredTagKeys(ctx, prev, diags)
	}
	raw, err := json.Marshal(slices.Sorted(maps.Keys(keys)))
	if err != nil {
		diags.AddError("failed to record the configured tags keys", err.Error())
		return nil
	}
	diags.Append(next.SetKey(ctx, configuredTagsKey, raw)...)
	return keys
}

// configuredTagKeys returns the recorded key set, or nil when there is none.
func configuredTagKeys(ctx context.Context, prev privateReader, diags *diag.Diagnostics) clustermodel.ConfiguredTagKeys {
	raw, d := prev.GetKey(ctx, configuredTagsKey)
	diags.Append(d...)
	if d.HasError() || len(raw) == 0 {
		return nil
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		diags.AddError("failed to read the configured tags keys", err.Error())
		return nil
	}
	keys := make(clustermodel.ConfiguredTagKeys, len(names))
	for _, name := range names {
		keys[name] = struct{}{}
	}
	return keys
}
