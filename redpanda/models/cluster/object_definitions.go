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

func getStateDescriptionType() map[string]attr.Type {
	return map[string]attr.Type{
		"message": types.StringType,
		"code":    types.Int32Type,
	}
}

func getAwsPrivateLinkType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":            types.BoolType,
		"connect_console":    types.BoolType,
		"allowed_principals": types.ListType{ElemType: types.StringType},
		"status":             types.ObjectType{AttrTypes: getAwsPrivateLinkStatusType()},
	}
}

func getAwsPrivateLinkStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"service_id":                    types.StringType,
		"service_name":                  types.StringType,
		"service_state":                 types.StringType,
		"created_at":                    types.StringType,
		"deleted_at":                    types.StringType,
		"vpc_endpoint_connections":      types.ListType{ElemType: types.ObjectType{AttrTypes: getVpcEndpointConnectionType()}},
		"kafka_api_seed_port":           types.Int32Type,
		"schema_registry_seed_port":     types.Int32Type,
		"redpanda_proxy_seed_port":      types.Int32Type,
		"kafka_api_node_base_port":      types.Int32Type,
		"redpanda_proxy_node_base_port": types.Int32Type,
		"console_port":                  types.Int32Type,
	}
}

func getVpcEndpointConnectionType() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                 types.StringType,
		"owner":              types.StringType,
		"state":              types.StringType,
		"created_at":         types.StringType,
		"connection_id":      types.StringType,
		"load_balancer_arns": types.ListType{ElemType: types.StringType},
		"dns_entries":        types.ListType{ElemType: types.ObjectType{AttrTypes: getDNSEntryType()}},
	}
}

func getDNSEntryType() map[string]attr.Type {
	return map[string]attr.Type{
		"dns_name":       types.StringType,
		"hosted_zone_id": types.StringType,
	}
}

func getGcpPrivateServiceConnectType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":               types.BoolType,
		"global_access_enabled": types.BoolType,
		"consumer_accept_list":  types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}}},
		"status":                types.ObjectType{AttrTypes: getGcpPrivateServiceConnectStatusType()},
	}
}

func getGcpPrivateServiceConnectStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"service_attachment":            types.StringType,
		"created_at":                    types.StringType,
		"deleted_at":                    types.StringType,
		"kafka_api_seed_port":           types.Int32Type,
		"schema_registry_seed_port":     types.Int32Type,
		"redpanda_proxy_seed_port":      types.Int32Type,
		"kafka_api_node_base_port":      types.Int32Type,
		"redpanda_proxy_node_base_port": types.Int32Type,
		"connected_endpoints":           types.ListType{ElemType: types.ObjectType{AttrTypes: getConnectedEndpointType()}},
		"dns_a_records":                 types.ListType{ElemType: types.StringType},
		"seed_hostname":                 types.StringType,
	}
}

func getConnectedEndpointType() map[string]attr.Type {
	return map[string]attr.Type{
		"connection_id":    types.StringType,
		"consumer_network": types.StringType,
		"endpoint":         types.StringType,
		"status":           types.StringType,
	}
}

func getAzurePrivateLinkType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":               types.BoolType,
		"connect_console":       types.BoolType,
		"allowed_subscriptions": types.ListType{ElemType: types.StringType},
		"status":                types.ObjectType{AttrTypes: getAzurePrivateLinkStatusType()},
	}
}

func getAzurePrivateLinkStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"service_id":                    types.StringType,
		"service_name":                  types.StringType,
		"created_at":                    types.StringType,
		"deleted_at":                    types.StringType,
		"private_endpoint_connections":  types.ListType{ElemType: types.ObjectType{AttrTypes: getAzureEndpointConnectionType()}},
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

func getAzureEndpointConnectionType() map[string]attr.Type {
	return map[string]attr.Type{
		"private_endpoint_name": types.StringType,
		"private_endpoint_id":   types.StringType,
		"connection_name":       types.StringType,
		"connection_id":         types.StringType,
		"status":                types.StringType,
		"created_at":            types.StringType,
	}
}

func getKafkaAPIType() map[string]attr.Type {
	return map[string]attr.Type{
		"seed_brokers": types.ListType{ElemType: types.StringType},
		"mtls":         types.ObjectType{AttrTypes: getMtlsType()},
	}
}

func getMtlsType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":                 types.BoolType,
		"ca_certificates_pem":     types.ListType{ElemType: types.StringType},
		"principal_mapping_rules": types.ListType{ElemType: types.StringType},
	}
}

func getHTTPProxyType() map[string]attr.Type {
	return map[string]attr.Type{
		"mtls": types.ObjectType{AttrTypes: getMtlsType()},
		"url":  types.StringType,
	}
}

func getSchemaRegistryType() map[string]attr.Type {
	return map[string]attr.Type{
		"mtls": types.ObjectType{AttrTypes: getMtlsType()},
		"url":  types.StringType,
	}
}

func getKafkaConnectType() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled": types.BoolType,
	}
}

func getCustomerManagedResourcesType() map[string]attr.Type {
	return map[string]attr.Type{
		"aws": types.ObjectType{AttrTypes: getAwsCmrType()},
		"gcp": types.ObjectType{AttrTypes: getGcpCmrType()},
	}
}

