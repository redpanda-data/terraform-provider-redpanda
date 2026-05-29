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
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type countingSource struct {
	mu     sync.Mutex
	count  int64
	tokens []*oauth2.Token
	err    error
}

func (c *countingSource) Token() (*oauth2.Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	atomic.AddInt64(&c.count, 1)
	if c.err != nil {
		return nil, c.err
	}
	if len(c.tokens) == 0 {
		return &oauth2.Token{AccessToken: "default", Expiry: time.Now().Add(time.Hour)}, nil
	}
	t := c.tokens[0]
	if len(c.tokens) > 1 {
		c.tokens = c.tokens[1:]
	}
	return t, nil
}

func (c *countingSource) calls() int64 {
	return atomic.LoadInt64(&c.count)
}

func newCachingSource(t *testing.T, inner oauth2.TokenSource) *cachingTokenSource {
	t.Helper()
	return &cachingTokenSource{
		cacheFile: filepath.Join(t.TempDir(), "creds-cache.json"),
		key:       "aud:client",
		inner:     inner,
	}
}

func TestCachingTokenSource_FetchWritesAndReuses(t *testing.T) {
	inner := &countingSource{tokens: []*oauth2.Token{
		{AccessToken: "first", Expiry: time.Now().Add(time.Hour)},
	}}
	src := newCachingSource(t, inner)

	tok1, err := src.Token()
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}
	if tok1.AccessToken != "first" {
		t.Errorf("first token mismatch: %q", tok1.AccessToken)
	}
	if got := inner.calls(); got != 1 {
		t.Fatalf("inner calls after first Token = %d, want 1", got)
	}

	tok2, err := src.Token()
	if err != nil {
		t.Fatalf("second Token: %v", err)
	}
	if tok2.AccessToken != "first" {
		t.Errorf("second token should be cached %q, got %q", "first", tok2.AccessToken)
	}
	if got := inner.calls(); got != 1 {
		t.Errorf("inner calls after second Token = %d, want 1 (served from cache)", got)
	}
}

func TestCachingTokenSource_StaleTriggersRefresh(t *testing.T) {
	inner := &countingSource{tokens: []*oauth2.Token{
		{AccessToken: "stale", Expiry: time.Now().Add(5 * time.Second)},
		{AccessToken: "fresh", Expiry: time.Now().Add(time.Hour)},
	}}
	src := newCachingSource(t, inner)

	tok1, err := src.Token()
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}
	if tok1.AccessToken != "stale" {
		t.Errorf("first token mismatch: %q", tok1.AccessToken)
	}

	tok2, err := src.Token()
	if err != nil {
		t.Fatalf("second Token: %v", err)
	}
	if tok2.AccessToken != "fresh" {
		t.Errorf("second token should refresh past minTTL, got %q", tok2.AccessToken)
	}
	if got := inner.calls(); got != 2 {
		t.Errorf("inner calls = %d, want 2 (one initial + one refresh)", got)
	}
}

func TestCachingTokenSource_CrossInstanceServedFromDisk(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "creds-cache.json")

	inner1 := &countingSource{tokens: []*oauth2.Token{
		{AccessToken: "tok-A", Expiry: time.Now().Add(time.Hour)},
	}}
	src1 := &cachingTokenSource{cacheFile: cachePath, key: "aud:client", inner: inner1}
	if _, err := src1.Token(); err != nil {
		t.Fatalf("src1 Token: %v", err)
	}

	inner2 := &countingSource{tokens: []*oauth2.Token{
		{AccessToken: "tok-B", Expiry: time.Now().Add(time.Hour)},
	}}
	src2 := &cachingTokenSource{cacheFile: cachePath, key: "aud:client", inner: inner2}
	tok, err := src2.Token()
	if err != nil {
		t.Fatalf("src2 Token: %v", err)
	}
	if tok.AccessToken != "tok-A" {
		t.Errorf("src2 should read tok-A from disk, got %q", tok.AccessToken)
	}
	if got := inner2.calls(); got != 0 {
		t.Errorf("src2 inner should not be called (cache hit), got %d", got)
	}
}

func TestCachingTokenSource_ConcurrentCoalesce(t *testing.T) {
	inner := &countingSource{tokens: []*oauth2.Token{
		{AccessToken: "shared", Expiry: time.Now().Add(time.Hour)},
	}}
	src := newCachingSource(t, inner)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			if _, err := src.Token(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("goroutine: %v", err)
	}
	if got := inner.calls(); got != 1 {
		t.Errorf("inner calls = %d, want 1 (mutex coalesces concurrent fetches)", got)
	}
}

func TestCachingTokenSource_InnerError(t *testing.T) {
	inner := &countingSource{err: errors.New("upstream down")}
	src := newCachingSource(t, inner)
	if _, err := src.Token(); err == nil {
		t.Error("expected error from inner.Token()")
	}
}
