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

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepCluster struct {
	ClusterName string
	Client      *cloud.ControlPlaneClientSet
}

func (s sweepCluster) SweepCluster(_ string) error {
	ctx := context.Background()
	cluster, err := s.Client.ClusterForName(ctx, s.ClusterName)
	if err != nil {
		return err
	}

	op, err := s.Client.Cluster.DeleteCluster(ctx, &controlplanev1beta2.DeleteClusterRequest{
		Id: cluster.GetId(),
	})
	if err != nil {
		return err
	}

	return utils.AreWeDoneYet(ctx, op.Operation, 45*time.Minute, time.Minute, s.Client.Operation)
}

func (s sweepCluster) SweepServerlessCluster(_ string) error {
	ctx := context.Background()
	serverless, err := s.Client.ServerlessClusterForName(ctx, s.ClusterName)
	if err != nil {
		return err
	}

	op, err := s.Client.ServerlessCluster.DeleteServerlessCluster(ctx, &controlplanev1beta2.DeleteServerlessClusterRequest{
		Id: serverless.GetId(),
	})
	if err != nil {
		return err
	}
	return utils.AreWeDoneYet(ctx, op.Operation, 1*time.Minute, 3*time.Second, s.Client.Operation)
}
