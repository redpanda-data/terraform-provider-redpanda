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

func nullCredentialConfig() models.Redpanda {
	return models.Redpanda{ClientID: types.StringNull(), ClientSecret: types.StringNull(), AccessToken: types.StringNull()}
}

func TestGetCredentials_KnownTokenIgnoresUnknownPair(t *testing.T) {
	clearCredentialEnv(t)
	conf := nullCredentialConfig()
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
	conf := nullCredentialConfig()
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
		"access_token":  func(conf *models.Redpanda) { conf.AccessToken = types.StringUnknown() },
		"client_id":     func(conf *models.Redpanda) { conf.ClientID = types.StringUnknown() },
		"client_secret": func(conf *models.Redpanda) { conf.ClientSecret = types.StringUnknown() },
	} {
		t.Run(attr, func(t *testing.T) {
			clearCredentialEnv(t)
			conf := nullCredentialConfig()
			set(&conf)
			_, diags := getCredentials(context.Background(), "pre", conf)
			if !diags.HasError() || !strings.Contains(diags.Errors()[0].Summary(), attr) || !strings.Contains(diags.Errors()[0].Summary(), "must be known at plan") {
				t.Fatalf("want an error naming %s and 'must be known at plan', got %v", attr, diags)
			}
		})
	}
}
