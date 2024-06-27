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

// Package cloud provides the methods to connect and talk to the Redpanda Cloud
// public API.
package cloud

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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Endpoint is a representation of a cloud endpoint for a single environment. It
// contains the URLs, audience for authentication and the API URL.
type Endpoint struct {
	APIURL   string // CloudV2 public API URL.
	authURL  string // CloudV2 URL for authorization token exchange.
	audience string // CloudV2 audience used for token exchange.
}

var endpoints = map[string]Endpoint{
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

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// RequestTokenAndEnv requests an authentication token and return the Endpoint
// for a given environment.
func RequestTokenAndEnv(ctx context.Context, cloudEnv, clientID, clientSecret string) (string, *Endpoint, error) {
	if clientID == "" {
		return "", nil, fmt.Errorf("client_id is not set")
	}
	if clientSecret == "" {
		return "", nil, fmt.Errorf("client_secret is not set")
	}
	endpoint, found := endpoints[cloudEnv]
	if !found {
		return "", nil, fmt.Errorf("unable to find requested environment: %q", cloudEnv)
	}
	payload := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s", clientID, clientSecret, endpoint.audience)
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

var rl = newRateLimiter(500)

// SpawnConn returns a grpc connection to the given URL, it adds a bearer token
// to each request with the given 'authToken'.
func SpawnConn(url string, authToken string) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		url,
		// Chain the interceptors using grpc_middleware.ChainUnaryClient
		grpc.WithUnaryInterceptor(grpcmiddleware.ChainUnaryClient(
			// Interceptor to add the Bearer token
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", authToken)), method, req, reply, cc, opts...)
			},
			func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				start := time.Now()
				err := invoker(ctx, method, req, reply, cc, opts...)
				tflog.Debug(ctx, "method: %s, duration: %v, error: %v\n", map[string]any{
					"method":   method,
					"duration": time.Since(start),
					"error":    err,
				})
				return err
			},
			rl.Limiter,
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
