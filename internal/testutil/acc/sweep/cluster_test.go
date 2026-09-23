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

package sweep_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// TestSweepCluster_LogsFailedStateBeforeDeleting pins that a cluster the
// control plane marked FAILED has its state and state_description logged
// before the sweep deletes it, since the public API cannot read a deleted
// cluster and the job log is the only place the reason survives.
func TestSweepCluster_LogsFailedStateBeforeDeleting(t *testing.T) {
	srv := mock.New(t)
	conn, err := grpc.NewClient("passthrough:///bufnet", srv.Dialer()...)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	const name = "tfrp-sweep-failed"
	srv.Cluster.Seed(&controlplanev1.Cluster{
		Id:    "d0000000000000000000",
		Name:  name,
		Type:  controlplanev1.Cluster_TYPE_DEDICATED,
		State: controlplanev1.Cluster_STATE_FAILED,
		StateDescription: &rpcstatus.Status{
			Code:    int32(codes.FailedPrecondition),
			Message: "agent never registered",
		},
	})

	var logged []string
	s := sweep.Cluster{
		ClusterName: name,
		Client:      cloud.NewControlPlaneClientSet(conn),
		Logf:        func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) },
	}
	if err := s.SweepCluster(""); err != nil {
		t.Fatalf("SweepCluster: %v", err)
	}
	joined := strings.Join(logged, "\n")
	for _, want := range []string{name, "STATE_FAILED", "FailedPrecondition", "agent never registered"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("sweep log %q does not mention %q", joined, want)
		}
	}
	if _, err := s.Client.ClusterForName(context.Background(), name); err == nil {
		t.Fatalf("cluster %q still exists after the sweep", name)
	}
}
