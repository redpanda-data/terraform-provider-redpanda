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

package mocks

import (
	"context"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta1/controlplanev1beta1grpc"
	controlplanev1beta1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta1"
	"google.golang.org/grpc"
)

var _ controlplanev1beta1grpc.NamespaceServiceClient = MockNamespaceServiceClient{}

// MockNamespaceServiceClient is a mocked client that follows the
// controlplanev1beta1.NamespaceServiceClient interface.
type MockNamespaceServiceClient struct {
	// You can add fields to store mock responses or any other information needed for your tests
}

// CreateNamespace is a mocked CreateNamespace method to satisfy the interface.
func (MockNamespaceServiceClient) CreateNamespace(_ context.Context, _ *controlplanev1beta1.CreateNamespaceRequest, _ ...grpc.CallOption) (*controlplanev1beta1.Namespace, error) {
	// Implement mock logic here
	// Return a mock Namespace object and nil error for successful scenario
	// Return nil and a mock error for failure scenario
	return &controlplanev1beta1.Namespace{}, nil
}

// UpdateNamespace is a mocked UpdateNamespace method to satisfy the interface.
func (MockNamespaceServiceClient) UpdateNamespace(_ context.Context, _ *controlplanev1beta1.UpdateNamespaceRequest, _ ...grpc.CallOption) (*controlplanev1beta1.Namespace, error) {
	// Implement mock logic here
	return &controlplanev1beta1.Namespace{}, nil
}

// GetNamespace is a mocked GetNamespace method to satisfy the interface.
func (MockNamespaceServiceClient) GetNamespace(_ context.Context, _ *controlplanev1beta1.GetNamespaceRequest, _ ...grpc.CallOption) (*controlplanev1beta1.Namespace, error) {
	// Implement mock logic here
	return &controlplanev1beta1.Namespace{}, nil
}

// ListNamespaces is a mocked ListNamespaces method to satisfy the interface.
func (MockNamespaceServiceClient) ListNamespaces(_ context.Context, _ *controlplanev1beta1.ListNamespacesRequest, _ ...grpc.CallOption) (*controlplanev1beta1.ListNamespacesResponse, error) {
	// Implement mock logic here
	return &controlplanev1beta1.ListNamespacesResponse{}, nil
}

// DeleteNamespace is a mocked DeleteNamespace method to satisfy the interface.
func (MockNamespaceServiceClient) DeleteNamespace(_ context.Context, _ *controlplanev1beta1.DeleteNamespaceRequest, _ ...grpc.CallOption) (*controlplanev1beta1.DeleteNamespaceResponse, error) {
	// Implement mock logic here
	return &controlplanev1beta1.DeleteNamespaceResponse{}, nil
}
