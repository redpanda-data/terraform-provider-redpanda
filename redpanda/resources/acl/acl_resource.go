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

// Package acl contains the implementation of the ACL resource following the Terraform framework interfaces.
package acl

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// ACL represents the ACL Terraform resource.
type ACL struct {
	ACLClient dataplanev1alpha1.ACLServiceClient
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ACL{}
	_ resource.ResourceWithConfigure   = &ACL{}
	_ resource.ResourceWithImportState = &ACL{}
)

// Metadata returns the metadata for the resource.
func (*ACL) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_acl"
}

// Configure configures the ACL resource clients
func (a *ACL) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at acl.Configure")
		return
	}

	p, ok := request.ProviderData.(utils.ResourceData)

	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData))
		return
	}

	client, err := clients.NewACLServiceClient(ctx, p.CloudEnv, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create cluster client", err.Error())
		return
	}
	a.ACLClient = client
}

// Schema returns the schema for the resource.
func (*ACL) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceACLSchema()
}

func resourceACLSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the resource",
			},
			"resource_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the resource",
			},
			"resource_pattern_type": schema.StringAttribute{
				Required:    true,
				Description: "The pattern type of the resource",
			},
			"principal": schema.StringAttribute{
				Required:    true,
				Description: "The principal",
			},
			"host": schema.StringAttribute{
				Required:    true,
				Description: "The host",
			},
			"operation": schema.StringAttribute{
				Required:    true,
				Description: "The operation type",
			},
			"permission_type": schema.StringAttribute{
				Required:    true,
				Description: "The permission type",
			},
		},
	}
}

// Create creates a new ACL resource.
func (a *ACL) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.ACL
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	resourceType, err := utils.StringToACLResourceType(model.ResourceType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource type", err.Error())
		return
	}

	resourcePatternType, err := utils.StringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource pattern type", err.Error())
		return
	}

	operation, err := utils.StringToACLOperation(model.Operation.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting operation", err.Error())
		return
	}

	permissionType, err := utils.StringToACLPermissionType(model.PermissionType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting permission type", err.Error())
		return
	}

	// TODO doesn't return an acl object in the response, check on this
	_, err = a.ACLClient.CreateACL(ctx, &dataplanev1alpha1.CreateACLRequest{
		ResourceType:        resourceType,
		ResourceName:        model.ResourceName.ValueString(),
		ResourcePatternType: resourcePatternType,
		Principal:           model.Principal.ValueString(),
		Host:                model.Host.ValueString(),
		Operation:           operation,
		PermissionType:      permissionType,
	})

	if err != nil {
		response.Diagnostics.AddError("Failed to create ACL", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &models.ACL{
		ResourceType:        model.ResourceType,
		ResourceName:        model.ResourceName,
		ResourcePatternType: model.ResourcePatternType,
		Principal:           model.Principal,
		Host:                model.Host,
		Operation:           model.Operation,
		PermissionType:      model.PermissionType,
	})...)
}

// Read checks for the existence of an ACL resource
func (a *ACL) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.ACL
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	resourceType, err := utils.StringToACLResourceType(model.ResourceType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource type", err.Error())
		return
	}

	resourcePatternType, err := utils.StringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource pattern type", err.Error())
		return
	}

	operation, err := utils.StringToACLOperation(model.Operation.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting operation", err.Error())
		return
	}

	permissionType, err := utils.StringToACLPermissionType(model.PermissionType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting permission type", err.Error())
		return
	}

	filter := &dataplanev1alpha1.ACL_Filter{
		ResourceType:        resourceType,
		ResourceName:        utils.StringToStringPointer(model.ResourceName.ValueString()),
		ResourcePatternType: resourcePatternType,
		Principal:           utils.StringToStringPointer(model.Principal.ValueString()),
		Host:                utils.StringToStringPointer(model.Host.ValueString()),
		Operation:           operation,
		PermissionType:      permissionType,
	}

	aclList, err := a.ACLClient.ListACLs(ctx, &dataplanev1alpha1.ListACLsRequest{Filter: filter})
	if err != nil {
		response.Diagnostics.AddError("Failed to list ACLs", err.Error())
		return
	}

	for _, res := range aclList.Resources {
		if res.ResourceName == model.ResourceName.ValueString() && res.ResourceType == resourceType && res.ResourcePatternType == resourcePatternType {
			response.Diagnostics.Append(response.State.Set(ctx, &models.ACL{
				ResourceType:        types.StringValue(utils.ACLResourceTypeToString(res.ResourceType)),
				ResourceName:        types.StringValue(res.ResourceName),
				ResourcePatternType: types.StringValue(utils.ACLResourcePatternTypeToString(res.ResourcePatternType)),
				Principal:           model.Principal,
				Host:                model.Host,
				Operation:           model.Operation,
				PermissionType:      model.PermissionType,
			})...)
			return
		}
	}

	// If no matching ACL found, remove the resource from state
	response.State.RemoveResource(ctx)
}

// Update updates an ACL resource
func (*ACL) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes an ACL resource
func (a *ACL) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.ACL
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	resourceType, err := utils.StringToACLResourceType(model.ResourceType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource type", err.Error())
		return
	}

	resourcePatternType, err := utils.StringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource pattern type", err.Error())
		return
	}

	operation, err := utils.StringToACLOperation(model.Operation.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting operation", err.Error())
		return
	}

	permissionType, err := utils.StringToACLPermissionType(model.PermissionType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting permission type", err.Error())
		return
	}

	filter := &dataplanev1alpha1.ACL_Filter{
		ResourceType:        resourceType,
		ResourceName:        utils.StringToStringPointer(model.ResourceName.ValueString()),
		ResourcePatternType: resourcePatternType,
		Principal:           utils.StringToStringPointer(model.Principal.ValueString()),
		Host:                utils.StringToStringPointer(model.Host.ValueString()),
		Operation:           operation,
		PermissionType:      permissionType,
	}

	deleteResponse, err := a.ACLClient.DeleteACLs(ctx, &dataplanev1alpha1.DeleteACLsRequest{Filter: filter})
	if err != nil {
		response.Diagnostics.AddError("Failed to delete ACL", err.Error())
		return
	}

	// Check for errors in the response
	for _, matchingACL := range deleteResponse.MatchingAcls {
		if matchingACL.ErrorCode != 0 {
			response.Diagnostics.AddError("Error deleting ACL", matchingACL.ErrorMessage)
			return
		}
	}

	// Remove the resource from state
	response.State.RemoveResource(ctx)
}

// ImportState imports an ACL resource
func (*ACL) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(ctx, &models.Cluster{
		ID: types.StringValue(request.ID),
	})...)
}
