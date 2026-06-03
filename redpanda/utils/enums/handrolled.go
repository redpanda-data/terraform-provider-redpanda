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

// Package enums holds the proto enum ↔ TF-string mappers used across the
// provider. The bulk live in enums_gen.go, emitted by cmd/enumgen from a
// proto walk during codegen. This file holds the carve-outs that can't be
// derived mechanically from protoc's `_value` map because their
// user-facing string forms aren't derivable from the proto value names:
//   - CloudProvider: lowercase "aws"/"gcp"/"azure"
//   - Cluster_Type:  lowercase "dedicated"/"byoc"
//
// String→enum mappers are non-strict (return UNSPECIFIED for unknown
// inputs) so the generated conv code can call them in a single-value
// expression. Callers that need explicit error handling for invalid input
// can compare the returned enum against UNSPECIFIED.
//
// Every exported mapper here MUST be listed in
// redpanda/resources/codegen.yaml `enum_carveouts:`. cmd/enumgen
// parity-checks both directions at generation time.
package enums

import (
	"strings"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/genproto/googleapis/type/dayofweek"
)

// CloudProvider* friendly string constants. Used to round-trip between
// user-visible names and the proto's CLOUD_PROVIDER_<X> enum variants.
const (
	CloudProviderStringAws   = "aws"
	CloudProviderStringAzure = "azure"
	CloudProviderStringGcp   = "gcp"

	providerUnspecified = "unspecified"
)

// StringToCloudProvider returns the controlplanev1.CloudProvider enum
// matching the input string. Accepts case-insensitive input. Returns
// UNSPECIFIED for unknown input; callers needing input-validation can
// compare the result.
func StringToCloudProvider(p string) controlplanev1.CloudProvider {
	switch strings.ToLower(p) {
	case CloudProviderStringAws:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS
	case CloudProviderStringGcp:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP
	case CloudProviderStringAzure:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE
	default:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED
	}
}

// StringToCloudProviderBeta is the v1beta2 parallel of StringToCloudProvider.
// Naming: the function name treats `CloudProviderBeta` as the logical enum
// (a separate carve-out entry in codegen.yaml) so the parity check sees
// `<Enum>ToString` / `StringTo<Enum>` symmetry.
func StringToCloudProviderBeta(p string) controlplanev1beta2.CloudProvider {
	switch strings.ToLower(p) {
	case CloudProviderStringAws:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS
	case CloudProviderStringGcp:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP
	case CloudProviderStringAzure:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AZURE
	default:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED
	}
}

// CloudProviderToString returns the user-facing string form of a
// controlplanev1.CloudProvider enum value.
func CloudProviderToString(provider controlplanev1.CloudProvider) string {
	switch provider {
	case controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS:
		return CloudProviderStringAws
	case controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP:
		return CloudProviderStringGcp
	case controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE:
		return CloudProviderStringAzure
	default:
		return providerUnspecified
	}
}

// CloudProviderBetaToString is the v1beta2 parallel of CloudProviderToString.
func CloudProviderBetaToString(provider controlplanev1beta2.CloudProvider) string {
	switch provider {
	case controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS:
		return CloudProviderStringAws
	case controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP:
		return CloudProviderStringGcp
	case controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AZURE:
		return CloudProviderStringAzure
	default:
		return providerUnspecified
	}
}

// StringToClusterType returns the controlplanev1.Cluster_Type enum
// matching the input string. Returns UNSPECIFIED for unknown input.
func StringToClusterType(p string) controlplanev1.Cluster_Type {
	switch strings.ToLower(p) {
	case "dedicated":
		return controlplanev1.Cluster_TYPE_DEDICATED
	case "byoc":
		return controlplanev1.Cluster_TYPE_BYOC
	default:
		return controlplanev1.Cluster_TYPE_UNSPECIFIED
	}
}

// ClusterTypeToString returns the user-facing string form of a
// controlplanev1.Cluster_Type enum value.
func ClusterTypeToString(t controlplanev1.Cluster_Type) string {
	switch t {
	case controlplanev1.Cluster_TYPE_DEDICATED:
		return "dedicated"
	case controlplanev1.Cluster_TYPE_BYOC:
		return "byoc"
	default:
		return providerUnspecified
	}
}

