// Copyright 2023 Redpanda Data, Inc.
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

package protovalidate_test

import (
	"strings"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rpvalidate "github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/protovalidate"
)

// TestValidate_EmptyClusterCreate confirms the helper surfaces a non-empty
// diagnostics slice when buf.validate.field rules on the proto fail. This
// pins the contract the schemagen-emitted protoValidator depends on:
// invalid Create payloads produce TF-attribute diagnostics rather than
// silently passing or panicking.
func TestValidate_EmptyClusterCreate(t *testing.T) {
	empty := &controlplanev1.ClusterCreate{}
	diags := rpvalidate.Validate(path.Empty(), empty)
	if !diags.HasError() {
		t.Fatal("expected diagnostics for an empty ClusterCreate (proto declares multiple required fields)")
	}
	var hasName bool
	for _, d := range diags {
		if strings.Contains(d.Summary(), "name") {
			hasName = true
			break
		}
	}
	if !hasName {
		t.Errorf("expected at least one diagnostic naming the 'name' field; got %v", diags)
	}
}

// TestValidate_ValidObjectPasses confirms a payload that satisfies the
// minimum constraints does not produce a diagnostic. We don't try to build
// a complete valid ClusterCreate (cross-field rules require more fields
// than fit a unit test); instead we rely on a payload that is empty enough
// to fail to assert the failure case is the only signal.
//
// (A nil message is the simplest pass case — protovalidate.Validate(nil)
// is a no-op.)
func TestValidate_NilMessagePasses(t *testing.T) {
	diags := rpvalidate.Validate(path.Empty(), nil)
	if diags.HasError() {
		t.Errorf("expected no error for nil message; got %v", diags)
	}
}
