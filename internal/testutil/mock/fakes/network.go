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
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// networkIDBase offsets the fake's id sequence so generated ids don't collide
// with other resource fakes that share the [a-v0-9]{20} alphabet.
const networkIDBase uint64 = 0x4000_0000_0000_0000

// NetworkFake is a stateful in-memory implementation of NetworkService.
// Create, Update and Delete are async: each publishes a completed Operation
// via op.Set so the provider's AreWeDoneYet polling loop resolves on the
// first GetOperation call. Update models the one settable-after-create leaf,
// customer_managed_resources.aws.public_subnets, with the control plane's
// guards (cloudv2 apps/controlplane-api/internal/services/network and
// apps/public-api-go/internal/services/network/v1).
type NetworkFake struct {
	controlplanev1grpc.UnimplementedNetworkServiceServer

	op       *OperationFake
	mu       sync.Mutex
	networks map[string]*controlplanev1.Network
	seq      atomic.Uint64
}

// NewNetworkFake returns an empty NetworkFake bound to op.
func NewNetworkFake(op *OperationFake) *NetworkFake {
	return &NetworkFake{op: op, networks: map[string]*controlplanev1.Network{}}
}

// CreateNetwork stores a new network in STATE_READY (test mode short-circuits
// the real CREATING → READY transition) and returns a completed Operation.
func (f *NetworkFake) CreateNetwork(_ context.Context, req *controlplanev1.CreateNetworkRequest) (*controlplanev1.CreateNetworkOperation, error) {
	in := req.GetNetwork()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "network is required")
	}
	if err := validateNetworkCreateShape(in); err != nil {
		return nil, err
	}
	if es := in.GetEgressSpec(); es != nil {
		if es.GetGcp() == nil && es.GetAws() == nil && es.GetAzure() == nil {
			return nil, status.Error(codes.InvalidArgument, "egress_spec.cloud_provider: exactly one field is required in oneof")
		}
		cp := in.GetCloudProvider()
		armMatches := (cp == controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS && es.GetAws() != nil) ||
			(cp == controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP && es.GetGcp() != nil) ||
			(cp == controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE && es.GetAzure() != nil)
		if !armMatches {
			return nil, status.Error(codes.InvalidArgument, "egress_spec.cloud_provider must match cloud_provider")
		}
	}
	id := xidLike(networkIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	nw := &controlplanev1.Network{
		Id:                       id,
		Name:                     in.GetName(),
		ResourceGroupId:          in.GetResourceGroupId(),
		CloudProvider:            in.GetCloudProvider(),
		Region:                   in.GetRegion(),
		CidrBlock:                in.GetCidrBlock(),
		ClusterType:              in.GetClusterType(),
		CustomerManagedResources: readShapeCustomerManagedResources(in.GetCustomerManagedResources()),
		EgressSpec:               in.GetEgressSpec(),
		State:                    controlplanev1.Network_STATE_READY,
		CreatedAt:                now,
		UpdatedAt:                now,
		Zones:                    []string{"use1-az1"},
	}
	if in.GetCustomerManagedResources() != nil {
		nw.CidrBlock = "0.0.0.0/0"
	}

	f.mu.Lock()
	f.networks[id] = nw
	f.mu.Unlock()

	return &controlplanev1.CreateNetworkOperation{Operation: completedOp(f.op, id)}, nil
}

// GetNetwork returns the stored network or NotFound.
func (f *NetworkFake) GetNetwork(_ context.Context, req *controlplanev1.GetNetworkRequest) (*controlplanev1.GetNetworkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	nw, ok := f.networks[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "network %q not found", req.GetId())
	}
	return &controlplanev1.GetNetworkResponse{Network: nw}, nil
}

// DeleteNetwork removes the stored network and returns a completed Operation
// so AreWeDoneYet resolves immediately. Returns NotFound if absent so the
// provider's IsNotFound short-circuit fires.
func (f *NetworkFake) DeleteNetwork(_ context.Context, req *controlplanev1.DeleteNetworkRequest) (*controlplanev1.DeleteNetworkOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.networks[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "network %q not found", req.GetId())
	}
	delete(f.networks, req.GetId())
	return &controlplanev1.DeleteNetworkOperation{Operation: completedOp(f.op, req.GetId())}, nil
}

const networkCMRMaskPath = "customer_managed_resources"

// isPublicSubnetsMaskPath reports whether a mask path names the one leaf the
// control plane can update, in any of the three spellings its mapper accepts.
func isPublicSubnetsMaskPath(path string) bool {
	switch path {
	case networkCMRMaskPath, networkCMRMaskPath + ".aws", networkCMRMaskPath + ".aws.public_subnets":
		return true
	default:
		return false
	}
}

