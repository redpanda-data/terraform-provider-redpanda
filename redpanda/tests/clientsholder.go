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
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

type clientHolder struct {
	OpsClient     cloudv1beta1.OperationServiceClient
	NetClient     cloudv1beta1.NetworkServiceClient
	NsClient      cloudv1beta1.NamespaceServiceClient
	ClusterClient cloudv1beta1.ClusterServiceClient
}

func newClients(ctx context.Context, clientID, clientSecret, cloudEnv string) (*clientHolder, error) {
	token, endpoint, err := cloud.RequestTokenAndEnv(ctx, cloudEnv, clientID, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to request auth token: %v", err)
	}
	conn, err := cloud.SpawnConn(ctx, endpoint.APIURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to spawn connection with control plane API: %v", err)
	}
	return &clientHolder{
		OpsClient:     cloudv1beta1.NewOperationServiceClient(conn),
		NetClient:     cloudv1beta1.NewNetworkServiceClient(conn),
		NsClient:      cloudv1beta1.NewNamespaceServiceClient(conn),
		ClusterClient: cloudv1beta1.NewClusterServiceClient(conn),
	}, nil
}
