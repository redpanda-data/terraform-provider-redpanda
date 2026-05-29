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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type cacheEntry struct {
	AccessToken string    `json:"access_token,omitempty"`
	TokenType   string    `json:"token_type,omitempty"`
	Expiry      time.Time `json:"expiry,omitempty"`
}

type cacheFile struct {
	ServicePrincipals map[string]cacheEntry `json:"service-principals,omitempty"`
}

func newCacheFile() *cacheFile {
	return &cacheFile{ServicePrincipals: map[string]cacheEntry{}}
}

func readCache(path string) (*cacheFile, error) {
	cf := newCacheFile()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cf, nil
		}
		return cf, fmt.Errorf("read cache %q: %w", path, err)
	}
	if err := json.Unmarshal(data, cf); err != nil {
		return newCacheFile(), fmt.Errorf("unmarshal cache %q: %w", path, err)
	}
	if cf.ServicePrincipals == nil {
		cf.ServicePrincipals = map[string]cacheEntry{}
	}
	return cf, nil
}

func (c *cacheFile) write(path string) error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), folderMode); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.WriteFile(path, data, fileMode); err != nil {
		return fmt.Errorf("write cache %q: %w", path, err)
	}
	return nil
}

func (c *cacheFile) removeExpired() {
	now := time.Now()
	for k, entry := range c.ServicePrincipals {
		if entry.Expiry.Before(now) {
			delete(c.ServicePrincipals, k)
		}
	}
}
