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

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc"
)

// CpClientSet defines the interface for ControlPlaneClientSet
type CpClientSet interface {
	CreateResourceGroup(ctx context.Context, name string) (*controlplanev1.ResourceGroup, error)
	ResourceGroupForID(ctx context.Context, id string) (*controlplanev1.ResourceGroup, error)
	ResourceGroupForName(ctx context.Context, name string) (*controlplanev1.ResourceGroup, error)
	NetworkForID(ctx context.Context, id string) (*controlplanev1.Network, error)
	NetworkForName(ctx context.Context, name string) (*controlplanev1.Network, error)
	ClusterForName(ctx context.Context, name string) (*controlplanev1.Cluster, error)
	ServerlessClusterForID(ctx context.Context, id string) (*controlplanev1.ServerlessCluster, error)
	ServerlessClusterForName(ctx context.Context, name string) (*controlplanev1.ServerlessCluster, error)
	GetCluster(ctx context.Context, in *controlplanev1.GetClusterRequest, opts ...grpc.CallOption) (*controlplanev1.GetClusterResponse, error)
	ClusterForID(ctx context.Context, id string) (*controlplanev1.Cluster, error)
}

// ControlPlaneClientSet holds the respective service clients to interact with
// the control plane endpoints of the Public API.
type ControlPlaneClientSet struct {
	ResourceGroup         controlplanev1grpc.ResourceGroupServiceClient
	Network               controlplanev1grpc.NetworkServiceClient
	Cluster               controlplanev1grpc.ClusterServiceClient
	ServerlessCluster     controlplanev1grpc.ServerlessClusterServiceClient
	ServerlessPrivateLink controlplanev1grpc.ServerlessPrivateLinkServiceClient
	ServerlessRegion      controlplanev1grpc.ServerlessRegionServiceClient
	Operation             controlplanev1grpc.OperationServiceClient
	Region                controlplanev1grpc.RegionServiceClient
	ThroughputTier        controlplanev1beta2grpc.ThroughputTierServiceClient
}

// NewControlPlaneClientSet uses the passed grpc connection to create a control
// plane client set.
func NewControlPlaneClientSet(conn *grpc.ClientConn) *ControlPlaneClientSet {
	return &ControlPlaneClientSet{
		ResourceGroup:         controlplanev1grpc.NewResourceGroupServiceClient(conn),
		Network:               controlplanev1grpc.NewNetworkServiceClient(conn),
		Cluster:               controlplanev1grpc.NewClusterServiceClient(conn),
		ServerlessCluster:     controlplanev1grpc.NewServerlessClusterServiceClient(conn),
		ServerlessPrivateLink: controlplanev1grpc.NewServerlessPrivateLinkServiceClient(conn),
		ServerlessRegion:      controlplanev1grpc.NewServerlessRegionServiceClient(conn),
		Operation:             controlplanev1grpc.NewOperationServiceClient(conn),
		Region:                controlplanev1grpc.NewRegionServiceClient(conn),
		ThroughputTier:        controlplanev1beta2grpc.NewThroughputTierServiceClient(conn),
	}
}

