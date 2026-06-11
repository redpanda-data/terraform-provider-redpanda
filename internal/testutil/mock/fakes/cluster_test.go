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

package fakes

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// TestClusterFake_UpdateMaskFidelity pins the fake's UpdateCluster to the real
// control-plane field-mask contract (cloudv2
// apps/public-api-go/internal/services/cluster/v1/mapper.go pathMap): the API
// translates the public mask via exact-match lookup and SILENTLY DROPS any path
// without a mapping. Crucially, rpsql and kafka_connect have NO top-level
// pathMap entry — the API accepts them only at leaf granularity. A fake that
// applies the bare object path by reflection is more permissive than the API,
// so a provider that emits the wrong (un-expanded) mask would pass tests the
// real API would reject. This test makes the fake reject what the API rejects.
func TestClusterFake_UpdateMaskFidelity(t *testing.T) {
	const id = "c1"
	const az1, az2 = "use1-az1", "use1-az2"

	rpsql := func(enabled bool, replicas int32, zones ...string) *controlplanev1.RPSql {
		return &controlplanev1.RPSql{Enabled: enabled, Replicas: replicas, Zones: zones}
	}

	cases := []struct {
		name    string
		seed    *controlplanev1.Cluster
		update  *controlplanev1.ClusterUpdate
		mask    []string
		wantErr codes.Code
		assert  func(t *testing.T, cl *controlplanev1.Cluster)
	}{
		{
			// Top-level "rpsql" has no pathMap entry → API drops it. The fake
			// must not apply the rpsql payload.
			name:   "top-level rpsql dropped",
			seed:   &controlplanev1.Cluster{Id: id, Rpsql: rpsql(false, 1)},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 5)},
			mask:   []string{"rpsql"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if cl.GetRpsql().GetEnabled() {
					t.Fatal("top-level rpsql mask was applied; API would drop it")
				}
			},
		},
		{
			name:   "rpsql.enabled leaf applied",
			seed:   &controlplanev1.Cluster{Id: id},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 3)},
			mask:   []string{"rpsql.enabled"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if !cl.GetRpsql().GetEnabled() {
					t.Fatal("rpsql.enabled leaf was not applied")
				}
			},
		},
		{
			// The provider emits rpsql.zones as one of the expanded leaves; the
			// fake must honor a zones-only mask.
			name:   "rpsql.zones leaf applied",
			seed:   &controlplanev1.Cluster{Id: id, Rpsql: rpsql(true, 3)},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 3, az1)},
			mask:   []string{"rpsql.zones"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				z := cl.GetRpsql().GetZones()
				if len(z) != 1 || z[0] != az1 {
					t.Fatalf("rpsql.zones leaf not applied: got %v", z)
				}
			},
		},
		{
			// Top-level "kafka_connect" has no pathMap entry → API drops it.
			name:   "top-level kafka_connect dropped",
			seed:   &controlplanev1.Cluster{Id: id, KafkaConnect: &controlplanev1.KafkaConnect{Enabled: false}},
			update: &controlplanev1.ClusterUpdate{Id: id, KafkaConnect: &controlplanev1.KafkaConnect{Enabled: true}},
			mask:   []string{"kafka_connect"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if cl.GetKafkaConnect().GetEnabled() {
					t.Fatal("top-level kafka_connect mask was applied; API would drop it")
				}
			},
		},
		{
			// kafka_connect.enabled IS in pathMap → API applies it. The fake
			// must too (it currently has no handler and would drop it).
			name:   "kafka_connect.enabled leaf applied",
			seed:   &controlplanev1.Cluster{Id: id},
			update: &controlplanev1.ClusterUpdate{Id: id, KafkaConnect: &controlplanev1.KafkaConnect{Enabled: true}},
			mask:   []string{"kafka_connect.enabled"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if !cl.GetKafkaConnect().GetEnabled() {
					t.Fatal("kafka_connect.enabled leaf was not applied")
				}
			},
		},
		{
			// Top-level aws_private_link HAS a pathMap entry → API accepts it.
			name:   "top-level aws_private_link applied",
			seed:   &controlplanev1.Cluster{Id: id},
			update: &controlplanev1.ClusterUpdate{Id: id, AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{Enabled: true}},
			mask:   []string{"aws_private_link"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if !cl.GetAwsPrivateLink().GetEnabled() {
					t.Fatal("top-level aws_private_link was not applied")
				}
			},
		},
		{
			// The CP's top-level aws_private_link mapping covers the whole
			// spec.private_link_service subtree, supported_regions included —
			// the fake must apply the incoming value, not preserve the old one.
			name: "aws_private_link supported_regions applied",
			seed: &controlplanev1.Cluster{Id: id, AwsPrivateLink: &controlplanev1.Cluster_AWSPrivateLink{
				Enabled: true, SupportedRegions: []string{"us-east-1"},
			}},
			update: &controlplanev1.ClusterUpdate{Id: id, AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
				Enabled: true, SupportedRegions: []string{"eu-west-1"},
			}},
			mask: []string{"aws_private_link"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				sr := cl.GetAwsPrivateLink().GetSupportedRegions()
				if len(sr) != 1 || sr[0] != "eu-west-1" {
					t.Fatalf("supported_regions not applied: got %v", sr)
				}
			},
		},
		{
			// validateOxlaZonesImmutable: zones are immutable once set; only the
			// one-time populate from empty is allowed (pinned by "rpsql.zones
			// leaf applied" above). A change must be rejected.
			name:    "rpsql.zones change rejected",
			seed:    &controlplanev1.Cluster{Id: id, Rpsql: rpsql(true, 3, az1)},
			update:  &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 3, az2)},
			mask:    []string{"rpsql.zones"},
			wantErr: codes.InvalidArgument,
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				z := cl.GetRpsql().GetZones()
				if len(z) != 1 || z[0] != az1 {
					t.Fatalf("zones mutated despite rejection: got %v", z)
				}
			},
		},
		{
			// CP defaulter: enabling Redpanda SQL with no zones assigns the
			// first cluster zone and persists it.
			name:   "rpsql enable auto-assigns first cluster zone",
			seed:   &controlplanev1.Cluster{Id: id, Zones: []string{az1, az2}},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 1)},
			mask:   []string{"rpsql.enabled"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				z := cl.GetRpsql().GetZones()
				if len(z) != 1 || z[0] != az1 {
					t.Fatalf("defaulter did not assign first cluster zone: got %v", z)
				}
			},
		},
		{
			// Explicit zones beat the defaulter.
			name:   "explicit zones win over defaulter",
			seed:   &controlplanev1.Cluster{Id: id, Zones: []string{az1, az2}},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 1, az2)},
			mask:   []string{"rpsql.enabled"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				z := cl.GetRpsql().GetZones()
				if len(z) != 1 || z[0] != az2 {
					t.Fatalf("explicit zones overridden: got %v", z)
				}
			},
		},
		{
			// Disable keeps zones (the defaulter never clears; the provider
			// sends the pinned value) and clears the url.
			name: "disable retains zones and clears url",
			seed: &controlplanev1.Cluster{
				Id: id, Zones: []string{az1},
				Rpsql: rpsql(true, 3, az1),
			},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(false, 3, az1)},
			mask:   []string{"rpsql.enabled"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				z := cl.GetRpsql().GetZones()
				if len(z) != 1 || z[0] != az1 {
					t.Fatalf("zones lost on disable: got %v", z)
				}
				if cl.GetRpsql().GetUrl() != "" {
					t.Fatalf("url not cleared on disable: got %q", cl.GetRpsql().GetUrl())
				}
			},
		},
		{
			// validateOxlaZones: the zone must be one of the cluster's zones —
			// the rejection live validation observed on a single-AZ cluster.
			name:    "zone outside cluster zones rejected",
			seed:    &controlplanev1.Cluster{Id: id, Zones: []string{az1}},
			update:  &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 1, "use1-az9")},
			mask:    []string{"rpsql.zones"},
			wantErr: codes.InvalidArgument,
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if cl.GetRpsql().GetEnabled() {
					t.Fatal("rpsql applied despite membership rejection")
				}
			},
		},
		{
			// Pure immutability on a multi-AZ cluster: the target zone IS a
			// cluster zone, so membership passes and immutability fires.
			name: "zones change to another cluster zone still immutable",
			seed: &controlplanev1.Cluster{
				Id: id, Zones: []string{az1, az2},
				Rpsql: rpsql(true, 3, az1),
			},
			update:  &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(true, 3, az2)},
			mask:    []string{"rpsql.zones"},
			wantErr: codes.InvalidArgument,
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				z := cl.GetRpsql().GetZones()
				if len(z) != 1 || z[0] != az1 {
					t.Fatalf("zones mutated despite immutability: got %v", z)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewClusterFake(NewOperationFake())
			f.Seed(tc.seed)
			_, err := f.UpdateCluster(context.Background(), &controlplanev1.UpdateClusterRequest{
				Cluster:    tc.update,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: tc.mask},
			})
			if tc.wantErr != codes.OK {
				if status.Code(err) != tc.wantErr {
					t.Fatalf("UpdateCluster: got error %v, want code %v", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("UpdateCluster: %v", err)
			}
			resp, err := f.GetCluster(context.Background(), &controlplanev1.GetClusterRequest{Id: id})
			if err != nil {
				t.Fatalf("GetCluster: %v", err)
			}
			tc.assert(t, resp.GetCluster())
		})
	}
}
