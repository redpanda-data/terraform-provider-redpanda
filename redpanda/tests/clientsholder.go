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

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta1/controlplanev1beta1grpc"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

type clientHolder struct {
	OpsClient     controlplanev1beta1grpc.OperationServiceClient
	NetClient     controlplanev1beta1grpc.NetworkServiceClient
	NsClient      controlplanev1beta1grpc.NamespaceServiceClient
	ClusterClient controlplanev1beta1grpc.ClusterServiceClient
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
		OpsClient:     controlplanev1beta1grpc.NewOperationServiceClient(conn),
		NetClient:     controlplanev1beta1grpc.NewNetworkServiceClient(conn),
		NsClient:      controlplanev1beta1grpc.NewNamespaceServiceClient(conn),
		ClusterClient: controlplanev1beta1grpc.NewClusterServiceClient(conn),
	}, nil
}
