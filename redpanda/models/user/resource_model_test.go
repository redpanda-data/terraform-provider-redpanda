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

package user

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestResourceModel_GetEffectivePassword(t *testing.T) {
	tests := []struct {
		name     string
		user     ResourceModel
		expected string
	}{
		{
			name: "password_wo takes precedence over password",
			user: ResourceModel{
				Password:   types.StringValue("legacy-password"),
				PasswordWO: types.StringValue("write-only-password"),
			},
			expected: "write-only-password",
		},
		{
			name: "falls back to password when password_wo is null",
			user: ResourceModel{
				Password:   types.StringValue("legacy-password"),
				PasswordWO: types.StringNull(),
			},
			expected: "legacy-password",
		},
		{
			name: "falls back to password when password_wo is unknown",
			user: ResourceModel{
				Password:   types.StringValue("legacy-password"),
				PasswordWO: types.StringUnknown(),
			},
			expected: "legacy-password",
		},
		{
			name: "returns empty string when both are null",
			user: ResourceModel{
				Password:   types.StringNull(),
				PasswordWO: types.StringNull(),
			},
			expected: "",
		},
		{
			name: "returns password_wo when password is null",
			user: ResourceModel{
				Password:   types.StringNull(),
				PasswordWO: types.StringValue("write-only-password"),
			},
			expected: "write-only-password",
		},
		{
			name: "returns empty password_wo if explicitly set to empty",
			user: ResourceModel{
				Password:   types.StringValue("legacy-password"),
				PasswordWO: types.StringValue(""),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.user.GetEffectivePassword()
			assert.Equal(t, tt.expected, result)
		})
	}
}