// CreateResourceGroup creates the resource group with the given name
func (c *ControlPlaneClientSet) CreateResourceGroup(ctx context.Context, name string) (*controlplanev1.ResourceGroup, error) {
	rgResp, err := c.ResourceGroup.CreateResourceGroup(ctx, &controlplanev1.CreateResourceGroupRequest{
		ResourceGroup: &controlplanev1.ResourceGroupCreate{
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
func (c *ControlPlaneClientSet) ResourceGroupForID(ctx context.Context, id string) (*controlplanev1.ResourceGroup, error) {
	rg, err := c.ResourceGroup.GetResourceGroup(ctx, &controlplanev1.GetResourceGroupRequest{
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
func (c *ControlPlaneClientSet) ResourceGroupForName(ctx context.Context, name string) (*controlplanev1.ResourceGroup, error) {
	listResp, err := c.ResourceGroup.ListResourceGroups(ctx, &controlplanev1.ListResourceGroupsRequest{
		Filter: &controlplanev1.ListResourceGroupsRequest_Filter{
			NameContains: name,
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

// ResourceGroupForIDOrName gets the resource group for a given ID and/or name, or neither,
// and handles the error if the returned resource group is nil.
func (c *ControlPlaneClientSet) ResourceGroupForIDOrName(ctx context.Context, id, name string) (*controlplanev1.ResourceGroup, error) {
	if id != "" {
		rg, err := c.ResourceGroupForID(ctx, id)
		if err != nil {
			return nil, err
		}
		if name != "" && rg.Name != name {
			return nil, fmt.Errorf("unable to find resource group with id %q and name %q", id, name)
		}
		return rg, nil
	}

	if name != "" {
		return c.ResourceGroupForName(ctx, name)
	}

	request := &controlplanev1.ListResourceGroupsRequest{}
	listResp, err := c.ResourceGroup.ListResourceGroups(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("unable to find resource groups: %w", err)
	}
	if listResp.ResourceGroups == nil {
		return nil, errors.New("provider response was empty. Please report this issue to the provider developers")
	}
	if len(listResp.ResourceGroups) > 1 {
		return nil, errors.New("found more than one resource group matching filters")
	} else if len(listResp.ResourceGroups) == 0 {
		return nil, errors.New("unable to find any resource group matching filters")
	}
	return listResp.ResourceGroups[0], nil
}

// NetworkForID gets the Network for a given ID and handles the error if the
// returned network is nil.
func (c *ControlPlaneClientSet) NetworkForID(ctx context.Context, id string) (*controlplanev1.Network, error) {
	gnr, err := c.Network.GetNetwork(ctx, &controlplanev1.GetNetworkRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request network %q information: %w", id, err)
	}
	if gnr.Network == nil {
		return nil, fmt.Errorf("unable to find network %q; please report this bug to Redpanda Support", id)
	}
	return gnr.Network, nil
}

// NetworkForName lists all networks with a name filter, returns the network for
// the given name.
func (c *ControlPlaneClientSet) NetworkForName(ctx context.Context, name string) (*controlplanev1.Network, error) {
	ns, err := c.Network.ListNetworks(ctx, &controlplanev1.ListNetworksRequest{
		Filter: &controlplanev1.ListNetworksRequest_Filter{NameContains: name},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list networks: %v", err)
	}
	for _, v := range ns.GetNetworks() {
		if v.GetName() == name {
			return v, nil
		}
	}
	return nil, errors.New("network not found")
}

// ClusterForID gets the Cluster for a given ID and handles the error if the
// returned cluster is nil.
func (c *ControlPlaneClientSet) ClusterForID(ctx context.Context, id string) (*controlplanev1.Cluster, error) {
	if id == "" {
		return nil, errors.New("cluster ID is empty")
	}
	cl, err := c.Cluster.GetCluster(ctx, &controlplanev1.GetClusterRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request cluster %q information: %w", id, err)
	}
	if cl.Cluster == nil {
		return nil, fmt.Errorf("unable to find cluster %q; please report this bug to Redpanda Support", id)
	}
	return cl.Cluster, nil
}

// ClusterForName lists all clusters with a name filter, returns the cluster for
// the given name.
func (c *ControlPlaneClientSet) ClusterForName(ctx context.Context, name string) (*controlplanev1.Cluster, error) {
	clusters, err := c.Cluster.ListClusters(ctx, &controlplanev1.ListClustersRequest{
		Filter: &controlplanev1.ListClustersRequest_Filter{NameContains: name},
	})
	if err != nil {
		return nil, err
	}
	for _, c := range clusters.GetClusters() {
		if c.GetName() == name {
			return c, nil
		}
	}
	return nil, errors.New("cluster not found")
}

// ServerlessClusterForID gets the ServerlessCluster for a given ID and handles the error if the
// returned serverless cluster is nil.
func (c *ControlPlaneClientSet) ServerlessClusterForID(ctx context.Context, id string) (*controlplanev1.ServerlessCluster, error) {
	cl, err := c.ServerlessCluster.GetServerlessCluster(ctx, &controlplanev1.GetServerlessClusterRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request serverless cluster %q information: %w", id, err)
	}
	if cl.ServerlessCluster == nil {
		return nil, fmt.Errorf("unable to find serverless cluster %q; please report this bug to Redpanda Support", id)
	}
	return cl.ServerlessCluster, nil
}

// ServerlessClusterForName lists all serverless clusters with a name filter, returns the serverless cluster for
// the given name.
func (c *ControlPlaneClientSet) ServerlessClusterForName(ctx context.Context, name string) (*controlplanev1.ServerlessCluster, error) {
	serverlessClusters, err := c.ServerlessCluster.ListServerlessClusters(ctx, &controlplanev1.ListServerlessClustersRequest{
		Filter: &controlplanev1.ListServerlessClustersRequest_Filter{NameContains: name},
	})
	if err != nil {
		return nil, err
	}
	for _, c := range serverlessClusters.GetServerlessClusters() {
		if c.GetName() == name {
			return c, nil
		}
	}
	return nil, errors.New("serverless cluster not found")
}

// GetCluster gets the cluster for a given request (primarily added to satisfy interface for mocks
func (c *ControlPlaneClientSet) GetCluster(ctx context.Context, in *controlplanev1.GetClusterRequest, opts ...grpc.CallOption) (*controlplanev1.GetClusterResponse, error) {
	return c.Cluster.GetCluster(ctx, in, opts...)
}

// ServerlessPrivateLinkForID gets the ServerlessPrivateLink for a given ID and handles the error if the
// returned serverless private link is nil.
func (c *ControlPlaneClientSet) ServerlessPrivateLinkForID(ctx context.Context, id string) (*controlplanev1.ServerlessPrivateLink, error) {
	resp, err := c.ServerlessPrivateLink.GetServerlessPrivateLink(ctx, &controlplanev1.GetServerlessPrivateLinkRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request serverless private link %q information: %w", id, err)
	}
	if resp.ServerlessPrivateLink == nil {
		return nil, fmt.Errorf("unable to find serverless private link %q; please report this bug to Redpanda Support", id)
	}
	return resp.ServerlessPrivateLink, nil
}

// ServerlessPrivateLinkForName lists all serverless private links with a name filter, returns the serverless private link for
// the given name.
func (c *ControlPlaneClientSet) ServerlessPrivateLinkForName(ctx context.Context, name string) (*controlplanev1.ServerlessPrivateLink, error) {
	privateLinks, err := c.ServerlessPrivateLink.ListServerlessPrivateLinks(ctx, &controlplanev1.ListServerlessPrivateLinksRequest{
		Filter: &controlplanev1.ListServerlessPrivateLinksRequest_Filter{NameContains: name},
	})
	if err != nil {
		return nil, err
	}
	for _, pl := range privateLinks.GetServerlessPrivateLinks() {
		if pl.GetName() == name {
			return pl, nil
		}
	}
	return nil, errors.New("serverless private link not found")
}
