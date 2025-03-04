package cluster

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var cmrType = map[string]attr.Type{
	"aws": types.ObjectType{
		AttrTypes: awsType,
	},
	"gcp": types.ObjectType{
		AttrTypes: gcpType,
	},
}

var awsType = map[string]attr.Type{
	"agent_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"connectors_node_group_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"utility_node_group_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_node_group_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"k8s_cluster_role": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_agent_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"connectors_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_node_group_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"utility_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"cluster_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"node_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"cloud_storage_bucket": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"permissions_boundary_policy": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
}

// Define null values for AWS fields
var awsValueDefaults = map[string]attr.Value{
	"agent_instance_profile":                 types.ObjectNull(singleElementContainer),
	"connectors_node_group_instance_profile": types.ObjectNull(singleElementContainer),
	"utility_node_group_instance_profile":    types.ObjectNull(singleElementContainer),
	"redpanda_node_group_instance_profile":   types.ObjectNull(singleElementContainer),
	"k8s_cluster_role":                       types.ObjectNull(singleElementContainer),
	"redpanda_agent_security_group":          types.ObjectNull(singleElementContainer),
	"connectors_security_group":              types.ObjectNull(singleElementContainer),
	"redpanda_node_group_security_group":     types.ObjectNull(singleElementContainer),
	"utility_security_group":                 types.ObjectNull(singleElementContainer),
	"cluster_security_group":                 types.ObjectNull(singleElementContainer),
	"node_security_group":                    types.ObjectNull(singleElementContainer),
	"cloud_storage_bucket":                   types.ObjectNull(singleElementContainer),
	"permissions_boundary_policy":            types.ObjectNull(singleElementContainer),
}

var singleElementContainer = map[string]attr.Type{
	"arn": types.StringType,
}

var mtlsType = map[string]attr.Type{
	"enabled":                 types.BoolType,
	"ca_certificates_pem":     types.ListType{ElemType: types.StringType},
	"principal_mapping_rules": types.ListType{ElemType: types.StringType},
}

var kafkaAPIType = map[string]attr.Type{
	"seed_brokers": types.ListType{ElemType: types.StringType},
	"mtls":         types.ObjectType{AttrTypes: mtlsType},
}

var httpProxyType = map[string]attr.Type{
	"mtls": types.ObjectType{AttrTypes: mtlsType},
	"url":  types.StringType,
}

var redpandaConsoleType = map[string]attr.Type{
	"url": types.StringType,
}

var schemaRegistryType = map[string]attr.Type{
	"mtls": types.ObjectType{AttrTypes: mtlsType},
	"url":  types.StringType,
}

var prometheusType = map[string]attr.Type{
	"url": types.StringType,
}

// DNS Entry type for VPC Endpoint Connections
var dnsEntryType = map[string]attr.Type{
	"dns_name":       types.StringType,
	"hosted_zone_id": types.StringType,
}

// VPC Endpoint Connection type definition
var vpcEndpointConnType = map[string]attr.Type{
	"id":                 types.StringType,
	"owner":              types.StringType,
	"state":              types.StringType,
	"created_at":         types.StringType, // Timestamp converted to string
	"connection_id":      types.StringType,
	"load_balancer_arns": types.ListType{ElemType: types.StringType},
	"dns_entries":        types.ListType{ElemType: types.ObjectType{AttrTypes: dnsEntryType}},
}

// Status object type definition
var statusObjectType = map[string]attr.Type{
	"service_id":                    types.StringType,
	"service_name":                  types.StringType,
	"service_state":                 types.StringType,
	"created_at":                    types.StringType, // Timestamp converted to string
	"deleted_at":                    types.StringType, // Timestamp converted to string
	"vpc_endpoint_connections":      types.ListType{ElemType: types.ObjectType{AttrTypes: vpcEndpointConnType}},
	"kafka_api_seed_port":           types.Int32Type,
	"schema_registry_seed_port":     types.Int32Type,
	"redpanda_proxy_seed_port":      types.Int32Type,
	"kafka_api_node_base_port":      types.Int32Type,
	"redpanda_proxy_node_base_port": types.Int32Type,
	"console_port":                  types.Int32Type,
}

// Updated AWS Private Link type to include status
var awsPrivateLinkType = map[string]attr.Type{
	"enabled":            types.BoolType,
	"connect_console":    types.BoolType,
	"allowed_principals": types.ListType{ElemType: types.StringType},
	"status":             types.ObjectType{AttrTypes: statusObjectType},
}

// Connected Endpoint type for GCP PSC Status
var connectedEndpointType = map[string]attr.Type{
	"connection_id":    types.StringType,
	"consumer_network": types.StringType,
	"endpoint":         types.StringType,
	"status":           types.StringType,
}

// Status object type definition for GCP PSC
var gcpPrivateServiceConnectStatusType = map[string]attr.Type{
	"service_attachment":            types.StringType,
	"created_at":                    types.StringType, // Timestamp as RFC3339 string
	"deleted_at":                    types.StringType, // Timestamp as RFC3339 string
	"kafka_api_seed_port":           types.Int32Type,
	"schema_registry_seed_port":     types.Int32Type,
	"redpanda_proxy_seed_port":      types.Int32Type,
	"kafka_api_node_base_port":      types.Int32Type,
	"redpanda_proxy_node_base_port": types.Int32Type,
	"connected_endpoints":           types.ListType{ElemType: types.ObjectType{AttrTypes: connectedEndpointType}},
	"dns_a_records":                 types.ListType{ElemType: types.StringType},
	"seed_hostname":                 types.StringType,
}

