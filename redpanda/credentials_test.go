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

const blockToken = "block-token"

func clearCredentialEnv(t *testing.T) {
	t.Helper()
	t.Setenv(ClientIDEnv, "")
	t.Setenv(ClientSecretEnv, "")
	t.Setenv(AccessTokenEnv, "")
}

// nullCredentialConfig models a provider block that sets only the two flags.
func nullCredentialConfig(skip, ignoreEnv bool) models.Redpanda {
	return models.Redpanda{
		ClientID:                     types.StringNull(),
		ClientSecret:                 types.StringNull(),
		AccessToken:                  types.StringNull(),
		SkipCredentialsValidation:    types.BoolValue(skip),
		IgnoreEnvironmentCredentials: types.BoolValue(ignoreEnv),
	}
}

func expectNoCredentials(t *testing.T, conf models.Redpanda) {
	t.Helper()
	creds, diags := getCredentials(context.Background(), "pre", conf)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !creds.NoCredentials {
		t.Fatal("NoCredentials should be set")
	}
	if creds.TokenSource != nil {
		t.Fatal("a configuration without credentials carries no TokenSource; consumers refuse it before any RPC")
	}
}

func TestGetCredentials_SkipValidation_NoCredentialsAnywhere(t *testing.T) {
	clearCredentialEnv(t)
	expectNoCredentials(t, nullCredentialConfig(true, false))
}

func TestGetCredentials_SkipValidation_UsesEnvironment(t *testing.T) {
	clearCredentialEnv(t)
	t.Setenv(ClientIDEnv, "env-id")
	t.Setenv(ClientSecretEnv, "env-secret")
	creds, diags := getCredentials(context.Background(), "pre", nullCredentialConfig(true, false))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if creds.NoCredentials || creds.ClientID != "env-id" {
		t.Fatalf("skip alone keeps the environment fallback, got NoCredentials=%v ClientID=%q", creds.NoCredentials, creds.ClientID)
	}
}

func TestGetCredentials_SkipValidation_PartialEnvironmentStillFails(t *testing.T) {
	clearCredentialEnv(t)
	t.Setenv(ClientIDEnv, "env-id")
	_, diags := getCredentials(context.Background(), "pre", nullCredentialConfig(true, false))
	if !diags.HasError() {
		t.Fatal("a client ID without a secret in the environment is a misconfiguration the flag must not hide")
	}
}

func TestGetCredentials_IgnoreEnvironment_IsSufficientAlone(t *testing.T) {
	for name, set := range map[string]func(t *testing.T){
		"empty environment": func(*testing.T) {},
		"token":             func(t *testing.T) { t.Setenv(AccessTokenEnv, "env-token") },
		"id and secret":     func(t *testing.T) { t.Setenv(ClientIDEnv, "env-id"); t.Setenv(ClientSecretEnv, "env-secret") },
		"id only":           func(t *testing.T) { t.Setenv(ClientIDEnv, "env-id") },
	} {
		t.Run(name, func(t *testing.T) {
			clearCredentialEnv(t)
			set(t)
			expectNoCredentials(t, nullCredentialConfig(false, true))
		})
	}
}

func TestGetCredentials_IgnoreEnvironment_BlockCredentialsStillUsed(t *testing.T) {
	clearCredentialEnv(t)
	t.Setenv(AccessTokenEnv, "env-token")
	conf := nullCredentialConfig(false, true)
	conf.AccessToken = types.StringValue(blockToken)
	creds, diags := getCredentials(context.Background(), "pre", conf)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	tok, err := creds.TokenSource.Token()
	if err != nil || tok.AccessToken != blockToken {
		t.Fatalf("expected the block token, got %v, %v", tok, err)
	}
}

func TestGetCredentials_FlagsTreatEmptyStringsAsUnset(t *testing.T) {
	shapes := map[string]func(conf *models.Redpanda){
		"id and secret": func(conf *models.Redpanda) {
			conf.ClientID = types.StringValue("")
			conf.ClientSecret = types.StringValue("")
		},
		"token": func(conf *models.Redpanda) { conf.AccessToken = types.StringValue("") },
		"all three": func(conf *models.Redpanda) {
			conf.ClientID, conf.ClientSecret, conf.AccessToken = types.StringValue(""), types.StringValue(""), types.StringValue("")
		},
	}
	for flag, conf := range map[string]models.Redpanda{"skip": nullCredentialConfig(true, false), "ignore_env": nullCredentialConfig(false, true)} {
		for shape, set := range shapes {
			t.Run(flag+"/"+shape, func(t *testing.T) {
				clearCredentialEnv(t)
				c := conf
				set(&c)
				expectNoCredentials(t, c)
			})
		}
	}
}

