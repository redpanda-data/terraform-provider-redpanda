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

	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
)

// CreateServiceAccount creates a new service account.
func (c *IAMClientSet) CreateServiceAccount(ctx context.Context, name, description string) (*iamv1.ServiceAccount, error) {
	resp, err := c.ServiceAccount.CreateServiceAccount(ctx, &iamv1.CreateServiceAccountRequest{
		ServiceAccount: &iamv1.ServiceAccountCreate{
			Name:        name,
			Description: description,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.GetServiceAccount() == nil {
		return nil, errors.New("error after creating service account; provider response was empty. Please report this issue to the provider developers")
	}
	return resp.GetServiceAccount(), nil
}

// ServiceAccountForID retrieves a service account by ID.
func (c *IAMClientSet) ServiceAccountForID(ctx context.Context, id string) (*iamv1.ServiceAccount, error) {
	resp, err := c.ServiceAccount.GetServiceAccount(ctx, &iamv1.GetServiceAccountRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request service account with ID %q: %w", id, err)
	}
	if resp.GetServiceAccount() == nil {
		return nil, fmt.Errorf("unable to request service account with ID %q. Please report this issue to the provider developers", id)
	}
	return resp.GetServiceAccount(), nil
}

// ServiceAccountForName retrieves a service account by name using the list API with a name filter.
func (c *IAMClientSet) ServiceAccountForName(ctx context.Context, name string) (*iamv1.ServiceAccount, error) {
	resp, err := c.ServiceAccount.ListServiceAccounts(ctx, &iamv1.ListServiceAccountsRequest{
		Filter: &iamv1.ListServiceAccountsRequest_Filter{
			Name: name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list service accounts with name %q: %w", name, err)
	}
	for _, sa := range resp.GetServiceAccounts() {
		if sa.GetName() == name {
			return sa, nil
		}
	}
	return nil, fmt.Errorf("unable to find service account with name %q", name)
}

// UpdateServiceAccount updates a service account's name and description.
func (c *IAMClientSet) UpdateServiceAccount(ctx context.Context, id, name, description string) (*iamv1.ServiceAccount, error) {
	resp, err := c.ServiceAccount.UpdateServiceAccount(ctx, &iamv1.UpdateServiceAccountRequest{
		Id: id,
		ServiceAccount: &iamv1.ServiceAccountUpdate{
			Name:        name,
			Description: description,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.GetServiceAccount() == nil {
		return nil, errors.New("error after updating service account; provider response was empty. Please report this issue to the provider developers")
	}
	return resp.GetServiceAccount(), nil
}

// DeleteServiceAccount deletes a service account by ID.
func (c *IAMClientSet) DeleteServiceAccount(ctx context.Context, id string) error {
	_, err := c.ServiceAccount.DeleteServiceAccount(ctx, &iamv1.DeleteServiceAccountRequest{
		Id: id,
	})
	return err
}
