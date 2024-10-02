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

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

func newTestClients(ctx context.Context, clientID, clientSecret, cloudEnv string) (*cloud.ControlPlaneClientSet, error) {
	endpoint, err := cloud.EndpointForEnv(cloudEnv)
	if err != nil {
		return nil, err
	}

	token, err := cloud.RequestToken(ctx, endpoint, clientID, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to request auth token: %v", err)
	}
	conn, err := cloud.SpawnConn(endpoint.APIURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to spawn connection with control plane API: %v", err)
	}
	return cloud.NewControlPlaneClientSet(conn), nil
}
