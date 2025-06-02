package cluster

import (
	"context"
	"encoding/json"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// used as an input to generateModel to allow populating these fields with either the model or the API
type modelOrAPI struct {
	RedpandaVersion       types.String
	AllowDeletion         types.Bool
	Tags                  types.Map
	GcpGlobalAccessConfig types.Bool
}

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(cluster *controlplanev1.Cluster, contingent modelOrAPI, diagnostics diag.Diagnostics) (*models.Cluster, diag.Diagnostics) {
	output := &models.Cluster{
		Name:                  types.StringValue(cluster.GetName()),
		ID:                    types.StringValue(cluster.GetId()),
		ConnectionType:        types.StringValue(utils.ConnectionTypeToString(cluster.GetConnectionType())),
		CloudProvider:         types.StringValue(utils.CloudProviderToString(cluster.GetCloudProvider())),
		ClusterType:           types.StringValue(utils.ClusterTypeToString(cluster.GetType())),
		RedpandaVersion:       contingent.RedpandaVersion,
		ThroughputTier:        types.StringValue(cluster.GetThroughputTier()),
		Region:                types.StringValue(cluster.GetRegion()),
		AllowDeletion:         contingent.AllowDeletion,
		Tags:                  contingent.Tags,
		ResourceGroupID:       types.StringValue(cluster.GetResourceGroupId()),
		NetworkID:             types.StringValue(cluster.GetNetworkId()),
		ReadReplicaClusterIDs: utils.StringSliceToTypeList(cluster.GetReadReplicaClusterIds()),
		Zones:                 utils.StringSliceToTypeList(cluster.GetZones()),
		State:                 types.StringValue(cluster.GetState().String()),
	}

	if cluster.HasGcpGlobalAccessEnabled() && utils.CloudProviderToString(cluster.CloudProvider) == utils.CloudProviderStringGcp {
		switch {
		case cluster.GetGcpGlobalAccessEnabled() || contingent.GcpGlobalAccessConfig.ValueBool():
			output.GCPGlobalAccessEnabled = types.BoolValue(true)
		case !cluster.GetGcpGlobalAccessEnabled() && !contingent.GcpGlobalAccessConfig.IsNull():
			output.GCPGlobalAccessEnabled = types.BoolValue(false)
		default:
			output.GCPGlobalAccessEnabled = types.BoolNull()
		}
	} else {
		output.GCPGlobalAccessEnabled = types.BoolNull()
	}

	if cluster.HasCreatedAt() {
		output.CreatedAt = types.StringValue(cluster.CreatedAt.AsTime().Format(time.RFC3339))
	}

	if cluster.HasStateDescription() {
		stateDescription, d := generateModelStateDescription(cluster, diagnostics)
		if d.HasError() {
			diagnostics.Append(d...)
			return nil, diagnostics
		}
		output.StateDescription = stateDescription
	}

	if cluster.HasDataplaneApi() {
		output.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}

	kafkaAPI, d := generateModelKafkaAPI(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.KafkaAPI = kafkaAPI

	awsPrivateLink, d := generateModelAWSPrivateLink(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.AwsPrivateLink = awsPrivateLink

	gcpPSC, d := generateModelGCPPrivateServiceConnect(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.GcpPrivateServiceConnect = gcpPSC

	httpProxy, d := generateModelHTTPProxy(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.HTTPProxy = httpProxy

	rpc, d := generateModelRedpandaConsole(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.RedpandaConsole = rpc

	schemaRegistry, d := generateModelSchemaRegistry(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.SchemaRegistry = schemaRegistry

	prometheus, d := generateModelPrometheus(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.Prometheus = prometheus

	maintenanceWindow, d := generateModelMaintenanceWindow(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.MaintenanceWindowConfig = maintenanceWindow

	azurePrivateLink, d := generateModelAzurePrivateLink(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.AzurePrivateLink = azurePrivateLink

	kafkaConnect, d := generateModelKafkaConnect(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.KafkaConnect = kafkaConnect

	cmr, dg := generateModelCMR(cluster, diagnostics)
	if dg.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.CustomerManagedResources = cmr

	// Handle cluster configuration
	clusterConfig, dg := generateModelClusterConfiguration(cluster, diagnostics)
	if dg.HasError() {
		diagnostics.Append(dg...)
		return nil, diagnostics
	}
	output.ClusterConfiguration = clusterConfig

	return output, nil
}

func generateModelStateDescription(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasStateDescription() {
		return types.ObjectNull(stateDescriptionType), diagnostics
	}
	sd := cluster.GetStateDescription()
	obj, d := types.ObjectValue(stateDescriptionType, map[string]attr.Value{
		"message": types.StringValue(sd.GetMessage()),
		"code":    types.Int32Value(sd.GetCode()),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate state description object", "could not create state description object")
		return types.ObjectNull(stateDescriptionType), diagnostics
	}
	return obj, diagnostics
}

func generateModelKafkaConnect(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasKafkaConnect() {
		return types.ObjectNull(kafkaConnectType), diagnostics
	}

	kafkaConnect := cluster.GetKafkaConnect()
	if !kafkaConnect.GetEnabled() {
		return types.ObjectNull(kafkaConnectType), diagnostics
	}

	obj, d := types.ObjectValue(kafkaConnectType, map[string]attr.Value{
		"enabled": types.BoolValue(kafkaConnect.GetEnabled()),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate Kafka Connect object", "could not create Kafka Connect object")
		return types.ObjectNull(kafkaConnectType), diagnostics
	}

	return obj, diagnostics
}

func generateModelPrometheus(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasPrometheus() {
		return types.ObjectNull(prometheusType), diagnostics
	}

	prometheus := cluster.GetPrometheus()

	// Create the Prometheus object
	obj, d := types.ObjectValue(prometheusType, map[string]attr.Value{
		"url": types.StringValue(prometheus.GetUrl()),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate Prometheus object", "could not create Prometheus object")
		return types.ObjectNull(prometheusType), diagnostics
	}

	return obj, diagnostics
}

func generateModelSchemaRegistry(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasSchemaRegistry() {
		return types.ObjectNull(schemaRegistryType), diagnostics
	}

	schemaRegistry := cluster.GetSchemaRegistry()

	// Get MTLS configuration
	mtls, d := getMtlsModel(schemaRegistry.GetMtls(), diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return types.ObjectNull(schemaRegistryType), diagnostics
	}

	// Create the Schema Registry object
	obj, d := types.ObjectValue(schemaRegistryType, map[string]attr.Value{
		"mtls": mtls,
		"url":  types.StringValue(schemaRegistry.GetUrl()),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate Schema Registry object", "could not create Schema Registry object")
		return types.ObjectNull(schemaRegistryType), diagnostics
	}

	return obj, diagnostics
}

func generateModelRedpandaConsole(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasRedpandaConsole() {
		return types.ObjectNull(redpandaConsoleType), diagnostics
	}

	console := cluster.GetRedpandaConsole()

	// Create the Redpanda Console object
	obj, d := types.ObjectValue(redpandaConsoleType, map[string]attr.Value{
		"url": types.StringValue(console.GetUrl()),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate Redpanda Console object", "could not create Redpanda Console object")
		return types.ObjectNull(redpandaConsoleType), diagnostics
	}

	return obj, diagnostics
}

func getMtlsModel(mtls *controlplanev1.MTLSSpec, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	mtlsValue := map[string]attr.Value{
		"enabled":                 types.BoolNull(),
		"ca_certificates_pem":     types.ListNull(types.StringType),
		"principal_mapping_rules": types.ListNull(types.StringType),
	}
	if !mtls.GetEnabled() {
		return types.ObjectNull(mtlsType), diagnostics
	}
	if mtls != nil {
		mtlsValue["enabled"] = types.BoolValue(mtls.GetEnabled())
		mtlsValue["ca_certificates_pem"] = utils.StringSliceToTypeList(mtls.GetCaCertificatesPem())
		mtlsValue["principal_mapping_rules"] = utils.StringSliceToTypeList(mtls.GetPrincipalMappingRules())
	}
	out, d := types.ObjectValue(mtlsType, mtlsValue)
	if d.HasError() {
		if utils.IsNotFoundSpec(d) {
			// mtls not being found is valid, null it and move on
			return types.ObjectNull(mtlsType), diagnostics
		}
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate mtls object", "failed to generate mtls object")
		return types.ObjectNull(mtlsType), diagnostics
	}
	return out, diagnostics
}

func generateModelKafkaAPI(cluster *controlplanev1.Cluster, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasKafkaApi() {
		return types.ObjectNull(kafkaAPIType), diags
	}

	kafkaAPI := cluster.GetKafkaApi()
	mtls, d := getMtlsModel(kafkaAPI.GetMtls(), diags)
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}

	obj, d := types.ObjectValue(kafkaAPIType, map[string]attr.Value{
		"mtls":         mtls,
		"seed_brokers": utils.StringSliceToTypeList(kafkaAPI.GetSeedBrokers()),
	})
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}
	return obj, diags
}

func generateModelAWSPrivateLink(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasAwsPrivateLink() {
		return types.ObjectNull(awsPrivateLinkType), diagnostics
	}

	awsPrivateLink := cluster.GetAwsPrivateLink()
	if !awsPrivateLink.GetEnabled() {
		return types.ObjectNull(awsPrivateLinkType), diagnostics
	}

	// Convert allowed principals to TypeList
	var allowedPrincipals types.List
	if ap := awsPrivateLink.GetAllowedPrincipals(); ap != nil {
		allowedPrincipals = utils.StringSliceToTypeList(ap)
	} else {
		allowedPrincipals = types.ListNull(types.StringType)
	}

	// Get status if available
	status := awsPrivateLink.GetStatus()
	if status != nil {
		// Convert VPC endpoint connections
		var vpcEndpointConns []attr.Value
		for _, conn := range status.GetVpcEndpointConnections() {
			// Convert DNS entries - all fields optional
			var dnsEntries []attr.Value
			for _, dns := range conn.GetDnsEntries() {
				dnsEntry, d := types.ObjectValue(dnsEntryType, map[string]attr.Value{
					"dns_name":       types.StringValue(dns.GetDnsName()),
					"hosted_zone_id": types.StringValue(dns.GetHostedZoneId()),
				})
				if d.HasError() {
					diagnostics.Append(d...)
					diagnostics.AddError("failed to generate DNS entry", "could not create DNS entry object")
					return types.ObjectNull(awsPrivateLinkType), diagnostics
				}
				dnsEntries = append(dnsEntries, dnsEntry)
			}

			// Build DNS entries list
			dnsEntriesList, d := types.ListValue(types.ObjectType{AttrTypes: dnsEntryType}, dnsEntries)
			if d.HasError() {
				diagnostics.Append(d...)
				diagnostics.AddError("failed to generate DNS entries list", "could not create DNS entries list")
				return types.ObjectNull(awsPrivateLinkType), diagnostics
			}

			// Build VPC endpoint connection object - all fields optional
			connObj, d := types.ObjectValue(vpcEndpointConnType, map[string]attr.Value{
				"id":    types.StringValue(conn.GetId()),
				"owner": types.StringValue(conn.GetOwner()),
				"state": types.StringValue(conn.GetState()),
				"created_at": func() types.String {
					if status.CreatedAt != nil {
						return types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339))
					}
					return types.StringNull()
				}(), "connection_id": types.StringValue(conn.GetConnectionId()),
				"load_balancer_arns": utils.StringSliceToTypeList(conn.GetLoadBalancerArns()),
				"dns_entries":        dnsEntriesList,
			})
			if d.HasError() {
				diagnostics.Append(d...)
				diagnostics.AddError("failed to generate VPC endpoint connection", "could not create VPC endpoint connection object")
				return types.ObjectNull(awsPrivateLinkType), diagnostics
			}
			vpcEndpointConns = append(vpcEndpointConns, connObj)
		}

		// Build VPC endpoint connections list
		vpcEndpointConnsList, d := types.ListValue(types.ObjectType{AttrTypes: vpcEndpointConnType}, vpcEndpointConns)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate VPC endpoint connections list", "could not create VPC endpoint connections list")
			return types.ObjectNull(awsPrivateLinkType), diagnostics
		}

		// Build status object - all fields optional
		statusValues := map[string]attr.Value{
			"service_id":                    types.StringValue(status.GetServiceId()),
			"service_name":                  types.StringValue(status.GetServiceName()),
			"service_state":                 types.StringValue(status.GetServiceState()),
			"kafka_api_seed_port":           types.Int32Value(status.GetKafkaApiSeedPort()),
			"schema_registry_seed_port":     types.Int32Value(status.GetSchemaRegistrySeedPort()),
			"redpanda_proxy_seed_port":      types.Int32Value(status.GetRedpandaProxySeedPort()),
			"kafka_api_node_base_port":      types.Int32Value(status.GetKafkaApiNodeBasePort()),
			"redpanda_proxy_node_base_port": types.Int32Value(status.GetRedpandaProxyNodeBasePort()),
			"console_port":                  types.Int32Value(status.GetConsolePort()),
			"vpc_endpoint_connections":      vpcEndpointConnsList,
			"created_at": func() types.String {
				if status.CreatedAt != nil {
					return types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339))
				}
				return types.StringNull()
			}(),
			"deleted_at": func() types.String {
				if status.DeletedAt != nil {
					return types.StringValue(status.GetDeletedAt().AsTime().Format(time.RFC3339))
				}
				return types.StringNull()
			}(),
		}

		statusObj, d := types.ObjectValue(statusObjectType, statusValues)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(awsPrivateLinkType), diagnostics
		}

		// Create final object including status
		obj, d := types.ObjectValue(awsPrivateLinkType, map[string]attr.Value{
			"enabled":            types.BoolValue(awsPrivateLink.GetEnabled()),
			"connect_console":    types.BoolValue(awsPrivateLink.GetConnectConsole()),
			"allowed_principals": allowedPrincipals,
			"status":             statusObj,
		})
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate AWS Private Link object", "could not create AWS Private Link object")
			return types.ObjectNull(awsPrivateLinkType), diagnostics
		}
		return obj, diagnostics
	}

	// Return object without status if not available
	obj, d := types.ObjectValue(awsPrivateLinkType, map[string]attr.Value{
		"enabled":            types.BoolValue(awsPrivateLink.GetEnabled()),
		"connect_console":    types.BoolValue(awsPrivateLink.GetConnectConsole()),
		"allowed_principals": allowedPrincipals,
		"status":             types.ObjectNull(statusObjectType),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate AWS Private Link object", "could not create AWS Private Link object without status")
		return types.ObjectNull(awsPrivateLinkType), diagnostics
	}
	return obj, diagnostics
}

func generateModelGCPPrivateServiceConnect(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasGcpPrivateServiceConnect() {
		return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
	}

	gcpPSC := cluster.GetGcpPrivateServiceConnect()
	if !gcpPSC.GetEnabled() {
		return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
	}

	// Convert consumer accept list
	var consumerAcceptList []attr.Value
	for _, consumer := range gcpPSC.GetConsumerAcceptList() {
		consumerObj, d := types.ObjectValue(
			map[string]attr.Type{"source": types.StringType},
			map[string]attr.Value{"source": types.StringValue(consumer.GetSource())},
		)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate consumer accept list entry", "could not create consumer object")
			return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
		}
		consumerAcceptList = append(consumerAcceptList, consumerObj)
	}

	consumerList, d := types.ListValue(
		types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
		consumerAcceptList,
	)
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate consumer accept list", "could not create consumer list")
		return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
	}

	// Get status if available
	status := gcpPSC.GetStatus()
	if status != nil {
		// Convert connected endpoints - all fields optional
		var connectedEndpoints []attr.Value
		for _, endpoint := range status.GetConnectedEndpoints() {
			endpointObj, d := types.ObjectValue(
				connectedEndpointType,
				map[string]attr.Value{
					"connection_id":    types.StringValue(endpoint.GetConnectionId()),
					"consumer_network": types.StringValue(endpoint.GetConsumerNetwork()),
					"endpoint":         types.StringValue(endpoint.GetEndpoint()),
					"status":           types.StringValue(endpoint.GetStatus()),
				},
			)
			if d.HasError() {
				diagnostics.Append(d...)
				diagnostics.AddError("failed to generate connected endpoint", "could not create endpoint object")
				return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
			}
			connectedEndpoints = append(connectedEndpoints, endpointObj)
		}

		endpointList, d := types.ListValue(types.ObjectType{AttrTypes: connectedEndpointType}, connectedEndpoints)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate connected endpoints list", "could not create endpoints list")
			return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
		}

		statusValues := map[string]attr.Value{
			"service_attachment":            types.StringValue(status.GetServiceAttachment()),
			"kafka_api_seed_port":           types.Int32Value(status.GetKafkaApiSeedPort()),
			"schema_registry_seed_port":     types.Int32Value(status.GetSchemaRegistrySeedPort()),
			"redpanda_proxy_seed_port":      types.Int32Value(status.GetRedpandaProxySeedPort()),
			"kafka_api_node_base_port":      types.Int32Value(status.GetKafkaApiNodeBasePort()),
			"redpanda_proxy_node_base_port": types.Int32Value(status.GetRedpandaProxyNodeBasePort()),
			"connected_endpoints":           endpointList,
			"dns_a_records":                 utils.StringSliceToTypeList(status.GetDnsARecords()),
			"seed_hostname":                 types.StringValue(status.GetSeedHostname()),
			"created_at": func() types.String {
				if status.CreatedAt != nil {
					return types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339))
				}
				return types.StringNull()
			}(),
			"deleted_at": func() types.String {
				if status.DeletedAt != nil {
					return types.StringValue(status.GetDeletedAt().AsTime().Format(time.RFC3339))
				}
				return types.StringNull()
			}(),
		}

		statusObj, d := types.ObjectValue(gcpPrivateServiceConnectStatusType, statusValues)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
		}

		// Create final object including status
		obj, d := types.ObjectValue(gcpPrivateServiceConnectType, map[string]attr.Value{
			"enabled":               types.BoolValue(gcpPSC.GetEnabled()),
			"global_access_enabled": types.BoolValue(gcpPSC.GetGlobalAccessEnabled()),
			"consumer_accept_list":  consumerList,
			"status":                statusObj,
		})
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate GCP Private Service Connect object", "could not create final object")
			return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
		}
		return obj, diagnostics
	}

	// Return object without status if not available
	obj, d := types.ObjectValue(gcpPrivateServiceConnectType, map[string]attr.Value{
		"enabled":               types.BoolValue(gcpPSC.GetEnabled()),
		"global_access_enabled": types.BoolValue(gcpPSC.GetGlobalAccessEnabled()),
		"consumer_accept_list":  consumerList,
		"status":                types.ObjectNull(gcpPrivateServiceConnectStatusType),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate GCP Private Service Connect object", "could not create object without status")
		return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
	}
	return obj, diagnostics
}

// generateUpdateRequest populates an UpdateClusterRequest that will update a cluster from the
// // current state to a new state matching the plan.
func generateUpdateRequest(ctx context.Context, plan, state models.Cluster, diags diag.Diagnostics) (*controlplanev1.UpdateClusterRequest, diag.Diagnostics) {
	planUpdate, ds := generateClusterUpdate(ctx, plan, diags)
	if ds.HasError() {
		diags.Append(ds...)
		return nil, diags
	}
	stateUpdate, dg := generateClusterUpdate(ctx, state, diags)
	if dg.HasError() {
		diags.Append(dg...)
		return nil, diags
	}

	update, fieldmask := utils.GenerateProtobufDiffAndUpdateMask(planUpdate, stateUpdate)
	update.Id = planUpdate.Id
	return &controlplanev1.UpdateClusterRequest{
		Cluster:    update,
		UpdateMask: fieldmask,
	}, nil
}

func generateModelMaintenanceWindow(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasMaintenanceWindowConfig() {
		return types.ObjectNull(maintenanceWindowConfigType), diagnostics
	}

	maintenance := cluster.GetMaintenanceWindowConfig()

	// Handle the oneof field
	windowObj := types.ObjectNull(dayHourType)
	var unspec types.Bool
	var anytime types.Bool
	var d diag.Diagnostics

	if !maintenance.HasWindow() {
		return types.ObjectNull(maintenanceWindowConfigType), diagnostics
	}

	switch {
	case maintenance.HasDayHour():
		w := maintenance.GetDayHour()
		windowObj, d = types.ObjectValue(dayHourType, map[string]attr.Value{
			"hour_of_day": types.Int32Value(w.GetHourOfDay()),
			"day_of_week": types.StringValue(w.GetDayOfWeek().String()),
		})
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate maintenance window detail", "could not create window object")
			return types.ObjectNull(maintenanceWindowConfigType), diagnostics
		}
	case maintenance.HasAnytime():
		unspec = types.BoolValue(true)
	case maintenance.HasUnspecified():
		anytime = types.BoolValue(true)
	}

	// Create the maintenance window object
	obj, d := types.ObjectValue(maintenanceWindowConfigType, map[string]attr.Value{
		"day_hour":    windowObj,
		"anytime":     anytime,
		"unspecified": unspec,
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate maintenance window object", "could not create maintenance window object")
		return types.ObjectNull(maintenanceWindowConfigType), diagnostics
	}

	return obj, diagnostics
}

func generateModelHTTPProxy(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasHttpProxy() {
		return types.ObjectNull(httpProxyType), diagnostics
	}

	httpProxy := cluster.GetHttpProxy()

	// Get MTLS configuration
	mtls, d := getMtlsModel(httpProxy.GetMtls(), diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return types.ObjectNull(httpProxyType), diagnostics
	}

	// Create the HTTP Proxy object
	obj, d := types.ObjectValue(httpProxyType, map[string]attr.Value{
		"mtls": mtls,
		"url":  types.StringValue(httpProxy.GetUrl()),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate HTTP Proxy object", "could not create HTTP Proxy object")
		return types.ObjectNull(httpProxyType), diagnostics
	}

	return obj, diagnostics
}

func generateModelAzurePrivateLink(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasAzurePrivateLink() {
		return types.ObjectNull(azurePrivateLinkType), diagnostics
	}

	azurePrivateLink := cluster.GetAzurePrivateLink()
	if !azurePrivateLink.GetEnabled() {
		return types.ObjectNull(azurePrivateLinkType), diagnostics
	}

	// Convert allowed subscriptions to TypeList
	var allowedSubscriptions types.List
	if as := azurePrivateLink.GetAllowedSubscriptions(); as != nil {
		allowedSubscriptions = utils.StringSliceToTypeList(as)
	} else {
		allowedSubscriptions = types.ListNull(types.StringType)
	}

	// Get status if available
	status := azurePrivateLink.GetStatus()
	if status != nil {
		// Convert private endpoint connections
		var privateEndpointConns []attr.Value
		for _, conn := range status.GetPrivateEndpointConnections() {
			connObj, d := types.ObjectValue(azureEndpointConnType, map[string]attr.Value{
				"private_endpoint_name": types.StringValue(conn.GetPrivateEndpointName()),
				"private_endpoint_id":   types.StringValue(conn.GetPrivateEndpointId()),
				"connection_name":       types.StringValue(conn.GetConnectionName()),
				"connection_id":         types.StringValue(conn.GetConnectionId()),
				"status":                types.StringValue(conn.GetStatus()),
				"created_at": func() types.String {
					if conn.CreatedAt != nil {
						return types.StringValue(conn.GetCreatedAt().AsTime().Format(time.RFC3339))
					}
					return types.StringNull()
				}(),
			})
			if d.HasError() {
				diagnostics.Append(d...)
				diagnostics.AddError("failed to generate private endpoint connection", "could not create endpoint connection object")
				return types.ObjectNull(azurePrivateLinkType), diagnostics
			}
			privateEndpointConns = append(privateEndpointConns, connObj)
		}

		// Build private endpoint connections list
		endpointConnsList, d := types.ListValue(types.ObjectType{AttrTypes: azureEndpointConnType}, privateEndpointConns)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate private endpoint connections list", "could not create connections list")
			return types.ObjectNull(azurePrivateLinkType), diagnostics
		}

		// Build status object
		statusValues := map[string]attr.Value{
			"service_id":                    types.StringValue(status.GetServiceId()),
			"service_name":                  types.StringValue(status.GetServiceName()),
			"kafka_api_seed_port":           types.Int32Value(status.GetKafkaApiSeedPort()),
			"schema_registry_seed_port":     types.Int32Value(status.GetSchemaRegistrySeedPort()),
			"redpanda_proxy_seed_port":      types.Int32Value(status.GetRedpandaProxySeedPort()),
			"kafka_api_node_base_port":      types.Int32Value(status.GetKafkaApiNodeBasePort()),
			"redpanda_proxy_node_base_port": types.Int32Value(status.GetRedpandaProxyNodeBasePort()),
			"console_port":                  types.Int32Value(status.GetConsolePort()),
			"private_endpoint_connections":  endpointConnsList,
			"created_at": func() types.String {
				if status.CreatedAt != nil {
					return types.StringValue(status.GetCreatedAt().AsTime().Format(time.RFC3339))
				}
				return types.StringNull()
			}(),
			"deleted_at": func() types.String {
				if status.DeletedAt != nil {
					return types.StringValue(status.GetDeletedAt().AsTime().Format(time.RFC3339))
				}
				return types.StringNull()
			}(),
			"dns_a_record":           types.StringValue(status.GetDnsARecord()),
			"approved_subscriptions": utils.StringSliceToTypeList(status.GetApprovedSubscriptions()),
		}

		statusObj, d := types.ObjectValue(azurePrivateLinkStatusType, statusValues)
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(azurePrivateLinkType), diagnostics
		}

		// Create final object including status
		obj, d := types.ObjectValue(azurePrivateLinkType, map[string]attr.Value{
			"enabled":               types.BoolValue(azurePrivateLink.GetEnabled()),
			"connect_console":       types.BoolValue(azurePrivateLink.GetConnectConsole()),
			"allowed_subscriptions": allowedSubscriptions,
			"status":                statusObj,
		})
		if d.HasError() {
			diagnostics.Append(d...)
			diagnostics.AddError("failed to generate Azure Private Link object", "could not create Azure Private Link object")
			return types.ObjectNull(azurePrivateLinkType), diagnostics
		}
		return obj, diagnostics
	}

	// Return object without status if not available
	obj, d := types.ObjectValue(azurePrivateLinkType, map[string]attr.Value{
		"enabled":               types.BoolValue(azurePrivateLink.GetEnabled()),
		"connect_console":       types.BoolValue(azurePrivateLink.GetConnectConsole()),
		"allowed_subscriptions": allowedSubscriptions,
		"status":                types.ObjectNull(azurePrivateLinkStatusType),
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate Azure Private Link object", "could not create object without status")
		return types.ObjectNull(azurePrivateLinkType), diagnostics
	}
	return obj, diagnostics
}

func generateModelCMR(cluster *controlplanev1.Cluster, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	// Early return conditions
	if cluster == nil || !cluster.HasCustomerManagedResources() {
		return types.ObjectNull(cmrType), diags
	}

	cloudProvider := utils.CloudProviderToString(cluster.GetCloudProvider())

	if cluster.Type != controlplanev1.Cluster_TYPE_BYOC {
		diags.AddError("Customer Managed Resources with non-BYOC cluster type", "Customer Managed Resources are only supported for BYOC clusters")
		return types.ObjectNull(cmrType), diags
	}

	cmr := cluster.GetCustomerManagedResources()
	if cmr == nil {
		return types.ObjectNull(cmrType), diags
	}

	switch cloudProvider {
	case utils.CloudProviderStringAws:
		if !cmr.HasAws() {
			diags.AddError("Cloud Provider Mismatch", "AWS customer managed resources are missing for AWS BYOVPC Cluster")
			return types.ObjectNull(cmrType), diags
		}

		aws, d := generateModelCMRAWS(cmr.GetAws(), diags)
		if d.HasError() {
			diags.AddError("failed to generate AWS CMR object", "could not create AWS CMR object")
			diags.Append(d...)
			return types.ObjectNull(cmrType), diags
		}

		// Create final CMR object
		cmrObj, d := types.ObjectValue(cmrType, map[string]attr.Value{
			"aws": aws,
			"gcp": types.ObjectNull(gcpType),
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return types.ObjectNull(cmrType), diags
		}
		return cmrObj, diags

	case utils.CloudProviderStringGcp:
		if !cmr.HasGcp() {
			diags.AddError("Cloud Provider Mismatch", "GCP customer managed resources are missing for GCP BYOVPC Cluster")
			return types.ObjectNull(cmrType), diags
		}

		gcp, d := generateModelCMRGCP(cmr.GetGcp(), diags)
		if d.HasError() {
			diags.AddError("failed to generate GCP CMR object", "could not create GCP CMR object")
			diags.Append(d...)
			return types.ObjectNull(cmrType), diags
		}

		cmrObj, d := types.ObjectValue(cmrType, map[string]attr.Value{
			"aws": types.ObjectNull(awsType),
			"gcp": gcp,
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return types.ObjectNull(cmrType), diags
		}
		return cmrObj, diags
	}

	// Default return for unsupported cloud providers
	return types.ObjectNull(cmrType), diags
}

func generateModelCMRGCP(gcpData *controlplanev1.CustomerManagedResources_GCP, diags diag.Diagnostics) (basetypes.ObjectValue, diag.Diagnostics) {
	// Initialize GCP values map with default null values
	gcpVal := make(map[string]attr.Value)
	for k, v := range gcpValueDefaults {
		gcpVal[k] = v
	}

	// Set subnet configuration if it exists
	if gcpData.HasSubnet() {
		subnet := gcpData.GetSubnet()

		// Process secondary IPv4 range for pods
		var podsRangeObj types.Object
		if subnet.HasSecondaryIpv4RangePods() {
			podsRange, d := types.ObjectValue(gcpSecondaryIPv4RangeType, map[string]attr.Value{
				"name": types.StringValue(subnet.GetSecondaryIpv4RangePods().GetName()),
			})
			if d.HasError() {
				diags.AddError("failed to generate secondary IPv4 range pods object", "could not create secondary IPv4 range pods object")
				diags.Append(d...)
			} else {
				podsRangeObj = podsRange
			}
		} else {
			podsRangeObj = types.ObjectNull(gcpSecondaryIPv4RangeType)
		}

		// Process secondary IPv4 range for services
		var servicesRangeObj types.Object
		if subnet.HasSecondaryIpv4RangeServices() {
			servicesRange, d := types.ObjectValue(gcpSecondaryIPv4RangeType, map[string]attr.Value{
				"name": types.StringValue(subnet.GetSecondaryIpv4RangeServices().GetName()),
			})
			if d.HasError() {
				diags.AddError("failed to generate secondary IPv4 range services object", "could not create secondary IPv4 range services object")
				diags.Append(d...)
			} else {
				servicesRangeObj = servicesRange
			}
		} else {
			servicesRangeObj = types.ObjectNull(gcpSecondaryIPv4RangeType)
		}

		// Create the subnet object
		subnetObj, d := types.ObjectValue(gcpSubnetType, map[string]attr.Value{
			"name":                          types.StringValue(subnet.GetName()),
			"secondary_ipv4_range_pods":     podsRangeObj,
			"secondary_ipv4_range_services": servicesRangeObj,
			"k8s_master_ipv4_range":         types.StringValue(subnet.GetK8SMasterIpv4Range()),
		})
		if d.HasError() {
			diags.AddError("failed to generate subnet object", "could not create subnet object")
			diags.Append(d...)
		} else {
			gcpVal["subnet"] = subnetObj
		}
	}

	// Process agent service account
	if gcpData.HasAgentServiceAccount() {
		obj, d := types.ObjectValue(gcpServiceAccountType, map[string]attr.Value{
			"email": types.StringValue(gcpData.GetAgentServiceAccount().GetEmail()),
		})
		if d.HasError() {
			diags.AddError("failed to generate agent service account object", "could not create agent service account object")
			diags.Append(d...)
		} else {
			gcpVal["agent_service_account"] = obj
		}
	}

	// Process console service account
	if gcpData.HasConsoleServiceAccount() {
		obj, d := types.ObjectValue(gcpServiceAccountType, map[string]attr.Value{
			"email": types.StringValue(gcpData.GetConsoleServiceAccount().GetEmail()),
		})
		if d.HasError() {
			diags.AddError("failed to generate console service account object", "could not create console service account object")
			diags.Append(d...)
		} else {
			gcpVal["console_service_account"] = obj
		}
	}

	// Process connector service account
	if gcpData.HasConnectorServiceAccount() {
		obj, d := types.ObjectValue(gcpServiceAccountType, map[string]attr.Value{
			"email": types.StringValue(gcpData.GetConnectorServiceAccount().GetEmail()),
		})
		if d.HasError() {
			diags.AddError("failed to generate connector service account object", "could not create connector service account object")
			diags.Append(d...)
		} else {
			gcpVal["connector_service_account"] = obj
		}
	}

	// Process redpanda cluster service account
	if gcpData.HasRedpandaClusterServiceAccount() {
		obj, d := types.ObjectValue(gcpServiceAccountType, map[string]attr.Value{
			"email": types.StringValue(gcpData.GetRedpandaClusterServiceAccount().GetEmail()),
		})
		if d.HasError() {
			diags.AddError("failed to generate redpanda cluster service account object", "could not create redpanda cluster service account object")
			diags.Append(d...)
		} else {
			gcpVal["redpanda_cluster_service_account"] = obj
		}
	}

	// Process GKE service account
	if gcpData.HasGkeServiceAccount() {
		obj, d := types.ObjectValue(gcpServiceAccountType, map[string]attr.Value{
			"email": types.StringValue(gcpData.GetGkeServiceAccount().GetEmail()),
		})
		if d.HasError() {
			diags.AddError("failed to generate GKE service account object", "could not create GKE service account object")
			diags.Append(d...)
		} else {
			gcpVal["gke_service_account"] = obj
		}
	}

	// Process tiered storage bucket
	if gcpData.HasTieredStorageBucket() {
		obj, d := types.ObjectValue(gcpBucketType, map[string]attr.Value{
			"name": types.StringValue(gcpData.GetTieredStorageBucket().GetName()),
		})
		if d.HasError() {
			diags.AddError("failed to generate tiered storage bucket object", "could not create tiered storage bucket object")
			diags.Append(d...)
		} else {
			gcpVal["tiered_storage_bucket"] = obj
		}
	}

	// Process optional PSC NAT subnet name
	if gcpData.PscNatSubnetName != "" {
		gcpVal["psc_nat_subnet_name"] = types.StringValue(gcpData.PscNatSubnetName)
	}

	// Create GCP object
	gcpObj, d := types.ObjectValue(gcpType, gcpVal)
	if d.HasError() {
		diags.AddError("failed to generate GCP object", "could not create GCP object")
		diags.Append(d...)
		return types.ObjectNull(cmrType), diags
	}
	return gcpObj, diags
}

func generateModelCMRAWS(awsData *controlplanev1.CustomerManagedResources_AWS, diags diag.Diagnostics) (basetypes.ObjectValue, diag.Diagnostics) {
	// Initialize AWS values map with default null values
	awsVal := make(map[string]attr.Value)
	for k, v := range awsValueDefaults {
		awsVal[k] = v
	}

	// Now set values for fields that exist in the input
	if awsData.HasAgentInstanceProfile() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetAgentInstanceProfile().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate agent instance profile object", "could not create agent instance profile object")
			diags.Append(d...)
		} else {
			awsVal["agent_instance_profile"] = obj
		}
	}

	if awsData.HasConnectorsNodeGroupInstanceProfile() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetConnectorsNodeGroupInstanceProfile().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate connectors node group instance profile object", "could not create connectors node group instance profile object")
			diags.Append(d...)
		} else {
			awsVal["connectors_node_group_instance_profile"] = obj
		}
	}

	if awsData.HasUtilityNodeGroupInstanceProfile() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetUtilityNodeGroupInstanceProfile().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate utility node group instance profile object", "could not create utility node group instance profile object")
			diags.Append(d...)
		} else {
			awsVal["utility_node_group_instance_profile"] = obj
		}
	}

	if awsData.HasRedpandaNodeGroupInstanceProfile() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetRedpandaNodeGroupInstanceProfile().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate redpanda node group instance profile object", "could not create redpanda node group instance profile object")
			diags.Append(d...)
		} else {
			awsVal["redpanda_node_group_instance_profile"] = obj
		}
	}

	if awsData.HasK8SClusterRole() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetK8SClusterRole().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate k8s cluster role object", "could not create k8s cluster role object")
			diags.Append(d...)
		} else {
			awsVal["k8s_cluster_role"] = obj
		}
	}

	if awsData.HasRedpandaAgentSecurityGroup() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetRedpandaAgentSecurityGroup().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate redpanda agent security group object", "could not create redpanda agent security group object")
			diags.Append(d...)
		} else {
			awsVal["redpanda_agent_security_group"] = obj
		}
	}

	if awsData.HasConnectorsSecurityGroup() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetConnectorsSecurityGroup().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate connectors security group object", "could not create connectors security group object")
			diags.Append(d...)
		} else {
			awsVal["connectors_security_group"] = obj
		}
	}

	if awsData.HasRedpandaNodeGroupSecurityGroup() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetRedpandaNodeGroupSecurityGroup().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate redpanda node group security group object", "could not create redpanda node group security group object")
			diags.Append(d...)
		} else {
			awsVal["redpanda_node_group_security_group"] = obj
		}
	}

	if awsData.HasUtilitySecurityGroup() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetUtilitySecurityGroup().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate utility security group object", "could not create utility security group object")
			diags.Append(d...)
		} else {
			awsVal["utility_security_group"] = obj
		}
	}

	if awsData.HasClusterSecurityGroup() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetClusterSecurityGroup().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate cluster security group object", "could not create cluster security group object")
			diags.Append(d...)
		} else {
			awsVal["cluster_security_group"] = obj
		}
	}

	if awsData.HasNodeSecurityGroup() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetNodeSecurityGroup().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate node security group object", "could not create node security group object")
			diags.Append(d...)
		} else {
			awsVal["node_security_group"] = obj
		}
	}

	if awsData.HasCloudStorageBucket() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetCloudStorageBucket().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate cloud storage bucket object", "could not create cloud storage bucket object")
			diags.Append(d...)
		} else {
			awsVal["cloud_storage_bucket"] = obj
		}
	}

	if awsData.HasPermissionsBoundaryPolicy() {
		obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
			"arn": types.StringValue(awsData.GetPermissionsBoundaryPolicy().GetArn()),
		})
		if d.HasError() {
			diags.AddError("failed to generate permissions boundary policy object", "could not create permissions boundary policy object")
			diags.Append(d...)
		} else {
			awsVal["permissions_boundary_policy"] = obj
		}
	}

	// Create AWS object
	awsObj, d := types.ObjectValue(awsType, awsVal)
	if d.HasError() {
		diags.AddError("failed to generate AWS object", "could not create AWS object")
		diags.Append(d...)
		return types.ObjectNull(cmrType), diags
	}
	return awsObj, diags
}

