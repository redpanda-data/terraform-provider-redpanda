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

package acl

import (
	"context"
	"fmt"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	aclmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/acl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// ACL represents the ACL Terraform resource.
type ACL struct {
	ACLClient dataplanev1grpc.ACLServiceClient

	resData       config.Resource
	dataplaneConn *grpc.ClientConn
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource              = &ACL{}
	_ resource.ResourceWithConfigure = &ACL{}
)

// Metadata returns the metadata for the resource.
func (*ACL) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_acl"
}

// Configure configures the ACL resource clients
func (a *ACL) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	p, ok := request.ProviderData.(config.Resource)

	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData))
		return
	}
	a.resData = p
}

// Schema returns the schema for the resource.
func (*ACL) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = ResourceACLSchema()
}

// ResourceACLSchema returns the schema for the ACL resource.
func ResourceACLSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"resource_type": schema.StringAttribute{
				Required:      true,
				Description:   "The type of the resource (TOPIC, GROUP, etc...) this ACL shall target",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    aclResourceTypeValidator(),
			},
			"resource_name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the resource this ACL entry will be on",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resource_pattern_type": schema.StringAttribute{
				Required:      true,
				Description:   "The pattern type of the resource. It determines the strategy how the provided resource name is matched (LITERAL, MATCH, PREFIXED, etc ...) against the actual resource names",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    aclResourcePatternTypeValidator(),
			},
			"principal": schema.StringAttribute{
				Required:      true,
				Description:   "The principal to apply this ACL for",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"host": schema.StringAttribute{
				Required:      true,
				Description:   "The host address to use for this ACL",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"operation": schema.StringAttribute{
				Required:      true,
				Description:   "The operation type that shall be allowed or denied (e.g READ)",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    aclOperationValidator(),
			},
			"permission_type": schema.StringAttribute{
				Required:      true,
				Description:   "The permission type. It determines whether the operation should be ALLOWED or DENIED",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    aclPermissionTypeValidator(),
			},
			"cluster_api_url": schema.StringAttribute{
				Required: true,
				Description: "The cluster API URL. Changing this will prevent deletion of the resource on the existing " +
					"cluster. It is generally a better idea to delete an existing resource and create a new one than to " +
					"change this value unless you are planning to do state imports",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "When set to true, allows the resource to be removed from state even if the cluster is unreachable",
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// Create creates a new ACL resource.
func (a *ACL) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model aclmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	resourceType, err := stringToACLResourceType(model.ResourceType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource type", utils.DeserializeGrpcError(err))
		return
	}

	resourcePatternType, err := stringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource pattern type", utils.DeserializeGrpcError(err))
		return
	}

	operation, err := stringToACLOperation(model.Operation.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting operation", utils.DeserializeGrpcError(err))
		return
	}

	permissionType, err := stringToACLPermissionType(model.PermissionType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting permission type", utils.DeserializeGrpcError(err))
		return
	}

	if err := a.createACLClient(model.ClusterAPIURL.ValueString()); err != nil {
		response.Diagnostics.AddError("failed to create ACL client", utils.DeserializeGrpcError(err))
		return
	}
	defer a.dataplaneConn.Close()
	_, err = a.ACLClient.CreateACL(ctx, &dataplanev1.CreateACLRequest{
		ResourceType:        resourceType,
		ResourceName:        model.ResourceName.ValueString(),
		ResourcePatternType: resourcePatternType,
		Principal:           model.Principal.ValueString(),
		Host:                model.Host.ValueString(),
		Operation:           operation,
		PermissionType:      permissionType,
	})
	if err != nil {
		response.Diagnostics.AddError("Failed to create ACL", utils.DeserializeGrpcError(err))
		return
	}

	acl := &aclmodel.ResourceModel{
		ResourceType:        model.ResourceType,
		ResourceName:        model.ResourceName,
		ResourcePatternType: model.ResourcePatternType,
		Principal:           model.Principal,
		Host:                model.Host,
		Operation:           model.Operation,
		PermissionType:      model.PermissionType,
		ClusterAPIURL:       model.ClusterAPIURL,
		AllowDeletion:       model.AllowDeletion,
	}
	acl.ID = types.StringValue(acl.GenerateID())
	response.Diagnostics.Append(response.State.Set(ctx, acl)...)
}

// Read checks for the existence of an ACL resource
func (a *ACL) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model aclmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)

	if model.ClusterAPIURL.IsNull() || model.ClusterAPIURL.IsUnknown() || model.ClusterAPIURL.ValueString() == "" {
		response.State.RemoveResource(ctx)
		return
	}

	resourceType, err := stringToACLResourceType(model.ResourceType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource type", utils.DeserializeGrpcError(err))
		return
	}

	resourcePatternType, err := stringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource pattern type", utils.DeserializeGrpcError(err))
		return
	}

	operation, err := stringToACLOperation(model.Operation.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting operation", utils.DeserializeGrpcError(err))
		return
	}

	permissionType, err := stringToACLPermissionType(model.PermissionType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting permission type", utils.DeserializeGrpcError(err))
		return
	}

	filter := &dataplanev1.ListACLsRequest_Filter{
		ResourceType:        resourceType,
		ResourceName:        utils.StringToStringPointer(model.ResourceName.ValueString()),
		ResourcePatternType: resourcePatternType,
		Principal:           utils.StringToStringPointer(model.Principal.ValueString()),
		Host:                utils.StringToStringPointer(model.Host.ValueString()),
		Operation:           operation,
		PermissionType:      permissionType,
	}

	err = a.createACLClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "ACL", model.GenerateID(), model.AllowDeletion, err, "create ACL client")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}
	defer a.dataplaneConn.Close()
	aclList, err := a.ACLClient.ListACLs(ctx, &dataplanev1.ListACLsRequest{Filter: filter})
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "ACL", model.GenerateID(), model.AllowDeletion, err, "list ACLs")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	for _, res := range aclList.Resources {
		if res.ResourceName == model.ResourceName.ValueString() && res.ResourceType == resourceType && res.ResourcePatternType == resourcePatternType {
			acl := &aclmodel.ResourceModel{
				ResourceType:        types.StringValue(aclResourceTypeToString(res.ResourceType)),
				ResourceName:        types.StringValue(res.ResourceName),
				ResourcePatternType: types.StringValue(aclResourcePatternTypeToString(res.ResourcePatternType)),
				Principal:           model.Principal,
				Host:                model.Host,
				Operation:           model.Operation,
				PermissionType:      model.PermissionType,
				ClusterAPIURL:       model.ClusterAPIURL,
				AllowDeletion:       model.AllowDeletion,
			}
			if model.ID.IsNull() || model.ID.IsUnknown() {
				acl.ID = types.StringValue(acl.GenerateID())
			} else {
				acl.ID = model.ID
			}
			response.Diagnostics.Append(response.State.Set(ctx, acl)...)
			return
		}
	}

	// ACL not found - use helper for proper handling
	action, diags := utils.HandleGracefulRemoval(ctx, "ACL", model.GenerateID(), model.AllowDeletion, utils.NotFoundError{Message: "ACL not found in cluster"}, "find ACL")
	response.Diagnostics.Append(diags...)
	if action == utils.RemoveFromState {
		response.State.RemoveResource(ctx)
	}
}

