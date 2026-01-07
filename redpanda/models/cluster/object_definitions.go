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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GetStateDescriptionType returns the type definition for state description
func GetStateDescriptionType() map[string]attr.Type {
	return map[string]attr.Type{
		"message": types.StringType,
		"code":    types.Int32Type,
	}
}

// GetAwsPrivateLinkType returns the type definition for AWS PrivateLink configuration
func GetAwsPrivateLinkType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":            types.BoolType,
		"connect_console":    types.BoolType,
		"allowed_principals": types.ListType{ElemType: types.StringType},
		"status":             types.ObjectType{AttrTypes: GetAwsPrivateLinkStatusType()},
	}
}

// GetAwsPrivateLinkStatusType returns the type definition for AWS PrivateLink status
func GetAwsPrivateLinkStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"service_id":                    types.StringType,
		"service_name":                  types.StringType,
		"service_state":                 types.StringType,
		"created_at":                    types.StringType,
		"deleted_at":                    types.StringType,
		"vpc_endpoint_connections":      types.ListType{ElemType: types.ObjectType{AttrTypes: GetVpcEndpointConnectionType()}},
		"kafka_api_seed_port":           types.Int32Type,
		"schema_registry_seed_port":     types.Int32Type,
		"redpanda_proxy_seed_port":      types.Int32Type,
		"kafka_api_node_base_port":      types.Int32Type,
		"redpanda_proxy_node_base_port": types.Int32Type,
		"console_port":                  types.Int32Type,
	}
}

// GetVpcEndpointConnectionType returns the type definition for VPC endpoint connection
func GetVpcEndpointConnectionType() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                 types.StringType,
		"owner":              types.StringType,
		"state":              types.StringType,
		"created_at":         types.StringType,
		"connection_id":      types.StringType,
		"load_balancer_arns": types.ListType{ElemType: types.StringType},
		"dns_entries":        types.ListType{ElemType: types.ObjectType{AttrTypes: GetDNSEntryType()}},
	}
}

// GetDNSEntryType returns the type definition for DNS entry
func GetDNSEntryType() map[string]attr.Type {
	return map[string]attr.Type{
		"dns_name":       types.StringType,
		"hosted_zone_id": types.StringType,
	}
}

// GetGcpPrivateServiceConnectType returns the type definition for GCP Private Service Connect configuration
func GetGcpPrivateServiceConnectType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":               types.BoolType,
		"global_access_enabled": types.BoolType,
		"consumer_accept_list":  types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}}},
		"status":                types.ObjectType{AttrTypes: GetGcpPrivateServiceConnectStatusType()},
	}
}

// GetGcpPrivateServiceConnectStatusType returns the type definition for GCP Private Service Connect status
func GetGcpPrivateServiceConnectStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"service_attachment":            types.StringType,
		"created_at":                    types.StringType,
		"deleted_at":                    types.StringType,
		"kafka_api_seed_port":           types.Int32Type,
		"schema_registry_seed_port":     types.Int32Type,
		"redpanda_proxy_seed_port":      types.Int32Type,
		"kafka_api_node_base_port":      types.Int32Type,
		"redpanda_proxy_node_base_port": types.Int32Type,
		"connected_endpoints":           types.ListType{ElemType: types.ObjectType{AttrTypes: GetConnectedEndpointType()}},
		"dns_a_records":                 types.ListType{ElemType: types.StringType},
		"seed_hostname":                 types.StringType,
	}
}

// GetConnectedEndpointType returns the type definition for connected endpoint
func GetConnectedEndpointType() map[string]attr.Type {
	return map[string]attr.Type{
		"connection_id":    types.StringType,
		"consumer_network": types.StringType,
		"endpoint":         types.StringType,
		"status":           types.StringType,
	}
}

// GetAzurePrivateLinkType returns the type definition for Azure Private Link configuration
func GetAzurePrivateLinkType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":               types.BoolType,
		"connect_console":       types.BoolType,
		"allowed_subscriptions": types.ListType{ElemType: types.StringType},
		"status":                types.ObjectType{AttrTypes: GetAzurePrivateLinkStatusType()},
	}
}

// GetAzurePrivateLinkStatusType returns the type definition for Azure Private Link status
func GetAzurePrivateLinkStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"service_id":                    types.StringType,
		"service_name":                  types.StringType,
		"created_at":                    types.StringType,
		"deleted_at":                    types.StringType,
		"private_endpoint_connections":  types.ListType{ElemType: types.ObjectType{AttrTypes: GetAzureEndpointConnectionType()}},
		"dns_a_record":                  types.StringType,
		"approved_subscriptions":        types.ListType{ElemType: types.StringType},
		"kafka_api_seed_port":           types.Int32Type,
		"schema_registry_seed_port":     types.Int32Type,
		"redpanda_proxy_seed_port":      types.Int32Type,
		"kafka_api_node_base_port":      types.Int32Type,
		"redpanda_proxy_node_base_port": types.Int32Type,
		"console_port":                  types.Int32Type,
	}
}

