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
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
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
	APIURL         string // CloudV2 public API URL.
	InternalAPIURL string // CloudV2 internal API URL.
	authURL        string // CloudV2 URL for authorization token exchange.
	audience       string // CloudV2 audience used for token exchange.
}

var endpoints = map[string]Endpoint{
	"dev": {
		"api.dev.cloud.redpanda.com:443",
		"https://cloud-api.dev.cloud.redpanda.com",
		"https://dev-cloudv2.us.auth0.com/oauth/token",
		"cloudv2-dev.redpanda.cloud",
	},
	"ign": {
		"api.ign.cloud.redpanda.com:443",
		"https://cloud-api.ign.cloud.redpanda.com",
		"https://integration-cloudv2.us.auth0.com/oauth/token",
		"cloudv2-ign.redpanda.cloud",
	},
	"pre": {
		APIURL:         "api.ppd.cloud.redpanda.com:443",
		InternalAPIURL: "https://cloud-api.ppd.cloud.redpanda.com",
		authURL:        "https://preprod-cloudv2.us.auth0.com/oauth/token",
		audience:       "cloudv2-preprod.redpanda.cloud",
	},
	"prod": {
		"api.redpanda.com:443",
		"https://cloud-api.prd.cloud.redpanda.com",
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

// EndpointForEnv returns the Endpoint for a given environment.
func EndpointForEnv(cloudEnv string) (*Endpoint, error) {
	endpoint, found := endpoints[cloudEnv]
	if !found {
		return nil, fmt.Errorf("unable to find requested environment: %q", cloudEnv)
	}
	return &endpoint, nil
}

// RequestToken requests an authentication token for a given Endpoint.
func RequestToken(ctx context.Context, endpoint *Endpoint, clientID, clientSecret string) (string, error) {
	if clientID == "" {
		return "", errors.New("client_id is not set")
	}
	if clientSecret == "" {
		return "", errors.New("client_secret is not set")
	}
	payload := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s", clientID, clientSecret, endpoint.audience)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.authURL, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("unable to issue request to %v: %v", endpoint.authURL, err)
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request to %v failed: %v", endpoint.authURL, err)
	}
	if resp.StatusCode/100 != 2 {
		resBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("request to %v failed: unable to read body", endpoint.authURL)
		}
		return "", fmt.Errorf("request to %v failed: %v %v: %s", endpoint.authURL, resp.StatusCode, http.StatusText(resp.StatusCode), resBody)
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

var rl = newRateLimiter(500)

var urlRegex = regexp.MustCompile(`^(?:https://)?([^/]+?(?::\d+)?)/?$`)

// parseHTTPSURLAsGrpc parse an HTTPS URL into a valid GRPC URL
func parseHTTPSURLAsGrpc(url string) (string, error) {
	match := urlRegex.FindStringSubmatch(url)
	if match == nil {
		return "", fmt.Errorf("error converting url into grpc url: %v", url)
	}
	return match[1], nil
}

// SpawnConn returns a grpc connection to the given URL, it adds a bearer token
// to each request with the given 'authToken'.
func SpawnConn(url, authToken, providerVersion, terraformVersion string) (*grpc.ClientConn, error) {
	// we need a GRPC URL, but it's likely that we'll be given an HTTPS URL instead
	grpcURL, err := parseHTTPSURLAsGrpc(url)
	if err != nil {
		return nil, err
	}

	return grpc.NewClient(
		grpcURL,
		// Chain the interceptors using grpc_middleware.ChainUnaryClient
		grpc.WithUnaryInterceptor(grpcmiddleware.ChainUnaryClient(
			// Interceptor to add the Bearer token
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", authToken)), method, req, reply, cc, opts...)
			},
			func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				start := time.Now()
				err := invoker(ctx, method, req, reply, cc, opts...)
				tflog.Debug(ctx, "Redpanda API call", map[string]any{
					"method":   method,
					"duration": time.Since(start),
					"error":    err,
				})
				return err
			},
			rl.Limiter,
			// Retry interceptor
			grpcretry.UnaryClientInterceptor(
				grpcretry.WithCodes(codes.Unavailable, codes.Unknown, codes.Internal, codes.Unauthenticated),
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
		grpc.WithUserAgent(
			fmt.Sprintf("Terraform/%s %s_%s terraform-provider-redpanda/%s", terraformVersion, runtime.GOOS, runtime.GOARCH, providerVersion),
		),
		// We do not block (grpc.WithBlock) on purpose to avoid waiting
		// indefinitely if the cluster is not responding and to provide an
		// useful error on these cases. See:
		// https://github.com/grpc/grpc-go/blob/master/Documentation/anti-patterns.md#using-failonnontempdialerror-withblock-and-withreturnconnectionerror
	)
}
