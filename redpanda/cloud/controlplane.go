// Copyright 2024 Redpanda Data, Inc.
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

package cloud

import (
	"context"
	"errors"
	"fmt"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc"
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"google.golang.org/grpc"
)

// ControlPlaneClientSet holds the respective service clients to interact with
// the control plane endpoints of the Public API.
type ControlPlaneClientSet struct {
	ResourceGroup controlplanev1beta2grpc.ResourceGroupServiceClient
	Network       controlplanev1beta2grpc.NetworkServiceClient
	Cluster       controlplanev1beta2grpc.ClusterServiceClient
	Operation     controlplanev1beta2grpc.OperationServiceClient
}

// NewControlPlaneClientSet uses the passed grpc connection to create a control
// plane client set.
func NewControlPlaneClientSet(conn *grpc.ClientConn) *ControlPlaneClientSet {
	return &ControlPlaneClientSet{
		ResourceGroup: controlplanev1beta2grpc.NewResourceGroupServiceClient(conn),
		Network:       controlplanev1beta2grpc.NewNetworkServiceClient(conn),
		Cluster:       controlplanev1beta2grpc.NewClusterServiceClient(conn),
		Operation:     controlplanev1beta2grpc.NewOperationServiceClient(conn),
	}
}

// CreateResourceGroup creates the resource group with the given name
func (cpCl *ControlPlaneClientSet) CreateResourceGroup(ctx context.Context, name string) (*controlplanev1beta2.ResourceGroup, error) {
	rgResp, err := cpCl.ResourceGroup.CreateResourceGroup(ctx, &controlplanev1beta2.CreateResourceGroupRequest{
		ResourceGroup: &controlplanev1beta2.ResourceGroupCreate{
			Name: name,
		},
	})
	if err != nil {
		return nil, err
	}
	if rgResp.ResourceGroup == nil {
		return nil, errors.New("error after creating resource group; provider response was empty. Please report this issue to the provider developers")
	}
	return rgResp.ResourceGroup, nil
}

// ResourceGroupForID gets the resource group for a given ID and handles the
// error if the returned resource group is nil.
func (cpCl *ControlPlaneClientSet) ResourceGroupForID(ctx context.Context, id string) (*controlplanev1beta2.ResourceGroup, error) {
	rg, err := cpCl.ResourceGroup.GetResourceGroup(ctx, &controlplanev1beta2.GetResourceGroupRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request resource group with ID %q: %w", id, err)
	}
	if rg.ResourceGroup == nil {
		// This should not happen but the new API returns a pointer, and we
		// need to make sure that a ResourceGroup is returned
		return nil, fmt.Errorf("unable to request resource group with ID %q. Please report this issue to the provider developers", id)
	}
	return rg.ResourceGroup, nil
}

// ResourceGroupForName lists all resource group with a name filter, returns
// the resource group for the given name.
func (cpCl *ControlPlaneClientSet) ResourceGroupForName(ctx context.Context, name string) (*controlplanev1beta2.ResourceGroup, error) {
	listResp, err := cpCl.ResourceGroup.ListResourceGroups(ctx, &controlplanev1beta2.ListResourceGroupsRequest{
		Filter: &controlplanev1beta2.ListResourceGroupsRequest_Filter{
			Name: name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to find resource group with name %q: %w", name, err)
	}
	if listResp.ResourceGroups == nil {
		return nil, fmt.Errorf("unable to find resource group with name %q: provider response was empty. Please report this issue to the provider developers", name)
	}
	for _, rg := range listResp.ResourceGroups {
		if rg.GetName() == name {
			return rg, nil
		}
	}
	return nil, fmt.Errorf("resource group %s not found", name)
}

// NetworkForID gets the Network for a given ID and handles the error if the
// returned network is nil.
func (cpCl *ControlPlaneClientSet) NetworkForID(ctx context.Context, id string) (*controlplanev1beta2.Network, error) {
	c, err := cpCl.Network.GetNetwork(ctx, &controlplanev1beta2.GetNetworkRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request network %q information: %w", id, err)
	}
	if c.Network == nil {
		return nil, fmt.Errorf("unable to find network %q; please report this bug to Redpanda Support", id)
	}
	return c.Network, nil
}

// NetworkForName lists all networks with a name filter, returns the network for
// the given name.
func (cpCl *ControlPlaneClientSet) NetworkForName(ctx context.Context, name string) (*controlplanev1beta2.Network, error) {
	ns, err := cpCl.Network.ListNetworks(ctx, &controlplanev1beta2.ListNetworksRequest{
		Filter: &controlplanev1beta2.ListNetworksRequest_Filter{Name: name},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list networks: %v", err)
	}
	for _, v := range ns.GetNetworks() {
		if v.GetName() == name {
			return v, nil
		}
	}
	return nil, fmt.Errorf("network not found")
}

// ClusterForID gets the Cluster for a given ID and handles the error if the
// returned cluster is nil.
func (cpCl *ControlPlaneClientSet) ClusterForID(ctx context.Context, id string) (*controlplanev1beta2.Cluster, error) {
	c, err := cpCl.Cluster.GetCluster(ctx, &controlplanev1beta2.GetClusterRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request cluster %q information: %w", id, err)
	}
	if c.Cluster == nil {
		return nil, fmt.Errorf("unable to find cluster %q; please report this bug to Redpanda Support", id)
	}
	return c.Cluster, nil
}

// ClusterForName lists all clusters with a name filter, returns the cluster for
// the given name.
func (cpCl *ControlPlaneClientSet) ClusterForName(ctx context.Context, name string) (*controlplanev1beta2.Cluster, error) {
	clusters, err := cpCl.Cluster.ListClusters(ctx, &controlplanev1beta2.ListClustersRequest{
		Filter: &controlplanev1beta2.ListClustersRequest_Filter{Name: name},
	})
	if err != nil {
		return nil, err
	}
	for _, c := range clusters.GetClusters() {
		if c.GetName() == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("cluster not found")
}
