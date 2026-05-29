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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCache_FileMissing(t *testing.T) {
	cf, err := readCache(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("readCache on missing file: %v", err)
	}
	if cf == nil || cf.ServicePrincipals == nil {
		t.Fatalf("readCache returned nil cache or nil map: %#v", cf)
	}
	if len(cf.ServicePrincipals) != 0 {
		t.Errorf("missing file should yield empty map, got %d entries", len(cf.ServicePrincipals))
	}
}

func TestReadCache_CorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds-cache.json")
	if err := os.WriteFile(path, []byte("not json"), fileMode); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}
	cf, err := readCache(path)
	if err == nil {
		t.Error("readCache on corrupt file should return error")
	}
	if cf == nil || cf.ServicePrincipals == nil {
		t.Fatalf("corrupt readCache should still return initialized cache, got %#v", cf)
	}
}

func TestCacheFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub1", "sub2", "creds-cache.json")
	cf := newCacheFile()
	cf.ServicePrincipals["aud:client"] = cacheEntry{
		AccessToken: "tok-xyz",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour).UTC(),
	}
	if err := cf.write(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readCache(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	entry, ok := got.ServicePrincipals["aud:client"]
	if !ok {
		t.Fatal("missing entry after round trip")
	}
	if entry.AccessToken != "tok-xyz" || entry.TokenType != "Bearer" {
		t.Errorf("round-trip mismatch: %#v", entry)
	}
}

func TestCacheFileWriteMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds-cache.json")
	cf := newCacheFile()
	if err := cf.write(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != fileMode {
		t.Errorf("file mode = %o, want %o", got, fileMode)
	}
}

func TestRemoveExpired(t *testing.T) {
	cf := newCacheFile()
	cf.ServicePrincipals["expired"] = cacheEntry{Expiry: time.Now().Add(-time.Hour)}
	cf.ServicePrincipals["valid"] = cacheEntry{Expiry: time.Now().Add(time.Hour)}
	cf.removeExpired()
	if _, ok := cf.ServicePrincipals["expired"]; ok {
		t.Error("expired entry not removed")
	}
	if _, ok := cf.ServicePrincipals["valid"]; !ok {
		t.Error("valid entry incorrectly removed")
	}
}
