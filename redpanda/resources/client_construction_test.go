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

package resources_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResourcesBuildClientsThroughNewDataplaneClient pins how per-cluster API
// clients are constructed.
//
// utils.NewDataplaneClient wraps the pooled connection so every failure carries
// its method and endpoint, and so the shared transient classifier sees the same
// errors everywhere. A resource that calls DataplaneConnPool.GetConnection and
// builds its own client skips both, and the symptom is a bare
// "Unknown (raw: rpc error: code = Unknown desc = )" naming neither the call nor
// the cluster — which is what redpanda_role reported until it was routed here.
func TestResourcesBuildClientsThroughNewDataplaneClient(t *testing.T) {
	var offenders []string

	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// The helper itself and the pool's own package legitimately touch it.
		if strings.Contains(path, filepath.Join("utils", "dataplane_client.go")) ||
			strings.Contains(path, filepath.Join("redpanda", "cloud")) {
			return nil
		}
		src, readErr := os.ReadFile(path) // #nosec G304 -- walking the repo's own source
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(src), "DataplaneConnPool.GetConnection(") {
			offenders = append(offenders, path)
		}
		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"these build a client from the pool directly; route them through "+
			"utils.NewDataplaneClient so failures name their method and endpoint: %v", offenders)
}
