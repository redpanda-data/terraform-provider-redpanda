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
	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/iam/v1/iamv1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
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
	ShadowLinkForID(ctx context.Context, id string) (*controlplanev1.ShadowLink, error)
	ServiceAccountForID(ctx context.Context, id string) (*iamv1.ServiceAccount, error)
	ServiceAccountForName(ctx context.Context, name string) (*iamv1.ServiceAccount, error)
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
	ShadowLink            controlplanev1grpc.ShadowLinkServiceClient
	ThroughputTier        controlplanev1beta2grpc.ThroughputTierServiceClient
	ServiceAccount        iamv1grpc.ServiceAccountServiceClient
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
		ShadowLink:            controlplanev1grpc.NewShadowLinkServiceClient(conn),
		ThroughputTier:        controlplanev1beta2grpc.NewThroughputTierServiceClient(conn),
		ServiceAccount:        iamv1grpc.NewServiceAccountServiceClient(conn),
	}
}

// getByID issues a control-plane "get by ID" call and normalizes its two
// failure modes: a request error, and a non-error response whose entity is nil.
// get returns the extracted entity; proto getters are nil-safe, so extracting
// is safe even when err is non-nil.
func getByID[E comparable](kind, id string, get func() (E, error)) (E, error) {
	var zero E
	entity, err := get()
	if err != nil {
		return zero, fmt.Errorf("unable to request %s with ID %q: %w", kind, id, err)
	}
	if entity == zero {
		return zero, fmt.Errorf("unable to find %s with ID %q; please report this bug to Redpanda Support", kind, id)
	}
	return entity, nil
}

// getByName issues a control-plane "list with name filter" call and returns the
// first exact name match, or a not-found error.
func getByName[E any](kind, name string, list func() ([]E, error), nameOf func(E) string) (E, error) {
	var zero E
	items, err := list()
	if err != nil {
		return zero, fmt.Errorf("unable to list %ss: %w", kind, err)
	}
	for _, item := range items {
		if nameOf(item) == name {
			return item, nil
		}
	}
	return zero, fmt.Errorf("%s %q not found", kind, name)
}

// ServiceAccountForID gets the ServiceAccount for a given ID.
func (c *ControlPlaneClientSet) ServiceAccountForID(ctx context.Context, id string) (*iamv1.ServiceAccount, error) {
	return getByID("service account", id, func() (*iamv1.ServiceAccount, error) {
		resp, err := c.ServiceAccount.GetServiceAccount(ctx, &iamv1.GetServiceAccountRequest{Id: id})
		return resp.GetServiceAccount(), err
	})
}

// ServiceAccountForName lists service accounts filtering by name and returns the first match.
func (c *ControlPlaneClientSet) ServiceAccountForName(ctx context.Context, name string) (*iamv1.ServiceAccount, error) {
	return getByName("service account", name, func() ([]*iamv1.ServiceAccount, error) {
		resp, err := c.ServiceAccount.ListServiceAccounts(ctx, &iamv1.ListServiceAccountsRequest{
			Filter: &iamv1.ListServiceAccountsRequest_Filter{Name: name},
		})
		return resp.GetServiceAccounts(), err
	}, (*iamv1.ServiceAccount).GetName)
}

// ShadowLinkForID gets the shadow link for a given ID.
func (c *ControlPlaneClientSet) ShadowLinkForID(ctx context.Context, id string) (*controlplanev1.ShadowLink, error) {
	return getByID("shadow link", id, func() (*controlplanev1.ShadowLink, error) {
		resp, err := c.ShadowLink.GetShadowLink(ctx, &controlplanev1.GetShadowLinkRequest{Id: id})
		return resp.GetShadowLink(), err
	})
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
	return getByID("resource group", id, func() (*controlplanev1.ResourceGroup, error) {
		resp, err := c.ResourceGroup.GetResourceGroup(ctx, &controlplanev1.GetResourceGroupRequest{Id: id})
		return resp.GetResourceGroup(), err
	})
}