func TestGetCredentials_WithoutFlags_EmptyStringsAreCredentials(t *testing.T) {
	clearCredentialEnv(t)
	conf := nullCredentialConfig(false, false)
	conf.ClientID = types.StringValue("")
	conf.ClientSecret = types.StringValue("")
	_, diags := getCredentials(context.Background(), "pre", conf)
	if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), "client id is empty") {
		t.Fatalf("without a flag an empty pair takes the block path and fails there, got %v", diags)
	}
}

func TestGetCredentials_PartialBlockCredentialsFail(t *testing.T) {
	clearCredentialEnv(t)
	for _, conf := range []models.Redpanda{nullCredentialConfig(true, false), nullCredentialConfig(false, true)} {
		conf.ClientID = types.StringValue("block-id")
		if _, diags := getCredentials(context.Background(), "pre", conf); !diags.HasError() {
			t.Fatal("a client ID without a secret is a misconfiguration no flag may hide")
		}
	}
}

func TestGetCredentials_BlockCredentialsWinOverEnvironment(t *testing.T) {
	for name, conf := range map[string]models.Redpanda{"no flags": nullCredentialConfig(false, false), "skip": nullCredentialConfig(true, false)} {
		t.Run(name, func(t *testing.T) {
			clearCredentialEnv(t)
			t.Setenv(AccessTokenEnv, "env-token")
			conf.AccessToken = types.StringValue(blockToken)
			creds, diags := getCredentials(context.Background(), "pre", conf)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			tok, err := creds.TokenSource.Token()
			if err != nil || tok.AccessToken != blockToken {
				t.Fatalf("the block must win over the environment, got %v, %v", tok, err)
			}
		})
	}
}

func TestGetCredentials_KnownTokenIgnoresUnknownPair(t *testing.T) {
	clearCredentialEnv(t)
	conf := nullCredentialConfig(false, false)
	conf.AccessToken = types.StringValue(blockToken)
	conf.ClientID = types.StringUnknown()
	conf.ClientSecret = types.StringUnknown()
	creds, diags := getCredentials(context.Background(), "pre", conf)
	if diags.HasError() {
		t.Fatalf("a known token decides the credentials; the pair is never read: %v", diags)
	}
	if tok, err := creds.TokenSource.Token(); err != nil || tok.AccessToken != blockToken {
		t.Fatalf("expected the block token, got %v, %v", tok, err)
	}
}

func TestGetCredentials_UnknownDiagnosticsAreOrdered(t *testing.T) {
	clearCredentialEnv(t)
	conf := nullCredentialConfig(false, false)
	conf.ClientID = types.StringUnknown()
	conf.ClientSecret = types.StringUnknown()
	for range 20 {
		_, diags := getCredentials(context.Background(), "pre", conf)
		if len(diags.Errors()) != 2 || !strings.Contains(diags.Errors()[0].Summary(), "client_id") {
			t.Fatalf("diagnostics must come in declaration order, got %v", diags)
		}
	}
}

func TestGetCredentials_UnknownValuesFailAtPlanByName(t *testing.T) {
	for attr, set := range map[string]func(conf *models.Redpanda){
		"skip_credentials_validation":    func(conf *models.Redpanda) { conf.SkipCredentialsValidation = types.BoolUnknown() },
		"ignore_environment_credentials": func(conf *models.Redpanda) { conf.IgnoreEnvironmentCredentials = types.BoolUnknown() },
		"access_token":                   func(conf *models.Redpanda) { conf.AccessToken = types.StringUnknown() },
		"client_id":                      func(conf *models.Redpanda) { conf.ClientID = types.StringUnknown() },
		"client_secret":                  func(conf *models.Redpanda) { conf.ClientSecret = types.StringUnknown() },
	} {
		t.Run(attr, func(t *testing.T) {
			clearCredentialEnv(t)
			conf := nullCredentialConfig(true, false)
			set(&conf)
			creds, diags := getCredentials(context.Background(), "pre", conf)
			if creds.NoCredentials {
				t.Fatal("an unknown value is not an absent one")
			}
			if !diags.HasError() || !strings.Contains(diags.Errors()[0].Summary(), attr) || !strings.Contains(diags.Errors()[0].Summary(), "must be known at plan") {
				t.Fatalf("want an error naming %s and 'must be known at plan', got %v", attr, diags)
			}
		})
	}
}

func TestGetCredentials_WithoutFlagsMissingCredentialsErrors(t *testing.T) {
	clearCredentialEnv(t)
	for _, conf := range []models.Redpanda{
		nullCredentialConfig(false, false),
		{ClientID: types.StringNull(), ClientSecret: types.StringNull(), AccessToken: types.StringNull(), SkipCredentialsValidation: types.BoolNull(), IgnoreEnvironmentCredentials: types.BoolNull()},
	} {
		creds, diags := getCredentials(context.Background(), "pre", conf)
		if !diags.HasError() || diags.Errors()[0].Summary() != "Client configuration missing" {
			t.Fatalf("expected Client configuration missing, got %v", diags)
		}
		if creds.NoCredentials {
			t.Fatal("NoCredentials must only be set under a flag")
		}
	}
}
