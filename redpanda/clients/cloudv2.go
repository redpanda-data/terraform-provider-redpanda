package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var endpoints = map[string]map[string]map[string]string{
	"cloudv2": {
		"dev": {
			"api":      "api.dev.cloud.redpanda.com:443",
			"token":    "https://dev-cloudv2.us.auth0.com/oauth/token",
			"audience": "cloudv2-dev.redpanda.cloud",
		},
		"ign": {
			"api":      "api.ign.cloud.redpanda.com:443",
			"token":    "https://integration-cloudv2.us.auth0.com/oauth/token",
			"audience": "cloudv2-ign.redpanda.cloud",
		},
	},
}

type ClientRequest struct {
	ClientID     string
	ClientSecret string
}

// NewClusterServiceClient creates a new ClusterServiceClient
func NewClusterServiceClient(ctx context.Context, version string, cr ClientRequest) (cloudv1beta1.ClusterServiceClient, error) {
	conn, err := createConnection(ctx, version, cr)
	if err != nil {
		return nil, err
	}
	return cloudv1beta1.NewClusterServiceClient(conn), nil
}

// NewNamespaceServiceClient creates a new NamespaceServiceClient
func NewNamespaceServiceClient(ctx context.Context, version string, cr ClientRequest) (cloudv1beta1.NamespaceServiceClient, error) {
	conn, err := createConnection(ctx, version, cr)
	if err != nil {
		return nil, err
	}
	return cloudv1beta1.NewNamespaceServiceClient(conn), nil
}

// NewNetworkServiceClient creates a new NetworkServiceClient
func NewNetworkServiceClient(ctx context.Context, version string, cr ClientRequest) (cloudv1beta1.NetworkServiceClient, error) {
	conn, err := createConnection(ctx, version, cr)
	if err != nil {
		return nil, err
	}
	return cloudv1beta1.NewNetworkServiceClient(conn), nil
}

// NewOperationServiceClient creates a new OperationServiceClient
func NewOperationServiceClient(ctx context.Context, version string, cr ClientRequest) (cloudv1beta1.OperationServiceClient, error) {
	conn, err := createConnection(ctx, version, cr)
	if err != nil {
		return nil, err
	}
	return cloudv1beta1.NewOperationServiceClient(conn), nil
}

// createConnection is a helper function to create a connection based on the Redpanda model
func createConnection(ctx context.Context, version string, cr ClientRequest) (*grpc.ClientConn, error) {
	var token string
	var err error

	if cr.ClientID == "" {
		return nil, fmt.Errorf("client_id is not set")
	}

	if cr.ClientSecret == "" {
		return nil, fmt.Errorf("client_secret is not set")
	}

	token, err = requestToken(ctx, version, cr.ClientID, cr.ClientSecret)
	if err != nil {
		return nil, err
	}
	return spawnConn(ctx, version, token)
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// requestToken requests a token
func requestToken(ctx context.Context, version, clientID, clientSecret string) (string, error) {
	payload := strings.NewReader(fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s", clientID, clientSecret, endpoints["cloudv2"][version]["audience"]))
	req, err := http.NewRequestWithContext(ctx, "POST", endpoints["cloudv2"][version]["token"], payload)
	if err != nil {
		return "", err
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	tokenContainer := tokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenContainer); err != nil {
		return "", fmt.Errorf("error decoding token response: %v", err)
	}
	if tokenContainer.AccessToken == "" {
		return "", fmt.Errorf("no access token found in response: %v", tokenContainer)
	}
	return tokenContainer.AccessToken, nil
}

func spawnConn(ctx context.Context, version, authToken string) (*grpc.ClientConn, error) {
	return grpc.DialContext(
		ctx,
		endpoints["cloudv2"][version]["api"],
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", authToken)), method, req, reply, cc, opts...)
		}),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})))
}
