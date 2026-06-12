// Copyright 2025 Redpanda Data, Inc.
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

// Package cmdutil holds helpers shared by the codegen binaries under cmd/.
package cmdutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/bufdeps"
)

// FindRepoRoot walks up from the current working directory to the first
// directory containing a go.mod and returns its path.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found walking up from cwd")
		}
		dir = parent
	}
}

// ResolveCloudv2Root locates a cloudv2 checkout — a directory containing
// proto/public/cloud — preferring the flag value, then $CLOUDV2_ROOT, then
// common relative paths. Returns "" if none validate.
func ResolveCloudv2Root(flagValue string) string {
	if flagValue != "" {
		if _, err := os.Stat(filepath.Join(flagValue, "proto", "public", "cloud")); err == nil {
			return flagValue
		}
		log.Printf("WARNING: -cloudv2 %s does not contain proto/public/cloud", flagValue)
	}

	if envRoot := os.Getenv("CLOUDV2_ROOT"); envRoot != "" {
		if _, err := os.Stat(filepath.Join(envRoot, "proto", "public", "cloud")); err == nil {
			log.Printf("Using cloudv2 from CLOUDV2_ROOT: %s", envRoot)
			return envRoot
		}
	}

	for _, rel := range []string{"../cloudv2", "../../cloudv2", "../../../cloudv2"} {
		if _, err := os.Stat(filepath.Join(rel, "proto", "public", "cloud")); err == nil {
			log.Printf("Using cloudv2 from relative path: %s", rel)
			return rel
		}
	}

	return ""
}

// AssertCloudv2Pinned fails when the local cloudv2 checkout at cloudv2Root has
// drifted from the SHA pinned in internal/buf_dependencies.yaml, so codegen
// binaries don't silently emit output from an unpinned cloudv2.
func AssertCloudv2Pinned(cloudv2Root string) error {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	deps, err := bufdeps.Read(bufdeps.DefaultPath(repoRoot))
	if err != nil {
		return fmt.Errorf("read pin file: %w", err)
	}
	return bufdeps.AssertCheckoutAt(cloudv2Root, deps.Cloudv2.SHA, "cloudv2")
}
