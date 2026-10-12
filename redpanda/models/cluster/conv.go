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
	"slices"
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

// serverTagKeys are the common tags the control plane writes on its own
// (cloudv2-infra common_tags): the AWS Partner Network attribution tag and
// the Google Cloud partner label. With its "redpanda-" bookkeeping they are
// the keys isServerTag recognizes. The filter only applies where no
// configuration says which keys are the user's (datasource, import, a
// tagless config); a user's key that merely shares a prefix is the user's. A
// control-plane key not listed here is still kept out of a resource's state
// by RetainConfiguredTags and carried back unchanged by MergeConfiguredTags.
var serverTagKeys = []string{"aws-apn-id", "goog-partner-solution"}

// ReservedTagKeys are the keys the control plane refuses from a client on
// update and overwrites or prunes on create (controlplane-api reservedTagKeys:
// cptags.ReservedCloudTagKeys plus every common-tag key).
var ReservedTagKeys = []string{"redpanda-org", "redpanda-id", "network-id", "redpanda-client", "aws-apn-id", "goog-partner-solution"}

// IsReservedTagKey reports whether a configuration may not set key: one of
// ReservedTagKeys, or any "redpanda-" key, which tagsFromProto strips. Either
// would apply and then fail the consistency check, so the resource rejects it
// at plan time.
func IsReservedTagKey(key string) bool {
	return strings.HasPrefix(key, "redpanda-") || slices.Contains(ReservedTagKeys, key)
}

func isServerTag(key string) bool {
	return strings.HasPrefix(key, "redpanda-") || slices.Contains(serverTagKeys, key)
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
	return tagsToProto(m.Tags)
}

func tagsToProto(tags types.Map) map[string]string {
	if tags.IsNull() || tags.IsUnknown() {
		return nil
	}
	elements := tags.Elements()
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
// to consult, the known control-plane keys are the only filter.
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

// ConfiguredTagKeys is the set of keys a configuration sets on tags. A nil
// set means no configuration names any key (a tagless config, import, or a
// resource last applied before the set was recorded); an empty non-nil set is
// `tags = {}`.
type ConfiguredTagKeys map[string]struct{}

// ConfiguredTagKeysOf returns the key set of a known configured map, or nil
// when the configuration is null or unknown.
func ConfiguredTagKeysOf(configured types.Map) ConfiguredTagKeys {
	if configured.IsNull() || configured.IsUnknown() {
		return nil
	}
	keys := make(ConfiguredTagKeys, len(configured.Elements()))
	for k := range configured.Elements() {
		keys[k] = struct{}{}
	}
	return keys
}

// RetainConfiguredTags keeps only the keys the configuration names once it
// names any. The control plane stamps its own keys into cloud_provider_tags
// and a key isServerTag does not know would otherwise fail the post-apply
// consistency check. A nil key set names no keys to keep, so the echo stands
// minus the known control-plane keys: that is what import and a tagless
// config rely on. A known configuration yields a known map, empty when
// nothing matches, because a planned `tags = {}` must not come back null.
func RetainConfiguredTags(server types.Map, configured ConfiguredTagKeys) types.Map {
	if configured == nil {
		if server.IsNull() || server.IsUnknown() {
			return server
		}
		return withoutServerTags(server)
	}
	elements := make(map[string]attr.Value, len(configured))
	if !server.IsNull() && !server.IsUnknown() {
		for k, v := range server.Elements() {
			if _, ok := configured[k]; ok {
				elements[k] = v
			}
		}
	}
	return types.MapValueMust(types.StringType, elements)
}

// MergeConfiguredTags builds the cloud_provider_tags payload for an update
// that owns only the configuration's keys: it starts from what the control
// plane holds now, drops every key the prior state tracked, and sets the
// planned keys. The control plane replaces the map under the mask, so a key
// left out is removed; sending the current map back is what keeps a key set
// outside Terraform in place. The removal matches the plan exactly, because
// the prior state carries only the keys the configuration set. The known
// control-plane keys are left out since it stamps them itself; any other key,
// including one it added that this list does not know, is sent back
// unchanged, which it accepts.
func MergeConfiguredTags(current map[string]string, prior, planned types.Map) map[string]string {
	out := make(map[string]string, len(current))
	for k, v := range current {
		if !isServerTag(k) {
			out[k] = v
		}
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		for k := range prior.Elements() {
			delete(out, k)
		}
	}
	for k, v := range tagsToProto(planned) {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
