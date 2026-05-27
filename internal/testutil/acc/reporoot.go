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

// Package acc provides live-acceptance test helpers shared across resource packages.
package acc

import (
	"os"
	"path/filepath"
	"runtime"
)

// RepoRoot returns the absolute path to the repository root by walking up
// from this file's location until go.mod is found. Panics if go.mod is
// not located before reaching the filesystem root.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("acc: cannot find repository root (go.mod not found)")
		}
		dir = parent
	}
}
