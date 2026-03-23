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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Group represents a Redpanda Cloud group from the REST API.
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type createGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type createGroupResponse struct {
	Group *Group `json:"group"`
}

type getGroupResponse struct {
	Group *Group `json:"group"`
}

// GroupClient provides methods to interact with the Redpanda Cloud groups REST API.
type GroupClient struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// NewGroupClient creates a new REST client for the groups API.
func NewGroupClient(apiURL, authToken string) *GroupClient {
	return &GroupClient{
		baseURL:   apiURL,
		authToken: authToken,
		client:    &http.Client{},
	}
}

// CreateGroup creates a new group.
func (c *GroupClient) CreateGroup(ctx context.Context, name, description string) (*Group, error) {
	body := createGroupRequest{
		Name:        name,
		Description: description,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create group request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/groups", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	var result createGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode create group response: %w", err)
	}
	if result.Group == nil {
		return nil, fmt.Errorf("create group response was empty")
	}
	return result.Group, nil
}

// GetGroup retrieves a group by ID.
func (c *GroupClient) GetGroup(ctx context.Context, id string) (*Group, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/groups/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("failed to get group %q: %w", id, err)
	}

	var result getGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode get group response: %w", err)
	}
	if result.Group == nil {
		return nil, fmt.Errorf("get group response was empty")
	}
	return result.Group, nil
}

// DeleteGroup deletes a group by ID.
func (c *GroupClient) DeleteGroup(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/v1/groups/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("failed to delete group %q: %w", id, err)
	}
	return nil
}

func (c *GroupClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.authToken)
}

// IsHTTPNotFound returns true if the error wraps a 404 HTTP status.
func IsHTTPNotFound(err error) bool {
	if err == nil {
		return false
	}
	httpErr, ok := err.(*HTTPError)
	return ok && httpErr.StatusCode == http.StatusNotFound
}

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return &HTTPError{
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}