// ClusterConnectionTypeToString returns the lowercase trimmed form
// ("public" / "private" / "vpc_peering"). v1.9.0 customer state files
// store this lowercase form; the default TrimPrefix(e.String(),
// "CONNECTION_TYPE_") emits UPPERCASE which forces replace on upgrade.
func ClusterConnectionTypeToString(e controlplanev1.Cluster_ConnectionType) string {
	return strings.ToLower(strings.TrimPrefix(e.String(), "CONNECTION_TYPE_"))
}

// StringToClusterConnectionType is the reverse direction. Case-insensitive
// input.
func StringToClusterConnectionType(s string) controlplanev1.Cluster_ConnectionType {
	if v, ok := controlplanev1.Cluster_ConnectionType_value["CONNECTION_TYPE_"+strings.ToUpper(s)]; ok {
		return controlplanev1.Cluster_ConnectionType(v)
	}
	return controlplanev1.Cluster_CONNECTION_TYPE_UNSPECIFIED
}

// State-family enums preserve the proto's STATE_<x> form because v1.9.0
// customer state files store the with-prefix UPPERCASE form (STATE_READY,
// STATE_CREATING, etc.). The default trim-prefix mapper would emit
// READY/CREATING which forces churn on upgrade. Each StringTo* lookup
// accepts both prefixed and bare forms by treating bare input as
// STATE_<input>.

// ClusterStateToString preserves the STATE_<x> form.
func ClusterStateToString(e controlplanev1.Cluster_State) string {
	return e.String()
}

// StringToClusterState is the reverse direction. Accepts both STATE_FOO
// and FOO forms.
func StringToClusterState(s string) controlplanev1.Cluster_State {
	if v, ok := controlplanev1.Cluster_State_value[s]; ok {
		return controlplanev1.Cluster_State(v)
	}
	if v, ok := controlplanev1.Cluster_State_value["STATE_"+s]; ok {
		return controlplanev1.Cluster_State(v)
	}
	return controlplanev1.Cluster_STATE_UNSPECIFIED
}

// NetworkStateToString preserves the STATE_<x> form.
func NetworkStateToString(e controlplanev1.Network_State) string {
	return e.String()
}

// StringToNetworkState is the reverse direction.
func StringToNetworkState(s string) controlplanev1.Network_State {
	if v, ok := controlplanev1.Network_State_value[s]; ok {
		return controlplanev1.Network_State(v)
	}
	if v, ok := controlplanev1.Network_State_value["STATE_"+s]; ok {
		return controlplanev1.Network_State(v)
	}
	return controlplanev1.Network_STATE_UNSPECIFIED
}

// NetworkAccessModeToString preserves the NETWORK_ACCESS_MODE_<x> form
// (api_gateway_access state).
func NetworkAccessModeToString(e controlplanev1.NetworkAccessMode) string {
	return e.String()
}

// StringToNetworkAccessMode is the reverse direction.
func StringToNetworkAccessMode(s string) controlplanev1.NetworkAccessMode {
	if v, ok := controlplanev1.NetworkAccessMode_value[s]; ok {
		return controlplanev1.NetworkAccessMode(v)
	}
	if v, ok := controlplanev1.NetworkAccessMode_value["NETWORK_ACCESS_MODE_"+s]; ok {
		return controlplanev1.NetworkAccessMode(v)
	}
	return controlplanev1.NetworkAccessMode_NETWORK_ACCESS_MODE_UNSPECIFIED
}

// ServerlessClusterStateToString preserves the STATE_<x> form.
func ServerlessClusterStateToString(e controlplanev1.ServerlessCluster_State) string {
	return e.String()
}

// StringToServerlessClusterState is the reverse direction.
func StringToServerlessClusterState(s string) controlplanev1.ServerlessCluster_State {
	if v, ok := controlplanev1.ServerlessCluster_State_value[s]; ok {
		return controlplanev1.ServerlessCluster_State(v)
	}
	if v, ok := controlplanev1.ServerlessCluster_State_value["STATE_"+s]; ok {
		return controlplanev1.ServerlessCluster_State(v)
	}
	return controlplanev1.ServerlessCluster_STATE_UNSPECIFIED
}

