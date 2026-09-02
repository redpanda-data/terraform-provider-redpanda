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

package validators_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

func TestDurationCanonicalValidator(t *testing.T) {
	testCases := []struct {
		name        string
		value       types.String
		expectError bool
	}{
		{name: "null - skip validation", value: types.StringNull()},
		{name: "unknown - skip validation", value: types.StringUnknown()},
		{name: "canonical seconds", value: types.StringValue("30s")},
		{name: "canonical zero", value: types.StringValue("0s")},
		{name: "canonical minute", value: types.StringValue("1m0s")},
		{name: "canonical mixed", value: types.StringValue("1m30s")},
		{name: "canonical sub-second", value: types.StringValue("1.5s")},
		{name: "non-canonical bare minute", value: types.StringValue("1m"), expectError: true},
		{name: "non-canonical overflow seconds", value: types.StringValue("90s"), expectError: true},
		{name: "non-canonical bare zero", value: types.StringValue("0"), expectError: true},
		{name: "non-canonical hour", value: types.StringValue("1h"), expectError: true},
		{name: "invalid - not a duration", value: types.StringValue("thirty seconds"), expectError: true},
		{name: "invalid - empty string", value: types.StringValue(""), expectError: true},
		{name: "invalid - bare number", value: types.StringValue("30"), expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				Path:           path.Root("interval"),
				PathExpression: path.Root("interval").Expression(),
				ConfigValue:    tc.value,
			}
			var resp validator.StringResponse

			validators.DurationCanonicalValidator{}.ValidateString(context.Background(), req, &resp)

			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected validation error but got none")
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected validation error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestDurationCanonicalValidator_Description(t *testing.T) {
	v := validators.DurationCanonicalValidator{}

	desc := v.Description(context.Background())
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "canonical")

	mdDesc := v.MarkdownDescription(context.Background())
	assert.NotEmpty(t, mdDesc)
	assert.Contains(t, mdDesc, "canonical")
}
