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

// tagsFromProto reads the proto cloud_provider_tags map, filters out
// server-injected keys (prefix "redpanda-"), and returns a types.Map for
// the user-facing `tags` TF attribute.
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
