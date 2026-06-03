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

func TestPSCConsumerSourceValidator(t *testing.T) {
	cases := []struct {
		name      string
		input     types.String
		wantError bool
	}{
		{"bare project id accepted", types.StringValue("my-project-id"), false},
		{"project number accepted", types.StringValue("123456789012"), false},
		{"projects/ prefix rejected", types.StringValue("projects/my-project-id"), true},
		{"projects/ number rejected", types.StringValue("projects/123456789012"), true},
		{"any slash rejected", types.StringValue("foo/bar"), true},
		{"null skipped", types.StringNull(), false},
		{"unknown skipped", types.StringUnknown(), false},
	}

	v := validators.PSCConsumerSourceValidator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			v.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("source"),
				ConfigValue: tc.input,
			}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("ValidateString(%q) error = %v, want %v", tc.input, got, tc.wantError)
			}
		})
	}
}
