// Copyright 2024 Redpanda Data, Inc.
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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
	"github.com/stretchr/testify/assert"
)

func TestServerlessPrivateLinkConfigRemovedValidator(t *testing.T) {
	awsType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"allowed_principals": types.ListType{ElemType: types.StringType},
	}}
	cfgType := map[string]attr.Type{"aws": awsType}

	setValue := types.ObjectValueMust(cfgType, map[string]attr.Value{
		"aws": types.ObjectValueMust(awsType.AttrTypes, map[string]attr.Value{
			"allowed_principals": types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("arn:aws:iam::123456789012:root"),
			}),
		}),
	})

	cases := []struct {
		name        string
		value       types.Object
		expectError bool
	}{
		{name: "null - allowed", value: types.ObjectNull(cfgType)},
		{name: "unknown - allowed", value: types.ObjectUnknown(cfgType)},
		{name: "set - rejected", value: setValue, expectError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.ObjectRequest{
				Path:           path.Root("cloud_provider_config"),
				PathExpression: path.Root("cloud_provider_config").Expression(),
				ConfigValue:    tc.value,
			}
			var resp validator.ObjectResponse
			validators.ServerlessPrivateLinkConfigRemovedValidator{}.ValidateObject(context.Background(), req, &resp)

			if tc.expectError {
				assert.True(t, resp.Diagnostics.HasError(), "expected error but got none")
				assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "aws_config")
			} else {
				assert.False(t, resp.Diagnostics.HasError(), "unexpected error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}
