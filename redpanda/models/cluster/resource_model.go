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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/genproto/googleapis/type/dayofweek"
	"google.golang.org/protobuf/types/known/structpb"
)

// ResourceModel represents the Terraform schema for the cluster resource.
type ResourceModel struct {
	Name                     types.String        `tfsdk:"name"`
	ID                       types.String        `tfsdk:"id"`
	ConnectionType           types.String        `tfsdk:"connection_type"`
	CloudProvider            types.String        `tfsdk:"cloud_provider"`
	ClusterType              types.String        `tfsdk:"cluster_type"`
	RedpandaVersion          types.String        `tfsdk:"redpanda_version"`
	ThroughputTier           types.String        `tfsdk:"throughput_tier"`
	RedpandaNodeCount        types.Int32         `tfsdk:"redpanda_node_count"`
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
		GCPGlobalAccessEnabled:   types.BoolNull(),
		AllowDeletion:            types.BoolValue(true),
		ReadReplicaClusterIDs:    types.ListNull(types.StringType),
		Zones:                    types.ListNull(types.StringType),
		Prometheus:               types.ObjectNull(GetPrometheusType()),
		CustomerManagedResources: types.ObjectNull(GetCustomerManagedResourcesType()),
		KafkaAPI:                 types.ObjectNull(GetKafkaAPIType()),
		HTTPProxy:                types.ObjectNull(GetHTTPProxyType()),
		SchemaRegistry:           types.ObjectNull(GetSchemaRegistryType()),
		AwsPrivateLink:           types.ObjectNull(GetAwsPrivateLinkType()),
		GcpPrivateServiceConnect: types.ObjectNull(GetGcpPrivateServiceConnectType()),
		AzurePrivateLink:         types.ObjectNull(GetAzurePrivateLinkType()),
		RedpandaConsole:          types.ObjectNull(GetRedpandaConsoleType()),
		StateDescription:         types.ObjectNull(GetStateDescriptionType()),
		MaintenanceWindowConfig:  types.ObjectNull(GetMaintenanceWindowConfigType()),
		KafkaConnect:             types.ObjectNull(GetKafkaConnectType()),
		ClusterConfiguration:     types.ObjectNull(GetClusterConfigurationType()),
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
	r.ReadReplicaClusterIDs = utils.StringSliceToTypeList(cluster.GetReadReplicaClusterIds())

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
		r.AwsPrivateLink = awsPrivateLink
	}

	if gcpPsc, d := r.generateModelGcpPrivateServiceConnect(cluster); d.HasError() {
		diags.Append(d...)
	} else {
		r.GcpPrivateServiceConnect = gcpPsc
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

	if !r.AwsPrivateLink.IsNull() {
		awsSpec, d := r.generateClusterAwsPrivateLinkSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		output.AwsPrivateLink = awsSpec
	}

	if !r.GcpPrivateServiceConnect.IsNull() {
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

	return output, diags
}

func (r *ResourceModel) getClusterUpdate(ctx context.Context) (*controlplanev1.ClusterUpdate, diag.Diagnostics) {
	var diags diag.Diagnostics

	update := &controlplanev1.ClusterUpdate{
		Id:   r.ID.ValueString(),
		Name: r.Name.ValueString(),
	}

	if !r.ReadReplicaClusterIDs.IsNull() {
		update.ReadReplicaClusterIds = utils.TypeListToStringSlice(r.ReadReplicaClusterIDs)
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

	if !r.AwsPrivateLink.IsNull() {
		awsSpec, d := r.generateClusterAwsPrivateLinkSpec(ctx)
		if d.HasError() {
			diags.Append(d...)
		}
		update.AwsPrivateLink = awsSpec
	}

	if !r.GcpPrivateServiceConnect.IsNull() {
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

	if r.AwsPrivateLink.IsNull() {
		return nil, nil
	}

	attrs := r.AwsPrivateLink.Attributes()
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

	if r.GcpPrivateServiceConnect.IsNull() {
		return nil, nil
	}

	attrs := r.GcpPrivateServiceConnect.Attributes()

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

func (*ResourceModel) generateModelStateDescription(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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

func (*ResourceModel) generateModelAwsPrivateLink(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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

			dnsEntriesList, d := types.ListValue(types.ObjectType{AttrTypes: GetDNSEntryType()}, dnsEntries)
			if d.HasError() {
				diags.Append(d...)
				diags.AddError("failed to generate DNS entries list", "could not create DNS entries list")
				return types.ObjectNull(GetAwsPrivateLinkType()), diags
			}

			connObj, d := types.ObjectValue(GetVpcEndpointConnectionType(), map[string]attr.Value{
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
				return types.ObjectNull(GetAwsPrivateLinkType()), diags
			}
			vpcEndpointConns = append(vpcEndpointConns, connObj)
		}

		vpcEndpointConnsList, d := types.ListValue(types.ObjectType{AttrTypes: GetVpcEndpointConnectionType()}, vpcEndpointConns)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate VPC endpoint connections list", "could not create VPC endpoint connections list")
			return types.ObjectNull(GetAwsPrivateLinkType()), diags
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

		statusObj, d := types.ObjectValue(GetAwsPrivateLinkStatusType(), statusValues)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(GetAwsPrivateLinkType()), diags
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

	obj, d := types.ObjectValue(GetAwsPrivateLinkType(), map[string]attr.Value{
		"enabled":            types.BoolValue(awsPrivateLink.GetEnabled()),
		"connect_console":    types.BoolValue(awsPrivateLink.GetConnectConsole()),
		"allowed_principals": allowedPrincipals,
		"status":             types.ObjectNull(GetAwsPrivateLinkStatusType()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate AWS Private Link object", "could not create AWS Private Link object without status")
		return types.ObjectNull(GetAwsPrivateLinkType()), diags
	}
	return obj, diags
}

func (*ResourceModel) generateModelGcpPrivateServiceConnect(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasGcpPrivateServiceConnect() {
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}

	gcpPsc := cluster.GetGcpPrivateServiceConnect()
	if !gcpPsc.GetEnabled() {
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
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
		diags.AddError("failed to generate consumer accept list", "could not create consumer list")
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}

	status := gcpPsc.GetStatus()
	if status != nil {
		var connectedEndpoints []attr.Value
		for _, endpoint := range status.GetConnectedEndpoints() {
			endpointObj, d := types.ObjectValue(
				GetConnectedEndpointType(),
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
				return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
			}
			connectedEndpoints = append(connectedEndpoints, endpointObj)
		}

		endpointList, d := types.ListValue(types.ObjectType{AttrTypes: GetConnectedEndpointType()}, connectedEndpoints)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate connected endpoints list", "could not create endpoints list")
			return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
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

		statusObj, d := types.ObjectValue(GetGcpPrivateServiceConnectStatusType(), statusValues)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
		}

		obj, d := types.ObjectValue(GetGcpPrivateServiceConnectType(), map[string]attr.Value{
			"enabled":               types.BoolValue(gcpPsc.GetEnabled()),
			"global_access_enabled": types.BoolValue(gcpPsc.GetGlobalAccessEnabled()),
			"consumer_accept_list":  consumerList,
			"status":                statusObj,
		})
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate GCP Private Service Connect object", "could not create final object")
			return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
		}
		return obj, diags
	}

	obj, d := types.ObjectValue(GetGcpPrivateServiceConnectType(), map[string]attr.Value{
		"enabled":               types.BoolValue(gcpPsc.GetEnabled()),
		"global_access_enabled": types.BoolValue(gcpPsc.GetGlobalAccessEnabled()),
		"consumer_accept_list":  consumerList,
		"status":                types.ObjectNull(GetGcpPrivateServiceConnectStatusType()),
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(GetGcpPrivateServiceConnectType()), diags
	}
	return obj, diags
}

func (*ResourceModel) generateModelAzurePrivateLink(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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
	if status != nil {
		var privateEndpointConns []attr.Value
		for _, conn := range status.GetPrivateEndpointConnections() {
			connObj, d := types.ObjectValue(GetAzureEndpointConnectionType(), map[string]attr.Value{
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
				return types.ObjectNull(GetAzurePrivateLinkType()), diags
			}
			privateEndpointConns = append(privateEndpointConns, connObj)
		}

		endpointConnsList, d := types.ListValue(types.ObjectType{AttrTypes: GetAzureEndpointConnectionType()}, privateEndpointConns)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate private endpoint connections list", "could not create connections list")
			return types.ObjectNull(GetAzurePrivateLinkType()), diags
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

		statusObj, d := types.ObjectValue(GetAzurePrivateLinkStatusType(), statusValues)
		if d.HasError() {
			diags.Append(d...)
			diags.AddError("failed to generate status object", "could not create status object")
			return types.ObjectNull(GetAzurePrivateLinkType()), diags
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

	obj, d := types.ObjectValue(GetAzurePrivateLinkType(), map[string]attr.Value{
		"enabled":               types.BoolValue(azurePrivateLink.GetEnabled()),
		"connect_console":       types.BoolValue(azurePrivateLink.GetConnectConsole()),
		"allowed_subscriptions": allowedSubscriptions,
		"status":                types.ObjectNull(GetAzurePrivateLinkStatusType()),
	})
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("failed to generate Azure Private Link object", "could not create object without status")
		return types.ObjectNull(GetAzurePrivateLinkType()), diags
	}
	return obj, diags
}

func (r *ResourceModel) generateModelKafkaAPI(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasKafkaApi() {
		return types.ObjectNull(GetKafkaAPIType()), diags
	}

	kafkaAPI := cluster.GetKafkaApi()

	mtls, d := r.generateMtlsModel(kafkaAPI.GetMtls())
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

func (r *ResourceModel) generateModelHTTPProxy(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasHttpProxy() {
		return types.ObjectNull(GetHTTPProxyType()), diags
	}

	httpProxy := cluster.GetHttpProxy()

	mtls, d := r.generateMtlsModel(httpProxy.GetMtls())
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

func (r *ResourceModel) generateModelSchemaRegistry(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasSchemaRegistry() {
		return types.ObjectNull(GetSchemaRegistryType()), diags
	}

	schemaRegistry := cluster.GetSchemaRegistry()

	mtls, d := r.generateMtlsModel(schemaRegistry.GetMtls())
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

func (*ResourceModel) generateModelKafkaConnect(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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

func (*ResourceModel) generateModelCustomerManagedResources(ctx context.Context, cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !cluster.HasCustomerManagedResources() {
		return types.ObjectNull(GetCustomerManagedResourcesType()), diags
	}

	if cluster.Type != controlplanev1.Cluster_TYPE_BYOC {
		diags.AddError("Customer Managed Resources with non-BYOC cluster type", "Customer Managed Resources are only supported for BYOC clusters")
		return types.ObjectNull(GetCustomerManagedResourcesType()), diags
	}

	cmr := cluster.GetCustomerManagedResources()

	switch {
	case cmr.HasAws():
		awsObj, d := generateModelClusterAWSCMR(ctx, cmr.GetAws())
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}
		obj, d := types.ObjectValue(GetCustomerManagedResourcesType(), map[string]attr.Value{
			"aws": awsObj,
			"gcp": types.ObjectNull(GetGcpCmrType()),
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}
		return obj, diags

	case cmr.HasGcp():
		gcpObj, d := generateModelClusterGCPCMR(ctx, cmr.GetGcp())
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}
		obj, d := types.ObjectValue(GetCustomerManagedResourcesType(), map[string]attr.Value{
			"aws": types.ObjectNull(GetAwsCmrType()),
			"gcp": gcpObj,
		})
		if d.HasError() {
			diags.Append(d...)
			return types.ObjectNull(GetCustomerManagedResourcesType()), diags
		}
		return obj, diags

	default:
		return types.ObjectNull(GetCustomerManagedResourcesType()), diags
	}
}

func (*ResourceModel) generateModelPrometheus(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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

func (*ResourceModel) generateModelRedpandaConsole(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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

func (*ResourceModel) generateModelMaintenanceWindow(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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
			return types.ObjectNull(GetMaintenanceWindowConfigType()), diags
		}
		windowObj = obj
	case maintenance.HasAnytime():
		unspec = types.BoolValue(true)
	case maintenance.HasUnspecified():
		anytime = types.BoolValue(true)
	}

	obj, d := types.ObjectValue(GetMaintenanceWindowConfigType(), map[string]attr.Value{
		"day_hour":    windowObj,
		"anytime":     anytime,
		"unspecified": unspec,
	})
	if d.HasError() {
		diags.Append(d...)
		return types.ObjectNull(GetMaintenanceWindowConfigType()), diags
	}

	return obj, diags
}

func (*ResourceModel) generateMtlsModel(mtls *controlplanev1.MTLSSpec) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if mtls == nil || !mtls.GetEnabled() {
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

func (*ResourceModel) generateModelClusterConfiguration(cluster *controlplanev1.Cluster) (types.Object, diag.Diagnostics) {
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
