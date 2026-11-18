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

package cluster

import (
	"context"
	"testing"
)

// redpanda_version is a create input the control plane never returns, so the
// data source has nothing to report for it.
func TestRedpandaVersionIsResourceOnly(t *testing.T) {
	ctx := context.Background()
	if _, ok := ResourceClusterSchema(ctx).Attributes["redpanda_version"]; !ok {
		t.Error("resource schema lost redpanda_version")
	}
	if _, ok := DatasourceClusterSchema(ctx).Attributes["redpanda_version"]; ok {
		t.Error("data source schema declares redpanda_version, which it can only ever read as null")
	}
}
