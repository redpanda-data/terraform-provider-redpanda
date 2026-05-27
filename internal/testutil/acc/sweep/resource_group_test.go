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

package sweep_test

import (
	"context"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"google.golang.org/grpc"
)

// TestSweepResourceGroup_SendsUUIDNotName pins the contract that
// SweepResourceGroup passes the resource group's UUID, not its human-readable
// Name, to DeleteResourceGroup. DeleteResourceGroupRequest.id is marked
// [string.uuid] in the proto descriptor, so passing the Name fails server-side
// protovalidate with InvalidArgument and leaves orphaned resource groups
// behind after cluster acc-test sweeps.
func TestSweepResourceGroup_SendsUUIDNotName(t *testing.T) {
	srv := mock.New(t)

	conn, err := grpc.NewClient("passthrough:///bufnet", srv.Dialer()...)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := cloud.NewControlPlaneClientSet(conn)
	ctx := context.Background()

	const name = "tfrp-sweep-uuid-test"
	rg, err := client.CreateResourceGroup(ctx, name)
	if err != nil {
		t.Fatalf("seed CreateResourceGroup: %v", err)
	}
	if rg.GetId() == "" || rg.GetId() == rg.GetName() {
		t.Fatalf("seed produced unusable RG: id=%q name=%q", rg.GetId(), rg.GetName())
	}

	sweeper := sweep.ResourceGroup{ResourceGroupName: name, Client: client}
	if err := sweeper.SweepResourceGroup(""); err != nil {
		t.Fatalf("SweepResourceGroup: %v", err)
	}

	if _, getErr := client.ResourceGroupForName(ctx, name); getErr == nil {
		t.Fatal("expected resource group to be gone after sweep, but ResourceGroupForName returned no error")
	}
}
