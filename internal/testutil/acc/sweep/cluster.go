// Copyright 2026 Redpanda Data, Inc.
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
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Cluster sweeps dedicated and serverless clusters by name.
type Cluster struct {
	ClusterName string
	Client      *cloud.ControlPlaneClientSet
	// Logf receives one line per swept cluster that is not READY, carrying
	// its state and state_description. The public API cannot read a deleted
	// cluster, so this line is the only place a failed create's reason
	// survives. Nil prints to stdout like the cleanup registry does.
	Logf func(format string, args ...any)
}

// SweepCluster deletes the dedicated cluster matching ClusterName.
func (s Cluster) SweepCluster(_ string) error {
	ctx := context.Background()
	cluster, err := s.Client.ClusterForName(ctx, s.ClusterName)
	if err != nil {
		return err
	}
	if cluster.GetState() != controlplanev1.Cluster_STATE_READY {
		s.logf("acc cleanup: cluster %q (%s) is %s: %s", cluster.GetName(), cluster.GetId(), cluster.GetState(), utils.DescribeStatus(cluster.GetStateDescription()))
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

func (s Cluster) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}