// ResourceGroupForName lists all resource group with a name filter, returns
// the resource group for the given name.
func (c *ControlPlaneClientSet) ResourceGroupForName(ctx context.Context, name string) (*controlplanev1.ResourceGroup, error) {
	return getByName("resource group", name, func() ([]*controlplanev1.ResourceGroup, error) {
		resp, err := c.ResourceGroup.ListResourceGroups(ctx, &controlplanev1.ListResourceGroupsRequest{
			Filter: &controlplanev1.ListResourceGroupsRequest_Filter{NameContains: name},
		})
		return resp.GetResourceGroups(), err
	}, (*controlplanev1.ResourceGroup).GetName)
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
	return getByID("network", id, func() (*controlplanev1.Network, error) {
		resp, err := c.Network.GetNetwork(ctx, &controlplanev1.GetNetworkRequest{Id: id})
		return resp.GetNetwork(), err
	})
}

// NetworkForName lists all networks with a name filter, returns the network for
// the given name.
func (c *ControlPlaneClientSet) NetworkForName(ctx context.Context, name string) (*controlplanev1.Network, error) {
	return getByName("network", name, func() ([]*controlplanev1.Network, error) {
		resp, err := c.Network.ListNetworks(ctx, &controlplanev1.ListNetworksRequest{
			Filter: &controlplanev1.ListNetworksRequest_Filter{NameContains: name},
		})
		return resp.GetNetworks(), err
	}, (*controlplanev1.Network).GetName)
}

// ClusterForID gets the Cluster for a given ID and handles the error if the
// returned cluster is nil.
func (c *ControlPlaneClientSet) ClusterForID(ctx context.Context, id string) (*controlplanev1.Cluster, error) {
	if id == "" {
		return nil, errors.New("cluster ID is empty")
	}
	return getByID("cluster", id, func() (*controlplanev1.Cluster, error) {
		resp, err := c.Cluster.GetCluster(ctx, &controlplanev1.GetClusterRequest{Id: id})
		return resp.GetCluster(), err
	})
}

// ClusterForName lists all clusters with a name filter, returns the cluster for
// the given name.
func (c *ControlPlaneClientSet) ClusterForName(ctx context.Context, name string) (*controlplanev1.Cluster, error) {
	return getByName("cluster", name, func() ([]*controlplanev1.Cluster, error) {
		resp, err := c.Cluster.ListClusters(ctx, &controlplanev1.ListClustersRequest{
			Filter: &controlplanev1.ListClustersRequest_Filter{NameContains: name},
		})
		return resp.GetClusters(), err
	}, (*controlplanev1.Cluster).GetName)
}

// ServerlessClusterForID gets the ServerlessCluster for a given ID and handles the error if the
// returned serverless cluster is nil.
func (c *ControlPlaneClientSet) ServerlessClusterForID(ctx context.Context, id string) (*controlplanev1.ServerlessCluster, error) {
	return getByID("serverless cluster", id, func() (*controlplanev1.ServerlessCluster, error) {
		resp, err := c.ServerlessCluster.GetServerlessCluster(ctx, &controlplanev1.GetServerlessClusterRequest{Id: id})
		return resp.GetServerlessCluster(), err
	})
}

// ServerlessClusterForName lists all serverless clusters with a name filter, returns the serverless cluster for
// the given name.
func (c *ControlPlaneClientSet) ServerlessClusterForName(ctx context.Context, name string) (*controlplanev1.ServerlessCluster, error) {
	return getByName("serverless cluster", name, func() ([]*controlplanev1.ServerlessCluster, error) {
		resp, err := c.ServerlessCluster.ListServerlessClusters(ctx, &controlplanev1.ListServerlessClustersRequest{
			Filter: &controlplanev1.ListServerlessClustersRequest_Filter{NameContains: name},
		})
		return resp.GetServerlessClusters(), err
	}, (*controlplanev1.ServerlessCluster).GetName)
}

