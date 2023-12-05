package tests

import (
	"context"
	"fmt"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"time"
)

type sweepNetwork struct {
	NetworkName string
	NetClient   cloudv1beta1.NetworkServiceClient
	OpsClient   cloudv1beta1.OperationServiceClient
}

func (s sweepNetwork) SweepNetworks(r string) error {
	ctx := context.Background()
	networks, err := s.NetClient.ListNetworks(ctx, &cloudv1beta1.ListNetworksRequest{
		Filter: &cloudv1beta1.ListNetworksRequest_Filter{
			Name: s.NetworkName,
		},
	})
	if err != nil {
		return err
	}
	for _, v := range networks.GetNetworks() {
		if v.GetName() == s.NetworkName {
			op, err := s.NetClient.DeleteNetwork(ctx, &cloudv1beta1.DeleteNetworkRequest{
				Id: v.GetId(),
			})
			if err != nil {
				return err
			}
			if err := utils.AreWeDoneYet(ctx, op, 15*time.Minute, s.OpsClient); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("network %s not found", s.NetworkName)
}
