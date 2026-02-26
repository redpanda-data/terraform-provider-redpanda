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

// Package cluster contains the model for the cluster resource.
package cluster

import (
	"context"
	"encoding/json"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/genproto/googleapis/type/dayofweek"
	"google.golang.org/protobuf/types/known/structpb"
)

// GetID returns the cluster ID.
func (r *ResourceModel) GetID() string {
	return r.ID.ValueString()
}

// GenerateMinimalResourceModel creates a minimal ResourceModel with only enough state for Terraform to
// track an existing cluster and to delete it, if necessary. Used in creation to track
// partially created clusters, and on reading to null out cluster that are found in the
// deleting state and force them to be recreated.
func GenerateMinimalResourceModel(clusterID string, timeout timeouts.Value) *ResourceModel {
	return &ResourceModel{
		ID:                       types.StringValue(clusterID),
		Tags:                     types.MapNull(types.StringType),
		Name:                     types.StringNull(),
		ConnectionType:           types.StringNull(),
		CloudProvider:            types.StringNull(),
		ClusterType:              types.StringNull(),
		RedpandaVersion:          types.StringNull(),
		ThroughputTier:           types.StringNull(),
		Region:                   types.StringNull(),
		ResourceGroupID:          types.StringNull(),
		NetworkID:                types.StringNull(),
		ClusterAPIURL:            types.StringNull(),
		State:                    types.StringNull(),
		CreatedAt:                types.StringNull(),
		CurrentRedpandaVersion:   types.StringNull(),
		DesiredRedpandaVersion:   types.StringNull(),
		APIGatewayAccess:         types.StringNull(),
		GCPGlobalAccessEnabled:   types.BoolNull(),
		AllowDeletion:            types.BoolValue(true),
		ReadReplicaClusterIds:    types.ListNull(types.StringType),
		NatGateways:              types.ListNull(types.StringType),
		Zones:                    types.ListNull(types.StringType),
		Prometheus:               types.ObjectNull(getPrometheusType()),
		CustomerManagedResources: types.ObjectNull(getCustomerManagedResourcesType()),
		KafkaAPI:                 types.ObjectNull(getKafkaAPIType()),
		HTTPProxy:                types.ObjectNull(getHTTPProxyType()),
		SchemaRegistry:           types.ObjectNull(getSchemaRegistryType()),
		AWSPrivateLink:           types.ObjectNull(getAwsPrivateLinkType()),
		GCPPrivateServiceConnect: types.ObjectNull(getGcpPrivateServiceConnectType()),
		AzurePrivateLink:         types.ObjectNull(getAzurePrivateLinkType()),
		RedpandaConsole:          types.ObjectNull(getRedpandaConsoleType()),
		StateDescription:         types.ObjectNull(getStateDescriptionType()),
		MaintenanceWindowConfig:  types.ObjectNull(getMaintenanceWindowConfigType()),
		KafkaConnect:             types.ObjectNull(getKafkaConnectType()),
		ClusterConfiguration:     types.ObjectNull(getClusterConfigurationType()),
		CloudStorage:             types.ObjectNull(getCloudStorageType()),
		Timeouts:                 timeout,
	}
}

