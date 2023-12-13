package tests

import (
	"context"
	"time"

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepCluster struct {
	ClusterName string
	CluClient   cloudv1beta1.ClusterServiceClient
	OpsClient   cloudv1beta1.OperationServiceClient
}

func (s sweepCluster) SweepCluster(r string) error {
	ctx := context.Background()
	cluster, err := utils.FindClusterByName(ctx, s.ClusterName, s.CluClient)
	if err != nil {
		return err
	}

	op, err := s.CluClient.DeleteCluster(ctx, &cloudv1beta1.DeleteClusterRequest{
		Id: cluster.GetId(),
	})
	if err != nil {
		return err
	}

	return utils.AreWeDoneYet(ctx, op, 45*time.Minute, s.OpsClient)
}
