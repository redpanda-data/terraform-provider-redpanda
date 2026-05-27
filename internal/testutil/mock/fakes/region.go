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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegionFake is a read-only in-memory implementation of RegionServiceServer
// pre-seeded with AWS and GCP regions. Covers both the redpanda_region and
// redpanda_regions datasources (both call RegionService).
type RegionFake struct {
	controlplanev1grpc.UnimplementedRegionServiceServer
	regions []*controlplanev1.Region
}

// NewRegionFake returns a RegionFake pre-seeded with AWS and GCP regions.
func NewRegionFake() *RegionFake {
	return &RegionFake{
		regions: []*controlplanev1.Region{
			{
				Name:          "us-east-1",
				CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Zones:         []string{"use1-az1", "use1-az2", "use1-az6"},
			},
			{
				Name:          "us-west-2",
				CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Zones:         []string{"usw2-az1", "usw2-az2", "usw2-az3"},
			},
			{
				Name:          "us-central1",
				CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Zones:         []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
		},
	}
}

// GetRegion returns the region matching Name + CloudProvider; NotFound otherwise.
func (f *RegionFake) GetRegion(_ context.Context, req *controlplanev1.GetRegionRequest) (*controlplanev1.GetRegionResponse, error) {
	for _, r := range f.regions {
		if r.GetName() == req.GetName() && r.GetCloudProvider() == req.GetCloudProvider() {
			return &controlplanev1.GetRegionResponse{Region: r}, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "region %q / %v not found", req.GetName(), req.GetCloudProvider())
}

// ListRegions returns all regions matching the CloudProvider filter (all if unspecified).
func (f *RegionFake) ListRegions(_ context.Context, req *controlplanev1.ListRegionsRequest) (*controlplanev1.ListRegionsResponse, error) {
	out := []*controlplanev1.Region{}
	for _, r := range f.regions {
		if req.GetCloudProvider() == controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED || r.GetCloudProvider() == req.GetCloudProvider() {
			out = append(out, r)
		}
	}
	return &controlplanev1.ListRegionsResponse{Regions: out}, nil
}
