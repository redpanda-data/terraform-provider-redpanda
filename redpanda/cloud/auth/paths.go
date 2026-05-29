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

// Package auth builds the oauth2.TokenSource chain used by the provider to
// authenticate to Redpanda Cloud. The chain layers a disk cache over the
// standard client_credentials flow so token endpoint calls stay rare across
// terraform invocations.
package auth

import (
	"os"
	"path/filepath"
)

const (
	cacheSubdir     = "redpanda/provider"
	cacheFilename   = "creds-cache.json"
	cacheDisableEnv = "REDPANDA_TOKEN_CACHE_DISABLE"

	folderMode os.FileMode = 0o750
	fileMode   os.FileMode = 0o600
)

func cacheFilePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, cacheSubdir, cacheFilename)
}

func cacheKey(audience, clientID string) string {
	return audience + ":" + clientID
}

func cacheDisabled() bool {
	return os.Getenv(cacheDisableEnv) == "1"
}
