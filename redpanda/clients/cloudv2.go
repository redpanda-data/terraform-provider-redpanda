package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"log"
	"net/http"
	"strings"
)

// TODO not in love with this abomination but it at least tracks all the usage spots and ensures consistency
var endpoints = map[string]map[string]map[string]string{
	"cloudv2": {
		"dev": {
			"api":      "api.dev.cloud.redpanda.com:443",
			"token":    "https://dev-cloudv2.us.auth0.com/oauth/token",
			"audience": "cloudv2-dev.redpanda.cloud",
		},
	},
}

// CloudV2 wrapper for various clients used to access cloudv2 API
type CloudV2 struct {
	cloudv1beta1.ClusterServiceClient
	cloudv1beta1.NamespaceServiceClient
	cloudv1beta1.NetworkServiceClient
	cloudv1beta1.OperationServiceClient
}

func NewCloudV2Client(ctx context.Context, version string, model models.Redpanda) CloudV2 {
	switch {
	case !model.AuthToken.IsNull():
		conn, err := spawnConn(ctx, version, model.AuthToken.String())
		if err != nil {
			log.Fatal(err)
		}
		return CloudV2{
			ClusterServiceClient:   cloudv1beta1.NewClusterServiceClient(conn),
			NamespaceServiceClient: cloudv1beta1.NewNamespaceServiceClient(conn),
			NetworkServiceClient:   cloudv1beta1.NewNetworkServiceClient(conn),
			OperationServiceClient: cloudv1beta1.NewOperationServiceClient(conn),
		}
	case !(model.ClientID.IsNull() && model.ClientSecret.IsNull()):
		token, err := requestToken(version, model.ClientID.String(), model.ClientSecret.String())
		if err != nil {
			log.Fatal(err) // TODO dont
		}
		conn, err := spawnConn(ctx, version, token)
		if err != nil {
			log.Fatal(err) // TODO dont
		}
		return CloudV2{
			ClusterServiceClient:   cloudv1beta1.NewClusterServiceClient(conn),
			NamespaceServiceClient: cloudv1beta1.NewNamespaceServiceClient(conn),
			NetworkServiceClient:   cloudv1beta1.NewNetworkServiceClient(conn),
			OperationServiceClient: cloudv1beta1.NewOperationServiceClient(conn),
		}
	default:
		log.Fatal("neither auth_token nor client_id and client_secret are set") // TODO dont
		return CloudV2{}                                                        // good luck getting here but the compiler needs it
	}
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// requestToken requests a token
func requestToken(version, clientId, clientSecret string) (string, error) {
	payload := strings.NewReader(fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s", clientId, clientSecret, endpoints["cloudv2"][version]["audience"]))
	req, err := http.NewRequest("POST", endpoints["cloudv2"][version]["token"], payload)
	if err != nil {
		return "", err
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	tokenContainer := TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenContainer); err != nil {
		return "", fmt.Errorf("error decoding token response: %v", err)
	}
	return tokenContainer.AccessToken, nil
}

func spawnConn(ctx context.Context, version, authToken string) (*grpc.ClientConn, error) {
	return grpc.DialContext(
		ctx,
		endpoints["cloudv2"][version]["api"],
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", authToken)), method, req, reply, cc, opts...)
		}),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})))
}
