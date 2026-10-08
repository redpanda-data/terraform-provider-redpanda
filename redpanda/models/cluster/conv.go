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
	"strings"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type clusterAPIURLProto interface {
	GetDataplaneApi() *controlplanev1.Cluster_DataplaneAPI
}

func clusterAPIURLFromProto(proto clusterAPIURLProto) types.String {
	dp := proto.GetDataplaneApi()
	if dp == nil {
		return types.StringNull()
	}
	return types.StringValue(dp.GetUrl())
}

type tagsProto interface {
	GetCloudProviderTags() map[string]string
}

// TODO(schemagen): replace tagsFromProto / TagsForProto with a schemagen
// `deprecated_for:` directive once that directive ships. Today the cluster
// schema.yaml uses `proto_only: true` on cloud_provider_tags plus
// flatten_via/expand_via overrides on the user-facing `tags` attribute; the
// directive would let schemagen emit both names with one canonical/deprecated
// pair instead of these hand-rolled bridges.

// serverTagPrefixes are the cloud_provider_tags keys the control plane writes
// on its own: its "redpanda-" bookkeeping and the AWS Partner Network
// attribution tag, whose key carries the partner account id. The list only
// applies where no configuration says which keys are the user's (datasource,
// import, a tagless config), because a user may legitimately set a key with
// one of these prefixes. A control-plane key not listed here is still kept
// out of a resource's state by RetainConfiguredTags.
var serverTagPrefixes = []string{"redpanda-", "aws-apn-id"}

func isServerTag(key string) bool {
	for _, p := range serverTagPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// tagsFromProto drops the "redpanda-" keys, which the control plane writes on
// every cluster (redpanda-managed) and the provider treats as reserved, and
// keeps every other key for the caller to filter; an echo with nothing left
// is null, not empty.
func tagsFromProto(proto tagsProto) types.Map {
	raw := proto.GetCloudProviderTags()
	if len(raw) == 0 {
		return types.MapNull(types.StringType)
	}
	elements := make(map[string]attr.Value, len(raw))
	for k, v := range raw {
		if strings.HasPrefix(k, "redpanda-") {
			continue
		}
		elements[k] = types.StringValue(v)
	}
	if len(elements) == 0 {
		return types.MapNull(types.StringType)
	}
	return types.MapValueMust(types.StringType, elements)
}

// TagsForProto materializes the user-facing `tags` types.Map into a plain
// map[string]string for the proto cloud_provider_tags field on Expand.
func (m *ResourceModel) TagsForProto() map[string]string {
	if m.Tags.IsNull() || m.Tags.IsUnknown() {
		return nil
	}
	elements := m.Tags.Elements()
	if len(elements) == 0 {
		return nil
	}
	out := make(map[string]string, len(elements))
	for k, v := range elements {
		s, ok := v.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		out[k] = s.ValueString()
	}
	return out
}

// datasourceTagsFromProto is the datasource flatten: with no configuration
// to consult, the known control-plane prefixes are the only filter.
func datasourceTagsFromProto(proto tagsProto) types.Map {
	return withoutServerTags(tagsFromProto(proto))
}

func withoutServerTags(m types.Map) types.Map {
	if m.IsNull() || m.IsUnknown() {
		return m
	}
	elements := make(map[string]attr.Value, len(m.Elements()))
	for k, v := range m.Elements() {
		if !isServerTag(k) {
			elements[k] = v
		}
	}
	if len(elements) == 0 {
		return types.MapNull(types.StringType)
	}
	return types.MapValueMust(types.StringType, elements)
}

// RetainConfiguredTags keeps only the keys the configuration names once it
// names any. The control plane stamps its own keys into cloud_provider_tags
// and a key not yet in serverTagPrefixes would otherwise fail the post-apply
// consistency check. A null or unknown configuration names no keys to keep,
// so the echo stands minus the known control-plane prefixes: that is what
// import and a tagless config rely on. A known configuration yields a known
// map, empty when nothing matches, because a planned `tags = {}` must not
// come back null.
func RetainConfiguredTags(server, configured types.Map) types.Map {
	if configured.IsNull() || configured.IsUnknown() {
		if server.IsNull() || server.IsUnknown() {
			return server
		}
		return withoutServerTags(server)
	}
	elements := make(map[string]attr.Value, len(configured.Elements()))
	if !server.IsNull() && !server.IsUnknown() {
		for k, v := range server.Elements() {
			if _, ok := configured.Elements()[k]; ok {
				elements[k] = v
			}
		}
	}
	return types.MapValueMust(types.StringType, elements)
}
