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

package tests

import (
	"context"
	"fmt"
	"time"

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepNetwork struct {
	NetworkName string
	NetClient   cloudv1beta1.NetworkServiceClient
	OpsClient   cloudv1beta1.OperationServiceClient
}

func (s sweepNetwork) SweepNetworks(_ string) error {
	ctx := context.Background()
	network, err := utils.FindNetworkByName(ctx, s.NetworkName, s.NetClient)
	if err != nil {
		return fmt.Errorf("unable to sweep network: unable to find network %q: %v", s.NetworkName, err)
	}
	op, err := s.NetClient.DeleteNetwork(ctx, &cloudv1beta1.DeleteNetworkRequest{
		Id: network.GetId(),
	})
	if err != nil {
		return fmt.Errorf("unable to sweep network: unable to delete network %q: %v", s.NetworkName, err)
	}

	return utils.AreWeDoneYet(ctx, op, 15*time.Minute, time.Minute, s.OpsClient)
}
