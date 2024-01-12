// Copyright 2023 Redpanda Data, Inc.
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

package tests

import (
	"context"
	"fmt"

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"golang.org/x/sync/errgroup"
)

type clientHolder struct {
	OpsClient     cloudv1beta1.OperationServiceClient
	NetClient     cloudv1beta1.NetworkServiceClient
	NsClient      cloudv1beta1.NamespaceServiceClient
	ClusterClient cloudv1beta1.ClusterServiceClient
}

func newClients(ctx context.Context, clientID, clientSecret, cloudEnv string) (*clientHolder, error) {
	g, egCtx := errgroup.WithContext(ctx)
	var (
		opsClient     cloudv1beta1.OperationServiceClient
		netClient     cloudv1beta1.NetworkServiceClient
		nsClient      cloudv1beta1.NamespaceServiceClient
		clusterClient cloudv1beta1.ClusterServiceClient
	)
	g.Go(func() (rerr error) {
		opsClient, rerr = clients.NewOperationServiceClient(egCtx, cloudEnv, clients.ClientRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		})
		if rerr != nil {
			return fmt.Errorf("unable to create operation service client: %v", rerr)
		}
		return nil
	})
	g.Go(func() (rerr error) {
		nsClient, rerr = clients.NewNamespaceServiceClient(egCtx, cloudEnv, clients.ClientRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		})
		if rerr != nil {
			return fmt.Errorf("unable to create namespace service client: %v", rerr)
		}
		return nil
	})
	g.Go(func() (rerr error) {
		netClient, rerr = clients.NewNetworkServiceClient(egCtx, cloudEnv, clients.ClientRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		})
		if rerr != nil {
			return fmt.Errorf("unable to create network service client: %v", rerr)
		}
		return nil
	})
	g.Go(func() (rerr error) {
		clusterClient, rerr = clients.NewClusterServiceClient(egCtx, cloudEnv, clients.ClientRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		})
		if rerr != nil {
			return fmt.Errorf("unable to create cluster service client: %v", rerr)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return &clientHolder{
		OpsClient:     opsClient,
		NetClient:     netClient,
		NsClient:      nsClient,
		ClusterClient: clusterClient,
	}, nil
}
