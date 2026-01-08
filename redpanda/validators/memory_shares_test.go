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

package validators

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestMemorySharesValidator(t *testing.T) {
	tests := []struct {
		name        string
		value       types.String
		expectError bool
	}{
		// Valid values (>= 400MB minimum)
		{"valid 400M", types.StringValue("400M"), false},
		{"valid 512Mi", types.StringValue("512Mi"), false},
		{"valid 1Gi", types.StringValue("1Gi"), false},
		{"valid 2Gi", types.StringValue("2Gi"), false},
		{"valid 1G", types.StringValue("1G"), false},
		{"valid 1.5Gi", types.StringValue("1.5Gi"), false},
		{"valid 500M", types.StringValue("500M"), false},

		// Invalid: below 400MB minimum
		{"below min 256Ki", types.StringValue("256Ki"), true},
		{"below min 128M", types.StringValue("128M"), true},
		{"below min 500k", types.StringValue("500k"), true},
		{"below min 1024 bytes", types.StringValue("1024"), true},
		{"below min 1048576 bytes", types.StringValue("1048576"), true},
		{"below min 2.5M", types.StringValue("2.5M"), true},
		{"below min 399M", types.StringValue("399M"), true},

		// Invalid: bad format
		{"invalid format", types.StringValue("invalid"), true},
		{"invalid empty", types.StringValue(""), true},
		{"invalid suffix", types.StringValue("100X"), true},

		// Null/unknown values are allowed
		{"null value", types.StringNull(), false},
		{"unknown value", types.StringUnknown(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MemorySharesValidator{}
			req := validator.StringRequest{
				Path:        path.Root("memory_shares"),
				ConfigValue: tt.value,
			}
			resp := &validator.StringResponse{}

			v.ValidateString(context.Background(), req, resp)

			if tt.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error")
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "expected no validation error")
			}
		})
	}
}

func TestMemorySharesValidator_Methods(t *testing.T) {
	v := MemorySharesValidator{}
	ctx := context.Background()

	t.Run("description", func(t *testing.T) {
		desc := v.Description(ctx)
		assert.Contains(t, desc, "Kubernetes quantity format")
	})

	t.Run("markdown description", func(t *testing.T) {
		desc := v.MarkdownDescription(ctx)
		assert.Contains(t, desc, "Kubernetes quantity format")
	})
}
