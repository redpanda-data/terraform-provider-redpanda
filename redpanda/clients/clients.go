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

// Package clients provides the CloudV2 clients used by the Redpanda terraform
// provider and the generated resources.
package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// cloudEndpoint is a representation of a cloud V2 endpoint, containing the URLs
// for authentication and the API URL.
type cloudEndpoint struct {
	apiURL   string // CloudV2 public API URL.
	authURL  string // CloudV2 URL for authorization token exchange.
	audience string // CloudV2 audience used for token exchange.
}

var cloudAuthEnvironments = map[string]cloudEndpoint{
	"dev": {
		"api.dev.cloud.redpanda.com:443",
		"https://dev-cloudv2.us.auth0.com/oauth/token",
		"cloudv2-dev.redpanda.cloud",
	},
	"ign": {
		"api.ign.cloud.redpanda.com:443",
		"https://integration-cloudv2.us.auth0.com/oauth/token",
		"cloudv2-ign.redpanda.cloud",
	},
	"prod": {
		"api.redpanda.com:443",
		"https://auth.prd.cloud.redpanda.com/oauth/token",
		"cloudv2-production.redpanda.cloud",
	},
}

// ClientRequest are the client request credentials used to create a connection.
type ClientRequest struct {
	ClientID     string
	ClientSecret string
	// TODO: we can use this as the only source of truth for Client Credentials and Envs.
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// requestTokenAndEnv requests a token.
func requestTokenAndEnv(ctx context.Context, cloudEnv string, cr ClientRequest) (string, *cloudEndpoint, error) {
	if cr.ClientID == "" {
		return "", nil, fmt.Errorf("client_id is not set")
	}
	if cr.ClientSecret == "" {
		return "", nil, fmt.Errorf("client_secret is not set")
	}
	endpoint, found := cloudAuthEnvironments[cloudEnv]
	if !found {
		return "", nil, fmt.Errorf("unable to find requested environment: %q", cloudEnv)
	}
	payload := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s", cr.ClientID, cr.ClientSecret, endpoint.audience)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.authURL, strings.NewReader(payload))
	if err != nil {
		return "", nil, fmt.Errorf("unable to issue request to %v: %v", endpoint.authURL, err)
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request to %v failed: %v", endpoint.authURL, err)
	}
	if resp.StatusCode/100 != 2 {
		resBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", nil, fmt.Errorf("request to %v failed: unable to read body", endpoint.authURL)
		}
		return "", nil, fmt.Errorf("request to %v failed: %v %v: %s", endpoint.authURL, resp.StatusCode, http.StatusText(resp.StatusCode), resBody)
	}

	defer resp.Body.Close()

	tokenContainer := tokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenContainer); err != nil {
		return "", nil, fmt.Errorf("error decoding token response: %v", err)
	}
	if tokenContainer.AccessToken == "" {
		return "", nil, fmt.Errorf("no access token found in response: %v", tokenContainer)
	}
	return tokenContainer.AccessToken, &endpoint, nil
}

func spawnConn(ctx context.Context, url string, authToken string) (*grpc.ClientConn, error) {
	return grpc.DialContext(
		ctx,
		url,
		// Chain the interceptors using grpc_middleware.ChainUnaryClient
		grpc.WithUnaryInterceptor(grpcmiddleware.ChainUnaryClient(
			// Interceptor to add the Bearer token
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", authToken)), method, req, reply, cc, opts...)
			},
			// Retry interceptor
			grpcretry.UnaryClientInterceptor(
				grpcretry.WithCodes(codes.Unavailable, codes.Unknown, codes.Internal),
				grpcretry.WithMax(5),
				grpcretry.WithBackoff(grpcretry.BackoffExponential(time.Millisecond*100)),
			),
		)),
		// And provide TLS config.
		grpc.WithTransportCredentials(
			credentials.NewTLS(
				&tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			),
		),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.DefaultConfig,
		}),
		// We do not block (grpc.WithBlock) on purpose to avoid waiting
		// indefinitely if the cluster is not responding and to provide an
		// useful error on these cases. See:
		// https://github.com/grpc/grpc-go/blob/master/Documentation/anti-patterns.md#using-failonnontempdialerror-withblock-and-withreturnconnectionerror
	)
}