func getAwsCmrType() map[string]attr.Type {
	return map[string]attr.Type{
		"agent_instance_profile":                 types.ObjectType{AttrTypes: getArnContainerType()},
		"connectors_node_group_instance_profile": types.ObjectType{AttrTypes: getArnContainerType()},
		"utility_node_group_instance_profile":    types.ObjectType{AttrTypes: getArnContainerType()},
		"redpanda_node_group_instance_profile":   types.ObjectType{AttrTypes: getArnContainerType()},
		"k8s_cluster_role":                       types.ObjectType{AttrTypes: getArnContainerType()},
		"redpanda_agent_security_group":          types.ObjectType{AttrTypes: getArnContainerType()},
		"connectors_security_group":              types.ObjectType{AttrTypes: getArnContainerType()},
		"redpanda_node_group_security_group":     types.ObjectType{AttrTypes: getArnContainerType()},
		"utility_security_group":                 types.ObjectType{AttrTypes: getArnContainerType()},
		"cluster_security_group":                 types.ObjectType{AttrTypes: getArnContainerType()},
		"node_security_group":                    types.ObjectType{AttrTypes: getArnContainerType()},
		"cloud_storage_bucket":                   types.ObjectType{AttrTypes: getArnContainerType()},
		"permissions_boundary_policy":            types.ObjectType{AttrTypes: getArnContainerType()},
	}
}

func getArnContainerType() map[string]attr.Type {
	return map[string]attr.Type{
		"arn": types.StringType,
	}
}

func getGcpCmrType() map[string]attr.Type {
	return map[string]attr.Type{
		"subnet":                           types.ObjectType{AttrTypes: getGcpSubnetType()},
		"agent_service_account":            types.ObjectType{AttrTypes: getGcpServiceAccountType()},
		"console_service_account":          types.ObjectType{AttrTypes: getGcpServiceAccountType()},
		"connector_service_account":        types.ObjectType{AttrTypes: getGcpServiceAccountType()},
		"redpanda_cluster_service_account": types.ObjectType{AttrTypes: getGcpServiceAccountType()},
		"gke_service_account":              types.ObjectType{AttrTypes: getGcpServiceAccountType()},
		"tiered_storage_bucket":            types.ObjectType{AttrTypes: getGcpBucketType()},
		"psc_nat_subnet_name":              types.StringType,
	}
}

func getGcpSubnetType() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                          types.StringType,
		"secondary_ipv4_range_pods":     types.ObjectType{AttrTypes: getGcpSecondaryIPv4RangeType()},
		"secondary_ipv4_range_services": types.ObjectType{AttrTypes: getGcpSecondaryIPv4RangeType()},
		"k8s_master_ipv4_range":         types.StringType,
	}
}

func getGcpSecondaryIPv4RangeType() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
	}
}

func getGcpServiceAccountType() map[string]attr.Type {
	return map[string]attr.Type{
		"email": types.StringType,
	}
}

func getGcpBucketType() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
	}
}

func getPrometheusType() map[string]attr.Type {
	return map[string]attr.Type{
		"url": types.StringType,
	}
}

func getRedpandaConsoleType() map[string]attr.Type {
	return map[string]attr.Type{
		"url": types.StringType,
	}
}

func getMaintenanceWindowConfigType() map[string]attr.Type {
	return map[string]attr.Type{
		"day_hour":    types.ObjectType{AttrTypes: getDayHourType()},
		"anytime":     types.BoolType,
		"unspecified": types.BoolType,
	}
}

func getDayHourType() map[string]attr.Type {
	return map[string]attr.Type{
		"hour_of_day": types.Int32Type,
		"day_of_week": types.StringType,
	}
}

func getAwsCmrNullAttributes() map[string]attr.Value {
	return map[string]attr.Value{
		"agent_instance_profile":                 types.ObjectNull(getArnContainerType()),
		"connectors_node_group_instance_profile": types.ObjectNull(getArnContainerType()),
		"utility_node_group_instance_profile":    types.ObjectNull(getArnContainerType()),
		"redpanda_node_group_instance_profile":   types.ObjectNull(getArnContainerType()),
		"k8s_cluster_role":                       types.ObjectNull(getArnContainerType()),
		"redpanda_agent_security_group":          types.ObjectNull(getArnContainerType()),
		"connectors_security_group":              types.ObjectNull(getArnContainerType()),
		"redpanda_node_group_security_group":     types.ObjectNull(getArnContainerType()),
		"utility_security_group":                 types.ObjectNull(getArnContainerType()),
		"cluster_security_group":                 types.ObjectNull(getArnContainerType()),
		"node_security_group":                    types.ObjectNull(getArnContainerType()),
		"cloud_storage_bucket":                   types.ObjectNull(getArnContainerType()),
		"permissions_boundary_policy":            types.ObjectNull(getArnContainerType()),
	}
}

func getGcpCmrNullAttributes() map[string]attr.Value {
	return map[string]attr.Value{
		"subnet":                           types.ObjectNull(getGcpSubnetType()),
		"agent_service_account":            types.ObjectNull(getGcpServiceAccountType()),
		"console_service_account":          types.ObjectNull(getGcpServiceAccountType()),
		"connector_service_account":        types.ObjectNull(getGcpServiceAccountType()),
		"redpanda_cluster_service_account": types.ObjectNull(getGcpServiceAccountType()),
		"gke_service_account":              types.ObjectNull(getGcpServiceAccountType()),
		"tiered_storage_bucket":            types.ObjectNull(getGcpBucketType()),
		"psc_nat_subnet_name":              types.StringNull(),
	}
}
