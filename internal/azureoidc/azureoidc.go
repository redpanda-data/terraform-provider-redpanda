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

// Package azureoidc is a minimal OIDC issuer for exercising Azure workload
// identity federation locally: it keeps an RSA signing key, renders the
// discovery document and key set Entra fetches, and mints RS256 tokens.
package azureoidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateKeyPEM returns a new 2048-bit RSA private key as PKCS#8 PEM.
func GenerateKeyPEM() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParseKeyPEM parses a PKCS#8 PEM RSA private key.
func ParseKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is %T, want an RSA key", parsed)
	}
	return key, nil
}

// Discovery returns the OpenID configuration document for issuer. Entra
// reads jwks_uri from it to fetch the signing keys.
func Discovery(issuer string) ([]byte, error) {
	return json.MarshalIndent(map[string]any{
		"issuer":                                issuer,
		"jwks_uri":                              issuer + "/jwks.json",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}, "", "  ")
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func keyID(pub *rsa.PublicKey) string {
	sum := sha256.Sum256(pub.N.Bytes())
	return b64(sum[:])[:16]
}

// JWKS returns the public key set for key.
func JWKS(key *rsa.PrivateKey) ([]byte, error) {
	pub := &key.PublicKey
	return json.MarshalIndent(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": keyID(pub),
			"n":   b64(pub.N.Bytes()),
			"e":   b64(big.NewInt(int64(pub.E)).Bytes()),
		}},
	}, "", "  ")
}

// Claims are the token claims Entra checks on a federated assertion.
type Claims struct {
	Issuer   string
	Subject  string
	Audience string
	Lifetime time.Duration
}

// Mint returns a compact RS256 JWT for c, issued at now.
func Mint(key *rsa.PrivateKey, c Claims, now time.Time) (string, error) {
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID(&key.PublicKey)})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"iss": c.Issuer,
		"sub": c.Subject,
		"aud": c.Audience,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": now.Add(c.Lifetime).Unix(),
		"jti": hex.EncodeToString(jti),
	})
	if err != nil {
		return "", err
	}
	signingInput := b64(header) + "." + b64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + b64(sig), nil
}

// ActionsTokenHandler serves the GitHub Actions ID-token endpoint that azurerm
// calls through ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN:
// a GET with the requested audience in the query and the request token as a
// bearer, answered with {"value": <jwt>}. Each request mints a fresh token.
func ActionsTokenHandler(key *rsa.PrivateKey, c Claims, requestToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+requestToken {
			http.Error(w, "bad request token", http.StatusUnauthorized)
			return
		}
		audience := r.URL.Query().Get("audience")
		if audience == "" {
			http.Error(w, "audience is required", http.StatusBadRequest)
			return
		}
		claims := c
		claims.Audience = audience
		token, err := Mint(key, claims, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": token})
	})
}

// StaticActionsTokenHandler serves the GitHub Actions ID-token endpoint shape
// from an already minted token, so a runner can use it without the signing key.
// The token is re-read per request, so replacing the file refreshes it.
func StaticActionsTokenHandler(tokenPath, requestToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+requestToken {
			http.Error(w, "bad request token", http.StatusUnauthorized)
			return
		}
		raw, err := os.ReadFile(filepath.Clean(tokenPath))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		token := strings.TrimSpace(string(raw))
		audience, err := tokenAudience(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if want := r.URL.Query().Get("audience"); want == "" || want != audience {
			http.Error(w, fmt.Sprintf("token audience is %q, request asked for %q", audience, want), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": token})
	})
}

func tokenAudience(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("token is not a compact JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Aud string `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	return claims.Aud, nil
}
