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

package fakes

import (
	"context"
	"strings"
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
// pathMap entry: the API accepts them only at leaf granularity. A fake that
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
			// must too.
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
			// spec.private_link_service subtree, supported_regions included, so
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
			// CP defaulter: disabling replaces the whole spec with a bare
			// {Enabled: false}, so zones, replicas and url are all cleared,
			// even when the caller still sends the prior zones.
			name: "disable clears zones replicas and url",
			seed: &controlplanev1.Cluster{
				Id: id, Zones: []string{az1},
				Rpsql: rpsql(true, 3, az1),
			},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(false, 3, az1)},
			mask:   []string{"rpsql.enabled"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if z := cl.GetRpsql().GetZones(); len(z) != 0 {
					t.Fatalf("zones retained on disable: got %v", z)
				}
				if r := cl.GetRpsql().GetReplicas(); r != 0 {
					t.Fatalf("replicas not reset on disable: got %d", r)
				}
				if cl.GetRpsql().GetUrl() != "" {
					t.Fatalf("url not cleared on disable: got %q", cl.GetRpsql().GetUrl())
				}
			},
		},
		{
			// validateOxlaZonesImmutable early-returns when the update disables,
			// so dropping zones on the way down is not a blocked "zone change".
			name: "disable with empty zones not blocked by immutability",
			seed: &controlplanev1.Cluster{
				Id: id, Zones: []string{az1},
				Rpsql: rpsql(true, 3, az1),
			},
			update: &controlplanev1.ClusterUpdate{Id: id, Rpsql: rpsql(false, 0)},
			mask:   []string{"rpsql.enabled", "rpsql.zones"},
			assert: func(t *testing.T, cl *controlplanev1.Cluster) {
				if cl.GetRpsql().GetEnabled() {
					t.Fatal("disable rejected")
				}
				if z := cl.GetRpsql().GetZones(); len(z) != 0 {
					t.Fatalf("zones retained on disable: got %v", z)
				}
			},
		},
		{
			// validateOxlaZones: the zone must be one of the cluster's zones,
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

const (
	testPubEP = "pub-ep"
	testPrvEP = "prv-ep"
)

func connSpec(t controlplanev1.Cluster_ConnectionType, m controlplanev1.AuthMode) *controlplanev1.ConnectionSpec {
	return &controlplanev1.ConnectionSpec{Type: t, Auth: &controlplanev1.AuthSpec{Mode: m}}
}

// TestReconcileConnections pins the fake's mirror of the control plane's
// listener reconcile (apiConnectionsToListenersForUpdateCluster): retained and
// auth-switched entries keep their stored POSITION and ENDPOINT, new entries
// append in request order, undesired entries drop, so the echoed order
// deliberately diverges from request order, like the real backend.
func TestReconcileConnections(t *testing.T) {
	pubSASL := connSpec(controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, controlplanev1.AuthMode_AUTH_MODE_SASL)
	prvSASL := connSpec(controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE, controlplanev1.AuthMode_AUTH_MODE_SASL)
	pubMTLS := connSpec(controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, controlplanev1.AuthMode_AUTH_MODE_MTLS)

	stored := []*controlplanev1.ConnectionStatus{
		{Config: pubSASL, Endpoint: testPubEP},
		{Config: prvSASL, Endpoint: testPrvEP},
	}

	t.Run("auth switch preserves endpoint and position", func(t *testing.T) {
		got := reconcileConnections("kafka_api", stored, []*controlplanev1.ConnectionSpec{pubMTLS, prvSASL})
		if len(got) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(got))
		}
		if got[0].GetConfig().GetAuth().GetMode() != controlplanev1.AuthMode_AUTH_MODE_MTLS || got[0].GetEndpoint() != testPubEP {
			t.Fatalf("entry 0 = %v/%s, want mtls with preserved pub-ep", got[0].GetConfig(), got[0].GetEndpoint())
		}
		if got[1].GetEndpoint() != testPrvEP {
			t.Fatalf("entry 1 endpoint = %s, want prv-ep", got[1].GetEndpoint())
		}
	})

	t.Run("request reorder keeps stored order", func(t *testing.T) {
		got := reconcileConnections("kafka_api", stored, []*controlplanev1.ConnectionSpec{prvSASL, pubSASL})
		if got[0].GetConfig().GetType() != controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC {
			t.Fatalf("entry 0 type = %v, want stored-order public first", got[0].GetConfig().GetType())
		}
		if got[0].GetEndpoint() != testPubEP || got[1].GetEndpoint() != testPrvEP {
			t.Fatalf("endpoints = %s/%s, want preserved pub-ep/prv-ep", got[0].GetEndpoint(), got[1].GetEndpoint())
		}
	})

	t.Run("add appends and remove drops", func(t *testing.T) {
		got := reconcileConnections("kafka_api", stored, []*controlplanev1.ConnectionSpec{pubSASL, pubMTLS})
		if len(got) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(got))
		}
		if got[0].GetEndpoint() != testPubEP {
			t.Fatalf("entry 0 endpoint = %s, want retained pub-ep", got[0].GetEndpoint())
		}
		// prv-sasl dropped; pub-mtls is genuinely new (pub-sasl still desired,
		// so no rename) with a fresh endpoint appended last.
		if got[1].GetConfig().GetAuth().GetMode() != controlplanev1.AuthMode_AUTH_MODE_MTLS || got[1].GetEndpoint() == testPrvEP {
			t.Fatalf("entry 1 = %v/%s, want appended fresh mtls", got[1].GetConfig(), got[1].GetEndpoint())
		}
	})
}

// TestClusterFake_PrivateOnlyGainsPublicRejected pins the fake's mirror of the
// control plane's in-place topology restriction: a cluster whose stored
// listeners are all private cannot gain a public listener through a
// connections update.
func TestClusterFake_PrivateOnlyGainsPublicRejected(t *testing.T) {
	f := NewClusterFake(NewOperationFake())
	ctx := context.Background()

	conn := func(ct controlplanev1.Cluster_ConnectionType) *controlplanev1.ConnectionSpec {
		return &controlplanev1.ConnectionSpec{Type: ct, Auth: &controlplanev1.AuthSpec{Mode: controlplanev1.AuthMode_AUTH_MODE_SASL}}
	}
	prv := []*controlplanev1.ConnectionSpec{conn(connTypePrivate)}

	op, err := f.CreateCluster(ctx, &controlplanev1.CreateClusterRequest{
		Cluster: &controlplanev1.ClusterCreate{
			Name:           "private-only",
			CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
			Type:           controlplanev1.Cluster_TYPE_BYOC,
			KafkaApi:       &controlplanev1.KafkaAPISpec{Connections: prv},
			HttpProxy:      &controlplanev1.HTTPProxySpec{Connections: prv},
			SchemaRegistry: &controlplanev1.SchemaRegistrySpec{Connections: prv},
		},
	})
	if err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	id := op.GetOperation().GetResourceId()

	_, err = f.UpdateCluster(ctx, &controlplanev1.UpdateClusterRequest{
		Cluster: &controlplanev1.ClusterUpdate{
			Id:       id,
			KafkaApi: &controlplanev1.KafkaAPISpec{Connections: []*controlplanev1.ConnectionSpec{conn(connTypePublic), conn(connTypePrivate)}},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"kafka_api.connections"}},
	})
	if err == nil {
		t.Fatal("UpdateCluster accepted a public listener on a private-only cluster")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot gain public listeners") {
		t.Fatalf("expected the private-only rejection, got a different error: %v", err)
	}
}
