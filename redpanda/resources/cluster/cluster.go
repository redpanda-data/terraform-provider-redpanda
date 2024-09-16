// Copyright 2024 Redpanda Data, Inc.
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
	"context"
	"fmt"
	"reflect"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func gcpConnectConsumerModelToStruct(accept []*models.GcpPrivateServiceConnectConsumer) []*controlplanev1beta2.GCPPrivateServiceConnectConsumer {
	var output []*controlplanev1beta2.GCPPrivateServiceConnectConsumer
	for _, a := range accept {
		output = append(output, &controlplanev1beta2.GCPPrivateServiceConnectConsumer{
			Source: a.Source,
		})
	}
	return output
}

func gcpConnectConsumerStructToModel(accept []*controlplanev1beta2.GCPPrivateServiceConnectConsumer) []*models.GcpPrivateServiceConnectConsumer {
	var output []*models.GcpPrivateServiceConnectConsumer
	for _, a := range accept {
		output = append(output, &models.GcpPrivateServiceConnectConsumer{
			Source: a.Source,
		})
	}
	return output
}

func toMtlsModel(ctx context.Context, mtls *controlplanev1beta2.MTLSSpec) (*models.Mtls, diag.Diagnostics) {
	if isMtlsSpecNil(mtls) {
		return nil, nil
	}

	capem, err := types.ListValueFrom(ctx, types.StringType, mtls.GetCaCertificatesPem())
	if err != nil {
		return nil, err
	}
	maprules, err := types.ListValueFrom(ctx, types.StringType, mtls.GetPrincipalMappingRules())
	if err != nil {
		return nil, err
	}
	return &models.Mtls{
		Enabled:               types.BoolValue(mtls.GetEnabled()),
		CaCertificatesPem:     capem,
		PrincipalMappingRules: maprules,
	}, nil
}

func toMtlsSpec(mtls *models.Mtls) *controlplanev1beta2.MTLSSpec {
	if isMtlsStructNil(mtls) {
		return &controlplanev1beta2.MTLSSpec{
			Enabled:               false,
			CaCertificatesPem:     make([]string, 0),
			PrincipalMappingRules: make([]string, 0),
		}
	}
	return &controlplanev1beta2.MTLSSpec{
		Enabled:               mtls.Enabled.ValueBool(),
		CaCertificatesPem:     utils.TypeListToStringSlice(mtls.CaCertificatesPem),
		PrincipalMappingRules: utils.TypeListToStringSlice(mtls.PrincipalMappingRules),
	}
}

func isMtlsNil(container any) bool {
	v := reflect.ValueOf(container)
	if v.Kind() != reflect.Struct && v.Kind() != reflect.Ptr {
		return true
	}

	if !v.IsValid() || v.IsNil() {
		return true
	}

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return true
	}
	mtlsField := v.FieldByName("Mtls")
	if !mtlsField.IsValid() || mtlsField.IsNil() {
		return true
	}
	return isMtlsStructNil(mtlsField.Interface().(*models.Mtls))
}

func isMtlsStructNil(m *models.Mtls) bool {
	return m == nil || (m.Enabled.IsNull() && m.CaCertificatesPem.IsNull() && m.PrincipalMappingRules.IsNull())
}

func isMtlsSpecNil(m *controlplanev1beta2.MTLSSpec) bool {
	return m == nil || (!m.GetEnabled() && len(m.GetCaCertificatesPem()) == 0 && len(m.GetPrincipalMappingRules()) == 0)
}

func isAwsPrivateLinkStructNil(m *models.AwsPrivateLink) bool {
	return m == nil || (m.Enabled.IsNull() && m.ConnectConsole.IsNull() && m.AllowedPrincipals.IsNull())
}

func isAwsPrivateLinkSpecNil(m *controlplanev1beta2.AWSPrivateLinkStatus) bool {
	return m == nil || (!m.Enabled && !m.ConnectConsole && len(m.AllowedPrincipals) == 0)
}

func isAzurePrivateLinkStructNil(m *models.AzurePrivateLink) bool {
	return m == nil || (m.Enabled.IsNull() && m.AllowedSubscriptions.IsNull() && m.ConnectConsole.IsNull())
}

func isAzurePrivateLinkSpecNil(m *controlplanev1beta2.AzurePrivateLinkStatus) bool {
	return m == nil || (!m.Enabled && len(m.AllowedSubscriptions) == 0 && !m.ConnectConsole)
}

