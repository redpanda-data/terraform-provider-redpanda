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

package schema_registry_acl

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// SchemaRegistryACL represents the Schema Registry ACL Terraform resource.
type SchemaRegistryACL struct {
	CpCl    *cloud.ControlPlaneClientSet
	resData config.Resource
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource              = &SchemaRegistryACL{}
	_ resource.ResourceWithConfigure = &SchemaRegistryACL{}
)

// Metadata returns the metadata for the resource.
func (*SchemaRegistryACL) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_schema_registry_acl"
}

// Configure configures the Schema Registry ACL resource clients
func (s *SchemaRegistryACL) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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
	s.resData = p
	s.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Schema returns the schema for the resource.
func (*SchemaRegistryACL) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceSchemaRegistryACLSchema()
}

func resourceSchemaRegistryACLSchema() schema.Schema {
	return schema.Schema{
		Description: "Resource for managing Redpanda Schema Registry ACLs (Access Control Lists). " +
			"This resource allows you to configure fine-grained access control for Schema Registry resources.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the cluster where the Schema Registry ACL will be created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal": schema.StringAttribute{
				Required:    true,
				Description: "The principal to apply this ACL for (e.g., User:alice or RedpandaRole:admin)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the resource: SUBJECT or REGISTRY",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: schemaRegistryResourceTypeValidator(),
			},
			"resource_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the resource this ACL entry will be on. Use '*' for wildcard",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pattern_type": schema.StringAttribute{
				Required:    true,
				Description: "The pattern type of the resource: LITERAL or PREFIXED",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: schemaRegistryPatternTypeValidator(),
			},
			"host": schema.StringAttribute{
				Required:    true,
				Description: "The host address to use for this ACL. Use '*' for wildcard",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"operation": schema.StringAttribute{
				Required:    true,
				Description: "The operation type that shall be allowed or denied: ALL, READ, WRITE, DELETE, DESCRIBE, DESCRIBE_CONFIGS, ALTER, ALTER_CONFIGS",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: schemaRegistryOperationValidator(),
			},
			"permission": schema.StringAttribute{
				Required:    true,
				Description: "The permission type: ALLOW or DENY",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: schemaRegistryPermissionValidator(),
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Username for authentication. Can be set via REDPANDA_SR_USERNAME environment variable",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password for authentication. Can be set via REDPANDA_SR_PASSWORD environment variable",
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Description: "When set to true, allows the resource to be removed from state even if deletion fails due to permission errors",
			},
		},
	}
}

// Create creates a new Schema Registry ACL resource.
func (s *SchemaRegistryACL) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.SchemaRegistryACL
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := s.getSchemaRegistryClient(ctx, &model)
	if err != nil {
		response.Diagnostics.AddError("Failed to create Schema Registry client", utils.DeserializeGrpcError(err))
		return
	}

	if err := client.CreateACL(ctx, &kclients.SchemaRegistryACLRequest{
		Principal:    model.Principal.ValueString(),
		Resource:     model.ResourceName.ValueString(),
		ResourceType: model.ResourceType.ValueString(),
		PatternType:  model.PatternType.ValueString(),
		Host:         model.Host.ValueString(),
		Operation:    model.Operation.ValueString(),
		Permission:   model.Permission.ValueString(),
	}); err != nil {
		response.Diagnostics.AddError("Failed to create Schema Registry ACL", utils.DeserializeGrpcError(err))
		return
	}

	model.ID = types.StringValue(generateSchemaRegistryACLID(&model))

	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

// Read checks for the existence of a Schema Registry ACL resource
func (s *SchemaRegistryACL) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.SchemaRegistryACL
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	if model.ClusterID.IsNull() || model.ClusterID.IsUnknown() || model.ClusterID.ValueString() == "" {
		response.State.RemoveResource(ctx)
		return
	}

	client, err := s.getSchemaRegistryClient(ctx, &model)
	if err != nil {
		if utils.IsClusterUnreachable(err) || utils.IsPermissionDenied(err) {
			if model.AllowDeletion.IsNull() || model.AllowDeletion.ValueBool() {
				response.State.RemoveResource(ctx)
				return
			}
		}
		response.Diagnostics.AddError("Failed to create Schema Registry client", utils.DeserializeGrpcError(err))
		return
	}

	acls, err := client.ListACLs(ctx, &kclients.SchemaRegistryACLFilter{
		Principal:    model.Principal.ValueString(),
		Resource:     model.ResourceName.ValueString(),
		ResourceType: model.ResourceType.ValueString(),
		PatternType:  model.PatternType.ValueString(),
		Host:         model.Host.ValueString(),
		Operation:    model.Operation.ValueString(),
		Permission:   model.Permission.ValueString(),
	})
	if err != nil {
		if utils.IsClusterUnreachable(err) {
			tflog.Warn(ctx, "Schema Registry ACL read failed due to cluster unreachable, removing from state", map[string]any{
				"principal": model.Principal.ValueString(),
				"resource":  model.ResourceName.ValueString(),
				"error":     err.Error(),
			})
			response.State.RemoveResource(ctx)
			return
		}
		if utils.IsPermissionDenied(err) {
			if !model.AllowDeletion.IsNull() && model.AllowDeletion.ValueBool() {
				tflog.Warn(ctx, "Schema Registry ACL read failed due to permission denied, removing from state", map[string]any{
					"principal":      model.Principal.ValueString(),
					"resource":       model.ResourceName.ValueString(),
					"allow_deletion": model.AllowDeletion.ValueBool(),
					"error":          err.Error(),
				})
				response.State.RemoveResource(ctx)
				return
			}
		}
		response.Diagnostics.AddError("Failed to list Schema Registry ACLs", utils.DeserializeGrpcError(err))
		return
	}

	found := false
	for _, acl := range acls {
		if aclMatches(&acl, &model) {
			found = true
			break
		}
	}

	if !found {
		response.State.RemoveResource(ctx)
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

// Update updates a Schema Registry ACL resource
func (*SchemaRegistryACL) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All fields require replacement, so Update is not needed
}

