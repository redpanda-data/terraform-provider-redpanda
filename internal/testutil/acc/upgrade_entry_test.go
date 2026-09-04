// Copyright 2026 Redpanda Data, Inc.
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

package acc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckCLIConfig pins which Terraform CLI configs the provider-upgrade
// entry tolerates: a cache-only config is harmless, anything that can redirect
// provider installation would silently replace the released provider.
func TestCheckCLIConfig(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "cli.tfrc")
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
		return p
	}
	cases := []struct {
		name    string
		path    string
		wantErr string
	}{
		{"unset is fine", "", ""},
		{"cache-only config is fine", write(t, "plugin_cache_dir = \"/tmp/cache\"\nplugin_cache_may_break_dependency_lock_file = true\n"), ""},
		{"dev_overrides rejected", write(t, "provider_installation {\n  dev_overrides {\n    \"redpanda-data/redpanda\" = \"/src\"\n  }\n  direct {}\n}\n"), "dev_overrides"},
		{"provider_installation rejected", write(t, "provider_installation {\n  filesystem_mirror {\n    path = \"/mirror\"\n  }\n}\n"), "provider_installation"},
		{"unreadable path rejected", filepath.Join(t.TempDir(), "missing.tfrc"), "missing.tfrc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkCLIConfig(tc.path)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestStaleImpliedMirror pins the detection of a locally installed
// redpanda-data/redpanda package in one of Terraform's implied filesystem
// mirror directories, which shadows the registry for step 0 without a message.
func TestStaleImpliedMirror(t *testing.T) {
	empty := t.TempDir()
	withMirror := t.TempDir()
	pkg := filepath.Join(withMirror, "registry.terraform.io", "redpanda-data", "redpanda", "1.9.0", "darwin_arm64")
	require.NoError(t, os.MkdirAll(pkg, 0o750))

	t.Run("no mirror", func(t *testing.T) {
		_, found := staleImpliedMirror([]string{empty, filepath.Join(empty, "absent")})
		assert.False(t, found)
	})
	t.Run("mirror found reports its path", func(t *testing.T) {
		got, found := staleImpliedMirror([]string{empty, withMirror})
		require.True(t, found)
		assert.Equal(t, filepath.Join(withMirror, "registry.terraform.io", "redpanda-data", "redpanda"), got)
	})
}

// TestImpliedMirrorDirs pins that the user's ~/.terraform.d/plugins, the
// directory task build:install writes to, is always among the checked dirs.
func TestImpliedMirrorDirs(t *testing.T) {
	home := t.TempDir()
	assert.Contains(t, impliedMirrorDirs(home), filepath.Join(home, ".terraform.d", "plugins"))
}
