// Copyright 2026 Redpanda Data, Inc.
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

package cloudtest

import (
	"context"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
)

// operationServer always returns STATE_COMPLETED so polling loops exit
// on the first call.
type operationServer struct {
	controlplanev1grpc.UnimplementedOperationServiceServer
}

func newOperationServer() *operationServer { return &operationServer{} }

func (*operationServer) GetOperation(_ context.Context, req *controlplanev1.GetOperationRequest) (*controlplanev1.GetOperationResponse, error) {
	return &controlplanev1.GetOperationResponse{
		Operation: &controlplanev1.Operation{
			Id:    req.GetId(),
			State: controlplanev1.Operation_STATE_COMPLETED,
		},
	}, nil
}
