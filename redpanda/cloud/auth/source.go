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
	"errors"
	"net/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// BuildTokenSource composes the layered TokenSource used to acquire
// access tokens for Redpanda Cloud. Layers (outermost first):
//
//  1. cachingTokenSource — on-disk cache keyed by audience:clientID.
//     Skipped entirely when REDPANDA_TOKEN_CACHE_DISABLE=1.
//  2. clientcredentials.Config.TokenSource — the standard client_credentials
//     fetch, using a retryablehttp client that handles transient 429s on
//     the token endpoint.
//
// The mutex inside cachingTokenSource serializes concurrent Token() calls
// in-process; the read-modify-write of the cache file gives cross-process
// eventual consistency.
//
// The ctx parameter is used only for construction-time logging; it is
// intentionally NOT propagated into the returned TokenSource because the
// TokenSource's lifetime exceeds any single request (provider Configure's
// ctx is canceled before resource RPCs fire, so capturing it here would
// cancel every subsequent token fetch). The HTTP client lookup is rooted
// on context.Background() instead.
func BuildTokenSource(ctx context.Context, tokenURL, audience, clientID, clientSecret string) (oauth2.TokenSource, error) {
	if clientID == "" {
		return nil, errors.New("client id is empty")
	}
	if clientSecret == "" {
		return nil, errors.New("client secret is empty")
	}
	if tokenURL == "" {
		return nil, errors.New("token url is empty")
	}

	cfg := clientcredentials.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TokenURL:       tokenURL,
		EndpointParams: url.Values{"audience": {audience}},
	}

	httpCl := newRetryClient()
	// Detach from the caller's ctx: the returned TokenSource outlives any single
	// request (e.g. provider Configure's ctx is canceled before resource RPCs
	// fire), so refresh fetches must root on context.Background().
	ctxWithClient := context.WithValue(context.Background(), oauth2.HTTPClient, httpCl)
	inner := cfg.TokenSource(ctxWithClient) //nolint:contextcheck // intentional detachment, see comment above

	key := cacheKey(audience, clientID)

	if cacheDisabled() {
		tflog.Debug(ctx, "token cache disabled via env; using uncached TokenSource", map[string]any{
			"key": key,
		})
		return inner, nil
	}

	path := cacheFilePath()
	if path == "" {
		tflog.Debug(ctx, "user cache dir unavailable; using uncached TokenSource", map[string]any{
			"key": key,
		})
		return inner, nil
	}

	tflog.Debug(ctx, "token cache enabled", map[string]any{
		"key":        key,
		"cache_file": path,
	})
	return &cachingTokenSource{
		cacheFile: path,
		key:       key,
		inner:     inner,
	}, nil
}

// BuildStaticTokenSource wraps a user-supplied access token in an
// oauth2.TokenSource. The zero Expiry value makes the token always valid;
// the cache layer is never consulted.
func BuildStaticTokenSource(token string) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
}
