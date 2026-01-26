package serverlesscluster

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	serverlessclustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlesscluster"
)

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(cluster *controlplanev1.ServerlessCluster) *serverlessclustermodel.ResourceModel {
	output := &serverlessclustermodel.ResourceModel{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
	}
	if cluster.DataplaneApi != nil {
		output.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}

	// Set private_link_id if present
	if cluster.PrivateLinkId != nil {
		output.PrivateLinkID = types.StringValue(*cluster.PrivateLinkId)
	} else {
		output.PrivateLinkID = types.StringNull()
	}

	// Set networking_config if present
	if cluster.NetworkingConfig != nil {
		networkingConfigObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"private": types.StringType,
				"public":  types.StringType,
			},
			map[string]attr.Value{
				"private": types.StringValue(cluster.NetworkingConfig.Private.String()),
				"public":  types.StringValue(cluster.NetworkingConfig.Public.String()),
			},
		)
		output.NetworkingConfig = networkingConfigObj
	} else {
		output.NetworkingConfig = types.ObjectNull(map[string]attr.Type{
			"private": types.StringType,
			"public":  types.StringType,
		})
	}

	return output
}

// generateDataModel populates the DataModel for datasource operations
func generateDataModel(cluster *controlplanev1.ServerlessCluster) *serverlessclustermodel.DataModel {
	output := &serverlessclustermodel.DataModel{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
	}
	if cluster.DataplaneApi != nil {
		output.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}
	return output
}
