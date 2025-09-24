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

package kclients

import (
	"context"
	"fmt"

	"github.com/redpanda-data/common-go/rpsr"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

// SchemaRegistryACLClientInterface defines the interface for Schema Registry ACL operations
type SchemaRegistryACLClientInterface interface {
	CreateACL(ctx context.Context, acl SchemaRegistryACLRequest) error
	ListACLs(ctx context.Context, filter SchemaRegistryACLFilter) ([]SchemaRegistryACLResponse, error)
	DeleteACL(ctx context.Context, acl SchemaRegistryACLRequest) error
}

// SchemaRegistryACLClient wraps the common-go rpsr.Client for ACL operations
type SchemaRegistryACLClient struct {
	client *rpsr.Client
}

// SchemaRegistryACLRequest represents an ACL creation/deletion request
type SchemaRegistryACLRequest struct {
	Principal    string `json:"principal"`
	Resource     string `json:"resource"`
	ResourceType string `json:"resource_type"`
	PatternType  string `json:"pattern_type"`
	Host         string `json:"host"`
	Operation    string `json:"operation"`
	Permission   string `json:"permission"`
}

// SchemaRegistryACLResponse represents an ACL in the list response
type SchemaRegistryACLResponse struct {
	Principal    string `json:"principal"`
	Resource     string `json:"resource"`
	ResourceType string `json:"resource_type"`
	PatternType  string `json:"pattern_type"`
	Host         string `json:"host"`
	Operation    string `json:"operation"`
	Permission   string `json:"permission"`
}

// SchemaRegistryACLFilter represents filter parameters for listing ACLs
type SchemaRegistryACLFilter struct {
	Principal    string
	Resource     string
	ResourceType string
	PatternType  string
	Host         string
	Operation    string
	Permission   string
}

// NewSchemaRegistryACLClient creates a new Schema Registry ACL client using common-go rpsr
func NewSchemaRegistryACLClient(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID, username, password string) (*SchemaRegistryACLClient, error) {
	// Create Schema Registry client using existing kclients functionality
	srClient, err := GetSchemaRegistryClientForCluster(ctx, cpCl, clusterID, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create Schema Registry client: %w", err)
	}

	// Wrap it with the rpsr ACL-aware client
	aclClient, err := rpsr.NewClient(srClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACL client: %w", err)
	}

	return &SchemaRegistryACLClient{
		client: aclClient,
	}, nil
}

// CreateACL creates a new Schema Registry ACL
func (c *SchemaRegistryACLClient) CreateACL(ctx context.Context, acl SchemaRegistryACLRequest) error {
	rpACL := c.convertToRPACL(&acl)
	return c.client.CreateACLs(ctx, []rpsr.ACL{rpACL})
}

// ListACLs lists Schema Registry ACLs with optional filtering
func (c *SchemaRegistryACLClient) ListACLs(ctx context.Context, filter SchemaRegistryACLFilter) ([]SchemaRegistryACLResponse, error) {
	rpFilter := &rpsr.ACL{
		Principal:    filter.Principal,
		Resource:     filter.Resource,
		ResourceType: rpsr.ResourceType(filter.ResourceType),
		PatternType:  rpsr.PatternType(filter.PatternType),
		Host:         filter.Host,
		Operation:    rpsr.Operation(filter.Operation),
		Permission:   rpsr.Permission(filter.Permission),
	}

	rpACLs, err := c.client.ListACLs(ctx, rpFilter)
	if err != nil {
		return nil, err
	}

	var result []SchemaRegistryACLResponse
	for _, rpACL := range rpACLs {
		result = append(result, c.convertFromRPACL(&rpACL))
	}

	return result, nil
}

// DeleteACL deletes a Schema Registry ACL
func (c *SchemaRegistryACLClient) DeleteACL(ctx context.Context, acl SchemaRegistryACLRequest) error {
	rpACL := c.convertToRPACL(&acl)
	return c.client.DeleteACLs(ctx, []rpsr.ACL{rpACL})
}

// convertToRPACL converts our ACL request to the common-go rpsr.ACL format
func (*SchemaRegistryACLClient) convertToRPACL(acl *SchemaRegistryACLRequest) rpsr.ACL {
	return rpsr.ACL{
		Principal:    acl.Principal,
		Resource:     acl.Resource,
		ResourceType: rpsr.ResourceType(acl.ResourceType),
		PatternType:  rpsr.PatternType(acl.PatternType),
		Host:         acl.Host,
		Operation:    rpsr.Operation(acl.Operation),
		Permission:   rpsr.Permission(acl.Permission),
	}
}

// convertFromRPACL converts the common-go rpsr.ACL format to our response format
func (*SchemaRegistryACLClient) convertFromRPACL(rpACL *rpsr.ACL) SchemaRegistryACLResponse {
	return SchemaRegistryACLResponse{
		Principal:    rpACL.Principal,
		Resource:     rpACL.Resource,
		ResourceType: string(rpACL.ResourceType),
		PatternType:  string(rpACL.PatternType),
		Host:         rpACL.Host,
		Operation:    string(rpACL.Operation),
		Permission:   string(rpACL.Permission),
	}
}
