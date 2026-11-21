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
	"slices"
	"strings"
	"testing"
)

func TestRedactor_String(t *testing.T) {
	jwt := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJhZ2VudCJ9.c2lnbmF0dXJlLWJ5dGVz"
	r := newRedactor("held-secret-value", "held-secret-value-longer", "short", "")
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"held value", "using held-secret-value now", "using [REDACTED] now"},
		{"longer held value replaced whole", "x held-secret-value-longer y", "x [REDACTED] y"},
		{"value under the minimum length is not held", "a short line", "a short line"},
		{"JWT", "token " + jwt + " expires", "token [REDACTED] expires"},
		{"unsigned JWT", "eyJhbGciOiJub25lIn0.eyJleHAiOjF9.", "[REDACTED]"},
		{"bearer token", "Authorization: Bearer abcdefgh12345678", "Authorization: Bearer [REDACTED]"},
		{"lower-case bearer", "authorization: bearer abcdefgh12345678", "authorization: bearer [REDACTED]"},
		{"PEM key on one line", `"private_key": "-----BEGIN PRIVATE KEY-----\nMIIBVQ\n-----END PRIVATE KEY-----\n"`, `"private_key": "[REDACTED]\n"`},
		{"PEM key with no END", "key -----BEGIN RSA PRIVATE KEY----- MIIEowIBAAKCAQEAkeymaterialkeymaterialkeymaterial", "key [REDACTED]"},
		{"HCL assignment", `        token = "opaque-agent-token-value"`, `        token = "[REDACTED]"`},
		{"suffixed HCL key", `join_token = "yZuubjTmwJPghW8z"`, `join_token = "[REDACTED]"`},
		{"JSON key", `{"client_secret": "abc123def456"}`, `{"client_secret": "[REDACTED]"}`},
		{"escaped JSON key", `{\"api_key\":\"abc123def456\"}`, `{\"api_key\":\"[REDACTED]\"}`},
		{"YAML key", `password: "abc123def456"`, `password: "[REDACTED]"`},

		{"resource id", "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg", "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg"},
		{"blob URL", "[id=https://rpagent0001.blob.core.windows.net/agent/redpanda-agent.toml]", "[id=https://rpagent0001.blob.core.windows.net/agent/redpanda-agent.toml]"},
		{"hash", "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
		{"base64 that is not a JWT", "dGhpcyBpcyBub3QgYSB0b2tlbg==", "dGhpcyBpcyBub3QgYSB0b2tlbg=="},
		{"one base64 JSON segment", "eyJraW5kIjoibm90LWEtand0In0=", "eyJraW5kIjoibm90LWEtand0In0="},
		{"dotted version", "eyJ.1.2", "eyJ.1.2"},
		{"property of a token", `token_lifetime = "5m"`, `token_lifetime = "5m"`},
		{"token file path", `token_file = "/var/run/secrets/token"`, `token_file = "/var/run/secrets/token"`},
		{"short assigned value", `token = "unset"`, `token = "unset"`},
		{"bearer word alone", "the bearer scheme", "the bearer scheme"},
		{"PEM public key", "-----BEGIN PUBLIC KEY-----", "-----BEGIN PUBLIC KEY-----"},
		{"redacted value is not re-wrapped", `token = "[REDACTED]"`, `token = "[REDACTED]"`},
		{"camelCase key", `"privateKey": "opaqueValue12345678"`, `"privateKey": "[REDACTED]"`},
		{"camelCase api key", `"apiKey": "opaqueValue12345678"`, `"apiKey": "[REDACTED]"`},
		{"hyphenated key", `api-key: "opaqueValue12345678"`, `api-key: "[REDACTED]"`},
		{"credentials key", `"credentials": "opaqueValue12345678"`, `"credentials": "[REDACTED]"`},
		{"access key", `access_key = "opaqueValue12345678"`, `access_key = "[REDACTED]"`},
		{"single-quoted value", `token: 'opaqueValue12345678'`, `token: '[REDACTED]'`},
		{"plan variable value", `"prometheus_password":{"value":"Xq7pLm2vR9sTk4Wz"}`, `"prometheus_password":{"value":"[REDACTED]"}`},
		{"escaped plan variable value", `\"log_sink_api_key\":{\"value\":\"glcSynthKey0123456789\"}`, `\"log_sink_api_key\":{\"value\":\"[REDACTED]\"}`},
		{
			"JSON plan dump in a log line",
			"2026-11-20T10:00:00Z\tERROR\tbyoc\tplanner.go:977\tforbidden deletion in the plan\t{\"plan\": \"{\\\"resource_changes\\\":[{\\\"before\\\":{\\\"value\\\":\\\"kvSynthSecretValue98765\\\"}}]}\"}",
			"2026-11-20T10:00:00Z\tERROR\tbyoc\tplanner.go:977\tforbidden deletion in the plan\t{\"plan\":\"[REDACTED] (Terraform plan JSON)\"}",
		},
		{"bearer then a word", "unsupported bearer authentication scheme", "unsupported bearer authentication scheme"},
		{"bearer then credentials", "invalid bearer credentials", "invalid bearer credentials"},
		{"PEM marker quoted in a diagnostic", `Error: invalid key: expected "-----BEGIN RSA PRIVATE KEY-----" header`, `Error: invalid key: expected "[REDACTED]" header`},
		{"property of a token with a long value", `token_lifetime = "3600seconds"`, `token_lifetime = "3600seconds"`},
		{"non-ASCII text survives", "café crème brûlée déjà vu", "café crème brûlée déjà vu"},
		{"apostrophe inside a quoted value", `password = "ab'cdefghijklmnop"`, `password = "[REDACTED]"`},
		{"escaped quote inside a quoted value", `"password": "abcd\"efghijklmnop"`, `"password": "[REDACTED]"`},
		{"unquoted key=value", "token=abcdefghijk1234567 rest", "token=[REDACTED] rest"},
		{"unquoted short property", "token: expired", "token: expired"},
		{"certificate assignment", `client_certificate = "MIIKsyntheticPkcs12BundleBase64Value"`, `client_certificate = "[REDACTED]"`},
		{"JWT glued to a word", "token_eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.c2ln", "token_[REDACTED]"},
		{"PEM marker then a word", "failed to parse -----BEGIN PRIVATE KEY----- header", "failed to parse [REDACTED] header"},
		{
			"plan dump keeps the address of the forbidden change",
			"t\tERROR\tbyoc\tplanner.go:977\tforbidden deletion in the plan\t{\"change_address\": \"azurerm_key_vault_secret.x\", \"plan\": \"{\\\"resource_changes\\\":[]}\"}",
			"t\tERROR\tbyoc\tplanner.go:977\tforbidden deletion in the plan\t{\"change_address\":\"azurerm_key_vault_secret.x\",\"plan\":\"[REDACTED] (Terraform plan JSON)\"}",
		},
		{"a diagnostic naming a plan key", `Error: invalid plan: missing "prior_state"`, `Error: invalid plan: missing "prior_state"`},
		{"a value inside the first zap fields", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.c2ln\tINFO\tbyoc\tf.go:1\tmessage", "[REDACTED]\tINFO\tbyoc\tf.go:1\tmessage"},
		{"a held value inside the first zap fields", "held-secret-value\tINFO\tbyoc\tf.go:1\tmessage", "[REDACTED]\tINFO\tbyoc\tf.go:1\tmessage"},
		{"Terraform's sensitive placeholder", "  + client_secret = (sensitive value)", "  + client_secret = (sensitive value)"},
		{"Terraform's unknown placeholder", "  + client_secret = (known after apply)", "  + client_secret = (known after apply)"},
		{"an azidentity error chain", "ManagedIdentityCredential: ManagedIdentityCredential authentication failed", "ManagedIdentityCredential: ManagedIdentityCredential authentication failed"},
		{"updated value after an arrow", `~ client_secret = "oldSecretValue123" -> "newSecretValue456"`, `~ client_secret = "[REDACTED]" -> "[REDACTED]"`},
		{
			"patterns before a plan dump",
			"x\tERROR\tbyoc\tp.go:1\tAuthorization: Bearer abcdefgh12345678\t{\"plan\": \"{\\\"resource_changes\\\":[]}\"}",
			"x\tERROR\tbyoc\tp.go:1\tAuthorization: Bearer [REDACTED]\t{\"plan\":\"[REDACTED] (Terraform plan JSON)\"}",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.String(c.in); got != c.want {
				t.Errorf("String(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRedactor_LinesRemovesAMultiLinePEMKey(t *testing.T) {
	in := []string{
		"before",
		"-----BEGIN RSA PRIVATE KEY-----",
		"MIIEowIBAAKCAQEAkeymaterialkeymaterialkeymaterialkeymaterial0123",
		"Mor3KeyMaterialHere==",
		"-----END RSA PRIVATE KEY----- after",
		"MIIEowIBAAKCAQEA is ordinary once the key ends",
	}
	want := []string{
		"before",
		"[REDACTED]",
		"[REDACTED]",
		"[REDACTED]",
		"[REDACTED] after",
		"MIIEowIBAAKCAQEA is ordinary once the key ends",
	}
	redact := newRedactor().Lines()
	var got []string
	for _, l := range in {
		got = append(got, redact(l))
	}
	if !slices.Equal(got, want) {
		t.Errorf("Lines = %q, want %q", got, want)
	}
}

func TestRedactor_LinesRecoversAfterAnUnterminatedPEMMarker(t *testing.T) {
	in := []string{
		"could not parse key: want -----BEGIN PRIVATE KEY----- block",
		"",
		"Error: apply failed: quota exceeded",
	}
	redact := newRedactor().Lines()
	var got []string
	for _, l := range in {
		got = append(got, redact(l))
	}
	if got[1] != "" || got[2] != in[2] {
		t.Errorf("Lines = %q, want the blank line and the failure line kept", got)
	}
}

func TestRedactor_LinesBoundsThePEMBody(t *testing.T) {
	redact := newRedactor().Lines()
	redact("could not parse key, expected -----BEGIN PRIVATE KEY-----")
	for _, l := range []string{"Terminating", "Error: failed_precondition: agent token is not yet provisioned; please retry later", ""} {
		if got := redact(l); got != l {
			t.Errorf("%q after an unterminated marker = %q, want it kept", l, got)
		}
	}
	redact = newRedactor().Lines()
	redact("-----BEGIN PRIVATE KEY-----")
	body := strings.Repeat("A", 64)
	for i := range maxPEMLines {
		if got := redact(body); got != redacted {
			t.Fatalf("key line %d = %q, want it redacted", i, got)
		}
	}
	if got := redact(body); got != body {
		t.Errorf("line %d past the bound = %q, want the key state ended", maxPEMLines, got)
	}
}

func TestRedactor_LinesRedactsAPEMKeyInAPlanDiff(t *testing.T) {
	in := []string{
		"          - -----BEGIN RSA PRIVATE KEY-----",
		"          - MIIEowIBAAKCAQEAkeymaterialkeymaterialkeymaterialkeymat",
		"          - Ow==",
		"          - -----END RSA PRIVATE KEY-----",
	}
	redact := newRedactor().Lines()
	for _, l := range in {
		if got := redact(l); strings.Contains(got, "MIIEow") || strings.Contains(got, "Ow==") {
			t.Errorf("%q survived as %q", l, got)
		}
	}
}

func TestRedactor_HoldsEachLineOfAMultiLineValue(t *testing.T) {
	key := "{\n  \"private_key_id\": \"0123456789abcdef0123456789abcdef01234567\",\n  \"client_email\": \"tfrp@tfrp-test.iam.gserviceaccount.com\"\n}"
	redact := newRedactor(key).Lines()
	for _, l := range strings.Split(key, "\n") {
		if got := redact(l); strings.Contains(got, "0123456789abcdef") || strings.Contains(got, "tfrp@") {
			t.Errorf("line %q survived as %q", l, got)
		}
	}
}

func TestNewSubprocessRedactor(t *testing.T) {
	args := []string{
		"gcp", "apply",
		"--api-key", "hyphenated-api-key-value",
		"--cloud-api-token", "cloud-token-value",
		"--client-secret=client-secret-value",
		"--credential-source", "workload",
		"--redpanda-id", "cluster-id-value",
		"--debug",
	}
	env := []string{
		"AZURE_CLIENT_SECRET=azure-secret-value",
		"ARM_CLIENT_CERTIFICATE=cGtjczEyLWJ1bmRsZQ==",
		"REDPANDA_CLIENT_SECRET=redpanda-secret-value",
		"AWS_SESSION_TOKEN=aws-session-token-value",
		"ARM_OIDC_TOKEN_FILE_PATH=/tmp/federated-token",
		"AZURE_TOKEN_CREDENTIALS=WorkloadIdentityCredential",
		"GOOGLE_APPLICATION_CREDENTIALS=/tmp/terraform-provider-redpanda-gcp/creds.json",
		"AWS_ACCESS_KEY_ID=AKIAEXAMPLEKEYID",
		"GOOGLE_CLOUD_KEYFILE_JSON=legacy-keyfile-json-value",
		"AWS_CREDENTIAL_EXPIRATION=2026-11-20T13:45:00Z",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI=http://169.254.170.23/v1/credentials",
		"CREDENTIALS_DIRECTORY=/run/credentials/ci-runner.service",
		"CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE=/home/runner/.config/gcloud/sa.json",
		"CLOUD_URL=https://cloud.example.invalid/api/v1",
	}
	r := newSubprocessRedactor(args, env, "held-google-key-json")
	for _, secret := range []string{
		"cloud-token-value", "client-secret-value", "azure-secret-value", "cGtjczEyLWJ1bmRsZQ==", "hyphenated-api-key-value",
		"redpanda-secret-value", "aws-session-token-value", "held-google-key-json", "legacy-keyfile-json-value",
	} {
		if got := r.String("x " + secret + " y"); strings.Contains(got, secret) {
			t.Errorf("%s survived: %q", secret, got)
		}
	}
	for _, kept := range []string{
		"workload", "cluster-id-value", "/tmp/federated-token", "WorkloadIdentityCredential",
		"/tmp/terraform-provider-redpanda-gcp/creds.json", "AKIAEXAMPLEKEYID", "https://cloud.example.invalid/api/v1",
		"2026-11-20T13:45:00Z", "http://169.254.170.23/v1/credentials", "/run/credentials/ci-runner.service", "/home/runner/.config/gcloud/sa.json",
	} {
		if got := r.String("x " + kept + " y"); got != "x "+kept+" y" {
			t.Errorf("%s was redacted: %q", kept, got)
		}
	}
}
