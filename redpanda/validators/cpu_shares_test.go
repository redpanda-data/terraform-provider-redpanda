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

func TestCPUSharesValidator(t *testing.T) {
	tests := []struct {
		name        string
		value       types.String
		expectError bool
	}{
		// Valid values (>= 100m minimum and multiple of 100m)
		{"valid 100m", types.StringValue("100m"), false},
		{"valid 200m", types.StringValue("200m"), false},
		{"valid 500m", types.StringValue("500m"), false},
		{"valid 1000m", types.StringValue("1000m"), false},
		{"valid 1 core", types.StringValue("1"), false},
		{"valid 2 cores", types.StringValue("2"), false},
		{"valid 0.5 core", types.StringValue("0.5"), false},
		{"valid 1.5 cores", types.StringValue("1.5"), false},

		// Invalid: not a multiple of 100m
		{"invalid 50m", types.StringValue("50m"), true},
		{"invalid 150m", types.StringValue("150m"), true},
		{"invalid 250m", types.StringValue("250m"), true},
		{"invalid 0.15 cores", types.StringValue("0.15"), true},

		// Invalid: below 100m minimum
		{"below min 0m", types.StringValue("0m"), true},
		{"below min 0", types.StringValue("0"), true},

		// Invalid: bad format
		{"invalid format", types.StringValue("invalid"), true},
		{"invalid empty", types.StringValue(""), true},

		// Null/unknown values are allowed
		{"null value", types.StringNull(), false},
		{"unknown value", types.StringUnknown(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := CPUSharesValidator{}
			req := validator.StringRequest{
				Path:        path.Root("cpu_shares"),
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

func TestCPUSharesValidator_Methods(t *testing.T) {
	v := CPUSharesValidator{}
	ctx := context.Background()

	t.Run("description", func(t *testing.T) {
		desc := v.Description(ctx)
		assert.Contains(t, desc, "100m")
	})

	t.Run("markdown description", func(t *testing.T) {
		desc := v.MarkdownDescription(ctx)
		assert.Contains(t, desc, "100m")
	})
}
