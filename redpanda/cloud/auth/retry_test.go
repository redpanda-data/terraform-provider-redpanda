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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func doGet(t *testing.T, cl *http.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return cl.Do(req)
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in     string
		want   time.Duration
		wantOK bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{"5", 5 * time.Second, true},
		{"3600", time.Hour, true},
		{"not-a-number", 0, false},
		{"-1", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parseRetryAfter(tc.in)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("parseRetryAfter(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestRetryClient_429WithShortRetryAfter(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := hits.Add(1)
		if n < 2 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "throttled", http.StatusTooManyRequests)
			return
		}
		fmt.Fprintln(w, "ok")
	}))
	defer srv.Close()

	cl := newRetryClient()
	resp, err := doGet(t, cl, srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("hits = %d, want 2 (1 retry)", got)
	}
}

func TestRetryClient_429WithLongRetryAfterFailsFast(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "99999")
		http.Error(w, "quota exhausted", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	cl := newRetryClient()
	resp, err := doGet(t, cl, srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected error when Retry-After exceeds max wait")
	}
	if !strings.Contains(err.Error(), "exceeds max wait") {
		t.Errorf("error %q should mention exceeds max wait", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("hits = %d, want 1 (fail-fast, no retry)", got)
	}
}

func TestRetryClient_5xxRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	}))
	defer srv.Close()

	cl := newRetryClient()
	resp, err := doGet(t, cl, srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := hits.Load(); got < 3 {
		t.Errorf("hits = %d, want >= 3", got)
	}
}

func TestRetryClient_4xxNoRetry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	cl := newRetryClient()
	resp, err := doGet(t, cl, srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if got := hits.Load(); got != 1 {
		t.Errorf("hits = %d, want 1 (no retry on 4xx)", got)
	}
}