// Delete deletes a Schema Registry ACL resource
func (s *SchemaRegistryACL) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.SchemaRegistryACL
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := s.getSchemaRegistryClient(ctx, &model)
	if err != nil {
		if utils.IsPermissionDenied(err) || utils.IsClusterUnreachable(err) {
			if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
				response.Diagnostics.AddError(
					"Cannot delete Schema Registry ACL - permission denied or cluster unreachable",
					fmt.Sprintf("Unable to delete Schema Registry ACL because of permission error or cluster is unreachable. Set allow_deletion=true to force removal from state. Error: %v", err),
				)
				return
			}
			tflog.Warn(ctx, "Schema Registry ACL deletion failed due to permission/cluster error during client creation, removing from state", map[string]any{
				"principal":      model.Principal.ValueString(),
				"resource":       model.ResourceName.ValueString(),
				"allow_deletion": model.AllowDeletion.ValueBool(),
				"error":          err.Error(),
			})
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError("Failed to create Schema Registry client", utils.DeserializeGrpcError(err))
		return
	}

	err = client.DeleteACL(ctx, &kclients.SchemaRegistryACLRequest{
		Principal:    model.Principal.ValueString(),
		Resource:     model.ResourceName.ValueString(),
		ResourceType: model.ResourceType.ValueString(),
		PatternType:  model.PatternType.ValueString(),
		Host:         model.Host.ValueString(),
		Operation:    model.Operation.ValueString(),
		Permission:   model.Permission.ValueString(),
	})
	if err != nil {
		if utils.IsPermissionDenied(err) || utils.IsNotFound(err) {
			if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
				response.Diagnostics.AddError(
					"Cannot delete Schema Registry ACL - permission denied",
					fmt.Sprintf("Unable to delete Schema Registry ACL due to permission error. Set allow_deletion=true to force removal from state. Error: %v", err),
				)
				return
			}
			tflog.Warn(ctx, "Schema Registry ACL deletion failed but removing from state", map[string]any{
				"principal":      model.Principal.ValueString(),
				"resource":       model.ResourceName.ValueString(),
				"allow_deletion": model.AllowDeletion.ValueBool(),
				"error":          err.Error(),
			})
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError("Failed to delete Schema Registry ACL", utils.DeserializeGrpcError(err))
		return
	}

	response.State.RemoveResource(ctx)
}

func (s *SchemaRegistryACL) getSchemaRegistryClient(ctx context.Context, model *models.SchemaRegistryACL) (*kclients.SchemaRegistryACLClient, error) {
	return kclients.NewSchemaRegistryACLClient(ctx, s.CpCl, model.ClusterID.ValueString(), model.Username.ValueString(), model.Password.ValueString())
}

func generateSchemaRegistryACLID(model *models.SchemaRegistryACL) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s",
		model.ClusterID.ValueString(),
		model.Principal.ValueString(),
		model.ResourceType.ValueString(),
		model.ResourceName.ValueString(),
		model.PatternType.ValueString(),
		model.Host.ValueString(),
		model.Operation.ValueString(),
		model.Permission.ValueString())
}

func aclMatches(acl *kclients.SchemaRegistryACLResponse, model *models.SchemaRegistryACL) bool {
	return acl.Principal == model.Principal.ValueString() &&
		acl.Resource == model.ResourceName.ValueString() &&
		acl.ResourceType == model.ResourceType.ValueString() &&
		acl.PatternType == model.PatternType.ValueString() &&
		acl.Host == model.Host.ValueString() &&
		acl.Operation == model.Operation.ValueString() &&
		acl.Permission == model.Permission.ValueString()
}