// generateModelClusterConfiguration transforms ClusterConfiguration proto to Terraform model
func generateModelClusterConfiguration(cluster *controlplanev1.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	// Define the object types for cluster configuration
	clusterConfigType := map[string]attr.Type{
		"custom_properties_json":   types.StringType,
		"computed_properties_json": types.StringType,
	}

	// If the cluster doesn't have configuration, return null object
	if !cluster.HasClusterConfiguration() {
		return types.ObjectNull(clusterConfigType), diagnostics
	}

	clusterConfig := cluster.GetClusterConfiguration()

	// Initialize attribute values
	configValues := map[string]attr.Value{
		"custom_properties_json":   types.StringNull(),
		"computed_properties_json": types.StringNull(),
	}

	// Handle custom properties if present
	if clusterConfig.HasCustomProperties() {
		customPropsBytes, err := json.Marshal(clusterConfig.GetCustomProperties().AsMap())
		if err != nil {
			diagnostics.AddError("failed to marshal custom properties", "could not convert custom properties to JSON")
			return types.ObjectNull(clusterConfigType), diagnostics
		}
		configValues["custom_properties_json"] = types.StringValue(string(customPropsBytes))
	}

	// Handle computed properties if present
	if clusterConfig.HasComputedProperties() {
		computedPropsBytes, err := json.Marshal(clusterConfig.GetComputedProperties().AsMap())
		if err != nil {
			diagnostics.AddError("failed to marshal computed properties", "could not convert computed properties to JSON")
			return types.ObjectNull(clusterConfigType), diagnostics
		}
		configValues["computed_properties_json"] = types.StringValue(string(computedPropsBytes))
	}

	// Create the cluster configuration object
	obj, d := types.ObjectValue(clusterConfigType, configValues)
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate cluster configuration object", "could not create cluster configuration object")
		return types.ObjectNull(clusterConfigType), diagnostics
	}

	return obj, diagnostics
}
