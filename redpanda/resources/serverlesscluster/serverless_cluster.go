package serverlesscluster

import (
	"fmt"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(cluster *controlplanev1beta2.ServerlessCluster) (*models.ServerlessCluster, error) {
	output := &models.ServerlessCluster{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
	}
	if cluster.DataplaneApi != nil {
		clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
		if err != nil {
			return nil, fmt.Errorf("unable to parse Cluster API URL: %w", err)
		}
		output.ClusterAPIURL = types.StringValue(clusterURL)
	}
	return output, nil
}

// generateServerlessClusterRequest was pulled out to enable unit testing
func generateServerlessClusterRequest(model models.ServerlessCluster) (*controlplanev1beta2.ServerlessClusterCreate, error) {
	return &controlplanev1beta2.ServerlessClusterCreate{
		Name:             model.Name.ValueString(),
		ServerlessRegion: model.ServerlessRegion.ValueString(),
		ResourceGroupId:  model.ResourceGroupID.ValueString(),
	}, nil
}
