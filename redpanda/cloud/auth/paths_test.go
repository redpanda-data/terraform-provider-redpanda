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
	"strings"
	"testing"
)

func TestCacheFilePath(t *testing.T) {
	got := cacheFilePath()
	if !strings.Contains(got, "redpanda/provider/creds-cache.json") {
		t.Errorf("path %q missing expected suffix redpanda/provider/creds-cache.json", got)
	}
}

func TestCacheKey(t *testing.T) {
	got := cacheKey("cloudv2-preprod.redpanda.cloud", "client-abc")
	want := "cloudv2-preprod.redpanda.cloud:client-abc"
	if got != want {
		t.Errorf("cacheKey = %q, want %q", got, want)
	}
}

func TestCacheKeyDisambiguatesAudience(t *testing.T) {
	dev := cacheKey("cloudv2-dev.redpanda.cloud", "shared-id")
	prod := cacheKey("cloudv2-production.redpanda.cloud", "shared-id")
	if dev == prod {
		t.Errorf("expected dev/prod keys to differ; both = %q", dev)
	}
}

func TestCacheDisabled(t *testing.T) {
	t.Setenv(cacheDisableEnv, "")
	if cacheDisabled() {
		t.Error("cacheDisabled() = true with env unset, want false")
	}
	t.Setenv(cacheDisableEnv, "1")
	if !cacheDisabled() {
		t.Error("cacheDisabled() = false with env=1, want true")
	}
	t.Setenv(cacheDisableEnv, "0")
	if cacheDisabled() {
		t.Error("cacheDisabled() = true with env=0, want false")
	}
}
