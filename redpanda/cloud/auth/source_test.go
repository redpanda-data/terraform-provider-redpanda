// Copyright 2026 Redpanda Data, Inc.
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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func newTokenServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("issued-%d", hits.Load()),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func TestBuildTokenSource_CacheDisabled_FetchesEachConstruction(t *testing.T) {
	t.Setenv(cacheDisableEnv, "1")
	var hits atomic.Int32
	srv := newTokenServer(t, &hits)
	defer srv.Close()

	ts, err := BuildTokenSource(context.Background(), srv.URL, "test-audience", "client", "secret")
	if err != nil {
		t.Fatalf("BuildTokenSource: %v", err)
	}
	tok1, err := ts.Token()
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}
	if tok1.AccessToken == "" {
		t.Error("empty access token")
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("hits after first Token = %d, want 1", got)
	}
}

func TestBuildTokenSource_CrossInstanceServedFromDisk(t *testing.T) {
	dir := t.TempDir()
	// os.UserCacheDir derives from HOME on darwin and XDG_CACHE_HOME/HOME on linux;
	// override both so the test never touches the real user cache.
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv(cacheDisableEnv, "")

	var hits atomic.Int32
	srv := newTokenServer(t, &hits)
	defer srv.Close()

	ts1, err := BuildTokenSource(context.Background(), srv.URL, "test-audience", "client", "secret")
	if err != nil {
		t.Fatalf("BuildTokenSource 1: %v", err)
	}
	if _, err := ts1.Token(); err != nil {
		t.Fatalf("ts1 Token: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("hits after first Token = %d, want 1", got)
	}

	ts2, err := BuildTokenSource(context.Background(), srv.URL, "test-audience", "client", "secret")
	if err != nil {
		t.Fatalf("BuildTokenSource 2: %v", err)
	}
	if _, err := ts2.Token(); err != nil {
		t.Fatalf("ts2 Token: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("second instance hits = %d, want 1 (served from disk)", got)
	}
}

func TestBuildTokenSource_ValidatesInputs(t *testing.T) {
	ctx := context.Background()
	if _, err := BuildTokenSource(ctx, "http://x", "aud", "", "secret"); err == nil {
		t.Error("expected error for empty client id")
	}
	if _, err := BuildTokenSource(ctx, "http://x", "aud", "client", ""); err == nil {
		t.Error("expected error for empty client secret")
	}
	if _, err := BuildTokenSource(ctx, "", "aud", "client", "secret"); err == nil {
		t.Error("expected error for empty token url")
	}
}

func TestBuildStaticTokenSource(t *testing.T) {
	ts := BuildStaticTokenSource("static-tok")
	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "static-tok" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "static-tok")
	}
}
