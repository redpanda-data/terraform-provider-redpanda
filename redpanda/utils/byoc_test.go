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

package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"github.com/redpanda-data/redpanda/src/go/rpk/pkg/cloudapi"
	"golang.org/x/oauth2"
)

func TestProgressLine(t *testing.T) {
	zap := func(level, msg string) string {
		return "2026-09-21T10:00:00.000Z\t" + level + "\tbyoc\tfile.go:1\t" + msg
	}
	cases := []struct {
		name string
		line string
		want string
		ok   bool
	}{
		{"info message forwarded", zap("INFO", "Reconciling agent infrastructure..."), "Reconciling agent infrastructure...", true},
		{"warn message forwarded", zap("WARN", "retrying"), "retrying", true},
		{"error message forwarded", zap("ERROR", "apply failed"), "apply failed", true},
		{"debug dropped", zap("DEBUG", "terraform plan output"), "", false},
		{"non-zap line forwarded raw", "Error: required flag(s) not set", "Error: required flag(s) not set", true},
		{"empty line dropped", "", "", false},
		{"azure sdk request dropped", "[Sep 30 00:54:20.551234] Request: ==> OUTGOING REQUEST (Try=1)", "", false},
		{"azure sdk authentication dropped", "[Sep 30 00:54:20.551234] Authentication: WorkloadIdentityCredential.GetToken() acquired a token", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := progressLine(tc.line)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("progressLine(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestByocClient_CheckCloudConfig(t *testing.T) {
	cases := []struct {
		name    string
		conf    ByocClientConfig
		cloud   string
		wantErr string
		env     map[string]string
	}{
		{"aws never checked", ByocClientConfig{}, "aws", "", nil},
		{"gcp missing project", ByocClientConfig{}, "gcp", "gcp_project_id", nil},
		{"gcp with project", ByocClientConfig{GcpProject: "p"}, "gcp", "", nil},
		{"azure missing subscription", ByocClientConfig{}, "azure", "azure_subscription_id", nil},
		{"azure with subscription", ByocClientConfig{AzureSubscriptionID: "s"}, "azure", "", nil},
		{"azure two auth flags", ByocClientConfig{AzureSubscriptionID: "s"}, "azure", "only one of", map[string]string{"ARM_USE_MSI": "true", "ARM_USE_CLI": "true"}},
		{"azure oidc without a token or az", ByocClientConfig{AzureSubscriptionID: "s"}, "azure", "AZURE_FEDERATED_TOKEN_FILE", map[string]string{"ARM_USE_OIDC": "true"}},
		{"azure oidc without a token falls back to az", ByocClientConfig{AzureSubscriptionID: "s"}, "azure", "", map[string]string{"ARM_USE_OIDC": "true", "PATH": "<with az>"}},
		{"azure oidc with a token file needs no az", ByocClientConfig{AzureSubscriptionID: "s", AzureTenantID: "t", AzureClientID: "c"}, "azure", "", map[string]string{"ARM_USE_OIDC": "true", "AZURE_FEDERATED_TOKEN_FILE": "/token"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAzureEnv(t)
			t.Setenv("PATH", t.TempDir())
			for k, v := range tc.env {
				if v == "<with az>" {
					v = t.TempDir()
					if err := os.Symlink("/bin/sh", filepath.Join(v, "az")); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv(k, v)
			}
			err := NewByocClient(tc.conf).CheckCloudConfig(tc.cloud)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("CheckCloudConfig(%s) = %v, want nil", tc.cloud, err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("CheckCloudConfig(%s) = %v, want error mentioning %q", tc.cloud, err, tc.wantErr)
			default:
			}
		})
	}
}

func clearAzureEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "ARM_") || strings.HasPrefix(k, "AZURE_") || strings.HasPrefix(k, "ACTIONS_ID_TOKEN_") {
			t.Setenv(k, "")
			if err := os.Unsetenv(k); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// logLevels returns the level of every captured log entry whose message
// contains substr.
func logLevels(t *testing.T, out *bytes.Buffer, substr string) []string {
	t.Helper()
	entries, err := tflogtest.MultilineJSONDecode(out)
	if err != nil {
		t.Fatal(err)
	}
	var levels []string
	for _, e := range entries {
		if msg, ok := e["@message"].(string); ok && strings.Contains(msg, substr) {
			levels = append(levels, fmt.Sprint(e["@level"]))
		}
	}
	return levels
}

func flagValue(args []string, flag string) (string, bool) {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

func lastEnv(env []string, key string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], key+"="); ok {
			return v, true
		}
	}
	return "", false
}

func TestByocClient_AzureCredentialArgs(t *testing.T) {
	const (
		tenant = "00000000-0000-0000-0000-00000000000a"
		client = "00000000-0000-0000-0000-00000000000b"
	)
	tokenFile := filepath.Join(t.TempDir(), "federated-token")
	unset := "<unset>"

	cases := []struct {
		name       string
		env        map[string]string
		conf       ByocClientConfig
		wantArgs   map[string]string
		wantEnv    map[string]string
		wantFiles  map[string]string
		wantReason string
		noAz       bool
		wantErr    string
	}{
		{
			name:       "client secret uses environment credential",
			wantReason: "client secret or certificate",
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client, AzureClientSecret: "s3cret"},
			wantArgs:   map[string]string{"--credential-source": "env", "--identity": unset},
			wantEnv: map[string]string{
				"AZURE_TOKEN_CREDENTIALS": unset,
				"ARM_USE_OIDC":            unset,
				"AZURE_CLIENT_ID":         client,
				"AZURE_CLIENT_SECRET":     "s3cret",
			},
		},
		{
			name:       "inline client certificate reaches the environment credential as a file",
			wantReason: "client secret or certificate",
			env:        map[string]string{"ARM_CLIENT_CERTIFICATE": base64.StdEncoding.EncodeToString([]byte("pkcs12-bundle"))},
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantArgs:   map[string]string{"--credential-source": "env"},
			wantFiles:  map[string]string{"AZURE_CLIENT_CERTIFICATE_PATH": "pkcs12-bundle"},
		},
		{
			name:     "certificate path wins over an inline certificate",
			conf:     ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			env:      map[string]string{"ARM_CLIENT_CERTIFICATE": base64.StdEncoding.EncodeToString([]byte("inline")), "ARM_CLIENT_CERTIFICATE_PATH": tokenFile},
			wantArgs: map[string]string{"--credential-source": "env"},
			wantEnv:  map[string]string{"AZURE_CLIENT_CERTIFICATE_PATH": tokenFile},
		},
		{
			name:    "inline client certificate that is not base64 fails before the plugin runs",
			conf:    ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			env:     map[string]string{"ARM_CLIENT_CERTIFICATE": "not base64!"},
			wantErr: "ARM_CLIENT_CERTIFICATE",
		},
		{
			name: "provider config credentials win over a different arm pair",
			env: map[string]string{
				"ARM_CLIENT_ID":     "00000000-0000-0000-0000-00000000000c",
				"ARM_CLIENT_SECRET": "other-secret",
			},
			conf:     ByocClientConfig{AzureTenantID: tenant, AzureClientID: client, AzureClientSecret: "s3cret"},
			wantArgs: map[string]string{"--credential-source": "env"},
			wantEnv:  map[string]string{"AZURE_CLIENT_ID": client, "AZURE_CLIENT_SECRET": "s3cret"},
		},
		{
			name:       "no flag, secret, or token leaves the plugin default and pins the cli",
			wantReason: "no Azure auth settings, plugin default Azure CLI",
			wantArgs:   map[string]string{"--credential-source": unset, "--identity": unset},
			wantEnv:    map[string]string{"AZURE_TOKEN_CREDENTIALS": "AzureCLICredential"},
		},
		{
			name:       "msi",
			wantReason: "ARM_USE_MSI",
			env:        map[string]string{"ARM_USE_MSI": "true"},
			wantArgs:   map[string]string{"--credential-source": "msi", "--identity": "msi"},
			wantEnv:    map[string]string{"AZURE_TOKEN_CREDENTIALS": "ManagedIdentityCredential"},
		},
		{
			name:       "cli",
			wantReason: "ARM_USE_CLI",
			env:        map[string]string{"ARM_USE_CLI": "true"},
			wantArgs:   map[string]string{"--credential-source": "cli", "--identity": "cli"},
			wantEnv:    map[string]string{"AZURE_TOKEN_CREDENTIALS": "AzureCLICredential"},
		},
		{
			name:       "aks workload identity",
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantReason: "ARM_USE_AKS_WORKLOAD_IDENTITY",
			env:        map[string]string{"ARM_USE_AKS_WORKLOAD_IDENTITY": "true"},
			wantArgs:   map[string]string{"--credential-source": "workload", "--identity": "none"},
			wantEnv:    map[string]string{"AZURE_TOKEN_CREDENTIALS": "WorkloadIdentityCredential"},
		},
		{
			name:       "oidc with client secret keeps environment credential",
			wantReason: "ARM_USE_OIDC with a client secret or certificate",
			env:        map[string]string{"ARM_USE_OIDC": "true"},
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client, AzureClientSecret: "s3cret"},
			wantArgs:   map[string]string{"--credential-source": "env", "--identity": "oidc"},
		},
		{
			name:       "oidc with azure federated token file",
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantReason: "ARM_USE_OIDC with AZURE_FEDERATED_TOKEN_FILE",
			env:        map[string]string{"ARM_USE_OIDC": "true", "AZURE_FEDERATED_TOKEN_FILE": tokenFile},
			wantArgs:   map[string]string{"--credential-source": "workload", "--identity": "oidc"},
			wantEnv: map[string]string{
				"AZURE_FEDERATED_TOKEN_FILE": tokenFile,
				"ARM_OIDC_TOKEN_FILE_PATH":   tokenFile,
				"AZURE_TOKEN_CREDENTIALS":    "WorkloadIdentityCredential",
			},
		},
		{
			name:       "oidc with arm token file path",
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantReason: "ARM_USE_OIDC with ARM_OIDC_TOKEN_FILE_PATH",
			env:        map[string]string{"ARM_USE_OIDC": "true", "ARM_OIDC_TOKEN_FILE_PATH": tokenFile},
			wantArgs:   map[string]string{"--credential-source": "workload", "--identity": "oidc"},
			wantEnv: map[string]string{
				"AZURE_FEDERATED_TOKEN_FILE": tokenFile,
				"AZURE_TOKEN_CREDENTIALS":    "WorkloadIdentityCredential",
			},
		},
		{
			name:       "oidc with raw arm token",
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantReason: "ARM_USE_OIDC with ARM_OIDC_TOKEN",
			env:        map[string]string{"ARM_USE_OIDC": "true", "ARM_OIDC_TOKEN": "raw-jwt"},
			wantArgs:   map[string]string{"--credential-source": "workload", "--identity": "oidc"},
			wantEnv:    map[string]string{"AZURE_TOKEN_CREDENTIALS": "WorkloadIdentityCredential"},
			wantFiles:  map[string]string{"AZURE_FEDERATED_TOKEN_FILE": "raw-jwt"},
		},
		{
			name:       "oidc with only the github request token",
			wantReason: "ARM_USE_OIDC without a token, Azure CLI session",
			env: map[string]string{
				"ARM_USE_OIDC":                   "true",
				"ACTIONS_ID_TOKEN_REQUEST_URL":   "https://token.actions.example/request",
				"ACTIONS_ID_TOKEN_REQUEST_TOKEN": "request-token",
				"AZURE_CLIENT_ID":                client,
			},
			conf:     ByocClientConfig{AzureClientID: client},
			wantArgs: map[string]string{"--credential-source": "cli", "--identity": "oidc"},
			wantEnv:  map[string]string{"AZURE_TOKEN_CREDENTIALS": "AzureCLICredential", "ARM_CLIENT_ID": client},
		},
		{
			name:    "oidc with no token and no az fails before the plugin runs",
			env:     map[string]string{"ARM_USE_OIDC": "true"},
			noAz:    true,
			wantErr: "AZURE_FEDERATED_TOKEN_FILE",
		},
		{
			name:    "workload identity without a tenant fails before the plugin runs",
			conf:    ByocClientConfig{AzureClientID: client},
			env:     map[string]string{"ARM_USE_OIDC": "true", "AZURE_FEDERATED_TOKEN_FILE": tokenFile},
			wantErr: "tenant",
		},
		{
			name:    "workload identity without a client id fails before the plugin runs",
			conf:    ByocClientConfig{AzureTenantID: tenant},
			env:     map[string]string{"ARM_USE_OIDC": "true", "AZURE_FEDERATED_TOKEN_FILE": tokenFile},
			wantErr: "client ID",
		},
		{
			name:    "client secret without a tenant fails before the plugin runs",
			conf:    ByocClientConfig{AzureClientID: client, AzureClientSecret: "s3cret"},
			wantErr: "tenant",
		},
		{
			name:    "client certificate without a client id fails before the plugin runs",
			conf:    ByocClientConfig{AzureTenantID: tenant},
			env:     map[string]string{"ARM_CLIENT_CERTIFICATE_PATH": tokenFile},
			wantErr: "client ID",
		},
		{
			name:       "federated token file without a flag or secret selects workload identity",
			wantReason: "AZURE_FEDERATED_TOKEN_FILE without ARM_USE_OIDC or a secret",
			env:        map[string]string{"AZURE_FEDERATED_TOKEN_FILE": tokenFile, "AZURE_CLIENT_ID": client},
			conf:       ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantArgs:   map[string]string{"--credential-source": "workload", "--identity": "oidc"},
			wantEnv: map[string]string{
				"ARM_OIDC_TOKEN_FILE_PATH": tokenFile,
				"AZURE_TOKEN_CREDENTIALS":  "WorkloadIdentityCredential",
				"ARM_USE_OIDC":             "true",
				"ARM_CLIENT_ID":            client,
			},
		},
		{
			name:     "user-set token credentials selection is kept",
			conf:     ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			env:      map[string]string{"ARM_USE_OIDC": "true", "AZURE_FEDERATED_TOKEN_FILE": tokenFile, "AZURE_TOKEN_CREDENTIALS": "prod"},
			wantArgs: map[string]string{"--credential-source": "workload"},
			wantEnv:  map[string]string{"AZURE_TOKEN_CREDENTIALS": "prod"},
		},
		{
			name:    "azure sdk logging is left to the user",
			wantEnv: map[string]string{"AZURE_SDK_GO_LOGGING": unset},
		},
		{
			name:    "user-set azure sdk logging is kept",
			env:     map[string]string{"AZURE_SDK_GO_LOGGING": "off"},
			wantEnv: map[string]string{"AZURE_SDK_GO_LOGGING": "off"},
		},
		{
			name:     "tenant and client from provider config reach the plugin",
			conf:     ByocClientConfig{AzureTenantID: tenant, AzureClientID: client},
			wantArgs: map[string]string{"--tenant-id": tenant},
			wantEnv:  map[string]string{"AZURE_TENANT_ID": tenant, "AZURE_CLIENT_ID": client},
		},
		{
			name:     "no tenant sends no tenant flag",
			wantArgs: map[string]string{"--tenant-id": unset},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAzureEnv(t)
			pathDir := t.TempDir()
			if !tc.noAz {
				if err := os.Symlink("/bin/sh", filepath.Join(pathDir, "az")); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", pathDir)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			tc.conf.TokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "cloud-token"})
			tc.conf.AzureSubscriptionID = "sub"
			cluster := cloudapi.Cluster{NameID: cloudapi.NameID{ID: "cluster-id"}, Spec: cloudapi.ClusterSpec{Provider: "Azure"}}

			if tc.wantReason != "" {
				auth, err := NewByocClient(tc.conf).resolveAzureAuth()
				if err != nil {
					t.Fatalf("resolveAzureAuth: %v", err)
				}
				if auth.reason != tc.wantReason {
					t.Errorf("reason = %q, want %q", auth.reason, tc.wantReason)
				}
			}
			var logs bytes.Buffer
			ctx := tflogtest.RootLogger(context.Background(), &logs)
			args, env, cleanup, err := NewByocClient(tc.conf).generateByocArgsAndEnv(ctx, cluster, "apply")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("generateByocArgsAndEnv error = %v, want one mentioning %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("generateByocArgsAndEnv: %v", err)
			}
			if cleanup != nil {
				defer cleanup()
			}
			for flag, want := range tc.wantArgs {
				got, ok := flagValue(args, flag)
				if !ok {
					got = unset
				}
				if got != want {
					t.Errorf("%s = %q, want %q (args %q)", flag, got, want, args)
				}
			}
			if got := logLevels(t, &logs, "byoc plugin Azure authentication"); !slices.Equal(got, []string{"debug"}) {
				t.Errorf("credential decision logged at %q, want once at debug", got)
			}
			for key, want := range tc.wantEnv {
				got, ok := lastEnv(env, key)
				if !ok {
					got = unset
				}
				if got != want {
					t.Errorf("env %s = %q, want %q", key, got, want)
				}
			}
			for key, want := range tc.wantFiles {
				path, _ := lastEnv(env, key)
				raw, err := os.ReadFile(filepath.Clean(path))
				if err != nil {
					t.Fatalf("read %s %q: %v", key, path, err)
				}
				if string(raw) != want {
					t.Errorf("%s holds %q, want %q", key, raw, want)
				}
				if cleanup == nil {
					t.Fatalf("no cleanup returned for %s", key)
				}
				cleanup()
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("%s %q survives cleanup: %v", key, path, err)
				}
			}
		})
	}
}

