// Copyright 2025 Redpanda Data, Inc.
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

package kclients

import (
	"context"
	"strings"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/twmb/franz-go/pkg/sr"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGetSchemaRegistryClientForCluster_ServerlessFallback proves that a
// serverless cluster ID — which the dedicated ClusterService rejects
// (PermissionDenied live, NotFound against the fake) — still resolves its
// Schema Registry URL via the ServerlessClusterService fallback, rather than
// failing because GetSchemaRegistryClientForCluster only calls ClusterForID.
func TestGetSchemaRegistryClientForCluster_ServerlessFallback(t *testing.T) {
	ctx := context.Background()
	srv := mock.New(t)

	op, err := srv.ServerlessCluster.CreateServerlessCluster(ctx, &controlplanev1.CreateServerlessClusterRequest{
		ServerlessCluster: &controlplanev1.ServerlessClusterCreate{Name: "sr-fallback"},
	})
	if err != nil {
		t.Fatalf("seed serverless cluster: %v", err)
	}
	id := op.GetOperation().GetResourceId()
	if id == "" {
		t.Fatal("seeded serverless cluster has empty ID")
	}

	conn, err := grpc.NewClient("passthrough:///bufnet", srv.Dialer()...)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	cpCl := cloud.NewControlPlaneClientSet(conn)

	// The live ClusterService returns PermissionDenied (not NotFound) for a
	// serverless id. The fake can't model that boundary on its own, so force
	// it to pin that the fallback triggers on PermissionDenied specifically,
	// not just any error.
	srv.OverrideOnce(
		"/redpanda.api.controlplane.v1.ClusterService/GetCluster",
		status.Error(codes.PermissionDenied, "Missing required permission read"),
	)

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "tok"})
	client, err := GetSchemaRegistryClientForCluster(ctx, cpCl, id, ts, "", "")
	if err != nil {
		t.Fatalf("expected serverless SR client, got error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil sr.Client")
	}

	// The seeded cluster defaults to public networking enabled, so the
	// resolved SR URL is the public one.
	publicURL, err := schemaRegistryURLForCluster(ctx, cpCl, id)
	if err != nil {
		t.Fatalf("resolve public SR URL: %v", err)
	}
	if want := "https://mock.schema-registry.redpanda.cloud"; publicURL != want {
		t.Fatalf("public-enabled serverless SR URL = %q, want %q", publicURL, want)
	}

	// A public-disabled serverless cluster rejects the public SR URL
	// ("public network is not enabled"); resolution must return the private
	// URL reachable from inside the private link.
	privOp, err := srv.ServerlessCluster.CreateServerlessCluster(ctx, &controlplanev1.CreateServerlessClusterRequest{
		ServerlessCluster: &controlplanev1.ServerlessClusterCreate{
			Name: "sr-private",
			NetworkingConfig: &controlplanev1.ServerlessNetworkingConfig{
				Public:  controlplanev1.ServerlessNetworkingConfig_STATE_DISABLED,
				Private: controlplanev1.ServerlessNetworkingConfig_STATE_ENABLED,
			},
		},
	})
	if err != nil {
		t.Fatalf("seed private serverless cluster: %v", err)
	}
	privID := privOp.GetOperation().GetResourceId()
	if privID == "" {
		t.Fatal("seeded private serverless cluster has empty ID")
	}
	privateURL, err := schemaRegistryURLForCluster(ctx, cpCl, privID)
	if err != nil {
		t.Fatalf("resolve private SR URL: %v", err)
	}
	if want := "https://mock.schema-registry.private.redpanda.cloud"; privateURL != want {
		t.Fatalf("public-disabled serverless SR URL = %q, want %q", privateURL, want)
	}
}

// TestSetSubjectCompatibility_InvalidLevelReturnsError proves that an
// unrecognised compatibility string is rejected rather than silently substituted
// with BACKWARD. The error is returned before SetCompatibility is called, so a
// nil client is safe to pass.
func TestSetSubjectCompatibility_InvalidLevelReturnsError(t *testing.T) {
	invalidCases := []struct {
		name        string
		input       string
		errContains string
	}{
		{"typo BACKWRD", "BACKWRD", `invalid compatibility level "BACKWRD"`},
		{"garbage value", "INVALID_LEVEL", `invalid compatibility level "INVALID_LEVEL"`},
		{"numeric", "42", `invalid compatibility level "42"`},
		{"mixed case typo", "Backward_Transitivee", `invalid compatibility level "Backward_Transitivee"`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := SetSubjectCompatibility(context.Background(), (*sr.Client)(nil), "test-subject", tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errContains)
			}
		})
	}

	t.Run("empty string is a no-op", func(t *testing.T) {
		err := SetSubjectCompatibility(context.Background(), (*sr.Client)(nil), "test-subject", "")
		if err != nil {
			t.Fatalf("unexpected error for empty string: %v", err)
		}
	})
}

// TestSchemaRegistryAuthOption_Precedence covers the cases the helper must
// distinguish: explicit Basic creds win, fall back to Bearer via TokenSource,
// error when nothing is available, explicit Basic wins even when a
// TokenSource is also supplied.
func TestSchemaRegistryAuthOption_Precedence(t *testing.T) {
	bearer := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "tok-bearer"})
	tests := []struct {
		name     string
		ts       oauth2.TokenSource
		username string
		password string
		wantErr  string
	}{
		{
			name:     "username and password — Basic auth",
			username: "alice",
			password: "p4ssw0rd",
		},
		{
			name: "token source only — Bearer auth",
			ts:   bearer,
		},
		{
			name:     "both Basic creds and token source — Basic wins",
			ts:       bearer,
			username: "alice",
			password: "p4ssw0rd",
		},
		{
			name:    "no credentials — error",
			wantErr: "no schema registry credentials available",
		},
		{
			name:     "username only with token source — Bearer (username alone is not enough for Basic)",
			ts:       bearer,
			username: "alice",
		},
		{
			name:     "password only with token source — Bearer (password alone is not enough for Basic)",
			ts:       bearer,
			password: "p4ssw0rd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := schemaRegistryAuthOption(tt.ts, tt.username, tt.password)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opt == nil {
				t.Fatal("expected non-nil sr.ClientOpt")
			}
		})
	}
}