// GetAzureEndpointConnectionType returns the type definition for Azure endpoint connection
func GetAzureEndpointConnectionType() map[string]attr.Type {
	return map[string]attr.Type{
		"private_endpoint_name": types.StringType,
		"private_endpoint_id":   types.StringType,
		"connection_name":       types.StringType,
		"connection_id":         types.StringType,
		"status":                types.StringType,
		"created_at":            types.StringType,
	}
}

// GetKafkaAPIType returns the type definition for Kafka API configuration
func GetKafkaAPIType() map[string]attr.Type {
	return map[string]attr.Type{
		"seed_brokers": types.ListType{ElemType: types.StringType},
		"mtls":         types.ObjectType{AttrTypes: GetMtlsType()},
	}
}

// GetMtlsType returns the type definition for MTLS configuration
func GetMtlsType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":                 types.BoolType,
		"ca_certificates_pem":     types.ListType{ElemType: types.StringType},
		"principal_mapping_rules": types.ListType{ElemType: types.StringType},
	}
}

// GetClusterConfigurationType returns the type definition for cluster configuration
func GetClusterConfigurationType() map[string]attr.Type {
	return map[string]attr.Type{
		"custom_properties_json": types.StringType,
	}
}

// GetHTTPProxyType returns the type definition for HTTP Proxy configuration
func GetHTTPProxyType() map[string]attr.Type {
	return map[string]attr.Type{
		"mtls": types.ObjectType{AttrTypes: GetMtlsType()},
		"url":  types.StringType,
	}
}

// GetSchemaRegistryType returns the type definition for Schema Registry configuration
func GetSchemaRegistryType() map[string]attr.Type {
	return map[string]attr.Type{
		"mtls": types.ObjectType{AttrTypes: GetMtlsType()},
		"url":  types.StringType,
	}
}

// GetKafkaConnectType returns the type definition for Kafka Connect configuration
func GetKafkaConnectType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled": types.BoolType,
	}
}

// GetCustomerManagedResourcesType returns the type definition for customer managed resources configuration
func GetCustomerManagedResourcesType() map[string]attr.Type {
	return map[string]attr.Type{
		"aws": types.ObjectType{AttrTypes: GetAwsCmrType()},
		"gcp": types.ObjectType{AttrTypes: GetGcpCmrType()},
	}
}

// GetAwsCmrType returns the type definition for AWS customer managed resources
func GetAwsCmrType() map[string]attr.Type {
	return map[string]attr.Type{
		"agent_instance_profile":                 types.ObjectType{AttrTypes: GetArnContainerType()},
		"connectors_node_group_instance_profile": types.ObjectType{AttrTypes: GetArnContainerType()},
		"utility_node_group_instance_profile":    types.ObjectType{AttrTypes: GetArnContainerType()},
		"redpanda_node_group_instance_profile":   types.ObjectType{AttrTypes: GetArnContainerType()},
		"k8s_cluster_role":                       types.ObjectType{AttrTypes: GetArnContainerType()},
		"redpanda_agent_security_group":          types.ObjectType{AttrTypes: GetArnContainerType()},
		"connectors_security_group":              types.ObjectType{AttrTypes: GetArnContainerType()},
		"redpanda_node_group_security_group":     types.ObjectType{AttrTypes: GetArnContainerType()},
		"utility_security_group":                 types.ObjectType{AttrTypes: GetArnContainerType()},
		"cluster_security_group":                 types.ObjectType{AttrTypes: GetArnContainerType()},
		"node_security_group":                    types.ObjectType{AttrTypes: GetArnContainerType()},
		"cloud_storage_bucket":                   types.ObjectType{AttrTypes: GetArnContainerType()},
		"permissions_boundary_policy":            types.ObjectType{AttrTypes: GetArnContainerType()},
	}
}

// GetArnContainerType returns the type definition for ARN container
func GetArnContainerType() map[string]attr.Type {
	return map[string]attr.Type{
		"arn": types.StringType,
	}
}