// Update the base type to include status
var gcpPrivateServiceConnectType = map[string]attr.Type{
	"enabled":               types.BoolType,
	"global_access_enabled": types.BoolType,
	"consumer_accept_list":  types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}}},
	"status":                types.ObjectType{AttrTypes: gcpPrivateServiceConnectStatusType},
}

// Azure Private Endpoint Connection type
var azureEndpointConnType = map[string]attr.Type{
	"private_endpoint_name": types.StringType,
	"private_endpoint_id":   types.StringType,
	"connection_name":       types.StringType,
	"connection_id":         types.StringType,
	"status":                types.StringType,
	"created_at":            types.StringType, // Timestamp as RFC3339 string
}

var kafkaConnectType = map[string]attr.Type{
	"enabled": types.BoolType,
}

// Status object type for Azure Private Link
var azurePrivateLinkStatusType = map[string]attr.Type{
	"service_id":                    types.StringType,
	"service_name":                  types.StringType,
	"created_at":                    types.StringType, // Timestamp as RFC3339 string
	"deleted_at":                    types.StringType, // Timestamp as RFC3339 string
	"private_endpoint_connections":  types.ListType{ElemType: types.ObjectType{AttrTypes: azureEndpointConnType}},
	"dns_a_record":                  types.StringType,
	"approved_subscriptions":        types.ListType{ElemType: types.StringType},
	"kafka_api_seed_port":           types.Int32Type,
	"schema_registry_seed_port":     types.Int32Type,
	"redpanda_proxy_seed_port":      types.Int32Type,
	"kafka_api_node_base_port":      types.Int32Type,
	"redpanda_proxy_node_base_port": types.Int32Type,
	"console_port":                  types.Int32Type,
}

// Base Azure Private Link type
var azurePrivateLinkType = map[string]attr.Type{
	"enabled":               types.BoolType,
	"connect_console":       types.BoolType,
	"allowed_subscriptions": types.ListType{ElemType: types.StringType},
	"status":                types.ObjectType{AttrTypes: azurePrivateLinkStatusType},
}

var dayHourType = map[string]attr.Type{
	"hour_of_day": types.Int32Type,
	"day_of_week": types.StringType,
}

var maintenanceWindowConfigType = map[string]attr.Type{
	"day_hour":    types.ObjectType{AttrTypes: dayHourType},
	"anytime":     types.BoolType,
	"unspecified": types.BoolType,
}

var connectivityGCPType = map[string]attr.Type{
	"enable_global_access": types.BoolType,
}

var connectivityType = map[string]attr.Type{
	"gcp": types.ObjectType{AttrTypes: connectivityGCPType},
}

var stateDescriptionType = map[string]attr.Type{
	"message": types.StringType,
	"code":    types.Int32Type,
}

// GCP customer managed resources type definitions
var gcpType = map[string]attr.Type{
	"subnet": types.ObjectType{
		AttrTypes: gcpSubnetType,
	},
	"agent_service_account": types.ObjectType{
		AttrTypes: gcpServiceAccountType,
	},
	"console_service_account": types.ObjectType{
		AttrTypes: gcpServiceAccountType,
	},
	"connector_service_account": types.ObjectType{
		AttrTypes: gcpServiceAccountType,
	},
	"redpanda_cluster_service_account": types.ObjectType{
		AttrTypes: gcpServiceAccountType,
	},
	"gke_service_account": types.ObjectType{
		AttrTypes: gcpServiceAccountType,
	},
	"tiered_storage_bucket": types.ObjectType{
		AttrTypes: gcpBucketType,
	},
	"psc_nat_subnet_name": types.StringType,
}

// Subnet definition
var gcpSubnetType = map[string]attr.Type{
	"name": types.StringType,
	"secondary_ipv4_range_pods": types.ObjectType{
		AttrTypes: gcpSecondaryIPv4RangeType,
	},
	"secondary_ipv4_range_services": types.ObjectType{
		AttrTypes: gcpSecondaryIPv4RangeType,
	},
	"k8s_master_ipv4_range": types.StringType,
}

// Secondary IPv4 range definition
var gcpSecondaryIPv4RangeType = map[string]attr.Type{
	"name": types.StringType,
}

// Service account definition
var gcpServiceAccountType = map[string]attr.Type{
	"email": types.StringType,
}

// Storage bucket definition
var gcpBucketType = map[string]attr.Type{
	"name": types.StringType,
}

// Define null values for GCP fields
var gcpValueDefaults = map[string]attr.Value{
	"subnet":                           types.ObjectNull(gcpSubnetType),
	"agent_service_account":            types.ObjectNull(gcpServiceAccountType),
	"console_service_account":          types.ObjectNull(gcpServiceAccountType),
	"connector_service_account":        types.ObjectNull(gcpServiceAccountType),
	"redpanda_cluster_service_account": types.ObjectNull(gcpServiceAccountType),
	"gke_service_account":              types.ObjectNull(gcpServiceAccountType),
	"tiered_storage_bucket":            types.ObjectNull(gcpBucketType),
	"psc_nat_subnet_name":              types.StringNull(),
}
