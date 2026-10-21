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
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
)

// validateReservedTagKeys rejects a configuration that sets a key the control
// plane owns: one of its reserved keys, or any "redpanda-" key, which read
// strips. The control plane refuses a reserved key on update but accepts it
// on create and then writes its own value, and it stores an unreserved
// "redpanda-" key as given; either way the apply ends in an "inconsistent
// result after apply" error with a live cluster behind it.
func validateReservedTagKeys(ctx context.Context, cfg tfsdk.Config, resp *resource.ValidateConfigResponse) {
	var tags types.Map
	if !getAttr(ctx, cfg, path.Root("tags"), &tags, resp) || tags.IsNull() || tags.IsUnknown() {
		return
	}
	for _, key := range slices.Sorted(maps.Keys(tags.Elements())) {
		if !clustermodel.IsReservedTagKey(key) {
			continue
		}
		resp.Diagnostics.AddAttributeError(path.Root("tags").AtMapKey(key),
			"Reserved Tag Key",
			fmt.Sprintf("tags key %q is reserved for the control plane, which manages redpanda-* and aws-apn-id-* keys and %s on its own; remove it from the configuration",
				key, strings.Join(clustermodel.ReservedTagKeys, ", ")))
	}
}
