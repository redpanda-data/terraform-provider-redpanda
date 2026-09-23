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
	"regexp"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

// TestIntegration_Cluster_CreateFailedCarriesStateDescription pins that a
// create the control plane fails surfaces the cluster's state_description
// in the error: the status code always, the message when the public API
// passes one through (it blanks the message for failures the provisioner
// did not classify as external, leaving only the code).
func TestIntegration_Cluster_CreateFailedCarriesStateDescription(t *testing.T) {
	for _, tc := range []struct {
		name string
		desc *rpcstatus.Status
		want string
	}{
		{"external message", &rpcstatus.Status{Code: int32(codes.FailedPrecondition), Message: "agent never registered"}, `(?s)STATE_FAILED.*FailedPrecondition.*agent never\s+registered`},
		{"code only", &rpcstatus.Status{Code: int32(codes.Internal)}, `(?s)STATE_FAILED.*Internal`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, factories := clusterSetup(t)
			srv.Cluster.CreateMutator = func(cl *controlplanev1.Cluster) {
				cl.State = controlplanev1.Cluster_STATE_FAILED
				cl.StateDescription = tc.desc
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{{
					Config:      awsDedicatedConfig("tfrp-mock-cl-f1"),
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}
