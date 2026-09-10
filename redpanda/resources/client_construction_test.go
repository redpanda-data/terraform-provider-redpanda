// Copyright 2025 Redpanda Data, Inc.
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
// "Unknown (raw: rpc error: code = Unknown desc = )" naming neither the call
// nor the cluster.
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

// TestSecurityServiceClientsDialConsole pins the endpoint SecurityService is
// reached on.
//
// Every other per-cluster service answers on the cluster's api- host, so routing
// SecurityService there too looks right and compiles fine. It is not: the api-
// host returns a bare UNKNOWN with an empty message for these methods, which is
// indistinguishable from the warm-up failures the retry policy is built to ride
// out, so the call retries to exhaustion and reports nothing useful. That took
// out three acceptance suites once. The only signal is which host was dialled,
// which is what this checks.
func TestSecurityServiceClientsDialConsole(t *testing.T) {
	var offenders []string

	for _, resource := range []string{"role", "roleassignment"} {
		matches, err := filepath.Glob(filepath.Join(resource, "*.go"))
		require.NoError(t, err)
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			src, readErr := os.ReadFile(path) // #nosec G304 -- walking this package's own source
			require.NoError(t, readErr)
			if !strings.Contains(string(src), "NewSecurityServiceClient") {
				continue
			}
			if !strings.Contains(string(src), "utils.NewConsoleClient(") {
				offenders = append(offenders, path)
			}
		}
	}

	require.Empty(t, offenders,
		"these build a SecurityService client without utils.NewConsoleClient, so "+
			"they dial the cluster's api- host, which does not serve it: %v", offenders)
}
