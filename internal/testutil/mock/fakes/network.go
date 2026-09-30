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

	// awsAccounts holds the AWS account the control plane stamps on a
	// network created with a cloud provider access (spec.cloud_account in
	// cloudv2), which the public read shape does not expose. A swap must
	// stay in it.
	awsAccounts map[string]string

	// CloudProviderAccessLookup resolves a cloud_provider_access_id so create
	// can apply the control plane's cross-account rules; the mock server
	// wires it to the cloud provider access fake. Nil treats every id as
	// unknown.
	CloudProviderAccessLookup func(id string) *controlplanev1.CloudProviderAccess

	// HasClusters reports whether a cluster not yet deleted is on the network;
	// the mock server wires it to the cluster fake. Nil means none.
	HasClusters func(networkID string) bool
}

// NewNetworkFake returns an empty NetworkFake bound to op.
func NewNetworkFake(op *OperationFake) *NetworkFake {
	return &NetworkFake{op: op, networks: map[string]*controlplanev1.Network{}, awsAccounts: map[string]string{}}
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
	account, err := f.validateCloudProviderAccess(in)
	if err != nil {
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
		CloudProviderAccessId:    in.GetCloudProviderAccessId(),
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
	if account != "" {
		f.awsAccounts[id] = account
	}
	f.mu.Unlock()

	return &controlplanev1.CreateNetworkOperation{Operation: completedOp(f.op, id)}, nil
}

// validateCloudProviderAccess mirrors resolveCloudProviderAccess in cloudv2's
// network service: a cloud provider access excludes customer-managed
// resources, needs a BYOC AWS network, and must exist in the organization.
// It returns the AWS account of the access's role, which create stamps on the
// network. Its rejections carry only a code: the create path attaches no
// ExternalError detail, so the public API drops the message.
func (f *NetworkFake) validateCloudProviderAccess(in *controlplanev1.NetworkCreate) (string, error) {
	id := in.GetCloudProviderAccessId()
	if id == "" {
		return "", nil
	}
	if in.GetCustomerManagedResources() != nil || in.GetClusterType() != controlplanev1.Cluster_TYPE_BYOC ||
		in.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS {
		return "", status.Error(codes.InvalidArgument, "")
	}
	_, account, err := f.cloudProviderAccessAccount(id)
	if err != nil {
		return "", status.Error(status.Code(err), "")
	}
	return account, nil
}

// cloudProviderAccessAccount resolves an access and the AWS account of its
// role, or NotFound.
func (f *NetworkFake) cloudProviderAccessAccount(id string) (*controlplanev1.CloudProviderAccess, string, error) {
	var cpa *controlplanev1.CloudProviderAccess
	if f.CloudProviderAccessLookup != nil {
		cpa = f.CloudProviderAccessLookup(id)
	}
	if cpa == nil {
		return nil, "", status.Errorf(codes.NotFound, "cloud_provider_access %q not found", id)
	}
	// arn:aws:iam::<account>:role/<name>; the mock server's protovalidate
	// interceptor enforces the role ARN pattern when the access is created.
	parts := strings.SplitN(cpa.GetAws().GetRoleArn(), ":", 6)
	if len(parts) < 5 || parts[4] == "" {
		return nil, "", status.Errorf(codes.FailedPrecondition, "cloud_provider_access has malformed role_arn: %q", cpa.GetAws().GetRoleArn())
	}
	return cpa, parts[4], nil
}

// validateCloudProviderAccessSwap mirrors validateCloudProviderAccessSwap in
// cloudv2's network service for an organization with cross-account BYOC
// enabled: a network created with an access can be re-pointed at another
// ACTIVE access for the same AWS account while READY, and an access can be
// neither added nor cleared. Called with f.mu held.
func (f *NetworkFake) validateCloudProviderAccessSwap(nw *controlplanev1.Network, newID string) error {
	oldID := nw.GetCloudProviderAccessId()
	if newID == oldID {
		return nil
	}
	if newID == "" {
		return status.Error(codes.InvalidArgument,
			"cloud_provider_access_id cannot be cleared: a cross-account network can only be re-pointed at another cloud provider access")
	}
	if oldID == "" {
		return status.Error(codes.FailedPrecondition,
			"cloud_provider_access_id can only be changed on a network created with one; moving an existing network onto cross-account provisioning is not supported")
	}
	if nw.GetState() != controlplanev1.Network_STATE_READY {
		return status.Errorf(codes.FailedPrecondition,
			"cloud_provider_access_id can only be changed on a network in state READY; the network is in state %s", nw.GetState())
	}
	networkAccount := f.awsAccounts[nw.GetId()]
	if networkAccount == "" {
		return status.Error(codes.FailedPrecondition,
			"the network has no AWS account recorded, so a cloud provider access for the same account cannot be verified")
	}
	cpa, account, err := f.cloudProviderAccessAccount(newID)
	if err != nil {
		return err
	}
	if cpa.GetState() != controlplanev1.CloudProviderAccess_STATE_ACTIVE {
		return status.Errorf(codes.FailedPrecondition,
			"cloud_provider_access %q is in state %s; only an ACTIVE access can provision a network", newID, cpa.GetState())
	}
	if account != networkAccount {
		return status.Errorf(codes.FailedPrecondition,
			"cloud_provider_access %q is for AWS account %s, but the network is provisioned in AWS account %s", newID, account, networkAccount)
	}
	return nil
}

// ReferencesCloudProviderAccess reports whether a stored network uses the
// cloud provider access with the given id.
func (f *NetworkFake) ReferencesCloudProviderAccess(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, nw := range f.networks {
		if nw.GetCloudProviderAccessId() == id {
			return true
		}
	}
	return false
}

// Lookup returns the stored network with the given id, or nil.
func (f *NetworkFake) Lookup(id string) *controlplanev1.Network {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.networks[id]
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
	// The control plane refuses while a cluster is on the network (cloudv2
	// network_service.go DeleteNetwork, REASON_NETWORK_CONTAINS_CLUSTERS);
	// its ExternalError carries no message, so the public API returns a
	// bare code. Checked before f.mu because the cluster fake takes this
	// fake's lock when it resolves a network.
	if f.HasClusters != nil && f.HasClusters(req.GetId()) {
		return nil, status.Error(codes.FailedPrecondition, "")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.networks[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "network %q not found", req.GetId())
	}
	delete(f.networks, req.GetId())
	delete(f.awsAccounts, req.GetId())
	return &controlplanev1.DeleteNetworkOperation{Operation: completedOp(f.op, req.GetId())}, nil
}

const (
	networkCMRMaskPath = "customer_managed_resources"
	networkCPAMaskPath = "cloud_provider_access_id"
)

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

	namesCMR, namesCPA := false, false
	for _, path := range paths {
		switch {
		case path == networkCPAMaskPath:
			namesCPA = true
		case isPublicSubnetsMaskPath(path):
			namesCMR = true
		case strings.HasPrefix(path, networkCMRMaskPath+"."):
			return nil, status.Errorf(codes.InvalidArgument,
				"update_mask path %q is not updatable. Only %s.aws.public_subnets may be changed after create.", path, networkCMRMaskPath)
		default:
		}
	}
	if namesCPA {
		if err := f.validateCloudProviderAccessSwap(nw, upd.GetCloudProviderAccessId()); err != nil {
			return nil, err
		}
	}
	if namesCMR {
		if err := applyPublicSubnetsUpdate(nw, upd); err != nil {
			return nil, err
		}
	}
	if namesCPA {
		nw.CloudProviderAccessId = upd.GetCloudProviderAccessId()
	}
	if namesCPA || namesCMR {
		nw.UpdatedAt = timestamppb.Now()
	}
	return &controlplanev1.UpdateNetworkOperation{Operation: completedOp(f.op, nw.GetId())}, nil
}

// applyPublicSubnetsUpdate applies the write-once public_subnets rule.
func applyPublicSubnetsUpdate(nw *controlplanev1.Network, upd *controlplanev1.NetworkUpdate) error {
	aws := nw.GetCustomerManagedResources().GetAws()
	if aws == nil {
		return status.Error(codes.InvalidArgument,
			"customer_managed_resources can only be updated on a network with AWS customer-managed resources: customer_managed_resources.aws.public_subnets is the only field that is settable after create")
	}
	before := aws.GetPublicSubnets().GetArns()
	after := upd.GetCustomerManagedResources().GetAws().GetPublicSubnets().GetArns()
	if len(before) > 0 {
		if len(after) == 0 {
			return status.Error(codes.FailedPrecondition,
				"customer_managed_resources.aws.public_subnets is write-once and cannot be cleared once set")
		}
		if !sameStringSet(before, after) {
			return status.Error(codes.FailedPrecondition,
				"customer_managed_resources.aws.public_subnets is write-once and cannot be changed once set")
		}
	}
	if len(after) > 0 {
		aws.PublicSubnets = &controlplanev1.CustomerManagedAWSSubnets{Arns: slices.Clone(after)}
	}
	return nil
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
