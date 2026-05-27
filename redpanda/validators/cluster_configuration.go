// Copyright 2024 Redpanda Data, Inc.
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

package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.Object = ClusterConfigurationValidator{}

// ClusterConfigurationValidator validates that cluster_configuration only contains allowed fields
type ClusterConfigurationValidator struct{}

// Description provides a description of the validator
func (ClusterConfigurationValidator) Description(_ context.Context) string {
	return "ensures cluster_configuration contains custom_properties (canonical) or custom_properties_json (deprecated alias) and rejects unknown keys"
}

// MarkdownDescription provides a description of the validator in markdown format
func (ClusterConfigurationValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures `cluster_configuration` contains `custom_properties` (canonical) or `custom_properties_json` (deprecated alias) and rejects unknown keys"
}

// allowedClusterConfigurationKeys is the set of keys cluster_configuration
// accepts at the top level: the canonical custom_properties plus the legacy
// custom_properties_json alias kept for the deprecation window. Both may be
// set when values agree; resolveLegacyAliases in resource_cluster.go promotes
// the legacy field onto the canonical pre-Expand and errors when they diverge.
var allowedClusterConfigurationKeys = map[string]bool{
	"custom_properties":      true,
	"custom_properties_json": true,
}

// ValidateObject validates that cluster_configuration contains only allowed
// keys (custom_properties / custom_properties_json) and that at least one of
// them is present in the object. Null values for those keys are accepted —
// matching the long-standing behavior of treating presence-of-key, not
// presence-of-value, as the required check.
func (ClusterConfigurationValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	var haveAllowedKey bool

	for key := range attrs {
		if !allowedClusterConfigurationKeys[key] {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid cluster_configuration field",
				fmt.Sprintf("cluster_configuration only supports custom_properties (canonical) and custom_properties_json (deprecated alias). Found unexpected field: %q.", key),
			)
			continue
		}
		haveAllowedKey = true
	}

	if !haveAllowedKey {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Missing required field in cluster_configuration",
			"cluster_configuration must contain custom_properties (canonical) or custom_properties_json (deprecated alias). "+
				"Use cluster_configuration = { custom_properties = jsonencode({...}) } format.",
		)
	}
}

// ClusterConfiguration returns a new ClusterConfigurationValidator
func ClusterConfiguration() validator.Object {
	return ClusterConfigurationValidator{}
}
