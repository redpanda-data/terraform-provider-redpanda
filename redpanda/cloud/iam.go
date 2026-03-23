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

package cloud

import (
	"context"
	"errors"
	"fmt"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/iam/v1/iamv1grpc"
	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
	"google.golang.org/grpc"
)

// IAMClientSet holds the respective service clients to interact with
// the IAM endpoints of the Public API.
type IAMClientSet struct {
	RoleBinding iamv1grpc.RoleBindingServiceClient
}

// NewIAMClientSet uses the passed grpc connection to create an IAM client set.
func NewIAMClientSet(conn *grpc.ClientConn) *IAMClientSet {
	return &IAMClientSet{
		RoleBinding: iamv1grpc.NewRoleBindingServiceClient(conn),
	}
}

// CreateRoleBinding creates a role binding with the given parameters.
func (c *IAMClientSet) CreateRoleBinding(ctx context.Context, roleName, accountID string, scope *iamv1.RoleBinding_Scope) (*iamv1.RoleBinding, error) {
	resp, err := c.RoleBinding.CreateRoleBinding(ctx, &iamv1.CreateRoleBindingRequest{
		RoleBinding: &iamv1.RoleBindingCreate{
			RoleName:  roleName,
			AccountId: accountID,
			Scope:     scope,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.RoleBinding == nil {
		return nil, errors.New("error after creating role binding; provider response was empty. Please report this issue to the provider developers")
	}
	return resp.RoleBinding, nil
}

// RoleBindingForID gets the role binding for a given ID.
func (c *IAMClientSet) RoleBindingForID(ctx context.Context, id string) (*iamv1.RoleBinding, error) {
	resp, err := c.RoleBinding.GetRoleBinding(ctx, &iamv1.GetRoleBindingRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request role binding with ID %q: %w", id, err)
	}
	if resp.RoleBinding == nil {
		return nil, fmt.Errorf("unable to request role binding with ID %q. Please report this issue to the provider developers", id)
	}
	return resp.RoleBinding, nil
}

// DeleteRoleBinding deletes the role binding with the given ID.
func (c *IAMClientSet) DeleteRoleBinding(ctx context.Context, id string) error {
	_, err := c.RoleBinding.DeleteRoleBinding(ctx, &iamv1.DeleteRoleBindingRequest{
		Id: id,
	})
	return err
}
