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
	return "ensures that cluster_configuration only contains the 'custom_properties_json' field at the top level"
}

// MarkdownDescription provides a description of the validator in markdown format
func (ClusterConfigurationValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that `cluster_configuration` only contains the `custom_properties_json` field at the top level"
}

// ValidateObject validates that the cluster_configuration object only contains allowed fields
func (ClusterConfigurationValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()

	for key := range attrs {
		if key != "custom_properties_json" {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid cluster_configuration field",
				fmt.Sprintf("The cluster_configuration block only supports the 'custom_properties_json' field with custom configuration defined inside custom_properties_json. Found unexpected field: '%s'. "+
					"Please use cluster_configuration = { custom_properties_json = jsonencode({...}) } format.", key),
			)
		}
	}

	if _, ok := attrs["custom_properties_json"]; !ok {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Missing required field in cluster_configuration",
			"The cluster_configuration block must contain the 'custom_properties_json' field. "+
				"Please use cluster_configuration = { custom_properties_json = jsonencode({...}) } format.",
		)
	}
}

// ClusterConfiguration returns a new ClusterConfigurationValidator
func ClusterConfiguration() validator.Object {
	return ClusterConfigurationValidator{}
}
