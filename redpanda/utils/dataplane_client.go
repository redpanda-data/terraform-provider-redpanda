// Copyright 2025 Redpanda Data, Inc.
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

package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"google.golang.org/grpc"
)

// NewDataplaneClient opens a pooled connection to clusterURL and builds a typed
// dataplane/console service client via build (e.g. dataplanev1grpc.NewUserServiceClient).
// It applies the guards every per-resource client constructor needs — empty URL
// and unconfigured pool — and wraps connection failures with the deserialized
// gRPC message, so all resources surface these the same way.
func NewDataplaneClient[T any](ctx context.Context, pool *cloud.ConnPool, clusterURL string, build func(grpc.ClientConnInterface) T) (T, error) {
	var zero T
	if clusterURL == "" {
		return zero, errors.New("unable to create client with empty target cluster API URL")
	}
	if pool == nil {
		return zero, errors.New("provider not configured: dataplane connection pool is nil")
	}
	conn, err := pool.GetConnection(ctx, clusterURL)
	if err != nil {
		return zero, fmt.Errorf("unable to open a connection with the cluster API: %v", DeserializeGrpcError(err))
	}
	return build(conn), nil
}
