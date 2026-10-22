// Copyright 2026 Redpanda Data, Inc.
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

package redpanda

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
)

func clearCredentialEnv(t *testing.T) {
	t.Helper()
	t.Setenv(ClientIDEnv, "")
	t.Setenv(ClientSecretEnv, "")
	t.Setenv(AccessTokenEnv, "")
}

func nullCredentialConfig(skip bool) models.Redpanda {
	return models.Redpanda{
		ClientID:                  types.StringNull(),
		ClientSecret:              types.StringNull(),
		AccessToken:               types.StringNull(),
		SkipCredentialsValidation: types.BoolValue(skip),
	}
}

func TestGetCredentials_SkipValidationWithoutCredentials(t *testing.T) {
	clearCredentialEnv(t)
	creds, diags := getCredentials(context.Background(), "pre", nullCredentialConfig(true))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !creds.NoCredentials {
		t.Fatal("NoCredentials should be set when the flag is on and no credentials are found")
	}
	if creds.EndpointAPIURL == "" {
		t.Fatal("endpoint must still resolve so the connection can be built")
	}
	if creds.TokenSource == nil {
		t.Fatal("TokenSource must be non-nil so a stray RPC errors instead of panicking")
	}
	if _, err := creds.TokenSource.Token(); err == nil || !strings.Contains(err.Error(), "skip_credentials_validation") {
		t.Fatalf("Token() should fail naming skip_credentials_validation, got %v", err)
	}
}

func TestGetCredentials_SkipValidationTreatsEmptyStringsAsAbsent(t *testing.T) {
	t.Run("falls through to the environment", func(t *testing.T) {
		clearCredentialEnv(t)
		t.Setenv(ClientIDEnv, "env-id")
		t.Setenv(ClientSecretEnv, "env-secret")
		conf := nullCredentialConfig(true)
		conf.ClientID = types.StringValue("")
		conf.ClientSecret = types.StringValue("")
		creds, diags := getCredentials(context.Background(), "pre", conf)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if creds.NoCredentials || creds.ClientID != "env-id" {
			t.Fatalf("expected env credentials, got NoCredentials=%v ClientID=%q", creds.NoCredentials, creds.ClientID)
		}
	})
	t.Run("ends unconfigured when the environment is empty too", func(t *testing.T) {
		clearCredentialEnv(t)
		conf := nullCredentialConfig(true)
		conf.ClientID = types.StringValue("")
		conf.ClientSecret = types.StringValue("")
		creds, diags := getCredentials(context.Background(), "pre", conf)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if !creds.NoCredentials {
			t.Fatal("NoCredentials should be set")
		}
	})
}

func TestGetCredentials_SkipValidationKeepsPartialCredentialErrors(t *testing.T) {
	clearCredentialEnv(t)
	t.Setenv(ClientIDEnv, "env-id")
	_, diags := getCredentials(context.Background(), "pre", nullCredentialConfig(true))
	if !diags.HasError() {
		t.Fatal("a client ID without a secret is a misconfiguration the flag must not hide")
	}
}

func TestGetCredentials_SkipValidationUsesCredentialsWhenSet(t *testing.T) {
	clearCredentialEnv(t)
	conf := nullCredentialConfig(true)
	conf.AccessToken = types.StringValue("static-token")
	creds, diags := getCredentials(context.Background(), "pre", conf)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if creds.NoCredentials {
		t.Fatal("NoCredentials must stay false when credentials are configured")
	}
	tok, err := creds.TokenSource.Token()
	if err != nil || tok.AccessToken != "static-token" {
		t.Fatalf("expected the configured token, got %v, %v", tok, err)
	}
}

func TestGetCredentials_WithoutSkipMissingCredentialsErrors(t *testing.T) {
	clearCredentialEnv(t)
	for _, conf := range []models.Redpanda{
		nullCredentialConfig(false),
		{ClientID: types.StringNull(), ClientSecret: types.StringNull(), AccessToken: types.StringNull(), SkipCredentialsValidation: types.BoolNull()},
	} {
		creds, diags := getCredentials(context.Background(), "pre", conf)
		if !diags.HasError() || diags.Errors()[0].Summary() != "Client configuration missing" {
			t.Fatalf("expected Client configuration missing, got %v", diags)
		}
		if creds.NoCredentials {
			t.Fatal("NoCredentials must only be set under the flag")
		}
	}
}
