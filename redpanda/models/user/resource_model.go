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

package user

import (
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// ContingentFields contains fields that are not returned by the User API
// but need to be preserved in Terraform state.
type ContingentFields struct {
	AllowDeletion types.Bool
}

// UserResponse is an interface that CreateUserResponse_User, ListUsersResponse_User, and UpdateUserResponse_User all satisfy
type UserResponse interface {
	GetName() string
	GetMechanism() dataplanev1.SASLMechanism
	HasMechanism() bool
}

// GetUpdatedModel populates the User model from a dataplane User response
// and contingent fields that are managed by Terraform only.
func (u *ResourceModel) GetUpdatedModel(user UserResponse, contingent ContingentFields) *ResourceModel {
	u.Name = types.StringValue(user.GetName())
	u.ID = types.StringValue(user.GetName())

	if user.HasMechanism() {
		mechanism := user.GetMechanism()
		u.Mechanism = types.StringValue(utils.UserMechanismToString(&mechanism))
	}

	// Set contingent fields from either model or defaults
	u.AllowDeletion = contingent.AllowDeletion
	if u.AllowDeletion.IsNull() || u.AllowDeletion.IsUnknown() {
		u.AllowDeletion = types.BoolValue(false)
	}

	return u
}
