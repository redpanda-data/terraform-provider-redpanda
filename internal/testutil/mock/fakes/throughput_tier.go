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

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc"
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
)

// ThroughputTierFake is a read-only in-memory implementation of
// ThroughputTierServiceServer (controlplane v1beta2). Pre-seeded with
// representative AWS and GCP tiers.
type ThroughputTierFake struct {
	controlplanev1beta2grpc.UnimplementedThroughputTierServiceServer
	tiers []*controlplanev1beta2.ThroughputTier
}

// NewThroughputTierFake returns a ThroughputTierFake pre-seeded with tiers.
func NewThroughputTierFake() *ThroughputTierFake {
	return &ThroughputTierFake{
		tiers: []*controlplanev1beta2.ThroughputTier{
			{
				Name:          "tier-1-10g",
				DisplayName:   "10 Gbps",
				CloudProvider: controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS,
			},
			{
				Name:          "tier-1-10g",
				DisplayName:   "10 Gbps",
				CloudProvider: controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
			},
		},
	}
}

// ListThroughputTiers returns tiers optionally filtered by CloudProvider.
func (f *ThroughputTierFake) ListThroughputTiers(_ context.Context, req *controlplanev1beta2.ListThroughputTiersRequest) (*controlplanev1beta2.ListThroughputTiersResponse, error) {
	out := []*controlplanev1beta2.ThroughputTier{}
	for _, t := range f.tiers {
		if req.GetFilter() == nil || req.GetFilter().GetCloudProvider() == controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED || t.GetCloudProvider() == req.GetFilter().GetCloudProvider() {
			out = append(out, t)
		}
	}
	return &controlplanev1beta2.ListThroughputTiersResponse{ThroughputTiers: out}, nil
}
