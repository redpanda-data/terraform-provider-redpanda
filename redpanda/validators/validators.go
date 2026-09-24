// Copyright 2024 Redpanda Data, Inc.
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

// Package validators contains general purpose validation functions for the Redpanda provider.
package validators

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// CloudProviders returns a list of cloud providers that the Redpanda provider supports.
func CloudProviders() []validator.String {
	return []validator.String{stringvalidator.OneOf("gcp", "aws", "azure")}
}

// CloudProviderAccessProviders returns the cloud providers a cloud provider
// access can target.
func CloudProviderAccessProviders() []validator.String {
	return []validator.String{stringvalidator.OneOf("aws")}
}

// CloudProviderAccessID validates a cloud provider access ID. An empty string
// would reach the API as no access at all and read back as null.
func CloudProviderAccessID() []validator.String {
	return []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-v0-9]{20}$`), "must be a cloud provider access ID")}
}

// ClusterTypes returns a list of cluster types that the Redpanda provider supports.
func ClusterTypes() []validator.String {
	return []validator.String{stringvalidator.OneOf("dedicated", "byoc")}
}
