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

// Package cluster contains cluster datasource models.
package cluster

import (
	"context"
	"encoding/json"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// DataModel represents cluster datasource schema.
type DataModel struct {
	Name                     types.String        `tfsdk:"name"`
	ID                       types.String        `tfsdk:"id"`
	ConnectionType           types.String        `tfsdk:"connection_type"`
	CloudProvider            types.String        `tfsdk:"cloud_provider"`
	ClusterType              types.String        `tfsdk:"cluster_type"`
	RedpandaVersion          types.String        `tfsdk:"redpanda_version"`
	ThroughputTier           types.String        `tfsdk:"throughput_tier"`
	Region                   types.String        `tfsdk:"region"`
	Zones                    types.List          `tfsdk:"zones"`
	AllowDeletion            types.Bool          `tfsdk:"allow_deletion"`
	CreatedAt                types.String        `tfsdk:"created_at"`
	State                    types.String        `tfsdk:"state"`
	StateDescription         types.Object        `tfsdk:"state_description"`
	Tags                     types.Map           `tfsdk:"tags"`
	ResourceGroupID          types.String        `tfsdk:"resource_group_id"`
	NetworkID                types.String        `tfsdk:"network_id"`
	ClusterAPIURL            types.String        `tfsdk:"cluster_api_url"`
	AwsPrivateLink           types.Object        `tfsdk:"aws_private_link"`
	GcpPrivateServiceConnect types.Object        `tfsdk:"gcp_private_service_connect"`
	AzurePrivateLink         types.Object        `tfsdk:"azure_private_link"`
	KafkaAPI                 types.Object        `tfsdk:"kafka_api"`
	HTTPProxy                types.Object        `tfsdk:"http_proxy"`
	SchemaRegistry           types.Object        `tfsdk:"schema_registry"`
	KafkaConnect             types.Object        `tfsdk:"kafka_connect"`
	ReadReplicaClusterIDs    types.List          `tfsdk:"read_replica_cluster_ids"`
	CustomerManagedResources types.Object        `tfsdk:"customer_managed_resources"`
	Prometheus               types.Object        `tfsdk:"prometheus"`
	RedpandaConsole          types.Object        `tfsdk:"redpanda_console"`
	MaintenanceWindowConfig  types.Object        `tfsdk:"maintenance_window_config"`
	GCPGlobalAccessEnabled   basetypes.BoolValue `tfsdk:"gcp_global_access_enabled"`
	ClusterConfiguration     types.Object        `tfsdk:"cluster_configuration"`
	Timeouts                 timeouts.Value      `tfsdk:"timeouts"`
}

// GetID returns the ID
func (r *DataModel) GetID() string {
	return r.ID.ValueString()
}

// GetUpdatedModel populates the DataModel from a protobuf cluster response
func (r *DataModel) GetUpdatedModel(_ context.Context, cluster *controlplanev1.Cluster, contingent ContingentFields) (*DataModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	r.Name = types.StringValue(cluster.GetName())
	r.ID = types.StringValue(cluster.GetId())
	r.ConnectionType = types.StringValue(utils.ConnectionTypeToString(cluster.GetConnectionType()))
	r.CloudProvider = types.StringValue(utils.CloudProviderToString(cluster.GetCloudProvider()))
	r.ClusterType = types.StringValue(utils.ClusterTypeToString(cluster.GetType()))
	r.ThroughputTier = types.StringValue(cluster.GetThroughputTier())
	r.Region = types.StringValue(cluster.GetRegion())
	r.ResourceGroupID = types.StringValue(cluster.GetResourceGroupId())
	r.NetworkID = types.StringValue(cluster.GetNetworkId())
	r.State = types.StringValue(cluster.GetState().String())

	r.Zones = utils.StringSliceToTypeList(cluster.GetZones())
	r.ReadReplicaClusterIDs = utils.StringSliceToTypeList(cluster.GetReadReplicaClusterIds())

	r.RedpandaVersion = contingent.RedpandaVersion
	r.AllowDeletion = contingent.AllowDeletion
	r.Tags = contingent.Tags

	if cluster.HasGcpGlobalAccessEnabled() && utils.CloudProviderToString(cluster.CloudProvider) == utils.CloudProviderStringGcp {
		switch {
		case cluster.GetGcpGlobalAccessEnabled() || contingent.GcpGlobalAccessConfig.ValueBool():
			r.GCPGlobalAccessEnabled = types.BoolValue(true)
		case !cluster.GetGcpGlobalAccessEnabled() && !contingent.GcpGlobalAccessConfig.IsNull():
			r.GCPGlobalAccessEnabled = types.BoolValue(false)
		default:
			r.GCPGlobalAccessEnabled = types.BoolNull()
		}
	} else {
		r.GCPGlobalAccessEnabled = types.BoolNull()
	}
	if cluster.GetCreatedAt() != nil {
		r.CreatedAt = types.StringValue(cluster.GetCreatedAt().AsTime().Format(time.RFC3339))
	}
	if cluster.HasDataplaneApi() {
		r.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}
	if stateDesc, d := r.generateModelStateDescription(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.StateDescription = stateDesc
	}

	if kafkaAPI, d := r.generateModelKafkaAPI(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.KafkaAPI = kafkaAPI
	}

	if awsPrivateLink, d := r.generateModelAWSPrivateLink(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.AwsPrivateLink = awsPrivateLink
	}

	if gcpPSC, d := r.generateModelGCPPrivateServiceConnect(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.GcpPrivateServiceConnect = gcpPSC
	}

	if azurePrivateLink, d := r.generateModelAzurePrivateLink(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.AzurePrivateLink = azurePrivateLink
	}

	if httpProxy, d := r.generateModelHTTPProxy(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.HTTPProxy = httpProxy
	}

	if schemaRegistry, d := r.generateModelSchemaRegistry(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.SchemaRegistry = schemaRegistry
	}

	if kafkaConnect, d := r.generateModelKafkaConnect(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.KafkaConnect = kafkaConnect
	}

	if prometheus, d := r.generateModelPrometheus(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.Prometheus = prometheus
	}

	if redpandaConsole, d := r.generateModelRedpandaConsole(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.RedpandaConsole = redpandaConsole
	}

	if maintenanceWindow, d := r.generateModelMaintenanceWindow(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.MaintenanceWindowConfig = maintenanceWindow
	}

	if cmr, d := r.generateModelCustomerManagedResources(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.CustomerManagedResources = cmr
	}

	if clusterConfiguration, d := r.generateModelClusterConfiguration(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.ClusterConfiguration = clusterConfiguration
	}

	return r, diags
}

// ContingentFields represents fields that can come from either the model or API
type ContingentFields struct {
	RedpandaVersion       types.String
	AllowDeletion         types.Bool
	Tags                  types.Map
	GcpGlobalAccessConfig types.Bool
}

func (*DataModel) generateModelStateDescription(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasStateDescription() {
		return types.ObjectNull(GetStateDescriptionType()), diags
	}
	sd := cluster.GetStateDescription()
	obj, d := types.ObjectValue(GetStateDescriptionType(), map[string]attr.Value{
		"message": types.StringValue(sd.GetMessage()),
		"code":    types.Int32Value(sd.GetCode()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate state description object", "could not create state description object")
		return types.ObjectNull(GetStateDescriptionType()), diags
	}
	return obj, diags
}

func (*DataModel) generateModelKafkaConnect(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasKafkaConnect() {
		return types.ObjectNull(GetKafkaConnectType()), diags
	}

	kafkaConnect := cluster.GetKafkaConnect()
	if !kafkaConnect.GetEnabled() {
		return types.ObjectNull(GetKafkaConnectType()), diags
	}

	obj, d := types.ObjectValue(GetKafkaConnectType(), map[string]attr.Value{
		"enabled": types.BoolValue(kafkaConnect.GetEnabled()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Kafka Connect object", "could not create Kafka Connect object")
		return types.ObjectNull(GetKafkaConnectType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelPrometheus(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasPrometheus() {
		return types.ObjectNull(GetPrometheusType()), diags
	}

	prometheus := cluster.GetPrometheus()

	obj, d := types.ObjectValue(GetPrometheusType(), map[string]attr.Value{
		"url": types.StringValue(prometheus.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Prometheus object", "could not create Prometheus object")
		return types.ObjectNull(GetPrometheusType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelRedpandaConsole(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasRedpandaConsole() {
		return types.ObjectNull(GetRedpandaConsoleType()), diags
	}

	console := cluster.GetRedpandaConsole()

	obj, d := types.ObjectValue(GetRedpandaConsoleType(), map[string]attr.Value{
		"url": types.StringValue(console.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Redpanda Console object", "could not create Redpanda Console object")
		return types.ObjectNull(GetRedpandaConsoleType()), diags
	}

	return obj, diags
}

func (r *DataModel) generateModelSchemaRegistry(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasSchemaRegistry() {
		return types.ObjectNull(GetSchemaRegistryType()), diags
	}

	schemaRegistry := cluster.GetSchemaRegistry()

	mtls, d := r.generateModelMtls(schemaRegistry.GetMtls())
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(GetSchemaRegistryType()), diags
	}

	obj, d := types.ObjectValue(GetSchemaRegistryType(), map[string]attr.Value{
		"mtls": mtls,
		"url":  types.StringValue(schemaRegistry.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Schema Registry object", "could not create Schema Registry object")
		return types.ObjectNull(GetSchemaRegistryType()), diags
	}

	return obj, diags
}

func (r *DataModel) generateModelHTTPProxy(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasHttpProxy() {
		return types.ObjectNull(GetHTTPProxyType()), diags
	}

	httpProxy := cluster.GetHttpProxy()

	mtls, d := r.generateModelMtls(httpProxy.GetMtls())
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(GetHTTPProxyType()), diags
	}

	obj, d := types.ObjectValue(GetHTTPProxyType(), map[string]attr.Value{
		"mtls": mtls,
		"url":  types.StringValue(httpProxy.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate HTTP Proxy object", "could not create HTTP Proxy object")
		return types.ObjectNull(GetHTTPProxyType()), diags
	}

	return obj, diags
}

func (r *DataModel) generateModelKafkaAPI(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasKafkaApi() {
		return types.ObjectNull(GetKafkaAPIType()), diags
	}

	kafkaAPI := cluster.GetKafkaApi()

	mtls, d := r.generateModelMtls(kafkaAPI.GetMtls())
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(GetKafkaAPIType()), diags
	}

	obj, d := types.ObjectValue(GetKafkaAPIType(), map[string]attr.Value{
		"seed_brokers": utils.StringSliceToTypeList(kafkaAPI.GetSeedBrokers()),
		"mtls":         mtls,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Kafka API object", "could not create Kafka API object")
		return types.ObjectNull(GetKafkaAPIType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelMtls(mtls *controlplanev1.MTLSSpec) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if mtls == nil {
		return types.ObjectNull(GetMtlsType()), diags
	}

	obj, d := types.ObjectValue(GetMtlsType(), map[string]attr.Value{
		"enabled":                 types.BoolValue(mtls.GetEnabled()),
		"ca_certificates_pem":     utils.StringSliceToTypeList(mtls.GetCaCertificatesPem()),
		"principal_mapping_rules": utils.StringSliceToTypeList(mtls.GetPrincipalMappingRules()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate MTLS object", "could not create MTLS object")
		return types.ObjectNull(GetMtlsType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelMaintenanceWindow(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasMaintenanceWindowConfig() {
		return types.ObjectNull(GetMaintenanceWindowConfigType()), diags
	}

	maintenance := cluster.GetMaintenanceWindowConfig()

	windowObj := types.ObjectNull(GetDayHourType())
	unspec := types.BoolNull()
	anytime := types.BoolNull()

	if !maintenance.HasWindow() {
		return types.ObjectNull(GetMaintenanceWindowConfigType()), diags
	}

	switch {
	case maintenance.HasDayHour():
		w := maintenance.GetDayHour()
		obj, d := types.ObjectValue(GetDayHourType(), map[string]attr.Value{
			"hour_of_day": types.Int32Value(w.GetHourOfDay()),
			"day_of_week": types.StringValue(w.GetDayOfWeek().String()),
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate maintenance window detail", "could not create window object")
			return types.ObjectNull(GetMaintenanceWindowConfigType()), diags
		}
		windowObj = obj
	case maintenance.HasAnytime():
		anytime = types.BoolValue(true)
	case maintenance.HasUnspecified():
		unspec = types.BoolValue(true)
	}

	obj, d := types.ObjectValue(GetMaintenanceWindowConfigType(), map[string]attr.Value{
		"day_hour":    windowObj,
		"anytime":     anytime,
		"unspecified": unspec,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Maintenance Window object", "could not create Maintenance Window object")
		return types.ObjectNull(GetMaintenanceWindowConfigType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelAWSPrivateLink(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasAwsPrivateLink() {
		return types.ObjectNull(GetAwsPrivateLinkType()), diags
	}

	awsPrivateLink := cluster.GetAwsPrivateLink()
	if !awsPrivateLink.GetEnabled() {
		return types.ObjectNull(GetAwsPrivateLinkType()), diags
	}

	var allowedPrincipals types.List
	if ap := awsPrivateLink.GetAllowedPrincipals(); ap != nil {
		allowedPrincipals = utils.StringSliceToTypeList(ap)
	} else {
		allowedPrincipals = types.ListNull(types.StringType)
	}

	status := awsPrivateLink.GetStatus()
	var statusObj types.Object
	if status != nil {
		var vpcEndpointConns []attr.Value
		for _, conn := range status.GetVpcEndpointConnections() {
			var dnsEntries []attr.Value
			for _, dns := range conn.GetDnsEntries() {
				dnsEntry, d := types.ObjectValue(GetDNSEntryType(), map[string]attr.Value{
					"dns_name":       types.StringValue(dns.GetDnsName()),
					"hosted_zone_id": types.StringValue(dns.GetHostedZoneId()),
				})
				if d.HasError() {
					diags.Append(d...)
					diags.AddError("failed to generate DNS entry", "could not create DNS entry object")
					return types.ObjectNull(GetAwsPrivateLinkType()), diags
				}
				dnsEntries = append(dnsEntries, dnsEntry)
			}

			dnsEntriesList, d := types.ListValue(
				types.ObjectType{AttrTypes: GetDNSEntryType()},
				dnsEntries,
			)
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate DNS entries list", "could not create DNS entries list")
				return types.ObjectNull(GetAwsPrivateLinkType()), diags
			}

			connObj, d := types.ObjectValue(GetVpcEndpointConnectionType(), map[string]attr.Value{
				"id":                 types.StringValue(conn.GetId()),
				"owner":              types.StringValue(conn.GetOwner()),
				"state":              types.StringValue(conn.GetState()),
				"created_at":         types.StringValue(conn.GetCreatedAt().AsTime().Format(time.RFC3339)),
				"connection_id":      types.StringValue(conn.GetConnectionId()),
				"load_balancer_arns": utils.StringSliceToTypeList(conn.GetLoadBalancerArns()),
				"dns_entries":        dnsEntriesList,
			})
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate VPC endpoint connection", "could not create VPC endpoint connection object")
				return types.ObjectNull(GetAwsPrivateLinkType()), diags
			}
			vpcEndpointConns = append(vpcEndpointConns, connObj)
		}

		vpcConnsList, d := types.ListValue(
			types.ObjectType{AttrTypes: GetVpcEndpointConnectionType()},
			vpcEndpointConns,
		)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate VPC endpoint connections list", "could not create VPC endpoint connections list")
			return types.ObjectNull(GetAwsPrivateLinkType()), diags
		}

		statusObj, d = types.ObjectValue(GetAwsPrivateLinkStatusType(), map[string]attr.Value{
			"service_id":                    types.StringValue(status.GetServiceId()),
			"service_name":                  types.StringValue(status.GetServiceName()),
			"service_state":                 types.StringValue(status.GetServiceState()),
			"created_at":                    types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339)),
			"deleted_at":                    types.StringValue(status.GetDeletedAt().AsTime().Format(time.RFC3339)),
			"vpc_endpoint_connections":      vpcConnsList,
			"kafka_api_seed_port":           types.Int32Value(status.GetKafkaApiSeedPort()),
			"schema_registry_seed_port":     types.Int32Value(status.GetSchemaRegistrySeedPort()),
			"redpanda_proxy_seed_port":      types.Int32Value(status.GetRedpandaProxySeedPort()),
			"kafka_api_node_base_port":      types.Int32Value(status.GetKafkaApiNodeBasePort()),
			"redpanda_proxy_node_base_port": types.Int32Value(status.GetRedpandaProxyNodeBasePort()),
			"console_port":                  types.Int32Value(status.GetConsolePort()),
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate AWS Private Link status", "could not create status object")
			return types.ObjectNull(GetAwsPrivateLinkType()), diags
		}
	} else {
		statusObj = types.ObjectNull(GetAwsPrivateLinkStatusType())
	}

	obj, d := types.ObjectValue(GetAwsPrivateLinkType(), map[string]attr.Value{
		"enabled":            types.BoolValue(awsPrivateLink.GetEnabled()),
		"connect_console":    types.BoolValue(awsPrivateLink.GetConnectConsole()),
		"allowed_principals": allowedPrincipals,
		"status":             statusObj,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate AWS Private Link object", "could not create AWS Private Link object")
		return types.ObjectNull(GetAwsPrivateLinkType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelGCPPrivateServiceConnect(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasGcpPrivateServiceConnect() {
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}

	gcpPSC := cluster.GetGcpPrivateServiceConnect()
	if !gcpPSC.GetEnabled() {
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}

	var consumerAcceptList []attr.Value
	for _, consumer := range gcpPSC.GetConsumerAcceptList() {
		consumerObj, d := types.ObjectValue(
			map[string]attr.Type{"source": types.StringType},
			map[string]attr.Value{"source": types.StringValue(consumer.GetSource())},
		)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate consumer accept list entry", "could not create consumer object")
			return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
		}
		consumerAcceptList = append(consumerAcceptList, consumerObj)
	}

	consumerList, d := types.ListValue(
		types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
		consumerAcceptList,
	)
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate consumer accept list", "could not create consumer accept list")
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}

	status := gcpPSC.GetStatus()
	var statusObj types.Object
	if status != nil {
		var connectedEndpoints []attr.Value
		for _, endpoint := range status.GetConnectedEndpoints() {
			endpointObj, d := types.ObjectValue(GetConnectedEndpointType(), map[string]attr.Value{
				"connection_id":    types.StringValue(endpoint.GetConnectionId()),
				"consumer_network": types.StringValue(endpoint.GetConsumerNetwork()),
				"endpoint":         types.StringValue(endpoint.GetEndpoint()),
				"status":           types.StringValue(endpoint.GetStatus()),
			})
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate connected endpoint", "could not create connected endpoint object")
				return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
			}
			connectedEndpoints = append(connectedEndpoints, endpointObj)
		}

		connectedEndpointsList, d := types.ListValue(
			types.ObjectType{AttrTypes: GetConnectedEndpointType()},
			connectedEndpoints,
		)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate connected endpoints list", "could not create connected endpoints list")
			return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
		}

		statusObj, d = types.ObjectValue(GetGcpPrivateServiceConnectStatusType(), map[string]attr.Value{
			"service_attachment":            types.StringValue(status.GetServiceAttachment()),
			"created_at":                    types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339)),
			"deleted_at":                    types.StringValue(status.GetDeletedAt().AsTime().Format(time.RFC3339)),
			"kafka_api_seed_port":           types.Int32Value(status.GetKafkaApiSeedPort()),
			"schema_registry_seed_port":     types.Int32Value(status.GetSchemaRegistrySeedPort()),
			"redpanda_proxy_seed_port":      types.Int32Value(status.GetRedpandaProxySeedPort()),
			"kafka_api_node_base_port":      types.Int32Value(status.GetKafkaApiNodeBasePort()),
			"redpanda_proxy_node_base_port": types.Int32Value(status.GetRedpandaProxyNodeBasePort()),
			"connected_endpoints":           connectedEndpointsList,
			"dns_a_records":                 utils.StringSliceToTypeList(status.GetDnsARecords()),
			"seed_hostname":                 types.StringValue(status.GetSeedHostname()),
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate GCP PSC status", "could not create status object")
			return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
		}
	} else {
		statusObj = types.ObjectNull(GetGcpPrivateServiceConnectStatusType())
	}

	obj, d := types.ObjectValue(GetGcpPrivateServiceConnectType(), map[string]attr.Value{
		"enabled":               types.BoolValue(gcpPSC.GetEnabled()),
		"global_access_enabled": types.BoolValue(gcpPSC.GetGlobalAccessEnabled()),
		"consumer_accept_list":  consumerList,
		"status":                statusObj,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate GCP Private Service Connect object", "could not create GCP PSC object")
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}

	return obj, diags
}

func (*DataModel) generateModelAzurePrivateLink(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasAzurePrivateLink() {
		return types.ObjectNull(GetAzurePrivateLinkType()), diags
	}

	azurePrivateLink := cluster.GetAzurePrivateLink()
	if !azurePrivateLink.GetEnabled() {
		return types.ObjectNull(GetAzurePrivateLinkType()), diags
	}

	var allowedSubscriptions types.List
	if as := azurePrivateLink.GetAllowedSubscriptions(); as != nil {
		allowedSubscriptions = utils.StringSliceToTypeList(as)
	} else {
		allowedSubscriptions = types.ListNull(types.StringType)
	}

	status := azurePrivateLink.GetStatus()
	var statusObj types.Object
	if status != nil {
		var privateEndpointConns []attr.Value
		for _, conn := range status.GetPrivateEndpointConnections() {
			connObj, d := types.ObjectValue(GetAzureEndpointConnectionType(), map[string]attr.Value{
				"private_endpoint_name": types.StringValue(conn.GetPrivateEndpointName()),
				"private_endpoint_id":   types.StringValue(conn.GetPrivateEndpointId()),
				"connection_name":       types.StringValue(conn.GetConnectionName()),
				"connection_id":         types.StringValue(conn.GetConnectionId()),
				"status":                types.StringValue(conn.GetStatus()),
				"created_at":            types.StringValue(conn.GetCreatedAt().AsTime().Format(time.RFC3339)),
			})
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate private endpoint connection", "could not create private endpoint connection object")
				return types.ObjectNull(GetAzurePrivateLinkType()), diags
			}
			privateEndpointConns = append(privateEndpointConns, connObj)
		}

		privateEndpointConnsList, d := types.ListValue(
			types.ObjectType{AttrTypes: GetAzureEndpointConnectionType()},
			privateEndpointConns,
		)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate private endpoint connections list", "could not create private endpoint connections list")
			return types.ObjectNull(GetAzurePrivateLinkType()), diags
		}

		statusObj, d = types.ObjectValue(GetAzurePrivateLinkStatusType(), map[string]attr.Value{
			"service_id":                    types.StringValue(status.GetServiceId()),
			"service_name":                  types.StringValue(status.GetServiceName()),
			"created_at":                    types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339)),
			"deleted_at":                    types.StringValue(status.GetDeletedAt().AsTime().Format(time.RFC3339)),
			"private_endpoint_connections":  privateEndpointConnsList,
			"dns_a_record":                  types.StringValue(status.GetDnsARecord()),
			"approved_subscriptions":        utils.StringSliceToTypeList(status.GetApprovedSubscriptions()),
			"kafka_api_seed_port":           types.Int32Value(status.GetKafkaApiSeedPort()),
			"schema_registry_seed_port":     types.Int32Value(status.GetSchemaRegistrySeedPort()),
			"redpanda_proxy_seed_port":      types.Int32Value(status.GetRedpandaProxySeedPort()),
			"kafka_api_node_base_port":      types.Int32Value(status.GetKafkaApiNodeBasePort()),
			"redpanda_proxy_node_base_port": types.Int32Value(status.GetRedpandaProxyNodeBasePort()),
			"console_port":                  types.Int32Value(status.GetConsolePort()),
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate Azure Private Link status", "could not create status object")
			return types.ObjectNull(GetAzurePrivateLinkType()), diags
		}
	} else {
		statusObj = types.ObjectNull(GetAzurePrivateLinkStatusType())
	}

	obj, d := types.ObjectValue(GetAzurePrivateLinkType(), map[string]attr.Value{
		"enabled":               types.BoolValue(azurePrivateLink.GetEnabled()),
		"connect_console":       types.BoolValue(azurePrivateLink.GetConnectConsole()),
		"allowed_subscriptions": allowedSubscriptions,
		"status":                statusObj,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Azure Private Link object", "could not create Azure Private Link object")
		return types.ObjectNull(GetAzurePrivateLinkType()), diags
	}

	return obj, diags
}

func (r *DataModel) generateModelCustomerManagedResources(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	if cluster == nil || !cluster.HasCustomerManagedResources() {
		return types.ObjectNull(GetCustomerManagedResourcesType()), diags
	}

	cloudProvider := utils.CloudProviderToString(cluster.GetCloudProvider())

	if cluster.Type != controlplanev1.Cluster_TYPE_BYOC {
		diags.AddError("Customer Managed Resources with non-BYOC cluster type", "Customer Managed Resources are only supported for BYOC clusters")
		return types.ObjectNull(GetCustomerManagedResourcesType()), diags
	}

	cmr := cluster.GetCustomerManagedResources()
	if cmr == nil {
		return types.ObjectNull(GetCustomerManagedResourcesType()), diags
	}

	switch cloudProvider {
	case utils.CloudProviderStringAws:
		if !cmr.HasAws() {
			diags.AddError("AWS Cloud Provider Mismatch", "Expected AWS CMR but not found")
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}

		aws, d := r.generateModelAWSCMR(cmr.GetAws())
		if d.HasError() {
			diags.AddError("failed to generate AWS CMR object", "could not create AWS CMR object")
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}

		cmrObj, d := types.ObjectValue(GetCustomerManagedResourcesType(), map[string]attr.Value{
			"aws": aws,
			"gcp": types.ObjectNull(GetGcpCmrType()),
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}
		return cmrObj, diags

	case utils.CloudProviderStringGcp:
		if !cmr.HasGcp() {
			diags.AddError("GCP Cloud Provider Mismatch", "Expected GCP CMR but not found")
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}

		gcp, d := r.generateModelGCPCMR(cmr.GetGcp())
		if d.HasError() {
			diags.AddError("failed to generate GCP CMR object", "could not create GCP CMR object")
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}

		cmrObj, d := types.ObjectValue(GetCustomerManagedResourcesType(), map[string]attr.Value{
			"aws": types.ObjectNull(GetAwsCmrType()),
			"gcp": gcp,
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}
		return cmrObj, diags
	}
	return types.ObjectNull(GetCustomerManagedResourcesType()), diags
}

func (*DataModel) generateModelAWSCMR(awsData *controlplanev1.CustomerManagedResources_AWS) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	awsVal := map[string]attr.Value{
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

	createArnObject := func(arn string) types.Object {
		obj, _ := types.ObjectValue(GetArnContainerType(), map[string]attr.Value{
			"arn": types.StringValue(arn),
		})
		return obj
	}

	if awsData.HasAgentInstanceProfile() {
		awsVal["agent_instance_profile"] = createArnObject(awsData.GetAgentInstanceProfile().GetArn())
	}
	if awsData.HasConnectorsNodeGroupInstanceProfile() {
		awsVal["connectors_node_group_instance_profile"] = createArnObject(awsData.GetConnectorsNodeGroupInstanceProfile().GetArn())
	}
	if awsData.HasUtilityNodeGroupInstanceProfile() {
		awsVal["utility_node_group_instance_profile"] = createArnObject(awsData.GetUtilityNodeGroupInstanceProfile().GetArn())
	}
	if awsData.HasRedpandaNodeGroupInstanceProfile() {
		awsVal["redpanda_node_group_instance_profile"] = createArnObject(awsData.GetRedpandaNodeGroupInstanceProfile().GetArn())
	}
	if awsData.HasK8SClusterRole() {
		awsVal["k8s_cluster_role"] = createArnObject(awsData.GetK8SClusterRole().GetArn())
	}
	if awsData.HasRedpandaAgentSecurityGroup() {
		awsVal["redpanda_agent_security_group"] = createArnObject(awsData.GetRedpandaAgentSecurityGroup().GetArn())
	}
	if awsData.HasConnectorsSecurityGroup() {
		awsVal["connectors_security_group"] = createArnObject(awsData.GetConnectorsSecurityGroup().GetArn())
	}
	if awsData.HasRedpandaNodeGroupSecurityGroup() {
		awsVal["redpanda_node_group_security_group"] = createArnObject(awsData.GetRedpandaNodeGroupSecurityGroup().GetArn())
	}
	if awsData.HasUtilitySecurityGroup() {
		awsVal["utility_security_group"] = createArnObject(awsData.GetUtilitySecurityGroup().GetArn())
	}
	if awsData.HasClusterSecurityGroup() {
		awsVal["cluster_security_group"] = createArnObject(awsData.GetClusterSecurityGroup().GetArn())
	}
	if awsData.HasNodeSecurityGroup() {
		awsVal["node_security_group"] = createArnObject(awsData.GetNodeSecurityGroup().GetArn())
	}
	if awsData.HasCloudStorageBucket() {
		awsVal["cloud_storage_bucket"] = createArnObject(awsData.GetCloudStorageBucket().GetArn())
	}
	if awsData.HasPermissionsBoundaryPolicy() {
		awsVal["permissions_boundary_policy"] = createArnObject(awsData.GetPermissionsBoundaryPolicy().GetArn())
	}

	awsObj, d := types.ObjectValue(GetAwsCmrType(), awsVal)
	if d.HasError() {
		diags.AddError("failed to generate AWS CMR object", "could not create AWS CMR object")
		diags.Append(d...)
		return types.ObjectNull(GetAwsCmrType()), diags
	}

	return awsObj, diags
}

func (*DataModel) generateModelGCPCMR(gcpData *controlplanev1.CustomerManagedResources_GCP) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	gcpVal := map[string]attr.Value{
		"subnet":                           types.ObjectNull(GetGcpSubnetType()),
		"agent_service_account":            types.ObjectNull(GetGcpServiceAccountType()),
		"console_service_account":          types.ObjectNull(GetGcpServiceAccountType()),
		"connector_service_account":        types.ObjectNull(GetGcpServiceAccountType()),
		"redpanda_cluster_service_account": types.ObjectNull(GetGcpServiceAccountType()),
		"gke_service_account":              types.ObjectNull(GetGcpServiceAccountType()),
		"tiered_storage_bucket":            types.ObjectNull(GetGcpBucketType()),
		"psc_nat_subnet_name":              types.StringNull(),
	}

	if gcpData.HasSubnet() {
		subnet := gcpData.GetSubnet()
		var podsRangeObj types.Object
		if subnet.HasSecondaryIpv4RangePods() {
			podsRange, d := types.ObjectValue(GetGcpSecondaryIPv4RangeType(), map[string]attr.Value{
				"name": types.StringValue(subnet.GetSecondaryIpv4RangePods().GetName()),
			})
			if d.HasError() {
				diags.AddError("failed to generate secondary IPv4 range pods object", "could not create secondary IPv4 range pods object")
				diags.Append(d...)
			} else {
				podsRangeObj = podsRange
			}
		} else {
			podsRangeObj = types.ObjectNull(GetGcpSecondaryIPv4RangeType())
		}

		var servicesRangeObj types.Object
		if subnet.HasSecondaryIpv4RangeServices() {
			servicesRange, d := types.ObjectValue(GetGcpSecondaryIPv4RangeType(), map[string]attr.Value{
				"name": types.StringValue(subnet.GetSecondaryIpv4RangeServices().GetName()),
			})
			if d.HasError() {
				diags.AddError("failed to generate secondary IPv4 range services object", "could not create secondary IPv4 range services object")
				diags.Append(d...)
			} else {
				servicesRangeObj = servicesRange
			}
		} else {
			servicesRangeObj = types.ObjectNull(GetGcpSecondaryIPv4RangeType())
		}

		subnetVal := map[string]attr.Value{
			"name":                          types.StringValue(subnet.GetName()),
			"secondary_ipv4_range_pods":     podsRangeObj,
			"secondary_ipv4_range_services": servicesRangeObj,
			"k8s_master_ipv4_range":         types.StringValue(subnet.GetK8SMasterIpv4Range()),
		}

		subnetObj, d := types.ObjectValue(GetGcpSubnetType(), subnetVal)
		if d.HasError() {
			diags.AddError("failed to generate subnet object", "could not create subnet object")
			diags.Append(d...)
		} else {
			gcpVal["subnet"] = subnetObj
		}
	}

	createServiceAccountObject := func(email string) types.Object {
		obj, _ := types.ObjectValue(GetGcpServiceAccountType(), map[string]attr.Value{
			"email": types.StringValue(email),
		})
		return obj
	}

	if gcpData.HasAgentServiceAccount() {
		gcpVal["agent_service_account"] = createServiceAccountObject(gcpData.GetAgentServiceAccount().GetEmail())
	}
	if gcpData.HasConsoleServiceAccount() {
		gcpVal["console_service_account"] = createServiceAccountObject(gcpData.GetConsoleServiceAccount().GetEmail())
	}
	if gcpData.HasConnectorServiceAccount() {
		gcpVal["connector_service_account"] = createServiceAccountObject(gcpData.GetConnectorServiceAccount().GetEmail())
	}
	if gcpData.HasRedpandaClusterServiceAccount() {
		gcpVal["redpanda_cluster_service_account"] = createServiceAccountObject(gcpData.GetRedpandaClusterServiceAccount().GetEmail())
	}
	if gcpData.HasGkeServiceAccount() {
		gcpVal["gke_service_account"] = createServiceAccountObject(gcpData.GetGkeServiceAccount().GetEmail())
	}

	if gcpData.HasTieredStorageBucket() {
		bucketObj, d := types.ObjectValue(GetGcpBucketType(), map[string]attr.Value{
			"name": types.StringValue(gcpData.GetTieredStorageBucket().GetName()),
		})
		if d.HasError() {
			diags.AddError("failed to generate tiered storage bucket object", "could not create tiered storage bucket object")
			diags.Append(d...)
		} else {
			gcpVal["tiered_storage_bucket"] = bucketObj
		}
	}

	if gcpData.GetPscNatSubnetName() != "" {
		gcpVal["psc_nat_subnet_name"] = types.StringValue(gcpData.GetPscNatSubnetName())
	}

	gcpObj, d := types.ObjectValue(GetGcpCmrType(), gcpVal)
	if d.HasError() {
		diags.AddError("failed to generate GCP CMR object", "could not create GCP CMR object")
		diags.Append(d...)
		return types.ObjectNull(GetGcpCmrType()), diags
	}

	return gcpObj, diags
}

func (*DataModel) generateModelClusterConfiguration(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasClusterConfiguration() {
		return types.ObjectNull(GetClusterConfigurationType()), diags
	}

	cfg := cluster.GetClusterConfiguration()
	configValues := map[string]attr.Value{
		"custom_properties_json": types.StringNull(),
	}

	// Handle custom properties
	if cfg.HasCustomProperties() {
		customPropsMap := cfg.GetCustomProperties().AsMap()
		if len(customPropsMap) > 0 {
			customPropsBytes, err := json.Marshal(customPropsMap)
			if err != nil {
				diags.AddError("failed to marshal custom properties", "could not convert custom properties to JSON")
				return types.ObjectNull(GetClusterConfigurationType()), diags
			}
			configValues["custom_properties_json"] = types.StringValue(string(customPropsBytes))
		}
	}

	// Only return null if custom properties are null
	if configValues["custom_properties_json"].IsNull() {
		return types.ObjectNull(GetClusterConfigurationType()), diags
	}

	obj, d := types.ObjectValue(GetClusterConfigurationType(), configValues)
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate cluster configuration object", "could not create cluster configuration object")
		return types.ObjectNull(GetClusterConfigurationType()), diags
	}

	return obj, diags
}
