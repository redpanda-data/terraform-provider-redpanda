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
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckCLIConfig pins which Terraform CLI configs the provider-upgrade
// entry tolerates: a cache-only config is harmless, anything that can redirect
// provider installation would silently replace the released provider. With
// TF_CLI_CONFIG_FILE unset Terraform reads the default file and the config
// directory's *.tfrc files, so those are inspected too; when it is set they are
// ignored, as Terraform ignores them.
func TestCheckCLIConfig(t *testing.T) {
	if runtime.GOOS == windowsGOOS {
		t.Skip("default CLI config paths under test are the unix ones")
	}
	write := func(t *testing.T, dir, name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
		return p
	}
	const cacheOnly = "plugin_cache_dir = \"/tmp/cache\"\nplugin_cache_may_break_dependency_lock_file = true\n"
	const devOverrides = "provider_installation {\n  dev_overrides {\n    \"redpanda-data/redpanda\" = \"/src\"\n  }\n  direct {}\n}\n"
	const mirror = "provider_installation {\n  filesystem_mirror {\n    path = \"/mirror\"\n  }\n}\n"
	emptyHome := t.TempDir()
	dirtyDefaultHome := t.TempDir()
	write(t, dirtyDefaultHome, ".terraformrc", devOverrides)
	dirtyDirHome := t.TempDir()
	write(t, dirtyDirHome, ".terraformrc", cacheOnly)
	write(t, dirtyDirHome, filepath.Join(".terraform.d", "local.tfrc"), mirror)
	ignoredDirHome := t.TempDir()
	write(t, ignoredDirHome, filepath.Join(".terraform.d", "notes.txt"), devOverrides)
	cases := []struct {
		name     string
		override string
		home     string
		wantErr  string
	}{
		{"unset with no default files is fine", "", emptyHome, ""},
		{"cache-only override is fine", write(t, t.TempDir(), "cli.tfrc", cacheOnly), emptyHome, ""},
		{"dev_overrides in override rejected", write(t, t.TempDir(), "cli.tfrc", devOverrides), emptyHome, "dev_overrides"},
		{"provider_installation in override rejected", write(t, t.TempDir(), "cli.tfrc", mirror), emptyHome, "provider_installation"},
		{"unreadable override rejected", filepath.Join(t.TempDir(), "missing.tfrc"), emptyHome, "missing.tfrc"},
		{"dev_overrides in default file rejected", "", dirtyDefaultHome, ".terraformrc"},
		{"provider_installation in config dir tfrc rejected", "", dirtyDirHome, "local.tfrc"},
		{"non-tfrc files in the config dir are not read", "", ignoredDirHome, ""},
		{"override set ignores dirty defaults", write(t, t.TempDir(), "cli.tfrc", cacheOnly), dirtyDefaultHome, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkCLIConfig(tc.override, tc.home)
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

// TestImpliedMirrorDirs pins that the user's ~/.terraform.d/plugins is always
// among the checked dirs on every platform.
func TestImpliedMirrorDirs(t *testing.T) {
	home := t.TempDir()
	assert.Contains(t, impliedMirrorDirs(home), filepath.Join(home, ".terraform.d", "plugins"))
}

// TestInlineConfigFromDir_SkipsTerraformBlock pins that a directory's
// terraform block is left out of the inline step-0 config: the framework
// writes required_providers itself for inline configs, and a config that
// already carries a terraform block makes it skip that, silently dropping
// the released-version pin the upgrade entry exists to apply.
func TestInlineConfigFromDir_SkipsTerraformBlock(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("main.tf", "provider \"redpanda\" {}\n\nresource \"redpanda_resource_group\" \"t\" {\n  name = \"t\"\n}\n")
	write("versions.tf", "terraform {\n  required_providers {\n    redpanda = {\n      source = \"redpanda-data/redpanda\"\n    }\n  }\n}\n")
	got := inlineConfigFromDir(t, dir)
	if !strings.Contains(got, "resource \"redpanda_resource_group\"") {
		t.Fatalf("inline config lost main.tf:\n%s", got)
	}
	if strings.Contains(got, "terraform {") {
		t.Fatalf("inline config carries the directory's terraform block, which suppresses the framework's version pin:\n%s", got)
	}
}
