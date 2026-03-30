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

// CreateGroup creates a new group.
func (c *IAMClientSet) CreateGroup(ctx context.Context, name, description string) (*iamv1.Group, error) {
	gc := &iamv1.GroupCreate{
		Name: name,
	}
	gc.SetDescription(description)

	resp, err := c.Group.CreateGroup(ctx, &iamv1.CreateGroupRequest{
		Group: gc,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetGroup() == nil {
		return nil, errors.New("error after creating group; provider response was empty. Please report this issue to the provider developers")
	}
	return resp.GetGroup(), nil
}

// GroupForID retrieves a group by ID.
func (c *IAMClientSet) GroupForID(ctx context.Context, id string) (*iamv1.Group, error) {
	resp, err := c.Group.GetGroup(ctx, &iamv1.GetGroupRequest{
		Id: id,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to request group with ID %q: %w", id, err)
	}
	if resp.GetGroup() == nil {
		return nil, fmt.Errorf("unable to request group with ID %q. Please report this issue to the provider developers", id)
	}
	return resp.GetGroup(), nil
}

// GroupForName retrieves a group by name using the list API with a name filter.
func (c *IAMClientSet) GroupForName(ctx context.Context, name string) (*iamv1.Group, error) {
	resp, err := c.Group.ListGroups(ctx, &iamv1.ListGroupsRequest{
		Filter: &iamv1.ListGroupsRequest_Filter{
			Name: name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list groups with name %q: %w", name, err)
	}
	for _, g := range resp.GetGroups() {
		if g.GetName() == name {
			return g, nil
		}
	}
	return nil, fmt.Errorf("unable to find group with name %q", name)
}

// DeleteGroup deletes a group by ID.
func (c *IAMClientSet) DeleteGroup(ctx context.Context, id string) error {
	_, err := c.Group.DeleteGroup(ctx, &iamv1.DeleteGroupRequest{
		Id: id,
	})
	return err
}
