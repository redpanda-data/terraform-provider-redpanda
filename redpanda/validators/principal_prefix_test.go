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

package validators_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

func TestCanonicalizePrincipal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bare username gets User: prefix", "alice", "User:alice"},
		{"bare with dots gets User: prefix", "alice.smith", "User:alice.smith"},
		{"bare with @ gets User: prefix", "alice@example.com", "User:alice@example.com"},
		{"User: prefix preserved", "User:bob", "User:bob"},
		{"Group: prefix preserved", "Group:engineers", "Group:engineers"},
		{"RedpandaRole: prefix preserved", "RedpandaRole:admin", "RedpandaRole:admin"},
		{"empty string preserved", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validators.CanonicalizePrincipal(tc.input); got != tc.want {
				t.Errorf("CanonicalizePrincipal(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPrincipalPrefix(t *testing.T) {
	cases := []struct {
		name      string
		input     types.String
		wantError bool
	}{
		{"User: prefix accepted", types.StringValue("User:alice"), false},
		{"Group: prefix accepted", types.StringValue("Group:engineers"), false},
		{"User: with dotted name accepted", types.StringValue("User:alice.smith"), false},
		{"Group: with email-style name accepted", types.StringValue("Group:eng@example.com"), false},
		{"bare username rejected", types.StringValue("alice"), true},
		{"RedpandaRole: rejected (ACL-only prefix)", types.StringValue("RedpandaRole:admin"), true},
		{"lowercase user: rejected", types.StringValue("user:alice"), true},
		{"empty after prefix rejected", types.StringValue("User:"), true},
		{"empty string rejected", types.StringValue(""), true},
		{"null skipped", types.StringNull(), false},
		{"unknown skipped", types.StringUnknown(), false},
	}

	v := validators.PrincipalPrefix()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				Path:        path.Root("principal"),
				ConfigValue: tc.input,
			}
			resp := &validator.StringResponse{}
			v.ValidateString(context.Background(), req, resp)

			gotError := resp.Diagnostics.HasError()
			if gotError != tc.wantError {
				t.Errorf("want error=%v, got error=%v (diags=%v)", tc.wantError, gotError, resp.Diagnostics)
			}
		})
	}
}