// GetCluster gets the cluster for a given request (primarily added to satisfy interface for mocks
func (c *ControlPlaneClientSet) GetCluster(ctx context.Context, in *controlplanev1.GetClusterRequest, opts ...grpc.CallOption) (*controlplanev1.GetClusterResponse, error) {
	return c.Cluster.GetCluster(ctx, in, opts...)
}

// ServerlessPrivateLinkForID gets the ServerlessPrivateLink for a given ID and handles the error if the
// returned serverless private link is nil.
func (c *ControlPlaneClientSet) ServerlessPrivateLinkForID(ctx context.Context, id string) (*controlplanev1.ServerlessPrivateLink, error) {
	return getByID("serverless private link", id, func() (*controlplanev1.ServerlessPrivateLink, error) {
		resp, err := c.ServerlessPrivateLink.GetServerlessPrivateLink(ctx, &controlplanev1.GetServerlessPrivateLinkRequest{Id: id})
		return resp.GetServerlessPrivateLink(), err
	})
}

// ServerlessPrivateLinkForName lists all serverless private links with a name filter, returns the serverless private link for
// the given name.
func (c *ControlPlaneClientSet) ServerlessPrivateLinkForName(ctx context.Context, name string) (*controlplanev1.ServerlessPrivateLink, error) {
	return getByName("serverless private link", name, func() ([]*controlplanev1.ServerlessPrivateLink, error) {
		resp, err := c.ServerlessPrivateLink.ListServerlessPrivateLinks(ctx, &controlplanev1.ListServerlessPrivateLinksRequest{
			Filter: &controlplanev1.ListServerlessPrivateLinksRequest_Filter{NameContains: name},
		})
		return resp.GetServerlessPrivateLinks(), err
	}, (*controlplanev1.ServerlessPrivateLink).GetName)
}

// DataplaneURLForCluster resolves clusterID to a dataplane API URL. Tries
// regular cluster first; falls back to serverless. Returns a clear diagnostic
// if the resolved cluster has no DataplaneApi populated yet (STATE_CREATING,
// pre-dataplane BYOC). The previous direct `.DataplaneApi.Url` access would
// nil-deref on those clusters.
func (c *ControlPlaneClientSet) DataplaneURLForCluster(ctx context.Context, clusterID string) (string, error) {
	cl, cerr := c.ClusterForID(ctx, clusterID)
	if cerr == nil && cl != nil {
		return dataplaneURLOrNotReady(cl.GetDataplaneApi().GetUrl(), cl.GetDataplaneApi() != nil, clusterID, false)
	}
	sl, serr := c.ServerlessClusterForID(ctx, clusterID)
	if serr == nil && sl != nil {
		return dataplaneURLOrNotReady(sl.GetDataplaneApi().GetUrl(), sl.GetDataplaneApi() != nil, clusterID, true)
	}
	if cerr != nil && serr != nil {
		return "", fmt.Errorf("failed to resolve cluster %q: %v; serverless lookup: %v", clusterID, cerr, serr)
	}
	return "", fmt.Errorf("cluster %q not found", clusterID)
}

// dataplaneURLOrNotReady returns the URL when dataplaneAPIPresent is true, or
// a "not ready yet" diagnostic naming the cluster otherwise. Extracted so the
// nil-DataplaneApi guard can be unit tested without standing up a fake
// controlplane.
func dataplaneURLOrNotReady(url string, dataplaneAPIPresent bool, clusterID string, serverless bool) (string, error) {
	if dataplaneAPIPresent {
		return url, nil
	}
	if serverless {
		return "", fmt.Errorf("serverless cluster %q has no dataplane API URL yet", clusterID)
	}
	return "", fmt.Errorf("cluster %q has no dataplane API URL yet; try importing after the cluster reaches READY state", clusterID)
}
