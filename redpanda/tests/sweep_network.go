package tests

import (
	"context"
	"time"

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepNetwork struct {
	NetworkName string
	NetClient   cloudv1beta1.NetworkServiceClient
	OpsClient   cloudv1beta1.OperationServiceClient
}

func (s sweepNetwork) SweepNetworks(r string) error {
	ctx := context.Background()
	network, err := utils.FindNetworkByName(ctx, s.NetworkName, s.NetClient)
	if err != nil {
		return err
	}
	op, err := s.NetClient.DeleteNetwork(ctx, &cloudv1beta1.DeleteNetworkRequest{
		Id: network.GetId(),
	})
	if err != nil {
		return err
	}

	return utils.AreWeDoneYet(ctx, op, 15*time.Minute, s.OpsClient)
}
