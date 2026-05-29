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
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2"
)

const minTTL = 15 * time.Second

var mu sync.Mutex

type cachingTokenSource struct {
	cacheFile string
	key       string
	inner     oauth2.TokenSource
}

func (s *cachingTokenSource) Token() (*oauth2.Token, error) {
	mu.Lock()
	defer mu.Unlock()

	ctx := context.Background()
	tflog.Trace(ctx, "token cache lookup", map[string]any{
		"cache_file": s.cacheFile,
		"key":        s.key,
	})

	cf, err := readCache(s.cacheFile)
	if err != nil {
		tflog.Debug(ctx, "token cache read failed; falling back to empty cache", map[string]any{
			"cache_file": s.cacheFile,
			"error":      err.Error(),
		})
		cf = newCacheFile()
	}
	cf.removeExpired()

	var hit *cacheEntry
	if entry, ok := cf.ServicePrincipals[s.key]; ok {
		hit = &entry
	}

	token, fromCache, err := s.getValidToken(ctx, hit)
	if err != nil {
		return nil, fmt.Errorf("acquire token: %w", err)
	}

	if fromCache {
		return token, nil
	}

	cf.ServicePrincipals[s.key] = cacheEntry{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		Expiry:      token.Expiry,
	}
	if err := cf.write(s.cacheFile); err != nil {
		tflog.Warn(ctx, "token cache write failed; token returned without persistence", map[string]any{
			"cache_file": s.cacheFile,
			"error":      err.Error(),
		})
	} else {
		tflog.Trace(ctx, "token cache write succeeded", map[string]any{
			"cache_file": s.cacheFile,
			"key":        s.key,
			"expiry":     token.Expiry.UTC().Format(time.RFC3339),
		})
	}

	return token, nil
}

func (s *cachingTokenSource) getValidToken(ctx context.Context, hit *cacheEntry) (*oauth2.Token, bool, error) {
	now := time.Now()
	if hit != nil && hit.Expiry.After(now.Add(minTTL)) {
		tflog.Trace(ctx, "token cache hit", map[string]any{
			"key":               s.key,
			"expiry":            hit.Expiry.UTC().Format(time.RFC3339),
			"seconds_remaining": int(time.Until(hit.Expiry).Seconds()),
		})
		return &oauth2.Token{
			AccessToken: hit.AccessToken,
			TokenType:   hit.TokenType,
			Expiry:      hit.Expiry,
		}, true, nil
	}

	reason := "not_present"
	if hit != nil {
		reason = "expiring_within_minTTL"
	}
	tflog.Trace(ctx, "token cache miss; fetching fresh token", map[string]any{
		"key":    s.key,
		"reason": reason,
	})
	tok, err := s.inner.Token()
	if err != nil {
		return nil, false, err
	}
	tflog.Trace(ctx, "fresh token fetched", map[string]any{
		"key":    s.key,
		"expiry": tok.Expiry.UTC().Format(time.RFC3339),
	})
	return tok, false, nil
}
