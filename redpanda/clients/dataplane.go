package clients

import (
	"context"
	"fmt"

	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
)

// NewTopicServiceClient creates a new TopicServiceClient.
func NewTopicServiceClient(ctx context.Context, cloudEnv, cloudURL string, cr ClientRequest) (dataplanev1alpha1.TopicServiceClient, error) {
	token, _, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, cloudURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return dataplanev1alpha1.NewTopicServiceClient(conn), nil
}

// NewUserServiceClient creates a new UserServiceClient.
func NewUserServiceClient(ctx context.Context, cloudEnv, cloudURL string, cr ClientRequest) (dataplanev1alpha1.UserServiceClient, error) {
	token, _, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, cloudURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return dataplanev1alpha1.NewUserServiceClient(conn), nil
}

// NewACLServiceClient creates a new ACLServiceClient.
func NewACLServiceClient(ctx context.Context, cloudEnv, cloudURL string, cr ClientRequest) (dataplanev1alpha1.ACLServiceClient, error) {
	token, _, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, cloudURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return dataplanev1alpha1.NewACLServiceClient(conn), nil
}

// NewSecretServiceClient creates a new SecretServiceClient.
func NewSecretServiceClient(ctx context.Context, cloudEnv, cloudURL string, cr ClientRequest) (dataplanev1alpha1.SecretServiceClient, error) {
	token, _, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, cloudURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return dataplanev1alpha1.NewSecretServiceClient(conn), nil
}

// NewKafkaConnectServiceClient creates a new KafkaConnectServiceClient.
func NewKafkaConnectServiceClient(ctx context.Context, cloudEnv, cloudURL string, cr ClientRequest) (dataplanev1alpha1.KafkaConnectServiceClient, error) {
	token, _, err := requestTokenAndEnv(ctx, cloudEnv, cr)
	if err != nil {
		return nil, fmt.Errorf("unable to request the Cloud authentication token: %v", err)
	}
	conn, err := spawnConn(ctx, cloudURL, token)
	if err != nil {
		return nil, fmt.Errorf("unable to create a connection: %v", err)
	}
	return dataplanev1alpha1.NewKafkaConnectServiceClient(conn), nil
}
