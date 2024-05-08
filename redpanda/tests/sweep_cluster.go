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
	"time"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta1/controlplanev1beta1grpc"
	controlplanev1beta1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepCluster struct {
	ClusterName string
	CluClient   controlplanev1beta1grpc.ClusterServiceClient
	OpsClient   controlplanev1beta1grpc.OperationServiceClient
}

func (s sweepCluster) SweepCluster(_ string) error {
	ctx := context.Background()
	cluster, err := utils.GetClusterUntilRunningState(ctx, 0, 50, s.ClusterName, s.CluClient)
	if err != nil {
		return err
	}

	op, err := s.CluClient.DeleteCluster(ctx, &controlplanev1beta1.DeleteClusterRequest{
		Id: cluster.GetId(),
	})
	if err != nil {
		return err
	}

	return utils.AreWeDoneYet(ctx, op, 45*time.Minute, time.Minute, s.OpsClient)
}
