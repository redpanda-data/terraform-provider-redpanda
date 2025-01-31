package cluster

import (
	"time"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(cfg models.Cluster, cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (*models.Cluster, diag.Diagnostics) {
	output := &models.Cluster{
		Name:                  types.StringValue(cluster.Name),
		ID:                    types.StringValue(cluster.Id),
		ConnectionType:        types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:         types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:           types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion:       cfg.RedpandaVersion,
		ThroughputTier:        types.StringValue(cluster.ThroughputTier),
		Region:                types.StringValue(cluster.Region),
		AllowDeletion:         cfg.AllowDeletion,
		Tags:                  cfg.Tags,
		ResourceGroupID:       types.StringValue(cluster.ResourceGroupId),
		NetworkID:             types.StringValue(cluster.NetworkId),
		ReadReplicaClusterIDs: utils.StringSliceToTypeList(cluster.ReadReplicaClusterIds),
		Zones:                 utils.StringSliceToTypeList(cluster.Zones),
		State:                 types.StringValue(cluster.State.String()),
	}
	if cluster.HasCreatedAt() {
		output.CreatedAt = types.StringValue(cluster.CreatedAt.AsTime().Format(time.RFC3339))
	}

	if cluster.HasUpdatedAt() {
		output.UpdatedAt = types.StringValue(cluster.UpdatedAt.AsTime().Format(time.RFC3339))
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

	kafkaAPI, d := generateModelKafkaAPI(cluster, output, diagnostics)
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

	connectivity, d := generateModelConnectivity(cluster, diagnostics)
	if d.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.Connectivity = connectivity

	cmr, dg := generateModelCMR(cluster.CloudProvider.String(), cluster, diagnostics)
	if dg.HasError() {
		diagnostics.Append(d...)
		return nil, diagnostics
	}
	output.CustomerManagedResources = cmr

	return output, nil
}

func generateModelStateDescription(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
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

func generateModelConnectivity(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasConnectivity() {
		return types.ObjectNull(connectivityType), diagnostics
	}

	connectivity := cluster.GetConnectivity()

	// Handle the oneof cloud_provider field
	gcpObj := types.ObjectNull(connectivityGCPType)
	var d diag.Diagnostics
	if utils.CloudProviderToString(cluster.GetCloudProvider()) == utils.CloudProviderStringGcp {
		if gcp := connectivity.GetGcp(); gcp != nil {
			gcpObj, d = types.ObjectValue(connectivityGCPType, map[string]attr.Value{
				"enable_global_access": types.BoolValue(gcp.GetEnableGlobalAccess()),
			})
			if d.HasError() {
				diagnostics.Append(d...)
				diagnostics.AddError("failed to generate GCP connectivity detail", "could not create GCP object")
				return types.ObjectNull(connectivityType), diagnostics
			}
		}
	}

	// Create the connectivity object
	obj, d := types.ObjectValue(connectivityType, map[string]attr.Value{
		"gcp": gcpObj,
	})
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate connectivity object", "could not create connectivity object")
		return types.ObjectNull(connectivityType), diagnostics
	}

	return obj, diagnostics
}

func generateModelKafkaConnect(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasKafkaConnect() {
		return types.ObjectNull(kafkaConnectType), diagnostics
	}

	kafkaConnect := cluster.GetKafkaConnect()

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

func generateModelPrometheus(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
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

func generateModelSchemaRegistry(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
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

func generateModelRedpandaConsole(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
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

func getMtlsModel(mtls *controlplanev1beta2.MTLSSpec, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	mtlsValue := map[string]attr.Value{
		"enabled":                 types.BoolNull(),
		"ca_certificates_pem":     types.ListNull(types.StringType),
		"principal_mapping_rules": types.ListNull(types.StringType),
	}
	if mtls != nil {
		mtlsValue["enabled"] = types.BoolValue(mtls.GetEnabled())
		mtlsValue["ca_certificates_pem"] = utils.StringSliceToTypeList(mtls.GetCaCertificatesPem())
		mtlsValue["principal_mapping_rules"] = utils.StringSliceToTypeList(mtls.GetPrincipalMappingRules())
	}
	out, d := types.ObjectValue(mtlsType, mtlsValue)
	if d.HasError() {
		diagnostics.Append(d...)
		diagnostics.AddError("failed to generate mtls object", "failed to generate mtls object")
		return types.ObjectNull(mtlsType), diagnostics
	}
	return out, diagnostics
}

func generateModelKafkaAPI(cluster *controlplanev1beta2.Cluster, output *models.Cluster, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasKafkaApi() {
		output.KafkaAPI = types.ObjectNull(kafkaAPIType)
		return types.ObjectNull(kafkaAPIType), diags
	}

	kafkaApi := cluster.GetKafkaApi()

	var seedBrokers types.List
	if sb := kafkaApi.GetSeedBrokers(); sb != nil {
		seedBrokers = utils.StringSliceToTypeList(sb)
	}
	mtls, d := getMtlsModel(kafkaApi.GetMtls(), diags)
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}

	obj, d := types.ObjectValue(kafkaAPIType, map[string]attr.Value{
		"mtls":         mtls,
		"seed_brokers": seedBrokers,
	})
	if d.HasError() {
		return types.ObjectNull(kafkaAPIType), d
	}
	return obj, diags
}

func generateModelAWSPrivateLink(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasAwsPrivateLink() {
		return types.ObjectNull(awsPrivateLinkType), diagnostics
	}

	awsPrivateLink := cluster.GetAwsPrivateLink()

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

func generateModelGCPPrivateServiceConnect(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasGcpPrivateServiceConnect() {
		return types.ObjectNull(gcpPrivateServiceConnectType), diagnostics
	}

	gcpPSC := cluster.GetGcpPrivateServiceConnect()

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

		// Build status object - all fields optional
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
func generateUpdateRequest(plan, state models.Cluster, diags diag.Diagnostics) (*controlplanev1beta2.UpdateClusterRequest, diag.Diagnostics) {
	planUpdate, ds := generateClusterUpdate(plan, diags)
	if ds.HasError() {
		diags.Append(ds...)
		return nil, diags
	}
	stateUpdate, dg := generateClusterUpdate(state, diags)
	if dg.HasError() {
		diags.Append(dg...)
		return nil, diags
	}

	update, fieldmask := utils.GenerateProtobufDiffAndUpdateMask(planUpdate, stateUpdate)
	update.Id = planUpdate.Id
	return &controlplanev1beta2.UpdateClusterRequest{
		Cluster:    update,
		UpdateMask: fieldmask,
	}, nil
}

func generateModelMaintenanceWindow(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
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

	if maintenance.HasDayHour() {
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
	} else if maintenance.HasAnytime() {
		unspec = types.BoolValue(true)
	} else if maintenance.HasUnspecified() {
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

func generateModelHTTPProxy(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
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

func generateModelAzurePrivateLink(cluster *controlplanev1beta2.Cluster, diagnostics diag.Diagnostics) (types.Object, diag.Diagnostics) {
	if !cluster.HasAzurePrivateLink() {
		return types.ObjectNull(azurePrivateLinkType), diagnostics
	}

	azurePrivateLink := cluster.GetAzurePrivateLink()

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

func generateModelCMR(cloudProvider string, cluster *controlplanev1beta2.Cluster, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	// Early return conditions
	if cluster == nil || !cluster.HasCustomerManagedResources() {
		return types.ObjectNull(cmrType), diags
	}

	if cluster.Type != controlplanev1beta2.Cluster_TYPE_BYOC {
		diags.AddError("Customer Managed Resources with non-BYOC cluster type", "Customer Managed Resources are only supported for BYOC clusters")
		return types.ObjectNull(cmrType), diags
	}

	switch cloudProvider {
	case "aws":
		if !cluster.CustomerManagedResources.HasAws() {
			diags.AddError("Cloud Provider Mismatch", "AWS customer managed resources are missing for AWS BYOVPC Cluster")
			return types.ObjectNull(cmrType), diags
		}

		// Get AWS data
		awsData := cluster.GetCustomerManagedResources().GetAws()

		// Initialize AWS attributes with null values
		awsVal := make(map[string]attr.Value)

		// Create null objects for all AWS fields
		instanceProfileFields := []string{
			"agent_instance_profile",
			"connectors_node_group_instance_profile",
			"utility_node_group_instance_profile",
			"redpanda_node_group_instance_profile",
		}

		securityGroupFields := []string{
			"redpanda_agent_security_group",
			"connectors_security_group",
			"redpanda_node_group_security_group",
			"utility_security_group",
			"cluster_security_group",
			"node_security_group",
		}

		// Initialize all fields with null objects
		for _, field := range instanceProfileFields {
			awsVal[field] = types.ObjectNull(singleElementContainer)
		}
		for _, field := range securityGroupFields {
			awsVal[field] = types.ObjectNull(singleElementContainer)
		}

		// Initialize remaining fields
		awsVal["k8s_cluster_role"] = types.ObjectNull(singleElementContainer)
		awsVal["cloud_storage_bucket"] = types.ObjectNull(singleElementContainer)
		awsVal["permissions_boundary_policy"] = types.ObjectNull(singleElementContainer)

		// Now set values for fields that exist in the input
		if awsData.HasAgentInstanceProfile() {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.AgentInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["agent_instance_profile"] = obj
		}

		if awsData.ConnectorsNodeGroupInstanceProfile != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ConnectorsNodeGroupInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["connectors_node_group_instance_profile"] = obj
		}

		if awsData.UtilityNodeGroupInstanceProfile != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.UtilityNodeGroupInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["utility_node_group_instance_profile"] = obj
		}

		if awsData.RedpandaNodeGroupInstanceProfile != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.RedpandaNodeGroupInstanceProfile.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["redpanda_node_group_instance_profile"] = obj
		}

		if awsData.K8SClusterRole != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.K8SClusterRole.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["k8s_cluster_role"] = obj
		}

		if awsData.RedpandaAgentSecurityGroup != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.RedpandaAgentSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["redpanda_agent_security_group"] = obj
		}

		if awsData.ConnectorsSecurityGroup != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ConnectorsSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["connectors_security_group"] = obj
		}

		if awsData.RedpandaNodeGroupSecurityGroup != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.RedpandaNodeGroupSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["redpanda_node_group_security_group"] = obj
		}

		if awsData.UtilitySecurityGroup != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.UtilitySecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["utility_security_group"] = obj
		}

		if awsData.ClusterSecurityGroup != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ClusterSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["cluster_security_group"] = obj
		}

		if awsData.NodeSecurityGroup != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.NodeSecurityGroup.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["node_security_group"] = obj
		}

		if awsData.CloudStorageBucket != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.CloudStorageBucket.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["cloud_storage_bucket"] = obj
		}

		if awsData.PermissionsBoundaryPolicy != nil {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.PermissionsBoundaryPolicy.Arn),
			})
			if d.HasError() {
				diags.Append(d...)
				return types.ObjectNull(cmrType), diags
			}
			awsVal["permissions_boundary_policy"] = obj
		}

		// Create AWS object
		awsObj, d := types.ObjectValue(awsType, awsVal)
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(cmrType), diags
		}

		// Create final CMR object
		cmrObj, d := types.ObjectValue(cmrType, map[string]attr.Value{
			"aws": awsObj,
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(cmrType), diags
		}

		return cmrObj, diags

	case "gcp":
		// TODO: Implement GCP support
		return types.ObjectNull(cmrType), diags
	}

	return types.ObjectNull(cmrType), diags
}
