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

package azureoidc

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const issuer = "https://issuer.example.test"

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	pemBytes, err := GenerateKeyPEM()
	if err != nil {
		t.Fatalf("GenerateKeyPEM: %v", err)
	}
	key, err := ParseKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParseKeyPEM: %v", err)
	}
	return key
}

func publicKeyFromJWKS(t *testing.T, raw []byte) (pub *rsa.PublicKey, kid string) {
	t.Helper()
	var set struct {
		Keys []struct {
			Kty, Use, Alg, Kid, N, E string
		} `json:"keys"`
	}
	if err := json.Unmarshal(raw, &set); err != nil || len(set.Keys) != 1 {
		t.Fatalf("JWKS = %s, want one key (%v)", raw, err)
	}
	k := set.Keys[0]
	if k.Kty != "RSA" || k.Alg != "RS256" || k.Use != "sig" || k.Kid == "" {
		t.Fatalf("JWK = %+v, want an RS256 signing key with a kid", k)
	}
	n, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		t.Fatal(err)
	}
	e, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		t.Fatal(err)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, k.Kid
}

func verify(t *testing.T, token string, pub *rsa.PublicKey) (header, claims map[string]any) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("signature does not verify against the published key: %v", err)
	}
	for i, dst := range []*map[string]any{&header, &claims} {
		raw, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			t.Fatal(err)
		}
	}
	return header, claims
}

func TestDiscoveryPointsAtTheKeySet(t *testing.T) {
	raw, err := Discovery(issuer)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("Discovery = %s: %v", raw, err)
	}
	if doc["issuer"] != issuer || doc["jwks_uri"] != issuer+"/jwks.json" {
		t.Fatalf("Discovery = %s, want issuer %q and jwks_uri %q", raw, issuer, issuer+"/jwks.json")
	}
}

func TestMintVerifiesAgainstThePublishedKey(t *testing.T) {
	key := testKey(t)
	jwks, err := JWKS(key)
	if err != nil {
		t.Fatal(err)
	}
	pub, kid := publicKeyFromJWKS(t, jwks)
	now := time.Unix(1_800_000_000, 0)
	token, err := Mint(key, Claims{Issuer: issuer, Subject: "tfrp-oidc-manual", Audience: "api://AzureADTokenExchange", Lifetime: time.Hour}, now)
	if err != nil {
		t.Fatal(err)
	}
	header, claims := verify(t, token, pub)
	if header["alg"] != "RS256" || header["kid"] != kid {
		t.Errorf("header = %v, want alg RS256 and kid %q", header, kid)
	}
	want := map[string]any{
		"iss": issuer,
		"sub": "tfrp-oidc-manual",
		"aud": "api://AzureADTokenExchange",
		"iat": float64(now.Unix()),
		"nbf": float64(now.Unix()),
		"exp": float64(now.Add(time.Hour).Unix()),
	}
	for k, v := range want {
		if claims[k] != v {
			t.Errorf("claim %s = %v, want %v", k, claims[k], v)
		}
	}
	if claims["jti"] == nil || claims["jti"] == "" {
		t.Error("claim jti is missing")
	}
}

func TestActionsTokenHandlerMintsForTheRequestedAudience(t *testing.T) {
	key := testKey(t)
	jwks, err := JWKS(key)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := publicKeyFromJWKS(t, jwks)
	h := ActionsTokenHandler(key, Claims{Issuer: issuer, Subject: "tfrp-oidc-manual", Lifetime: time.Hour}, "request-token")

	req := httptest.NewRequest(http.MethodGet, "/token?api-version=2.0&audience=api://AzureADTokenExchange", http.NoBody)
	req.Header.Set("Authorization", "Bearer request-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var body struct{ Value string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Value == "" {
		t.Fatalf("body = %s, want {\"value\": <jwt>} (%v)", rec.Body, err)
	}
	_, claims := verify(t, body.Value, pub)
	if claims["aud"] != "api://AzureADTokenExchange" || claims["sub"] != "tfrp-oidc-manual" {
		t.Errorf("claims = %v", claims)
	}

	unauth := httptest.NewRequest(http.MethodGet, "/token?audience=api://AzureADTokenExchange", http.NoBody)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, unauth)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status without the request token = %d, want 401", rec.Code)
	}
}

func TestStaticActionsTokenHandlerServesTheMintedToken(t *testing.T) {
	key := testKey(t)
	token, err := Mint(key, Claims{Issuer: issuer, Subject: "tfrp-oidc-manual", Audience: "api://AzureADTokenExchange", Lifetime: time.Hour}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := StaticActionsTokenHandler(path, "request-token")

	cases := []struct {
		name, bearer, audience string
		wantCode               int
	}{
		{"matching audience", "request-token", "api://AzureADTokenExchange", http.StatusOK},
		{"other audience", "request-token", "api://other", http.StatusBadRequest},
		{"no audience", "request-token", "", http.StatusBadRequest},
		{"wrong request token", "nope", "api://AzureADTokenExchange", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/token?api-version=2.0&audience="+tc.audience, http.NoBody)
			req.Header.Set("Authorization", "Bearer "+tc.bearer)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantCode, rec.Body)
			}
			if tc.wantCode != http.StatusOK {
				return
			}
			var body struct{ Value string }
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Value != token {
				t.Errorf("body = %s, want the minted token (%v)", rec.Body, err)
			}
		})
	}
}