// GetUpdatedModel populates the ResourceModel from a protobuf cluster response
func (r *ResourceModel) GetUpdatedModel(ctx context.Context, cluster *controlplanev1.Cluster, contingent ContingentFields) (*ResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	r.Name = types.StringValue(cluster.GetName())
	r.ID = types.StringValue(cluster.GetId())
	r.ConnectionType = types.StringValue(utils.ConnectionTypeToString(cluster.GetConnectionType()))
	r.CloudProvider = types.StringValue(utils.CloudProviderToString(cluster.GetCloudProvider()))
	r.ClusterType = types.StringValue(utils.ClusterTypeToString(cluster.GetType()))
	r.ThroughputTier = types.StringValue(cluster.GetThroughputTier())
	r.RedpandaNodeCount = types.Int32Value(cluster.GetRedpandaNodeCount())
	r.Region = types.StringValue(cluster.GetRegion())
	r.ResourceGroupID = types.StringValue(cluster.GetResourceGroupId())
	r.NetworkID = types.StringValue(cluster.GetNetworkId())
	r.State = types.StringValue(cluster.GetState().String())

	r.Zones = utils.StringSliceToTypeList(cluster.GetZones())
	r.ReadReplicaClusterIds = utils.StringSliceToTypeList(cluster.GetReadReplicaClusterIds())

	// Set contingent fields from either model or API
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

	r.CurrentRedpandaVersion = types.StringValue(cluster.GetCurrentRedpandaVersion())
	r.DesiredRedpandaVersion = types.StringValue(cluster.GetDesiredRedpandaVersion())
	r.NatGateways = utils.StringSliceToTypeList(cluster.GetNatGateways())
	r.APIGatewayAccess = types.StringValue(cluster.GetApiGatewayAccess().String())

	if cluster.HasDataplaneApi() {
		r.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}

	if stateDesc, d := r.generateModelStateDescription(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.StateDescription = stateDesc
	}

	if awsPrivateLink, d := r.generateModelAwsPrivateLink(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.AWSPrivateLink = awsPrivateLink
	}

	if gcpPsc, d := r.generateModelGcpPrivateServiceConnect(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.GCPPrivateServiceConnect = gcpPsc
	}

	if azurePrivateLink, d := r.generateModelAzurePrivateLink(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.AzurePrivateLink = azurePrivateLink
	}

	if kafkaAPI, d := r.generateModelKafkaAPI(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.KafkaAPI = kafkaAPI
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

	if cmr, d := r.generateModelCustomerManagedResources(ctx, cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.CustomerManagedResources = cmr
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

	if clusterConfiguration, d := r.generateModelClusterConfiguration(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.ClusterConfiguration = clusterConfiguration
	}

	if cloudStorage, d := r.generateModelCloudStorage(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.CloudStorage = cloudStorage
	}

	return r, diags
}

// GetClusterCreate composes a Cluster Create request from the model data
func (r *ResourceModel) GetClusterCreate(ctx context.Context) (*controlplanev1.ClusterCreate, diag.Diagnostics) {
	var diags diag.Diagnostics

	provider, err := utils.StringToCloudProvider(r.CloudProvider.ValueString())
	if err != nil {
		diags.AddError("unable to parse cloud provider", err.Error())
		return nil, diags
	}

	clusterType, err := utils.StringToClusterType(r.ClusterType.ValueString())
	if err != nil {
		diags.AddError("unable to parse cluster type", err.Error())
		return nil, diags
	}

	if diags.HasError() {
		return nil, diags
	}

	output := &controlplanev1.ClusterCreate{
		Name:              r.Name.ValueString(),
		ResourceGroupId:   r.ResourceGroupID.ValueString(),
		ThroughputTier:    r.ThroughputTier.ValueString(),
		Type:              clusterType,
		ConnectionType:    utils.StringToConnectionType(r.ConnectionType.ValueString()),
		NetworkId:         r.NetworkID.ValueString(),
		CloudProvider:     provider,
		Region:            r.Region.ValueString(),
		Zones:             utils.TypeListToStringSlice(r.Zones),
		CloudProviderTags: utils.TypeMapToStringMap(r.Tags),
	}

	if !r.RedpandaVersion.IsNull() {
		rpVersion := r.RedpandaVersion.ValueString()
		output.RedpandaVersion = &rpVersion
	}

	if !r.RedpandaNodeCount.IsNull() && !r.RedpandaNodeCount.IsUnknown() {
		output.RedpandaNodeCount = r.RedpandaNodeCount.ValueInt32()
	}

	if !r.KafkaAPI.IsNull() {
		kafkaSpec, d := r.generateClusterKafkaAPISpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.KafkaApi = kafkaSpec
	}

	if !r.HTTPProxy.IsNull() {
		proxySpec, d := r.generateClusterHTTPProxySpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.HttpProxy = proxySpec
	}

	if !r.SchemaRegistry.IsNull() {
		schemaSpec, d := r.generateClusterSchemaRegistrySpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.SchemaRegistry = schemaSpec
	}

	if !r.KafkaConnect.IsNull() {
		connectSpec, d := r.generateClusterKafkaConnectSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.KafkaConnect = connectSpec //nolint:staticcheck // Field is deprecated but still supported
	}

	if !r.CustomerManagedResources.IsNull() {
		cmr, d := r.generateClusterCMR(ctx)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("error attempting to generate Cluster Customer Managed Resources", "")
		}
		output.CustomerManagedResources = cmr
	}

	if !r.AWSPrivateLink.IsNull() {
		awsSpec, d := r.generateClusterAwsPrivateLinkSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.AwsPrivateLink = awsSpec
	}

	if !r.GCPPrivateServiceConnect.IsNull() {
		gcpSpec, d := r.generateClusterGcpPrivateServiceConnectSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.GcpPrivateServiceConnect = gcpSpec
	}

	if !r.AzurePrivateLink.IsNull() {
		azureSpec, d := r.generateClusterAzurePrivateLinkSpec()
		if d.HasError() {
			diags.Append(d...)
		}
		output.AzurePrivateLink = azureSpec
	}

	if !r.MaintenanceWindowConfig.IsNull() {
		mwSpec, d := r.generateClusterMaintenanceWindow()
		if d.HasError() {
			diags.Append(d...)
		}
		output.MaintenanceWindowConfig = mwSpec
	}

	if !r.GCPGlobalAccessEnabled.IsNull() {
		output.GcpEnableGlobalAccess = r.GCPGlobalAccessEnabled.ValueBool()
	}
	if !r.ClusterConfiguration.IsNull() {
		ccCr, d := r.generateClusterClusterConfiguration()
		if d.HasError() {
			diags.Append(d...)
		}
		output.ClusterConfiguration = &controlplanev1.ClusterCreate_ClusterConfiguration{
			CustomProperties: ccCr,
		}
	}

	if !r.APIGatewayAccess.IsNull() && !r.APIGatewayAccess.IsUnknown() {
		output.ApiGatewayAccess = utils.StringToNetworkAccessMode(r.APIGatewayAccess.ValueString())
	}

	if !r.CloudStorage.IsNull() {
		cs, d := r.generateClusterCloudStorageCreate()
		if d.HasError() {
			diags.Append(d...)
		}
		output.CloudStorage = cs
	}

	return output, diags
}

func (r *ResourceModel) getClusterUpdate(ctx context.Context) (*controlplanev1.ClusterUpdate, diag.Diagnostics) {
	var diags diag.Diagnostics

	update := &controlplanev1.ClusterUpdate{
		Id:   r.ID.ValueString(),
		Name: r.Name.ValueString(),
	}

	if !r.ReadReplicaClusterIds.IsNull() {
		update.ReadReplicaClusterIds = utils.TypeListToStringSlice(r.ReadReplicaClusterIds)
	}

	if !r.Tags.IsNull() {
		update.CloudProviderTags = utils.TypeMapToStringMap(r.Tags)
	}

	if !r.ThroughputTier.IsNull() {
		update.ThroughputTier = r.ThroughputTier.ValueString()
	}

	if !r.RedpandaNodeCount.IsNull() && !r.RedpandaNodeCount.IsUnknown() {
		update.RedpandaNodeCount = r.RedpandaNodeCount.ValueInt32()
	}

	if !r.KafkaAPI.IsNull() {
		kafkaSpec, d := r.generateClusterKafkaAPISpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.KafkaApi = kafkaSpec
	}

	if !r.HTTPProxy.IsNull() {
		proxySpec, d := r.generateClusterHTTPProxySpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.HttpProxy = proxySpec
	}

	if !r.SchemaRegistry.IsNull() {
		schemaSpec, d := r.generateClusterSchemaRegistrySpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.SchemaRegistry = schemaSpec
	}

	if !r.KafkaConnect.IsNull() {
		connectSpec, d := r.generateClusterKafkaConnectSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.KafkaConnect = connectSpec //nolint:staticcheck // Field is deprecated but still supported
	}

	if !r.AWSPrivateLink.IsNull() {
		awsSpec, d := r.generateClusterAwsPrivateLinkSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.AwsPrivateLink = awsSpec
	}

	if !r.GCPPrivateServiceConnect.IsNull() {
		gcpSpec, d := r.generateClusterGcpPrivateServiceConnectSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.GcpPrivateServiceConnect = gcpSpec
	}

	if !r.AzurePrivateLink.IsNull() {
		azureSpec, d := r.generateClusterAzurePrivateLinkSpec()
		if d.HasError() {
			diags.Append(d...)
		}
		update.AzurePrivateLink = azureSpec
	}

	if !r.MaintenanceWindowConfig.IsNull() {
		mwSpec, d := r.generateClusterMaintenanceWindow()
		if d.HasError() {
			diags.Append(d...)
		}
		update.MaintenanceWindowConfig = mwSpec
	}

	if !r.CustomerManagedResources.IsNull() {
		cmrUpdate, d := r.generateClusterCMRUpdate(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.CustomerManagedResources = cmrUpdate
	}

	if !r.ClusterConfiguration.IsNull() {
		ccUp, d := r.generateClusterClusterConfiguration()
		if d.HasError() {
			diags.Append(d...)
		}
		update.ClusterConfiguration = &controlplanev1.ClusterUpdate_ClusterConfiguration{
			CustomProperties: ccUp,
		}
	}

	if !r.APIGatewayAccess.IsNull() && !r.APIGatewayAccess.IsUnknown() {
		update.ApiGatewayAccess = utils.StringToNetworkAccessMode(r.APIGatewayAccess.ValueString())
	}

	return update, diags
}

// GetClusterUpdateRequest generates an UpdateClusterRequest with field mask from the current state and a previous state for comparison
func (r *ResourceModel) GetClusterUpdateRequest(ctx context.Context, previousState *ResourceModel) (*controlplanev1.UpdateClusterRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	planUpdate, d := r.getClusterUpdate(ctx)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	stateUpdate, d := previousState.getClusterUpdate(ctx)
	if d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	update, fieldMask := utils.GenerateProtobufDiffAndUpdateMask(planUpdate, stateUpdate)

	update.Id = planUpdate.Id

	return &controlplanev1.UpdateClusterRequest{
		Cluster:    update,
		UpdateMask: fieldMask,
	}, diags
}

func (r *ResourceModel) generateClusterCMRUpdate(ctx context.Context) (*controlplanev1.CustomerManagedResourcesUpdate, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	cmrUpdate := &controlplanev1.CustomerManagedResourcesUpdate{}

	switch r.CloudProvider.ValueString() {
	case "aws":
		return nil, diags
	case "gcp":
		gcpUpdateSpec, err := generateClusterGCPCMRUpdate(ctx, r.CustomerManagedResources)
		if err != nil {
			diags.AddError("error generating GCP CMR Update", err.Error())
			return nil, diags
		}
		cmrUpdate.CloudProvider = &controlplanev1.CustomerManagedResourcesUpdate_Gcp{
			Gcp: gcpUpdateSpec,
		}
		return cmrUpdate, diags
	case "azure":
		diags.AddError("Azure BYOVPC updates are not supported", "Azure BYOVPC updates are not supported")
		return nil, diags
	default:
		return nil, nil
	}
}

func generateClusterMTLSSpec(attrs map[string]attr.Value) *controlplanev1.MTLSSpec {
	if mtlsObj, ok := attrs["mtls"].(types.Object); ok && !mtlsObj.IsNull() {
		mtlsAttrs := mtlsObj.Attributes()
		if enabledVal, ok := mtlsAttrs["enabled"].(types.Bool); ok && !enabledVal.IsNull() {
			enabled := enabledVal.ValueBool()
			mtlsSpec := &controlplanev1.MTLSSpec{Enabled: enabled}
			if enabled {
				if certsVal, ok := mtlsAttrs["ca_certificates_pem"].(types.List); ok && !certsVal.IsNull() {
					mtlsSpec.CaCertificatesPem = utils.TypeListToStringSlice(certsVal)
				}
				if rulesVal, ok := mtlsAttrs["principal_mapping_rules"].(types.List); ok && !rulesVal.IsNull() {
					mtlsSpec.PrincipalMappingRules = utils.TypeListToStringSlice(rulesVal)
				}
			}
			return mtlsSpec
		}
	}
	return nil
}

func generateClusterSASLSpec(attrs map[string]attr.Value) *controlplanev1.SASLSpec {
	if saslObj, ok := attrs["sasl"].(types.Object); ok && !saslObj.IsNull() {
		saslAttrs := saslObj.Attributes()
		if enabledVal, ok := saslAttrs["enabled"].(types.Bool); ok && !enabledVal.IsNull() {
			return &controlplanev1.SASLSpec{Enabled: enabledVal.ValueBool()}
		}
	}
	return nil
}

func (r *ResourceModel) generateClusterKafkaAPISpec(_ context.Context) (*controlplanev1.KafkaAPISpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	if r.KafkaAPI.IsNull() {
		return nil, nil
	}
	spec := &controlplanev1.KafkaAPISpec{}
	spec.Mtls = generateClusterMTLSSpec(r.KafkaAPI.Attributes())
	spec.Sasl = generateClusterSASLSpec(r.KafkaAPI.Attributes())
	return spec, diags
}

func (r *ResourceModel) generateClusterHTTPProxySpec(_ context.Context) (*controlplanev1.HTTPProxySpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	if r.HTTPProxy.IsNull() {
		return nil, nil
	}
	spec := &controlplanev1.HTTPProxySpec{}
	spec.Mtls = generateClusterMTLSSpec(r.HTTPProxy.Attributes())
	spec.Sasl = generateClusterSASLSpec(r.HTTPProxy.Attributes())
	return spec, diags
}

func (r *ResourceModel) generateClusterSchemaRegistrySpec(_ context.Context) (*controlplanev1.SchemaRegistrySpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	if r.SchemaRegistry.IsNull() {
		return nil, nil
	}
	spec := &controlplanev1.SchemaRegistrySpec{}
	spec.Mtls = generateClusterMTLSSpec(r.SchemaRegistry.Attributes())
	spec.Sasl = generateClusterSASLSpec(r.SchemaRegistry.Attributes())
	return spec, diags
}

func (r *ResourceModel) generateClusterKafkaConnectSpec(_ context.Context) (*controlplanev1.KafkaConnect, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.KafkaConnect.IsNull() {
		return nil, nil
	}

	attrs := r.KafkaConnect.Attributes()

	if enabledVal, ok := attrs["enabled"].(types.Bool); ok && !enabledVal.IsNull() {
		return &controlplanev1.KafkaConnect{
			Enabled: enabledVal.ValueBool(),
		}, diags
	}
	return &controlplanev1.KafkaConnect{}, diags
}

func (r *ResourceModel) generateClusterCMR(ctx context.Context) (*controlplanev1.CustomerManagedResources, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	cmr := &controlplanev1.CustomerManagedResources{}

	switch r.CloudProvider.ValueString() {
	case "aws":
		awsSpec, err := generateClusterAWSCMR(ctx, r.CustomerManagedResources)
		if err != nil {
			diags.AddError("error generating AWS CMR", err.Error())
			return nil, diags
		}
		cmr.SetAws(awsSpec)
		return cmr, diags
	case "gcp":
		gcpSpec, err := generateClusterGCPCMR(ctx, r.CustomerManagedResources)
		if err != nil {
			diags.AddError("error generating GCP CMR", err.Error())
			return nil, diags
		}
		cmr.SetGcp(gcpSpec)
		return cmr, diags
	case "azure":
		diags.AddError("Azure BYOVPC is not supported", "Azure BYOVPC is not supported")
		return nil, diags
	default:
		return nil, nil
	}
}

func (r *ResourceModel) generateClusterAwsPrivateLinkSpec(_ context.Context) (*controlplanev1.AWSPrivateLinkSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.AWSPrivateLink.IsNull() {
		return nil, nil
	}

	attrs := r.AWSPrivateLink.Attributes()
	spec := &controlplanev1.AWSPrivateLinkSpec{}

	if enabledVal, ok := attrs["enabled"].(types.Bool); ok && !enabledVal.IsNull() {
		spec.Enabled = enabledVal.ValueBool()
	}

	if principalsVal, ok := attrs["allowed_principals"].(types.List); ok && !principalsVal.IsNull() {
		spec.AllowedPrincipals = utils.TypeListToStringSlice(principalsVal)
	}

	if connectConsoleVal, ok := attrs["connect_console"].(types.Bool); ok && !connectConsoleVal.IsNull() {
		val := connectConsoleVal.ValueBool()
		spec.ConnectConsole = &val
	}

	return spec, diags
}

func (r *ResourceModel) generateClusterGcpPrivateServiceConnectSpec(_ context.Context) (*controlplanev1.GCPPrivateServiceConnectSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.GCPPrivateServiceConnect.IsNull() {
		return nil, nil
	}

	attrs := r.GCPPrivateServiceConnect.Attributes()

	spec := &controlplanev1.GCPPrivateServiceConnectSpec{}

	if enabledVal, ok := attrs["enabled"].(types.Bool); ok && !enabledVal.IsNull() {
		spec.Enabled = enabledVal.ValueBool()
	}

	if globalAccessVal, ok := attrs["global_access_enabled"].(types.Bool); ok && !globalAccessVal.IsNull() {
		spec.GlobalAccessEnabled = globalAccessVal.ValueBool()
	}

	if consumerListVal, ok := attrs["consumer_accept_list"].(types.List); ok && !consumerListVal.IsNull() {
		var consumers []*controlplanev1.GCPPrivateServiceConnectConsumer

		for _, elem := range consumerListVal.Elements() {
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

		spec.ConsumerAcceptList = consumers
	}

	return spec, diags
}

func (r *ResourceModel) generateClusterAzurePrivateLinkSpec() (*controlplanev1.AzurePrivateLinkSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.AzurePrivateLink.IsNull() {
		return nil, nil
	}

	attrs := r.AzurePrivateLink.Attributes()

	spec := &controlplanev1.AzurePrivateLinkSpec{}

	if enabledVal, ok := attrs["enabled"].(types.Bool); ok && !enabledVal.IsNull() {
		spec.Enabled = enabledVal.ValueBool()
	}

	if connectConsoleVal, ok := attrs["connect_console"].(types.Bool); ok && !connectConsoleVal.IsNull() {
		val := connectConsoleVal.ValueBool()
		spec.ConnectConsole = &val
	}

	if subscriptionsVal, ok := attrs["allowed_subscriptions"].(types.List); ok && !subscriptionsVal.IsNull() {
		spec.AllowedSubscriptions = utils.TypeListToStringSlice(subscriptionsVal)
	}

	return spec, diags
}

func (r *ResourceModel) generateClusterMaintenanceWindow() (*controlplanev1.MaintenanceWindowConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.MaintenanceWindowConfig.IsNull() {
		return nil, nil
	}
	attrs := r.MaintenanceWindowConfig.Attributes()

	config := &controlplanev1.MaintenanceWindowConfig{}

	if dayHourObj, ok := attrs["day_hour"].(types.Object); ok && !dayHourObj.IsNull() {
		dayHourAttrs := dayHourObj.Attributes()

		hourAttr, ok := dayHourAttrs["hour_of_day"].(types.Int32)
		if !ok {
			diags.AddError("hour_of_day not found", "hour_of_day is missing or malformed")
			return nil, diags
		}
		dayAttr, ok := dayHourAttrs["day_of_week"].(types.String)
		if !ok {
			diags.AddError("day_of_week not found", "day_of_week is missing or malformed")
			return nil, diags
		}

		wdw := &controlplanev1.MaintenanceWindowConfig_DayHour{}
		wdw.SetHourOfDay(hourAttr.ValueInt32())

		// The protobuf uses google.type.DayOfWeek which has values like MONDAY, TUESDAY, etc.
		dayString := dayAttr.ValueString()
		dayValue, exists := dayofweek.DayOfWeek_value[dayString]
		if !exists {
			diags.AddError("Invalid day_of_week value", "day_of_week must be a valid day name")
			return nil, diags
		}
		wdw.SetDayOfWeek(dayofweek.DayOfWeek(dayValue))

		config.Window = &controlplanev1.MaintenanceWindowConfig_DayHour_{
			DayHour: wdw,
		}
		return config, diags
	}

	if anytimeVal, ok := attrs["anytime"].(types.Bool); ok && anytimeVal.ValueBool() {
		config.Window = &controlplanev1.MaintenanceWindowConfig_Anytime_{
			Anytime: &controlplanev1.MaintenanceWindowConfig_Anytime{},
		}
		return config, diags
	}

	if unspecVal, ok := attrs["unspecified"].(types.Bool); ok && unspecVal.ValueBool() {
		config.Window = &controlplanev1.MaintenanceWindowConfig_Unspecified_{
			Unspecified: &controlplanev1.MaintenanceWindowConfig_Unspecified{},
		}
		return config, diags
	}

	return nil, diags
}

func (r *ResourceModel) generateClusterClusterConfiguration() (*structpb.Struct, diag.Diagnostics) {
	var diags diag.Diagnostics
	if r.ClusterConfiguration.IsNull() {
		return nil, diags
	}
	// Get custom properties if defined
	customPropsJSON, err := utils.GetStringFromAttributes("custom_properties_json", r.ClusterConfiguration.Attributes())
	if err != nil {
		// custom_properties_json is optional, so it's okay if it doesn't exist
		// Return nil struct which means no custom properties
		return nil, diags
	}
	// Convert JSON string to a map
	customProps := map[string]any{}
	if customPropsJSON != "" {
		if err := json.Unmarshal([]byte(customPropsJSON), &customProps); err != nil {
			diags.AddError("failed to unmarshal custom_properties_json", err.Error())
			return nil, diags
		}
	}
	// Convert map to structpb.Struct
	customPropsStruct, err := structpb.NewStruct(customProps)
	if err != nil {
		diags.AddError("failed to convert custom_properties_json to structpb.Struct", err.Error())
		return nil, diags
	}
	return customPropsStruct, diags
}

func (r *ResourceModel) generateClusterCloudStorageCreate() (*controlplanev1.ClusterCreate_CloudStorage, diag.Diagnostics) {
	var diags diag.Diagnostics
	if r.CloudStorage.IsNull() {
		return nil, nil
	}

	attrs := r.CloudStorage.Attributes()
	cs := &controlplanev1.ClusterCreate_CloudStorage{}

	if skipDestroy, ok := attrs["skip_destroy"].(types.Bool); ok && !skipDestroy.IsNull() {
		cs.SkipDestroy = skipDestroy.ValueBool()
	}

	if awsObj, ok := attrs["aws"].(types.Object); ok && !awsObj.IsNull() {
		cs.CloudProvider = &controlplanev1.ClusterCreate_CloudStorage_Aws{
			Aws: &controlplanev1.ClusterCreate_CloudStorage_AWS{},
		}
	} else if gcpObj, ok := attrs["gcp"].(types.Object); ok && !gcpObj.IsNull() {
		cs.CloudProvider = &controlplanev1.ClusterCreate_CloudStorage_Gcp{
			Gcp: &controlplanev1.ClusterCreate_CloudStorage_GCP{},
		}
	} else if azureObj, ok := attrs["azure"].(types.Object); ok && !azureObj.IsNull() {
		cs.CloudProvider = &controlplanev1.ClusterCreate_CloudStorage_Azure_{
			Azure: &controlplanev1.ClusterCreate_CloudStorage_Azure{},
		}
	}

	return cs, diags
}

func (*ResourceModel) generateModelStateDescription(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasStateDescription() {
		return types.ObjectNull(getStateDescriptionType()), diags
	}
	sd := cluster.GetStateDescription()
	obj, d := types.ObjectValue(getStateDescriptionType(), map[string]attr.Value{
		"message": types.StringValue(sd.GetMessage()),
		"code":    types.Int32Value(sd.GetCode()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate state description object", "could not create state description object")
		return types.ObjectNull(getStateDescriptionType()), diags
	}
	return obj, diags
}

func (*ResourceModel) generateModelAwsPrivateLink(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasAwsPrivateLink() {
		return types.ObjectNull(getAwsPrivateLinkType()), diags
	}

	awsPrivateLink := cluster.GetAwsPrivateLink()
	if !awsPrivateLink.GetEnabled() {
		return types.ObjectNull(getAwsPrivateLinkType()), diags
	}

	var allowedPrincipals types.List
	if ap := awsPrivateLink.GetAllowedPrincipals(); ap != nil {
		allowedPrincipals = utils.StringSliceToTypeList(ap)
	} else {
		allowedPrincipals = types.ListNull(types.StringType)
	}

	status := awsPrivateLink.GetStatus()
	if status != nil {
		var vpcEndpointConns []attr.Value
		for _, conn := range status.GetVpcEndpointConnections() {
			var dnsEntries []attr.Value
			for _, dns := range conn.GetDnsEntries() {
				dnsEntry, d := types.ObjectValue(getDNSEntryType(), map[string]attr.Value{
					"dns_name":       types.StringValue(dns.GetDnsName()),
					"hosted_zone_id": types.StringValue(dns.GetHostedZoneId()),
				})
				if d.HasError() {
					diags.Append(d...)
					diags.AddError("failed to generate DNS entry", "could not create DNS entry object")
					return types.ObjectNull(getAwsPrivateLinkType()), diags
				}
				dnsEntries = append(dnsEntries, dnsEntry)
			}

			dnsEntriesList, d := types.ListValue(types.ObjectType{AttrTypes: getDNSEntryType()}, dnsEntries)
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate DNS entries list", "could not create DNS entries list")
				return types.ObjectNull(getAwsPrivateLinkType()), diags
			}

			connObj, d := types.ObjectValue(getVpcEndpointConnectionType(), map[string]attr.Value{
				"id":    types.StringValue(conn.GetId()),
				"owner": types.StringValue(conn.GetOwner()),
				"state": types.StringValue(conn.GetState()),
				"created_at": func() types.String {
					if conn.CreatedAt != nil {
						return types.StringValue(conn.GetCreatedAt().AsTime().Format(time.RFC3339))
					}
					return types.StringNull()
				}(),
				"connection_id":      types.StringValue(conn.GetConnectionId()),
				"load_balancer_arns": utils.StringSliceToTypeList(conn.GetLoadBalancerArns()),
				"dns_entries":        dnsEntriesList,
			})
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate VPC endpoint connection", "could not create VPC endpoint connection object")
				return types.ObjectNull(getAwsPrivateLinkType()), diags
			}
			vpcEndpointConns = append(vpcEndpointConns, connObj)
		}

		vpcEndpointConnsList, d := types.ListValue(types.ObjectType{AttrTypes: getVpcEndpointConnectionType()}, vpcEndpointConns)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate VPC endpoint connections list", "could not create VPC endpoint connections list")
			return types.ObjectNull(getAwsPrivateLinkType()), diags
		}

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

		statusObj, d := types.ObjectValue(getAwsPrivateLinkStatusType(), statusValues)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(getAwsPrivateLinkType()), diags
		}

		obj, d := types.ObjectValue(getAwsPrivateLinkType(), map[string]attr.Value{
			"enabled":            types.BoolValue(awsPrivateLink.GetEnabled()),
			"connect_console":    types.BoolValue(awsPrivateLink.GetConnectConsole()),
			"allowed_principals": allowedPrincipals,
			"status":             statusObj,
			"supported_regions":  utils.StringSliceToTypeList(awsPrivateLink.GetSupportedRegions()),
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate AWS Private Link object", "could not create AWS Private Link object")
			return types.ObjectNull(getAwsPrivateLinkType()), diags
		}
		return obj, diags
	}

	obj, d := types.ObjectValue(getAwsPrivateLinkType(), map[string]attr.Value{
		"enabled":            types.BoolValue(awsPrivateLink.GetEnabled()),
		"connect_console":    types.BoolValue(awsPrivateLink.GetConnectConsole()),
		"allowed_principals": allowedPrincipals,
		"status":             types.ObjectNull(getAwsPrivateLinkStatusType()),
		"supported_regions":  utils.StringSliceToTypeList(awsPrivateLink.GetSupportedRegions()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate AWS Private Link object", "could not create AWS Private Link object without status")
		return types.ObjectNull(getAwsPrivateLinkType()), diags
	}
	return obj, diags
}

func (*ResourceModel) generateModelGcpPrivateServiceConnect(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasGcpPrivateServiceConnect() {
		return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
	}

	gcpPsc := cluster.GetGcpPrivateServiceConnect()
	if !gcpPsc.GetEnabled() {
		return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
	}

	var consumerAcceptList []attr.Value
	for _, consumer := range gcpPsc.GetConsumerAcceptList() {
		consumerObj, d := types.ObjectValue(
			map[string]attr.Type{"source": types.StringType},
			map[string]attr.Value{"source": types.StringValue(consumer.GetSource())},
		)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate consumer accept list entry", "could not create consumer object")
			return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
		}
		consumerAcceptList = append(consumerAcceptList, consumerObj)
	}

	consumerList, d := types.ListValue(
		types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
		consumerAcceptList,
	)
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate consumer accept list", "could not create consumer list")
		return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
	}

	status := gcpPsc.GetStatus()
	if status != nil {
		var connectedEndpoints []attr.Value
		for _, endpoint := range status.GetConnectedEndpoints() {
			endpointObj, d := types.ObjectValue(
				getConnectedEndpointType(),
				map[string]attr.Value{
					"connection_id":    types.StringValue(endpoint.GetConnectionId()),
					"consumer_network": types.StringValue(endpoint.GetConsumerNetwork()),
					"endpoint":         types.StringValue(endpoint.GetEndpoint()),
					"status":           types.StringValue(endpoint.GetStatus()),
				},
			)
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate connected endpoint", "could not create endpoint object")
				return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
			}
			connectedEndpoints = append(connectedEndpoints, endpointObj)
		}

		endpointList, d := types.ListValue(types.ObjectType{AttrTypes: getConnectedEndpointType()}, connectedEndpoints)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate connected endpoints list", "could not create endpoints list")
			return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
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

		statusObj, d := types.ObjectValue(getGcpPrivateServiceConnectStatusType(), statusValues)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
		}

		obj, d := types.ObjectValue(getGcpPrivateServiceConnectType(), map[string]attr.Value{
			"enabled":               types.BoolValue(gcpPsc.GetEnabled()),
			"global_access_enabled": types.BoolValue(gcpPsc.GetGlobalAccessEnabled()),
			"consumer_accept_list":  consumerList,
			"status":                statusObj,
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate GCP Private Service Connect object", "could not create final object")
			return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
		}
		return obj, diags
	}

	obj, d := types.ObjectValue(getGcpPrivateServiceConnectType(), map[string]attr.Value{
		"enabled":               types.BoolValue(gcpPsc.GetEnabled()),
		"global_access_enabled": types.BoolValue(gcpPsc.GetGlobalAccessEnabled()),
		"consumer_accept_list":  consumerList,
		"status":                types.ObjectNull(getGcpPrivateServiceConnectStatusType()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(getGcpPrivateServiceConnectType()), diags
	}
	return obj, diags
}

func (*ResourceModel) generateModelAzurePrivateLink(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasAzurePrivateLink() {
		return types.ObjectNull(getAzurePrivateLinkType()), diags
	}

	azurePrivateLink := cluster.GetAzurePrivateLink()
	if !azurePrivateLink.GetEnabled() {
		return types.ObjectNull(getAzurePrivateLinkType()), diags
	}

	var allowedSubscriptions types.List
	if as := azurePrivateLink.GetAllowedSubscriptions(); as != nil {
		allowedSubscriptions = utils.StringSliceToTypeList(as)
	} else {
		allowedSubscriptions = types.ListNull(types.StringType)
	}

	status := azurePrivateLink.GetStatus()
	if status != nil {
		var privateEndpointConns []attr.Value
		for _, conn := range status.GetPrivateEndpointConnections() {
			connObj, d := types.ObjectValue(getAzureEndpointConnectionType(), map[string]attr.Value{
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
				diags.Append(d...)
				diags.AddError("failed to generate private endpoint connection", "could not create endpoint connection object")
				return types.ObjectNull(getAzurePrivateLinkType()), diags
			}
			privateEndpointConns = append(privateEndpointConns, connObj)
		}

		endpointConnsList, d := types.ListValue(types.ObjectType{AttrTypes: getAzureEndpointConnectionType()}, privateEndpointConns)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate private endpoint connections list", "could not create connections list")
			return types.ObjectNull(getAzurePrivateLinkType()), diags
		}

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

		statusObj, d := types.ObjectValue(getAzurePrivateLinkStatusType(), statusValues)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(getAzurePrivateLinkType()), diags
		}

		obj, d := types.ObjectValue(getAzurePrivateLinkType(), map[string]attr.Value{
			"enabled":               types.BoolValue(azurePrivateLink.GetEnabled()),
			"connect_console":       types.BoolValue(azurePrivateLink.GetConnectConsole()),
			"allowed_subscriptions": allowedSubscriptions,
			"status":                statusObj,
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate Azure Private Link object", "could not create Azure Private Link object")
			return types.ObjectNull(getAzurePrivateLinkType()), diags
		}
		return obj, diags
	}

	obj, d := types.ObjectValue(getAzurePrivateLinkType(), map[string]attr.Value{
		"enabled":               types.BoolValue(azurePrivateLink.GetEnabled()),
		"connect_console":       types.BoolValue(azurePrivateLink.GetConnectConsole()),
		"allowed_subscriptions": allowedSubscriptions,
		"status":                types.ObjectNull(getAzurePrivateLinkStatusType()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Azure Private Link object", "could not create object without status")
		return types.ObjectNull(getAzurePrivateLinkType()), diags
	}
	return obj, diags
}

func (r *ResourceModel) generateModelKafkaAPI(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasKafkaApi() {
		return types.ObjectNull(getKafkaAPIType()), diags
	}

	kafkaAPI := cluster.GetKafkaApi()

	mtls, d := r.generateMtlsModel(kafkaAPI.GetMtls())
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(getKafkaAPIType()), diags
	}

	saslObj := types.ObjectNull(getSASLType())
	if kafkaAPI.HasSasl() {
		s, d := types.ObjectValue(getSASLType(), map[string]attr.Value{
			"enabled": types.BoolValue(kafkaAPI.GetSasl().GetEnabled()),
		})
		if d.HasError() {
			diags.Append(d...)
		} else {
			saslObj = s
		}
	}

	allSeedBrokersObj := types.ObjectNull(getSeedBrokersType())
	if kafkaAPI.HasAllSeedBrokers() {
		asb := kafkaAPI.GetAllSeedBrokers()
		s, d := types.ObjectValue(getSeedBrokersType(), map[string]attr.Value{
			"sasl":              types.StringValue(asb.GetSasl()),
			"mtls":              types.StringValue(asb.GetMtls()),
			"private_link_sasl": types.StringValue(asb.GetPrivateLinkSasl()),
			"private_link_mtls": types.StringValue(asb.GetPrivateLinkMtls()),
		})
		if d.HasError() {
			diags.Append(d...)
		} else {
			allSeedBrokersObj = s
		}
	}

	obj, d := types.ObjectValue(getKafkaAPIType(), map[string]attr.Value{
		"seed_brokers":     utils.StringSliceToTypeList(kafkaAPI.GetSeedBrokers()),
		"mtls":             mtls,
		"sasl":             saslObj,
		"all_seed_brokers": allSeedBrokersObj,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Kafka API object", "could not create Kafka API object")
		return types.ObjectNull(getKafkaAPIType()), diags
	}

	return obj, diags
}

func (r *ResourceModel) generateModelHTTPProxy(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasHttpProxy() {
		return types.ObjectNull(getHTTPProxyType()), diags
	}

	httpProxy := cluster.GetHttpProxy()

	mtls, d := r.generateMtlsModel(httpProxy.GetMtls())
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(getHTTPProxyType()), diags
	}

	saslObj := types.ObjectNull(getSASLType())
	if httpProxy.HasSasl() {
		s, d := types.ObjectValue(getSASLType(), map[string]attr.Value{
			"enabled": types.BoolValue(httpProxy.GetSasl().GetEnabled()),
		})
		if d.HasError() {
			diags.Append(d...)
		} else {
			saslObj = s
		}
	}

	allUrlsObj := types.ObjectNull(getEndpointsType())
	if httpProxy.HasAllUrls() {
		au := httpProxy.GetAllUrls()
		s, d := types.ObjectValue(getEndpointsType(), map[string]attr.Value{
			"sasl":              types.StringValue(au.GetSasl()),
			"mtls":              types.StringValue(au.GetMtls()),
			"private_link_sasl": types.StringValue(au.GetPrivateLinkSasl()),
			"private_link_mtls": types.StringValue(au.GetPrivateLinkMtls()),
		})
		if d.HasError() {
			diags.Append(d...)
		} else {
			allUrlsObj = s
		}
	}

	obj, d := types.ObjectValue(getHTTPProxyType(), map[string]attr.Value{
		"mtls":     mtls,
		"url":      types.StringValue(httpProxy.GetUrl()),
		"sasl":     saslObj,
		"all_urls": allUrlsObj,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate HTTP Proxy object", "could not create HTTP Proxy object")
		return types.ObjectNull(getHTTPProxyType()), diags
	}

	return obj, diags
}

func (r *ResourceModel) generateModelSchemaRegistry(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasSchemaRegistry() {
		return types.ObjectNull(getSchemaRegistryType()), diags
	}

	schemaRegistry := cluster.GetSchemaRegistry()

	mtls, d := r.generateMtlsModel(schemaRegistry.GetMtls())
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(getSchemaRegistryType()), diags
	}

	allUrlsObj := types.ObjectNull(getEndpointsType())
	if schemaRegistry.HasAllUrls() {
		au := schemaRegistry.GetAllUrls()
		s, d := types.ObjectValue(getEndpointsType(), map[string]attr.Value{
			"sasl":              types.StringValue(au.GetSasl()),
			"mtls":              types.StringValue(au.GetMtls()),
			"private_link_sasl": types.StringValue(au.GetPrivateLinkSasl()),
			"private_link_mtls": types.StringValue(au.GetPrivateLinkMtls()),
		})
		if d.HasError() {
			diags.Append(d...)
		} else {
			allUrlsObj = s
		}
	}

	obj, d := types.ObjectValue(getSchemaRegistryType(), map[string]attr.Value{
		"mtls":     mtls,
		"url":      types.StringValue(schemaRegistry.GetUrl()),
		"all_urls": allUrlsObj,
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Schema Registry object", "could not create Schema Registry object")
		return types.ObjectNull(getSchemaRegistryType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateModelKafkaConnect(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasKafkaConnect() {
		return types.ObjectNull(getKafkaConnectType()), diags
	}

	kafkaConnect := cluster.GetKafkaConnect()
	if !kafkaConnect.GetEnabled() {
		return types.ObjectNull(getKafkaConnectType()), diags
	}

	obj, d := types.ObjectValue(getKafkaConnectType(), map[string]attr.Value{
		"enabled": types.BoolValue(kafkaConnect.GetEnabled()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Kafka Connect object", "could not create Kafka Connect object")
		return types.ObjectNull(getKafkaConnectType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateModelCustomerManagedResources(ctx context.Context, cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasCustomerManagedResources() {
		return types.ObjectNull(getCustomerManagedResourcesType()), diags
	}

	if cluster.Type != controlplanev1.Cluster_TYPE_BYOC {
		diags.AddError("Customer Managed Resources with non-BYOC cluster type", "Customer Managed Resources are only supported for BYOC clusters")
		return types.ObjectNull(getCustomerManagedResourcesType()), diags
	}

	cmr := cluster.GetCustomerManagedResources()

	switch {
	case cmr.HasAws():
		awsObj, d := generateModelClusterAWSCMR(ctx, cmr.GetAws())
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCustomerManagedResourcesType()), diags
		}
		obj, d := types.ObjectValue(getCustomerManagedResourcesType(), map[string]attr.Value{
			"aws": awsObj,
			"gcp": types.ObjectNull(getGcpCmrType()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCustomerManagedResourcesType()), diags
		}
		return obj, diags

	case cmr.HasGcp():
		gcpObj, d := generateModelClusterGCPCMR(ctx, cmr.GetGcp())
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCustomerManagedResourcesType()), diags
		}
		obj, d := types.ObjectValue(getCustomerManagedResourcesType(), map[string]attr.Value{
			"aws": types.ObjectNull(getAwsCmrType()),
			"gcp": gcpObj,
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCustomerManagedResourcesType()), diags
		}
		return obj, diags

	default:
		return types.ObjectNull(getCustomerManagedResourcesType()), diags
	}
}

func (*ResourceModel) generateModelPrometheus(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasPrometheus() {
		return types.ObjectNull(getPrometheusType()), diags
	}

	prometheus := cluster.GetPrometheus()

	obj, d := types.ObjectValue(getPrometheusType(), map[string]attr.Value{
		"url": types.StringValue(prometheus.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Prometheus object", "could not create Prometheus object")
		return types.ObjectNull(getPrometheusType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateModelRedpandaConsole(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasRedpandaConsole() {
		return types.ObjectNull(getRedpandaConsoleType()), diags
	}

	console := cluster.GetRedpandaConsole()

	obj, d := types.ObjectValue(getRedpandaConsoleType(), map[string]attr.Value{
		"url": types.StringValue(console.GetUrl()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Redpanda Console object", "could not create Redpanda Console object")
		return types.ObjectNull(getRedpandaConsoleType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateModelMaintenanceWindow(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasMaintenanceWindowConfig() {
		return types.ObjectNull(getMaintenanceWindowConfigType()), diags
	}

	maintenance := cluster.GetMaintenanceWindowConfig()

	windowObj := types.ObjectNull(getDayHourType())
	unspec := types.BoolNull()
	anytime := types.BoolNull()

	if !maintenance.HasWindow() {
		return types.ObjectNull(getMaintenanceWindowConfigType()), diags
	}

	switch {
	case maintenance.HasDayHour():
		w := maintenance.GetDayHour()
		obj, d := types.ObjectValue(getDayHourType(), map[string]attr.Value{
			"hour_of_day": types.Int32Value(w.GetHourOfDay()),
			"day_of_week": types.StringValue(w.GetDayOfWeek().String()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getMaintenanceWindowConfigType()), diags
		}
		windowObj = obj
	case maintenance.HasAnytime():
		unspec = types.BoolValue(true)
	case maintenance.HasUnspecified():
		anytime = types.BoolValue(true)
	}

	obj, d := types.ObjectValue(getMaintenanceWindowConfigType(), map[string]attr.Value{
		"day_hour":    windowObj,
		"anytime":     anytime,
		"unspecified": unspec,
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(getMaintenanceWindowConfigType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateMtlsModel(mtls *controlplanev1.MTLSSpec) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if mtls == nil || !mtls.GetEnabled() {
		return types.ObjectNull(getMtlsType()), diags
	}

	obj, d := types.ObjectValue(getMtlsType(), map[string]attr.Value{
		"enabled":                 types.BoolValue(mtls.GetEnabled()),
		"ca_certificates_pem":     utils.StringSliceToTypeList(mtls.GetCaCertificatesPem()),
		"principal_mapping_rules": utils.StringSliceToTypeList(mtls.GetPrincipalMappingRules()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate MTLS object", "could not create MTLS object")
		return types.ObjectNull(getMtlsType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateModelClusterConfiguration(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasClusterConfiguration() {
		return types.ObjectNull(getClusterConfigurationType()), diags
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
				return types.ObjectNull(getClusterConfigurationType()), diags
			}
			configValues["custom_properties_json"] = types.StringValue(string(customPropsBytes))
		}
	}

	// Only return null if custom properties are null
	if configValues["custom_properties_json"].IsNull() {
		return types.ObjectNull(getClusterConfigurationType()), diags
	}

	obj, d := types.ObjectValue(getClusterConfigurationType(), configValues)
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate cluster configuration object", "could not create cluster configuration object")
		return types.ObjectNull(getClusterConfigurationType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateModelCloudStorage(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasCloudStorage() {
		return types.ObjectNull(getCloudStorageType()), diags
	}

	cs := cluster.GetCloudStorage()
	values := map[string]attr.Value{
		"aws":          types.ObjectNull(getCloudStorageAwsType()),
		"gcp":          types.ObjectNull(getCloudStorageGcpType()),
		"azure":        types.ObjectNull(getCloudStorageAzureType()),
		"skip_destroy": types.BoolValue(cs.GetSkipDestroy()),
	}

	switch v := cs.GetCloudProvider().(type) {
	case *controlplanev1.Cluster_CloudStorage_Aws:
		awsObj, d := types.ObjectValue(getCloudStorageAwsType(), map[string]attr.Value{
			"arn": types.StringValue(v.Aws.GetArn()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCloudStorageType()), diags
		}
		values["aws"] = awsObj
	case *controlplanev1.Cluster_CloudStorage_Gcp:
		gcpObj, d := types.ObjectValue(getCloudStorageGcpType(), map[string]attr.Value{
			"name": types.StringValue(v.Gcp.GetName()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCloudStorageType()), diags
		}
		values["gcp"] = gcpObj
	case *controlplanev1.Cluster_CloudStorage_Azure_:
		azureObj, d := types.ObjectValue(getCloudStorageAzureType(), map[string]attr.Value{
			"container_name":       types.StringValue(v.Azure.GetContainerName()),
			"storage_account_name": types.StringValue(v.Azure.GetStorageAccountName()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(getCloudStorageType()), diags
		}
		values["azure"] = azureObj
	}

	obj, d := types.ObjectValue(getCloudStorageType(), values)
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(getCloudStorageType()), diags
	}
	return obj, diags
}