func isGcpPrivateServiceConnectStructNil(m *models.GcpPrivateServiceConnect) bool {
	return m == nil || (m.Enabled.IsNull() && m.GlobalAccessEnabled.IsNull() && len(m.ConsumerAcceptList) == 0)
}

func isGcpPrivateServiceConnectSpecNil(m *controlplanev1beta2.GCPPrivateServiceConnectStatus) bool {
	return m == nil || (!m.Enabled && !m.GlobalAccessEnabled && len(m.ConsumerAcceptList) == 0)
}

// generateClusterRequest was pulled out to enable unit testing
func generateClusterRequest(model models.Cluster) (*controlplanev1beta2.ClusterCreate, error) {
	provider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse cloud provider: %v", err)
	}
	clusterType, err := utils.StringToClusterType(model.ClusterType.ValueString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse cluster type: %v", err)
	}
	rpVersion := model.RedpandaVersion.ValueString()

	output := &controlplanev1beta2.ClusterCreate{
		Name:              model.Name.ValueString(),
		ConnectionType:    utils.StringToConnectionType(model.ConnectionType.ValueString()),
		CloudProvider:     provider,
		RedpandaVersion:   &rpVersion,
		ThroughputTier:    model.ThroughputTier.ValueString(),
		Region:            model.Region.ValueString(),
		Zones:             utils.TypeListToStringSlice(model.Zones),
		ResourceGroupId:   model.ResourceGroupID.ValueString(),
		NetworkId:         model.NetworkID.ValueString(),
		Type:              clusterType,
		CloudProviderTags: utils.TypeMapToStringMap(model.Tags),
	}
	if !isAwsPrivateLinkStructNil(model.AwsPrivateLink) {
		if !model.AwsPrivateLink.AllowedPrincipals.IsNull() {
			output.AwsPrivateLink = &controlplanev1beta2.AWSPrivateLinkSpec{
				Enabled:           model.AwsPrivateLink.Enabled.ValueBool(),
				AllowedPrincipals: utils.TypeListToStringSlice(model.AwsPrivateLink.AllowedPrincipals),
				ConnectConsole:    model.AwsPrivateLink.ConnectConsole.ValueBool(),
			}
		}
	}
	if !isGcpPrivateServiceConnectStructNil(model.GcpPrivateServiceConnect) {
		if len(model.GcpPrivateServiceConnect.ConsumerAcceptList) > 0 {
			output.GcpPrivateServiceConnect = &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             model.GcpPrivateServiceConnect.Enabled.ValueBool(),
				GlobalAccessEnabled: model.GcpPrivateServiceConnect.GlobalAccessEnabled.ValueBool(),
				ConsumerAcceptList:  gcpConnectConsumerModelToStruct(model.GcpPrivateServiceConnect.ConsumerAcceptList),
			}
		}
	}

	if !isAzurePrivateLinkStructNil(model.AzurePrivateLink) {
		if !model.AzurePrivateLink.AllowedSubscriptions.IsNull() {
			output.AzurePrivateLink = &controlplanev1beta2.AzurePrivateLinkSpec{
				Enabled:              model.AzurePrivateLink.Enabled.ValueBool(),
				AllowedSubscriptions: utils.TypeListToStringSlice(model.AzurePrivateLink.AllowedSubscriptions),
				ConnectConsole:       model.AzurePrivateLink.ConnectConsole.ValueBool(),
			}
		}
	}

	if model.KafkaAPI != nil {
		output.KafkaApi = &controlplanev1beta2.KafkaAPISpec{
			Mtls: toMtlsSpec(model.KafkaAPI.Mtls),
		}
	}
	if model.HTTPProxy != nil {
		output.HttpProxy = &controlplanev1beta2.HTTPProxySpec{
			Mtls: toMtlsSpec(model.HTTPProxy.Mtls),
		}
	}
	if model.SchemaRegistry != nil {
		output.SchemaRegistry = &controlplanev1beta2.SchemaRegistrySpec{
			Mtls: toMtlsSpec(model.SchemaRegistry.Mtls),
		}
	}
	if !model.ReadReplicaClusterIDs.IsNull() {
		output.ReadReplicaClusterIds = utils.TypeListToStringSlice(model.ReadReplicaClusterIDs)
	}

	return output, nil
}

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(ctx context.Context, cfg models.Cluster, cluster *controlplanev1beta2.Cluster) (*models.Cluster, error) {
	output := &models.Cluster{
		Name:            types.StringValue(cluster.Name),
		ConnectionType:  types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion: cfg.RedpandaVersion,
		ThroughputTier:  types.StringValue(cluster.ThroughputTier),
		Region:          types.StringValue(cluster.Region),
		AllowDeletion:   cfg.AllowDeletion,
		Tags:            cfg.Tags,
		ResourceGroupID: types.StringValue(cluster.ResourceGroupId),
		NetworkID:       types.StringValue(cluster.NetworkId),
		ID:              types.StringValue(cluster.Id),
	}

	clusterZones, d := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if d.HasError() {
		return nil, fmt.Errorf("failed to parse cluster zones: %v", d)
	}
	output.Zones = clusterZones

	if cluster.GetDataplaneApi() != nil {
		clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
		if err != nil {
			return nil, fmt.Errorf("unable to parse Cluster API URL: %v", err)
		}
		output.ClusterAPIURL = basetypes.NewStringValue(clusterURL)
	}

	rr, d := types.ListValueFrom(ctx, types.StringType, cluster.ReadReplicaClusterIds)
	if d.HasError() {
		return nil, fmt.Errorf("failed to parse read replica cluster IDs: %v", d)
	}
	output.ReadReplicaClusterIDs = rr

	if !isAwsPrivateLinkSpecNil(cluster.AwsPrivateLink) {
		ap, dg := types.ListValueFrom(ctx, types.StringType, cluster.AwsPrivateLink.AllowedPrincipals)
		if dg.HasError() {
			return nil, fmt.Errorf("failed to parse AWS Private Link: %v", dg)
		}
		output.AwsPrivateLink = &models.AwsPrivateLink{
			Enabled:           types.BoolValue(cluster.AwsPrivateLink.Enabled),
			ConnectConsole:    types.BoolValue(cluster.AwsPrivateLink.ConnectConsole),
			AllowedPrincipals: ap,
		}
	}
	if !isGcpPrivateServiceConnectSpecNil(cluster.GcpPrivateServiceConnect) {
		output.GcpPrivateServiceConnect = &models.GcpPrivateServiceConnect{
			Enabled:             types.BoolValue(cluster.GcpPrivateServiceConnect.Enabled),
			GlobalAccessEnabled: types.BoolValue(cluster.GcpPrivateServiceConnect.GlobalAccessEnabled),
			ConsumerAcceptList:  gcpConnectConsumerStructToModel(cluster.GcpPrivateServiceConnect.ConsumerAcceptList),
		}
	}

	if !isAzurePrivateLinkSpecNil(cluster.AzurePrivateLink) {
		as, dg := types.ListValueFrom(ctx, types.StringType, cluster.AzurePrivateLink.AllowedSubscriptions)
		if dg.HasError() {
			return nil, fmt.Errorf("failed to parse Azure Private Link: %v", dg)
		}
		output.AzurePrivateLink = &models.AzurePrivateLink{
			Enabled:              types.BoolValue(cluster.AzurePrivateLink.Enabled),
			ConnectConsole:       types.BoolValue(cluster.AzurePrivateLink.ConnectConsole),
			AllowedSubscriptions: as,
		}
	}
	kAPI, err := toMtlsModel(ctx, cluster.GetKafkaApi().GetMtls())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Kafka API MTLS: %v", err)
	}
	if kAPI != nil {
		output.KafkaAPI = &models.KafkaAPI{
			Mtls: kAPI,
		}
	}
	ht, err := toMtlsModel(ctx, cluster.GetHttpProxy().GetMtls())
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTP Proxy MTLS: %v", err)
	}
	if ht != nil {
		output.HTTPProxy = &models.HTTPProxy{
			Mtls: ht,
		}
	}
	sr, err := toMtlsModel(ctx, cluster.GetSchemaRegistry().GetMtls())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Schema Registry MTLS: %v", err)
	}
	if sr != nil {
		output.SchemaRegistry = &models.SchemaRegistry{
			Mtls: sr,
		}
	}
	return output, nil
}

// generateMinimalModel populates a Cluster model with only enough state for Terraform to
// track an existing cluster and to delete it, if necessary. Used in creation to track
// partially created clusters, and on reading to null out cluster that are found in the
// deleting state and force them to be recreated.
func generateMinimalModel(clusterID string) models.Cluster {
	// Terraform requires us to explicitly pass types to the collection values, even
	// when null :/
	return models.Cluster{
		AllowDeletion:         types.BoolValue(true),
		ID:                    types.StringValue(clusterID),
		ReadReplicaClusterIDs: types.ListNull(types.StringType),
		Tags:                  types.MapNull(types.StringType),
		Zones:                 types.ListNull(types.StringType),
	}
}
