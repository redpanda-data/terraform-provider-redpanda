package serverlesscluster

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
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

// generateServerlessClusterRequest was pulled out to enable unit testing
func generateServerlessClusterRequest(model serverlessclustermodel.ResourceModel) (*controlplanev1.ServerlessClusterCreate, error) {
	return &controlplanev1.ServerlessClusterCreate{
		Name:             model.Name.ValueString(),
		ServerlessRegion: model.ServerlessRegion.ValueString(),
		ResourceGroupId:  model.ResourceGroupID.ValueString(),
	}, nil
}
