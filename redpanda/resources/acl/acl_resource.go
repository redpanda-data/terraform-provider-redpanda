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

type Acl struct {
	AclClient dataplanev1alpha1.ACLServiceClient
}

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Acl{}
var _ resource.ResourceWithConfigure = &Acl{}
var _ resource.ResourceWithImportState = &Acl{}

func (a *Acl) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_acl"
}

func (a *Acl) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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

	client, err := clients.NewACLServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create cluster client", err.Error())
		return
	}
	a.AclClient = client
}

func (a *Acl) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = ResourceAclSchema()
}

func ResourceAclSchema() schema.Schema {
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

func (a *Acl) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.Acl
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

	_, err = a.AclClient.CreateACL(ctx, &dataplanev1alpha1.CreateACLRequest{
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

	response.Diagnostics.Append(response.State.Set(ctx, &models.Acl{
		ResourceType:        model.ResourceType,
		ResourceName:        model.ResourceName,
		ResourcePatternType: model.ResourcePatternType,
		Principal:           model.Principal,
		Host:                model.Host,
		Operation:           model.Operation,
		PermissionType:      model.PermissionType,
	})...)
}

func (a *Acl) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.Acl
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

	aclList, err := a.AclClient.ListACLs(ctx, &dataplanev1alpha1.ListACLsRequest{Filter: filter})
	if err != nil {
		response.Diagnostics.AddError("Failed to list ACLs", err.Error())
		return
	}

	for _, res := range aclList.Resources {
		if res.ResourceName == model.ResourceName.ValueString() && res.ResourceType == resourceType && res.ResourcePatternType == resourcePatternType {
			response.Diagnostics.Append(response.State.Set(ctx, &models.Acl{
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

func (a *Acl) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (a *Acl) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.Acl
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

	deleteResponse, err := a.AclClient.DeleteACLs(ctx, &dataplanev1alpha1.DeleteACLsRequest{Filter: filter})
	if err != nil {
		response.Diagnostics.AddError("Failed to delete ACL", err.Error())
		return
	}

	// Check for errors in the response
	for _, matchingAcl := range deleteResponse.MatchingAcls {
		if matchingAcl.ErrorCode != 0 {
			response.Diagnostics.AddError("Error deleting ACL", matchingAcl.ErrorMessage)
			return
		}
	}

	// Remove the resource from state
	response.State.RemoveResource(ctx)
}

func (a *Acl) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(ctx, &models.Cluster{
		ID: types.StringValue(request.ID),
	})...)
}
