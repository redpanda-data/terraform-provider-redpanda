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

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
)

// ServerlessRegionFake is a read-only in-memory implementation of
// ServerlessRegionServiceServer pre-seeded with AWS and GCP serverless regions.
type ServerlessRegionFake struct {
	controlplanev1grpc.UnimplementedServerlessRegionServiceServer
	regions []*controlplanev1.ServerlessRegion
}

// NewServerlessRegionFake returns a ServerlessRegionFake pre-seeded with
// AWS and GCP serverless regions.
func NewServerlessRegionFake() *ServerlessRegionFake {
	return &ServerlessRegionFake{
		regions: []*controlplanev1.ServerlessRegion{
			{
				Name:          "pro-us-east-1",
				CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Placement:     &controlplanev1.ServerlessRegion_Placement{Enabled: true},
			},
			{
				Name:          "pro-us-central1",
				CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Placement:     &controlplanev1.ServerlessRegion_Placement{Enabled: true},
			},
		},
	}
}

// ListServerlessRegions returns serverless regions matching the
// CloudProvider filter (all if unspecified).
func (f *ServerlessRegionFake) ListServerlessRegions(_ context.Context, req *controlplanev1.ListServerlessRegionsRequest) (*controlplanev1.ListServerlessRegionsResponse, error) {
	out := []*controlplanev1.ServerlessRegion{}
	for _, r := range f.regions {
		if req.GetCloudProvider() == controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED || r.GetCloudProvider() == req.GetCloudProvider() {
			out = append(out, r)
		}
	}
	return &controlplanev1.ListServerlessRegionsResponse{ServerlessRegions: out}, nil
}
