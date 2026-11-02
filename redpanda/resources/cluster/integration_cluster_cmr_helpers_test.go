//go:build integration

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

package cluster_test

import (
	"context"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// captureClusterID stores the applied cluster's id for a later out-of-band
// control plane call.
func captureClusterID(id *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		*id = s.RootModule().Resources[clusterAddr].Primary.ID
		return nil
	}
}

// updateCMROutOfBand changes the cluster's customer-managed resources through
// the control plane, the way the UI, the API or an older provider does. It
// goes through the mock server's gRPC front so the request meets the same
// validation the control plane applies, not the fake's handler alone.
func updateCMROutOfBand(t *testing.T, srv *mock.Server, id *string, upd *controlplanev1.CustomerManagedResourcesUpdate, paths ...string) func() {
	return func() {
		conn, err := grpc.NewClient("passthrough:///bufnet", srv.Dialer()...)
		if err != nil {
			t.Fatalf("grpc.NewClient: %v", err)
		}
		defer conn.Close()
		_, err = controlplanev1grpc.NewClusterServiceClient(conn).UpdateCluster(context.Background(), &controlplanev1.UpdateClusterRequest{
			Cluster:    &controlplanev1.ClusterUpdate{Id: *id, CustomerManagedResources: upd},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
		})
		if err != nil {
			t.Fatalf("out-of-band update: %v", err)
		}
	}
}