// ServerlessNetworkingConfigStateToString preserves the STATE_<x> form.
func ServerlessNetworkingConfigStateToString(e controlplanev1.ServerlessNetworkingConfig_State) string {
	return e.String()
}

// StringToServerlessNetworkingConfigState is the reverse direction.
func StringToServerlessNetworkingConfigState(s string) controlplanev1.ServerlessNetworkingConfig_State {
	if v, ok := controlplanev1.ServerlessNetworkingConfig_State_value[s]; ok {
		return controlplanev1.ServerlessNetworkingConfig_State(v)
	}
	if v, ok := controlplanev1.ServerlessNetworkingConfig_State_value["STATE_"+s]; ok {
		return controlplanev1.ServerlessNetworkingConfig_State(v)
	}
	return controlplanev1.ServerlessNetworkingConfig_STATE_UNSPECIFIED
}

// ServerlessPrivateLinkStateToString preserves the STATE_<x> form.
func ServerlessPrivateLinkStateToString(e controlplanev1.ServerlessPrivateLink_State) string {
	return e.String()
}

// StringToServerlessPrivateLinkState is the reverse direction.
func StringToServerlessPrivateLinkState(s string) controlplanev1.ServerlessPrivateLink_State {
	if v, ok := controlplanev1.ServerlessPrivateLink_State_value[s]; ok {
		return controlplanev1.ServerlessPrivateLink_State(v)
	}
	if v, ok := controlplanev1.ServerlessPrivateLink_State_value["STATE_"+s]; ok {
		return controlplanev1.ServerlessPrivateLink_State(v)
	}
	return controlplanev1.ServerlessPrivateLink_STATE_UNSPECIFIED
}

// SASLMechanismToString returns the lowercase-dashed user-facing form
// ("scram-sha-256" / "scram-sha-512"). Proto enum values are
// SASL_MECHANISM_SCRAM_SHA_256 etc.; v1.9.0 state stores the dashed form.
// The default trim-prefix mapper would emit SCRAM_SHA_256.
func SASLMechanismToString(e dataplanev1.SASLMechanism) string {
	return strings.ReplaceAll(
		strings.ToLower(strings.TrimPrefix(e.String(), "SASL_MECHANISM_")),
		"_", "-",
	)
}

// StringToSASLMechanism is the reverse direction. Accepts dashed
// lowercase ("scram-sha-256") or underscored uppercase ("SCRAM_SHA_256").
func StringToSASLMechanism(s string) dataplanev1.SASLMechanism {
	key := "SASL_MECHANISM_" + strings.ReplaceAll(strings.ToUpper(s), "-", "_")
	if v, ok := dataplanev1.SASLMechanism_value[key]; ok {
		return dataplanev1.SASLMechanism(v)
	}
	return dataplanev1.SASLMechanism_SASL_MECHANISM_UNSPECIFIED
}

// DayOfWeek mappers. The proto type is `google.type.DayOfWeek` from
// `google.golang.org/genproto/googleapis/type/dayofweek`. Hand-written
// because the google.type proto package contains multiple enums each in
// a SEPARATE Go package (dayofweek, month, etc.) and enumgen's
// per-proto-package import registry doesn't model that. Manual wrappers
// are cheap; adding more google.type enums = add another carve-out.
//
// google.type.DayOfWeek's values are `DAY_OF_WEEK_UNSPECIFIED`, `MONDAY`,
// `TUESDAY`, …, `SUNDAY` — Google's convention doesn't prefix values
// with the enum name, so .String() and _value[s] round-trip directly
// without TrimPrefix.

// DayOfWeekToString maps a proto enum value to its TF string form.
func DayOfWeekToString(e dayofweek.DayOfWeek) string {
	return e.String()
}

// StringToDayOfWeek maps a TF string back to the proto enum.
// Returns DAY_OF_WEEK_UNSPECIFIED for unknown inputs.
func StringToDayOfWeek(s string) dayofweek.DayOfWeek {
	if v, ok := dayofweek.DayOfWeek_value[s]; ok {
		return dayofweek.DayOfWeek(v)
	}
	return dayofweek.DayOfWeek_DAY_OF_WEEK_UNSPECIFIED
}