// Update updates an ACL resource
func (*ACL) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan aclmodel.ResourceModel
	var state aclmodel.ResourceModel

	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)

	if response.Diagnostics.HasError() {
		return
	}

	state.AllowDeletion = plan.AllowDeletion

	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

// Delete deletes an ACL resource
func (a *ACL) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model aclmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	aclID := model.GenerateID()

	// Block deletion only if allow_deletion is explicitly set to false
	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		response.Diagnostics.AddError(
			"Cannot delete ACL",
			fmt.Sprintf("Deletion of ACL for principal %s on resource %s is not allowed. Set allow_deletion=true to allow deletion of this resource.", model.Principal.ValueString(), model.ResourceName.ValueString()),
		)
		return
	}

	resourceType, err := stringToACLResourceType(model.ResourceType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource type", utils.DeserializeGrpcError(err))
		return
	}

	resourcePatternType, err := stringToACLResourcePatternType(model.ResourcePatternType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting resource pattern type", utils.DeserializeGrpcError(err))
		return
	}

	operation, err := stringToACLOperation(model.Operation.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting operation", utils.DeserializeGrpcError(err))
		return
	}

	permissionType, err := stringToACLPermissionType(model.PermissionType.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Error converting permission type", utils.DeserializeGrpcError(err))
		return
	}

	filter := &dataplanev1.DeleteACLsRequest_Filter{
		ResourceType:        resourceType,
		ResourceName:        utils.StringToStringPointer(model.ResourceName.ValueString()),
		ResourcePatternType: resourcePatternType,
		Principal:           utils.StringToStringPointer(model.Principal.ValueString()),
		Host:                utils.StringToStringPointer(model.Host.ValueString()),
		Operation:           operation,
		PermissionType:      permissionType,
	}
	err = a.createACLClient(model.ClusterAPIURL.ValueString())
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "ACL", aclID, model.AllowDeletion, err, "create ACL client")
		response.Diagnostics.Append(diags...)
		return
	}
	defer a.dataplaneConn.Close()
	deleteResponse, err := a.ACLClient.DeleteACLs(ctx, &dataplanev1.DeleteACLsRequest{Filter: filter})
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "ACL", aclID, model.AllowDeletion, err, "delete ACL")
		response.Diagnostics.Append(diags...)
		return
	}

	for _, matchingACL := range deleteResponse.MatchingAcls {
		if matchingACL.Error != nil && matchingACL.Error.Code != 0 {
			response.Diagnostics.AddError("Error deleting ACL", matchingACL.Error.Message)
			return
		}
	}
}

func (a *ACL) createACLClient(clusterURL string) error {
	if a.ACLClient != nil { // Client already started, no need to create another one.
		return nil
	}
	if a.dataplaneConn == nil {
		conn, err := cloud.SpawnConn(clusterURL, a.resData.AuthToken, a.resData.ProviderVersion, a.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
		}
		a.dataplaneConn = conn
	}
	a.ACLClient = dataplanev1grpc.NewACLServiceClient(a.dataplaneConn)
	return nil
}
