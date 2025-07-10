package cluster

import (
	"context"
	"fmt"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/genproto/googleapis/type/dayofweek"
)

func generateClusterRequest(ctx context.Context, model models.Cluster, diags diag.Diagnostics) (*controlplanev1.ClusterCreate, diag.Diagnostics) {
	// Handle required fields first
	provider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		diags.AddError("unable to parse cloud provider", err.Error())
		return nil, diags
	}

	clusterType, err := utils.StringToClusterType(model.ClusterType.ValueString())
	if err != nil {
		diags.AddError("unable to parse cluster type", err.Error())
		return nil, diags
	}

	// Create base request with required fields
	output := &controlplanev1.ClusterCreate{
		Name:              model.Name.ValueString(),
		ResourceGroupId:   model.ResourceGroupID.ValueString(),
		ThroughputTier:    model.ThroughputTier.ValueString(),
		Type:              clusterType,
		ConnectionType:    utils.StringToConnectionType(model.ConnectionType.ValueString()),
		NetworkId:         model.NetworkID.ValueString(),
		CloudProvider:     provider,
		Region:            model.Region.ValueString(),
		Zones:             utils.TypeListToStringSlice(model.Zones),
		CloudProviderTags: utils.TypeMapToStringMap(model.Tags),
	}

	// Handle optional fields
	if !model.RedpandaVersion.IsNull() {
		rpVersion := model.RedpandaVersion.ValueString()
		output.RedpandaVersion = &rpVersion
	}

	// Handle KafkaAPI configuration
	if !model.KafkaAPI.IsNull() {
		m, d := getMtlsSpec(ctx, model.KafkaAPI, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.KafkaApi = &controlplanev1.KafkaAPISpec{
			Mtls: m,
		}
	}

	if !model.HTTPProxy.IsNull() {
		m, d := getMtlsSpec(ctx, model.HTTPProxy, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.HttpProxy = &controlplanev1.HTTPProxySpec{
			Mtls: m,
		}
	}

	if !model.SchemaRegistry.IsNull() {
		m, d := getMtlsSpec(ctx, model.SchemaRegistry, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.SchemaRegistry = &controlplanev1.SchemaRegistrySpec{
			Mtls: m,
		}
	}

	// Handle CustomerManagedResources
	if !model.CustomerManagedResources.IsNull() {
		cmr, d := generateClusterCMR(ctx, model, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.CustomerManagedResources = cmr
	}

	// Handle AWS PrivateLink
	if !model.AwsPrivateLink.IsNull() {
		m, d := getAwsPrivateLinkSpec(ctx, model.AwsPrivateLink, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.AwsPrivateLink = m
	}

	// Handle GCP Private Service Connect
	if !model.GcpPrivateServiceConnect.IsNull() {
		m, d := getGcpPrivateServiceConnect(ctx, model.GcpPrivateServiceConnect, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.GcpPrivateServiceConnect = m
	}

	// Handle Azure Private Link
	if !model.AzurePrivateLink.IsNull() {
		m, d := getAzurePrivateLinkSpec(ctx, model.AzurePrivateLink, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.AzurePrivateLink = m
	}

	if !model.GCPGlobalAccessEnabled.IsNull() {
		output.GcpPrivateServiceConnect.GlobalAccessEnabled = model.GCPGlobalAccessEnabled.ValueBool()
	}

	// Handle Maintenance Window
	if !model.MaintenanceWindowConfig.IsNull() {
		m, d := getMaintenanceWindowConfig(ctx, model.MaintenanceWindowConfig, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.MaintenanceWindowConfig = m
	}

	// Handle Kafka Connect
	if !model.KafkaConnect.IsNull() {
		m, d := getKafkaConnectConfig(ctx, model.KafkaConnect, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		output.KafkaConnect = m
	}

	// Handle Read Replica Cluster IDs
	if !model.ReadReplicaClusterIDs.IsNull() {
		output.ReadReplicaClusterIds = utils.TypeListToStringSlice(model.ReadReplicaClusterIDs)
	}

	return output, diags
}

func getGcpPrivateServiceConnect(_ context.Context, connect types.Object, diags diag.Diagnostics) (*controlplanev1.GCPPrivateServiceConnectSpec, diag.Diagnostics) {
	if connect.IsNull() {
		return nil, diags
	}

	enabled, d := getBoolFromAttributes("enabled", connect.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	globalAccessEnabled, d := getBoolFromAttributes("global_access_enabled", connect.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	// Get consumer accept list
	consumerList, d := getListFromAttributes("consumer_accept_list", types.StringType, connect.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	var consumers []*controlplanev1.GCPPrivateServiceConnectConsumer
	for _, elem := range consumerList.Elements() {
		// Each element should be an object with a "source" field
		consumerObj, ok := elem.(types.Object)
		if !ok {
			diags.AddError(
				"Invalid consumer accept list element",
				"Expected object type for consumer accept list element",
			)
			return nil, diags
		}

		sourceAttr := consumerObj.Attributes()["source"]
		sourceVal, ok := sourceAttr.(types.String)
		if !ok {
			diags.AddError(
				"Invalid source field",
				"Expected string type for source field in consumer accept list",
			)
			return nil, diags
		}

		consumers = append(consumers, &controlplanev1.GCPPrivateServiceConnectConsumer{
			Source: sourceVal.ValueString(),
		})
	}

	return &controlplanev1.GCPPrivateServiceConnectSpec{
		Enabled:             enabled,
		GlobalAccessEnabled: globalAccessEnabled,
		ConsumerAcceptList:  consumers,
	}, diags
}

func getAwsPrivateLinkSpec(_ context.Context, aws types.Object, diags diag.Diagnostics) (*controlplanev1.AWSPrivateLinkSpec, diag.Diagnostics) {
	enabled, d := getBoolFromAttributes("enabled", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}

	allowedPrincipals, d := getListFromAttributes("allowed_principals", types.StringType, aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}

	connectConsole, d := getBoolFromAttributes("connect_console", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}

	return &controlplanev1.AWSPrivateLinkSpec{
		Enabled:           enabled,
		AllowedPrincipals: utils.TypeListToStringSlice(allowedPrincipals),
		ConnectConsole:    connectConsole,
	}, diags
}

func generateClusterUpdate(ctx context.Context, cluster models.Cluster, diags diag.Diagnostics) (*controlplanev1.ClusterUpdate, diag.Diagnostics) {
	update := &controlplanev1.ClusterUpdate{
		Id:                    cluster.ID.ValueString(),
		Name:                  cluster.Name.ValueString(),
		ReadReplicaClusterIds: utils.TypeListToStringSlice(cluster.ReadReplicaClusterIDs),
	}

	// Handle KafkaAPI configuration
	if !cluster.KafkaAPI.IsNull() {
		m, d := getMtlsSpec(ctx, cluster.KafkaAPI, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.KafkaApi = &controlplanev1.KafkaAPISpec{
			Mtls: m,
		}
	}

	// Handle HTTPProxy configuration
	if !cluster.HTTPProxy.IsNull() {
		m, d := getMtlsSpec(ctx, cluster.HTTPProxy, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.HttpProxy = &controlplanev1.HTTPProxySpec{
			Mtls: m,
		}
	}

	// Handle SchemaRegistry configuration
	if !cluster.SchemaRegistry.IsNull() {
		m, d := getMtlsSpec(ctx, cluster.SchemaRegistry, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.SchemaRegistry = &controlplanev1.SchemaRegistrySpec{
			Mtls: m,
		}
	}

	// Handle AWS Private Link configuration
	if !cluster.AwsPrivateLink.IsNull() {
		m, d := getAwsPrivateLinkSpec(ctx, cluster.AwsPrivateLink, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.AwsPrivateLink = m
	}

	// Handle GCP Private Service Connect configuration
	if !cluster.GcpPrivateServiceConnect.IsNull() {
		m, d := getGcpPrivateServiceConnect(ctx, cluster.GcpPrivateServiceConnect, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.GcpPrivateServiceConnect = m
	}

	// Handle Azure Private Link configuration
	if !cluster.AzurePrivateLink.IsNull() {
		m, d := getAzurePrivateLinkSpec(ctx, cluster.AzurePrivateLink, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.AzurePrivateLink = m
	}

	// Handle CustomerManagedResources updates
	if !cluster.CustomerManagedResources.IsNull() {
		cmr, d := generateClusterCMRUpdate(ctx, cluster, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.CustomerManagedResources = cmr
	}

	// Handle KafkaConnect configuration
	if !cluster.KafkaConnect.IsNull() {
		m, d := getKafkaConnectConfig(ctx, cluster.KafkaConnect, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.KafkaConnect = m
	}

	// Handle MaintenanceWindow configuration
	if !cluster.MaintenanceWindowConfig.IsNull() {
		m, d := getMaintenanceWindowConfig(ctx, cluster.MaintenanceWindowConfig, diags)
		if d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		update.MaintenanceWindowConfig = m
	}

	// Handle cloud provider tags if present
	update.CloudProviderTags = utils.TypeMapToStringMap(cluster.Tags)

	return update, diags
}

func getMtlsSpec(ctx context.Context, mtls types.Object, diags diag.Diagnostics) (*controlplanev1.MTLSSpec, diag.Diagnostics) {
	if mtls.IsNull() {
		return nil, diags
	}
	m, d := getObjectFromAttributes(ctx, "mtls", mtlsType, mtls.Attributes(), diags)
	if d.HasError() {
		if utils.IsNotFoundSpec(d) {
			return nil, diags
		}
		diags.Append(d...)
		return nil, diags
	}

	en, d := getBoolFromAttributes("enabled", m.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	caCerts, d := getListFromAttributes("ca_certificates_pem", types.StringType, m.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	pr, d := getListFromAttributes("principal_mapping_rules", types.StringType, m.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	return &controlplanev1.MTLSSpec{
		Enabled:               en,
		CaCertificatesPem:     utils.TypeListToStringSlice(caCerts),
		PrincipalMappingRules: utils.TypeListToStringSlice(pr),
	}, diags
}

func generateClusterCMRUpdate(ctx context.Context, cluster models.Cluster, diags diag.Diagnostics) (*controlplanev1.CustomerManagedResourcesUpdate, diag.Diagnostics) {
	// Early returns if not applicable
	if cluster.CustomerManagedResources.IsNull() {
		return nil, diags
	}

	// Only supports GCP in the API so no point going past here if not gcp
	if cluster.CloudProvider.ValueString() != utils.CloudProviderStringGcp {
		return nil, diags
	}

	// Get the CMR object
	var cmrObj types.Object
	if d := cluster.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); d.HasError() {
		return nil, d
	}

	// Get the GCP object from CustomerManagedResources
	gcp, d := getObjectFromAttributes(ctx, "gcp", gcpType, cmrObj.Attributes(), diags)
	if d.HasError() {
		if !utils.IsNotFoundSpec(d) {
			return nil, d
		}
	}

	gcpUpdate := &controlplanev1.CustomerManagedResourcesUpdate_GCP{}
	if pscNatSubnetName, ok := gcp.Attributes()["psc_nat_subnet_name"].(types.String); ok && !pscNatSubnetName.IsNull() {
		gcpUpdate.PscNatSubnetName = pscNatSubnetName.ValueString()
	}

	// Create and return the update object
	return &controlplanev1.CustomerManagedResourcesUpdate{
		CloudProvider: &controlplanev1.CustomerManagedResourcesUpdate_Gcp{
			Gcp: gcpUpdate,
		},
	}, diags
}

func getAzurePrivateLinkSpec(_ context.Context, azure types.Object, diags diag.Diagnostics) (*controlplanev1.AzurePrivateLinkSpec, diag.Diagnostics) {
	if azure.IsNull() {
		return nil, diags
	}

	enabled, d := getBoolFromAttributes("enabled", azure.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	connectConsole, d := getBoolFromAttributes("connect_console", azure.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	allowedSubs, d := getListFromAttributes("allowed_subscriptions", types.StringType, azure.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	return &controlplanev1.AzurePrivateLinkSpec{
		Enabled:              enabled,
		ConnectConsole:       connectConsole,
		AllowedSubscriptions: utils.TypeListToStringSlice(allowedSubs),
	}, diags
}

func getMaintenanceWindowConfig(_ context.Context, maintenance types.Object, diags diag.Diagnostics) (*controlplanev1.MaintenanceWindowConfig, diag.Diagnostics) {
	if maintenance.IsNull() {
		return nil, diags
	}

	config := &controlplanev1.MaintenanceWindowConfig{}

	attrs := maintenance.Attributes()
	// Check each potential window type
	if dayHourAttr, ok := attrs["day_hour"].(types.Object); ok && !dayHourAttr.IsNull() {
		dayHourAttrs := dayHourAttr.Attributes()

		hourAttr, ok := dayHourAttrs["hour_of_day"].(types.Int32)
		if !ok {
			diags.AddError("hour_of_day not found", "hour_of_day is missing or malformed")
			return config, diags
		}
		dayAttr, ok := dayHourAttrs["day_of_week"].(types.String)
		if !ok {
			diags.AddError("day_of_week not found", "day_of_week is missing or malformed")
			return config, diags
		}

		wdw := &controlplanev1.MaintenanceWindowConfig_DayHour{}
		wdw.SetHourOfDay(hourAttr.ValueInt32())
		wdw.SetDayOfWeek(dayofweek.DayOfWeek(dayofweek.DayOfWeek_value[dayAttr.ValueString()]))
		config.Window = &controlplanev1.MaintenanceWindowConfig_DayHour_{
			DayHour: wdw,
		}
		return config, diags
	}

	if anytimeAttr, ok := attrs["anytime"].(types.Bool); ok && anytimeAttr.ValueBool() {
		config.Window = &controlplanev1.MaintenanceWindowConfig_Anytime_{
			Anytime: &controlplanev1.MaintenanceWindowConfig_Anytime{},
		}
		return config, diags
	}

	if unspecAttr, ok := attrs["unspecified"].(types.Bool); ok && unspecAttr.ValueBool() {
		config.Window = &controlplanev1.MaintenanceWindowConfig_Unspecified_{
			Unspecified: &controlplanev1.MaintenanceWindowConfig_Unspecified{},
		}
		return config, diags
	}

	return nil, diags
}

func getKafkaConnectConfig(_ context.Context, connect types.Object, diags diag.Diagnostics) (*controlplanev1.KafkaConnect, diag.Diagnostics) {
	if connect.IsNull() {
		return nil, diags
	}

	enabled, d := getBoolFromAttributes("enabled", connect.Attributes(), diags)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}
	return &controlplanev1.KafkaConnect{
		Enabled: enabled,
	}, diags
}

func generateClusterCMR(ctx context.Context, model models.Cluster, diags diag.Diagnostics) (*controlplanev1.CustomerManagedResources, diag.Diagnostics) {
	cmr := &controlplanev1.CustomerManagedResources{}

	if model.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	switch model.CloudProvider.ValueString() {
	case "aws":
		aws, d := generateClusterCMRAWS(ctx, model, diags)
		if d.HasError() {
			return nil, d
		}
		cmr.SetAws(aws)
		return cmr, diags
	case "gcp":
		gcp, d := generateClusterCMRGCP(ctx, model, diags)
		if d.HasError() {
			return nil, d
		}
		cmr.SetGcp(gcp)
		return cmr, diags
	case "azure":
		diags.AddError("Azure BYOVPC is not supported", "Azure BYOVPC is not supported")
		return nil, diags
	default:
		return nil, nil
	}
}

func generateClusterCMRGCP(ctx context.Context, model models.Cluster, diags diag.Diagnostics) (*controlplanev1.CustomerManagedResources_GCP, diag.Diagnostics) {
	gcpRet := &controlplanev1.CustomerManagedResources_GCP{
		Subnet:                        &controlplanev1.CustomerManagedResources_GCP_Subnet{},
		AgentServiceAccount:           &controlplanev1.GCPServiceAccount{},
		ConsoleServiceAccount:         &controlplanev1.GCPServiceAccount{},
		ConnectorServiceAccount:       &controlplanev1.GCPServiceAccount{},
		RedpandaClusterServiceAccount: &controlplanev1.GCPServiceAccount{},
		GkeServiceAccount:             &controlplanev1.GCPServiceAccount{},
		TieredStorageBucket:           &controlplanev1.CustomerManagedGoogleCloudStorageBucket{},
	}

	// Get the GCP object from CustomerManagedResources
	var cmrObj types.Object
	if d := model.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); d.HasError() {
		return nil, d
	}

	gcp, d := getObjectFromAttributes(ctx, "gcp", gcpType, cmrObj.Attributes(), diags)
	if d.HasError() {
		if !utils.IsNotFoundSpec(d) {
			return nil, d
		}
	}

	// Get subnet configuration
	subnet, d := getObjectFromAttributes(ctx, "subnet", gcpSubnetType, gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}

	// Subnet name
	subnetName, d := getStringValue("name", subnet.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.Subnet.Name = subnetName

	// Secondary IPv4 range for pods
	podsRange, d := getObjectFromAttributes(ctx, "secondary_ipv4_range_pods", gcpSecondaryIPv4RangeType, subnet.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	podsRangeName, d := getStringValue("name", podsRange.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.Subnet.SecondaryIpv4RangePods = &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
		Name: podsRangeName,
	}

	// Secondary IPv4 range for services
	servicesRange, d := getObjectFromAttributes(ctx, "secondary_ipv4_range_services", gcpSecondaryIPv4RangeType, subnet.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	servicesRangeName, d := getStringValue("name", servicesRange.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.Subnet.SecondaryIpv4RangeServices = &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
		Name: servicesRangeName,
	}

	// K8s master IPv4 range
	k8sMasterRange, d := getStringValue("k8s_master_ipv4_range", subnet.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.Subnet.K8SMasterIpv4Range = k8sMasterRange

	// Service accounts
	agentEmail, d := getServiceAccountEmail(ctx, "agent_service_account", gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.AgentServiceAccount.Email = agentEmail

	consoleEmail, d := getServiceAccountEmail(ctx, "console_service_account", gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.ConsoleServiceAccount.Email = consoleEmail

	connectorEmail, d := getServiceAccountEmail(ctx, "connector_service_account", gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.ConnectorServiceAccount.Email = connectorEmail

	clusterEmail, d := getServiceAccountEmail(ctx, "redpanda_cluster_service_account", gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.RedpandaClusterServiceAccount.Email = clusterEmail

	gkeEmail, d := getServiceAccountEmail(ctx, "gke_service_account", gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.GkeServiceAccount.Email = gkeEmail

	// Tiered storage bucket
	bucketObj, d := getObjectFromAttributes(ctx, "tiered_storage_bucket", gcpBucketType, gcp.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	bucketName, d := getStringValue("name", bucketObj.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	gcpRet.TieredStorageBucket.Name = bucketName

	// Optional: PSC NAT subnet name
	if pscSubnetName, ok := gcp.Attributes()["psc_nat_subnet_name"].(types.String); ok && !pscSubnetName.IsNull() {
		gcpRet.PscNatSubnetName = pscSubnetName.ValueString()
	}

	return gcpRet, nil
}

// Helper to get service account email from nested object
func getServiceAccountEmail(ctx context.Context, key string, attributes map[string]attr.Value, diags diag.Diagnostics) (string, diag.Diagnostics) {
	serviceAccount, d := getObjectFromAttributes(ctx, key, gcpServiceAccountType, attributes, diags)
	if d.HasError() {
		return "", d
	}

	email, d := getStringValue("email", serviceAccount.Attributes(), diags)
	if d.HasError() {
		return "", d
	}

	return email, diags
}

// Helper to get string value directly
func getStringValue(key string, attributes map[string]attr.Value, diags diag.Diagnostics) (string, diag.Diagnostics) {
	if val, ok := attributes[key].(types.String); ok {
		return val.ValueString(), diags
	}

	diags.AddError(fmt.Sprintf("%s not found", key), "string value is missing or malformed")
	return "", diags
}

func generateClusterCMRAWS(ctx context.Context, model models.Cluster, diags diag.Diagnostics) (*controlplanev1.CustomerManagedResources_AWS, diag.Diagnostics) {
	awsRet := &controlplanev1.CustomerManagedResources_AWS{
		AgentInstanceProfile:               &controlplanev1.AWSInstanceProfile{},
		ConnectorsNodeGroupInstanceProfile: &controlplanev1.AWSInstanceProfile{},
		UtilityNodeGroupInstanceProfile:    &controlplanev1.AWSInstanceProfile{},
		RedpandaNodeGroupInstanceProfile:   &controlplanev1.AWSInstanceProfile{},
		K8SClusterRole:                     &controlplanev1.CustomerManagedResources_AWS_Role{},
		RedpandaAgentSecurityGroup:         &controlplanev1.AWSSecurityGroup{},
		ConnectorsSecurityGroup:            &controlplanev1.AWSSecurityGroup{},
		RedpandaNodeGroupSecurityGroup:     &controlplanev1.AWSSecurityGroup{},
		UtilitySecurityGroup:               &controlplanev1.AWSSecurityGroup{},
		ClusterSecurityGroup:               &controlplanev1.AWSSecurityGroup{},
		NodeSecurityGroup:                  &controlplanev1.AWSSecurityGroup{},
		CloudStorageBucket:                 &controlplanev1.CustomerManagedAWSCloudStorageBucket{},
		PermissionsBoundaryPolicy:          &controlplanev1.CustomerManagedResources_AWS_Policy{},
	}

	// Get the AWS object from CustomerManagedResources
	var cmrObj types.Object
	if d := model.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); d.HasError() {
		return nil, d
	}

	aws, d := getObjectFromAttributes(ctx, "aws", awsType, cmrObj.Attributes(), diags)
	if d.HasError() {
		if !utils.IsNotFoundSpec(d) {
			return nil, d
		}
	}

	// Agent instance profile
	agentProfileArn, d := getStringFromAttributes("agent_instance_profile", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.AgentInstanceProfile.Arn = agentProfileArn

	// Connectors node group instance profile
	connectorsProfileArn, d := getStringFromAttributes("connectors_node_group_instance_profile", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.ConnectorsNodeGroupInstanceProfile.Arn = connectorsProfileArn

	// Utility node group instance profile
	utilityProfileArn, d := getStringFromAttributes("utility_node_group_instance_profile", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.UtilityNodeGroupInstanceProfile.Arn = utilityProfileArn

	// Redpanda node group instance profile
	redpandaProfileArn, d := getStringFromAttributes("redpanda_node_group_instance_profile", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.RedpandaNodeGroupInstanceProfile.Arn = redpandaProfileArn

	// K8s cluster role
	k8sRoleArn, d := getStringFromAttributes("k8s_cluster_role", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.K8SClusterRole.Arn = k8sRoleArn

	policyArn, d := getStringFromAttributes("permissions_boundary_policy", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.PermissionsBoundaryPolicy.Arn = policyArn

	// Security groups
	agentSecurityGroupArn, d := getStringFromAttributes("redpanda_agent_security_group", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.RedpandaAgentSecurityGroup.Arn = agentSecurityGroupArn

	connectorsSecurityGroupArn, d := getStringFromAttributes("connectors_security_group", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.ConnectorsSecurityGroup.Arn = connectorsSecurityGroupArn

	redpandaNodeGroupSecurityGroupArn, d := getStringFromAttributes("redpanda_node_group_security_group", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.RedpandaNodeGroupSecurityGroup.Arn = redpandaNodeGroupSecurityGroupArn

	utilitySecurityGroupArn, d := getStringFromAttributes("utility_security_group", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.UtilitySecurityGroup.Arn = utilitySecurityGroupArn

	clusterSecurityGroupArn, d := getStringFromAttributes("cluster_security_group", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.ClusterSecurityGroup.Arn = clusterSecurityGroupArn

	nodeSecurityGroupArn, d := getStringFromAttributes("node_security_group", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.NodeSecurityGroup.Arn = nodeSecurityGroupArn

	// Cloud storage bucket
	bucketArn, d := getStringFromAttributes("cloud_storage_bucket", aws.Attributes(), diags)
	if d.HasError() {
		return nil, d
	}
	awsRet.CloudStorageBucket.Arn = bucketArn

	return awsRet, nil
}