// GetGcpCmrType returns the type definition for GCP customer managed resources
func GetGcpCmrType() map[string]attr.Type {
	return map[string]attr.Type{
		"subnet":                           types.ObjectType{AttrTypes: GetGcpSubnetType()},
		"agent_service_account":            types.ObjectType{AttrTypes: GetGcpServiceAccountType()},
		"console_service_account":          types.ObjectType{AttrTypes: GetGcpServiceAccountType()},
		"connector_service_account":        types.ObjectType{AttrTypes: GetGcpServiceAccountType()},
		"redpanda_cluster_service_account": types.ObjectType{AttrTypes: GetGcpServiceAccountType()},
		"gke_service_account":              types.ObjectType{AttrTypes: GetGcpServiceAccountType()},
		"tiered_storage_bucket":            types.ObjectType{AttrTypes: GetGcpBucketType()},
		"psc_nat_subnet_name":              types.StringType,
	}
}

// GetGcpSubnetType returns the type definition for GCP subnet configuration
func GetGcpSubnetType() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                          types.StringType,
		"secondary_ipv4_range_pods":     types.ObjectType{AttrTypes: GetGcpSecondaryIPv4RangeType()},
		"secondary_ipv4_range_services": types.ObjectType{AttrTypes: GetGcpSecondaryIPv4RangeType()},
		"k8s_master_ipv4_range":         types.StringType,
	}
}

// GetGcpSecondaryIPv4RangeType returns the type definition for GCP secondary IPv4 range
func GetGcpSecondaryIPv4RangeType() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
	}
}

// GetGcpServiceAccountType returns the type definition for GCP service account
func GetGcpServiceAccountType() map[string]attr.Type {
	return map[string]attr.Type{
		"email": types.StringType,
	}
}

// GetGcpBucketType returns the type definition for GCP bucket
func GetGcpBucketType() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
	}
}

// GetPrometheusType returns the type definition for Prometheus configuration
func GetPrometheusType() map[string]attr.Type {
	return map[string]attr.Type{
		"url": types.StringType,
	}
}

// GetRedpandaConsoleType returns the type definition for Redpanda Console configuration
func GetRedpandaConsoleType() map[string]attr.Type {
	return map[string]attr.Type{
		"url": types.StringType,
	}
}

// GetMaintenanceWindowConfigType returns the type definition for maintenance window configuration
func GetMaintenanceWindowConfigType() map[string]attr.Type {
	return map[string]attr.Type{
		"day_hour":    types.ObjectType{AttrTypes: GetDayHourType()},
		"anytime":     types.BoolType,
		"unspecified": types.BoolType,
	}
}

// GetDayHourType returns the type definition for day and hour configuration
func GetDayHourType() map[string]attr.Type {
	return map[string]attr.Type{
		"hour_of_day": types.Int32Type,
		"day_of_week": types.StringType,
	}
}

func getAwsCmrNullAttributes() map[string]attr.Value {
	return map[string]attr.Value{
		"agent_instance_profile":                 types.ObjectNull(GetArnContainerType()),
		"connectors_node_group_instance_profile": types.ObjectNull(GetArnContainerType()),
		"utility_node_group_instance_profile":    types.ObjectNull(GetArnContainerType()),
		"redpanda_node_group_instance_profile":   types.ObjectNull(GetArnContainerType()),
		"k8s_cluster_role":                       types.ObjectNull(GetArnContainerType()),
		"redpanda_agent_security_group":          types.ObjectNull(GetArnContainerType()),
		"connectors_security_group":              types.ObjectNull(GetArnContainerType()),
		"redpanda_node_group_security_group":     types.ObjectNull(GetArnContainerType()),
		"utility_security_group":                 types.ObjectNull(GetArnContainerType()),
		"cluster_security_group":                 types.ObjectNull(GetArnContainerType()),
		"node_security_group":                    types.ObjectNull(GetArnContainerType()),
		"cloud_storage_bucket":                   types.ObjectNull(GetArnContainerType()),
		"permissions_boundary_policy":            types.ObjectNull(GetArnContainerType()),
	}
}

func getGcpCmrNullAttributes() map[string]attr.Value {
	return map[string]attr.Value{
		"subnet":                           types.ObjectNull(GetGcpSubnetType()),
		"agent_service_account":            types.ObjectNull(GetGcpServiceAccountType()),
		"console_service_account":          types.ObjectNull(GetGcpServiceAccountType()),
		"connector_service_account":        types.ObjectNull(GetGcpServiceAccountType()),
		"redpanda_cluster_service_account": types.ObjectNull(GetGcpServiceAccountType()),
		"gke_service_account":              types.ObjectNull(GetGcpServiceAccountType()),
		"tiered_storage_bucket":            types.ObjectNull(GetGcpBucketType()),
		"psc_nat_subnet_name":              types.StringNull(),
	}
}
