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

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeygenPublishMint(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "signing-key.pem")
	site := filepath.Join(dir, "site")
	token := filepath.Join(dir, "token")
	const issuer = "https://issuer.example.test"

	steps := [][]string{
		{"keygen", "-out", key},
		{"publish", "-key", key, "-issuer", issuer, "-out", site},
		{"mint", "-key", key, "-issuer", issuer, "-subject", "tfrp-oidc-manual", "-out", token},
	}
	for _, args := range steps {
		if err := run(args, &bytes.Buffer{}); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, secret := range []string{key, token} {
		info, err := os.Stat(secret)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want 0600", secret, info.Mode().Perm())
		}
	}
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(site, ".well-known", "openid-configuration")))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc["issuer"] != issuer {
		t.Errorf("discovery = %s (%v)", raw, err)
	}
	if _, err := os.Stat(filepath.Join(site, "jwks.json")); err != nil {
		t.Error(err)
	}
	jwt, err := os.ReadFile(filepath.Clean(token))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(jwt), ".") != 2 {
		t.Errorf("token file holds %q, want a compact JWT", jwt)
	}
	if err := run([]string{"keygen", "-out", key}, &bytes.Buffer{}); err == nil {
		t.Error("keygen overwrote an existing key")
	}
}
