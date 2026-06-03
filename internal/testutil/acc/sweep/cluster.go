// Copyright 2026 Redpanda Data, Inc.
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

// Package sweep provides resource-level sweepers used by live-acceptance
// tests to tear down leaked Redpanda Cloud resources.
package sweep

import (
	"context"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Cluster sweeps dedicated and serverless clusters by name.
type Cluster struct {
	ClusterName string
	Client      *cloud.ControlPlaneClientSet
}

// SweepCluster deletes the dedicated cluster matching ClusterName.
func (s Cluster) SweepCluster(_ string) error {
	ctx := context.Background()
	cluster, err := s.Client.ClusterForName(ctx, s.ClusterName)
	if err != nil {
		return err
	}

	op, err := s.Client.Cluster.DeleteCluster(ctx, &controlplanev1.DeleteClusterRequest{
		Id: cluster.GetId(),
	})
	if err != nil {
		return err
	}

	return utils.AreWeDoneYet(ctx, op.Operation, 45*time.Minute, s.Client.Operation)
}

// SweepServerlessCluster deletes the serverless cluster matching ClusterName.
func (s Cluster) SweepServerlessCluster(_ string) error {
	ctx := context.Background()
	serverless, err := s.Client.ServerlessClusterForName(ctx, s.ClusterName)
	if err != nil {
		return err
	}

	op, err := s.Client.ServerlessCluster.DeleteServerlessCluster(ctx, &controlplanev1.DeleteServerlessClusterRequest{
		Id: serverless.GetId(),
	})
	if err != nil {
		return err
	}
	return utils.AreWeDoneYet(ctx, op.Operation, 15*time.Minute, s.Client.Operation)
}
