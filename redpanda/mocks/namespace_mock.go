package mocks

import (
	"context"
	controlplanev1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"google.golang.org/grpc"
)

var _ controlplanev1beta1.NamespaceServiceClient = MockNamespaceServiceClient{}

type MockNamespaceServiceClient struct {
	// You can add fields to store mock responses or any other information needed for your tests
}

func (m MockNamespaceServiceClient) CreateNamespace(ctx context.Context, in *controlplanev1beta1.CreateNamespaceRequest, opts ...grpc.CallOption) (*controlplanev1beta1.Namespace, error) {
	// Implement mock logic here
	// Return a mock Namespace object and nil error for successful scenario
	// Return nil and a mock error for failure scenario
	return &controlplanev1beta1.Namespace{}, nil
}

func (m MockNamespaceServiceClient) UpdateNamespace(ctx context.Context, in *controlplanev1beta1.UpdateNamespaceRequest, opts ...grpc.CallOption) (*controlplanev1beta1.Namespace, error) {
	// Implement mock logic here
	return &controlplanev1beta1.Namespace{}, nil
}

func (m MockNamespaceServiceClient) GetNamespace(ctx context.Context, in *controlplanev1beta1.GetNamespaceRequest, opts ...grpc.CallOption) (*controlplanev1beta1.Namespace, error) {
	// Implement mock logic here
	return &controlplanev1beta1.Namespace{}, nil
}

func (m MockNamespaceServiceClient) ListNamespaces(ctx context.Context, in *controlplanev1beta1.ListNamespacesRequest, opts ...grpc.CallOption) (*controlplanev1beta1.ListNamespacesResponse, error) {
	// Implement mock logic here
	return &controlplanev1beta1.ListNamespacesResponse{}, nil
}

func (m MockNamespaceServiceClient) DeleteNamespace(ctx context.Context, in *controlplanev1beta1.DeleteNamespaceRequest, opts ...grpc.CallOption) (*controlplanev1beta1.DeleteNamespaceResponse, error) {
	// Implement mock logic here
	return &controlplanev1beta1.DeleteNamespaceResponse{}, nil
}
