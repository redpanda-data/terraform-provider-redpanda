package clients

import (
	"context"
	"fmt"

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
)

// NewClusterServiceClient creates a new ClusterServiceClient.
func NewClusterServiceClient(ctx context.Context, cloudEnv string, cr ClientRequest) (cloudv1beta1.ClusterServiceClient, error) {
	token, endpoint, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, endpoint.apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return cloudv1beta1.NewClusterServiceClient(conn), nil
}

// NewNamespaceServiceClient creates a new NamespaceServiceClient.
func NewNamespaceServiceClient(ctx context.Context, cloudEnv string, cr ClientRequest) (cloudv1beta1.NamespaceServiceClient, error) {
	token, endpoint, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, endpoint.apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return cloudv1beta1.NewNamespaceServiceClient(conn), nil
}

// NewNetworkServiceClient creates a new NetworkServiceClient.
func NewNetworkServiceClient(ctx context.Context, cloudEnv string, cr ClientRequest) (cloudv1beta1.NetworkServiceClient, error) {
	token, endpoint, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, endpoint.apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return cloudv1beta1.NewNetworkServiceClient(conn), nil
}

// NewOperationServiceClient creates a new OperationServiceClient.
func NewOperationServiceClient(ctx context.Context, cloudEnv string, cr ClientRequest) (cloudv1beta1.OperationServiceClient, error) {
	token, endpoint, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, endpoint.apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return cloudv1beta1.NewOperationServiceClient(conn), nil
}
