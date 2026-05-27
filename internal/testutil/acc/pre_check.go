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

package acc

import (
	"os"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// PreCheck verifies that the OAuth client credentials are present before a
// live-acceptance test runs.
func PreCheck(t testing.TB) {
	if v := os.Getenv(redpanda.ClientIDEnv); v == "" {
		t.Fatalf("environment variable %v must be set for acceptance tests", redpanda.ClientIDEnv)
	}
	if v := os.Getenv(redpanda.ClientSecretEnv); v == "" {
		t.Fatalf("environment variable %v must be set for acceptance tests", redpanda.ClientSecretEnv)
	}
}