// UpdateNetwork applies public_subnets under the mask and rejects what the
// control plane rejects: a mask naming any other customer-managed leaf, a
// customer-managed update on a network without AWS resources, and a change
// or clear of subnets already set (compared as a set). A mask that does not
// name customer_managed_resources leaves the stored record untouched.
func (f *NetworkFake) UpdateNetwork(_ context.Context, req *controlplanev1.UpdateNetworkRequest) (*controlplanev1.UpdateNetworkOperation, error) {
	upd := req.GetNetwork()
	if upd == nil {
		return nil, status.Error(codes.InvalidArgument, "network is required")
	}
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	nw, ok := f.networks[upd.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "network %q not found", upd.GetId())
	}

	namesCMR := false
	for _, path := range paths {
		switch {
		case isPublicSubnetsMaskPath(path):
			namesCMR = true
		case strings.HasPrefix(path, networkCMRMaskPath+"."):
			return nil, status.Errorf(codes.InvalidArgument,
				"update_mask path %q is not updatable. Only %s.aws.public_subnets may be changed after create.", path, networkCMRMaskPath)
		default:
		}
	}
	if !namesCMR {
		return &controlplanev1.UpdateNetworkOperation{Operation: completedOp(f.op, nw.GetId())}, nil
	}

	aws := nw.GetCustomerManagedResources().GetAws()
	if aws == nil {
		return nil, status.Error(codes.InvalidArgument,
			"customer_managed_resources can only be updated on a network with AWS customer-managed resources: customer_managed_resources.aws.public_subnets is the only field that is settable after create")
	}
	before := aws.GetPublicSubnets().GetArns()
	after := upd.GetCustomerManagedResources().GetAws().GetPublicSubnets().GetArns()
	if len(before) > 0 {
		if len(after) == 0 {
			return nil, status.Error(codes.FailedPrecondition,
				"customer_managed_resources.aws.public_subnets is write-once and cannot be cleared once set")
		}
		if !sameStringSet(before, after) {
			return nil, status.Error(codes.FailedPrecondition,
				"customer_managed_resources.aws.public_subnets is write-once and cannot be changed once set")
		}
	}
	if len(after) > 0 {
		aws.PublicSubnets = &controlplanev1.CustomerManagedAWSSubnets{Arns: slices.Clone(after)}
	}
	nw.UpdatedAt = timestamppb.Now()
	return &controlplanev1.UpdateNetworkOperation{Operation: completedOp(f.op, nw.GetId())}, nil
}

func sameStringSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// readShapeCustomerManagedResources returns the block as the public read
// mapper (cloudv2 apps/public-api-go network mapper) reports it: the Azure
// management bucket always carries a resource_group, with an empty name when
// the request omitted one.
func readShapeCustomerManagedResources(cmr *controlplanev1.Network_CustomerManagedResources) *controlplanev1.Network_CustomerManagedResources {
	if cmr == nil {
		return nil
	}
	out := proto.CloneOf(cmr)
	if mb := out.GetAzure().GetManagementBucket(); mb != nil && mb.GetResourceGroup() == nil {
		mb.ResourceGroup = &controlplanev1.CustomerManagedAzureResourceGroupSpec{}
	}
	return out
}

// validateNetworkCreateShape mirrors the control plane's create-time rules
// that tie customer-managed resources to the rest of the network (cloudv2
// apps/controlplane-api/internal/services/network/network_service.go
// validateAndDefaultCreateRequest and validateCustomerManagedResources): a
// Redpanda-provisioned VPC needs a CIDR, a customer-managed one must not carry
// one, must be BYOC, and a GCP arm needs the GCP provider.
func validateNetworkCreateShape(in *controlplanev1.NetworkCreate) error {
	cmr := in.GetCustomerManagedResources()
	if cmr == nil {
		if in.GetCidrBlock() == "" {
			return status.Error(codes.InvalidArgument, "invalid cidr: empty")
		}
		return nil
	}
	if in.GetClusterType() != controlplanev1.Cluster_TYPE_BYOC {
		return status.Error(codes.InvalidArgument, "network.cluster_type must be TYPE_BYOC if network.customer_managed_resources is set")
	}
	if cmr.GetGcp() != nil && in.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP {
		return status.Error(codes.InvalidArgument, "network.cloud_provider must be CLOUD_PROVIDER_GCP for network.customer_managed_resources.gcp")
	}
	if in.GetCidrBlock() != "" {
		return status.Error(codes.InvalidArgument, "network.cidr_block must not be set if network.customer_managed_resources is set")
	}
	return nil
}
