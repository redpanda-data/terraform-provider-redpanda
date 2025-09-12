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

	if err := client.CreateACL(ctx, model.ToSchemaRegistryACLRequest()); err != nil {
		response.Diagnostics.AddError("Failed to create Schema Registry ACL", utils.DeserializeGrpcError(err))
		return
	}

	model.ID = types.StringValue(model.GenerateID())

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

	acls, err := client.ListACLs(ctx, model.ToSchemaRegistryACLFilter())
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
		if model.MatchesACLResponse(&acl) {
			found = true
			break
		}
	}

	if !found {
		response.State.RemoveResource(ctx)
		return
	}

	if model.ID.IsNull() || model.ID.IsUnknown() {
		model.ID = types.StringValue(model.GenerateID())
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

	if err := client.DeleteACL(ctx, model.ToSchemaRegistryACLRequest()); err != nil {
		if utils.IsPermissionDenied(err) || utils.IsNotFound(err) {
			if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
				response.Diagnostics.AddError(
					"Cannot delete Schema Registry ACL - permission denied",
					fmt.Sprintf("Unable to delete Schema Registry ACL due to permission error. Set allow_deletion=true to force removal from state. Error: %v", err),
				)
				return
			}
			// This is relevant in situations where the cluster isn't present as otherwise we enter a hung state where we can't
			// delete the SRs (because there's no cluster on which to delete them) so can't ever successfully conclude the terraform operation
			tflog.Warn(ctx, "Schema Registry ACL deletion failed due to missing cluster but removing from state as allow_deletion is true", map[string]any{
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