func TestForwardLogs(t *testing.T) {
	zap := "2026-09-21T10:00:00.000Z\tINFO\tbyoc\tfile.go:1\tDestroying agent infrastructure..."
	auth := "[Sep 30 00:54:20.551234] Authentication: DefaultAzureCredential: failed to acquire a token."
	authDetail := "\tManagedIdentityCredential: Identity not found"
	request := "[Sep  3 00:54:20.551234] Request: ==> OUTGOING REQUEST (Try=1)"
	requestDetail := "   GET https://management.azure.com/subscriptions/x"
	requestHeader := "   Authorization: REDACTED"
	pluginError := "failed to create azure client: incomplete environment variable configuration"
	// azcore ends each request and response event with an empty line; what
	// follows is the plugin's own output again, even when indented.
	pluginDetail := "\tcaused by: token request to login.microsoftonline.com failed"
	input := strings.Join([]string{zap, request, requestDetail, requestHeader, "", pluginDetail, auth, authDetail, pluginError}, "\n")

	var sunk []string
	var logs bytes.Buffer
	kept := &lastLogs{}
	forwardLogs(tflogtest.RootLogger(context.Background(), &logs), strings.NewReader(input), kept, func(line string) { sunk = append(sunk, line) })

	if got := logLevels(t, &logs, "azure Authentication:"); !slices.Equal(got, []string{"debug", "debug"}) {
		t.Errorf("Authentication events logged at %q, want both at debug", got)
	}

	if got, want := kept.GetLines(), []string{zap, pluginDetail, auth, authDetail, pluginError}; !slices.Equal(got, want) {
		t.Errorf("kept for the error excerpt = %q, want %q", got, want)
	}
	if want := []string{"Destroying agent infrastructure...", pluginDetail, pluginError}; !slices.Equal(sunk, want) {
		t.Errorf("progress = %q, want %q", sunk, want)
	}
}
