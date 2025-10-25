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
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// SchemaRegistryACLClientFactory is a function type for creating Schema Registry ACL clients
type SchemaRegistryACLClientFactory func(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID, username, password string) (kclients.SchemaRegistryACLClientInterface, error)

// SchemaRegistryACL represents the Schema Registry ACL Terraform resource.
type SchemaRegistryACL struct {
	CpCl          *cloud.ControlPlaneClientSet
	resData       config.Resource
	clientFactory SchemaRegistryACLClientFactory
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &SchemaRegistryACL{}
	_ resource.ResourceWithConfigure   = &SchemaRegistryACL{}
	_ resource.ResourceWithImportState = &SchemaRegistryACL{}
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

	if s.clientFactory == nil {
		s.clientFactory = func(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID, username, password string) (kclients.SchemaRegistryACLClientInterface, error) {
			return kclients.NewSchemaRegistryACLClient(ctx, cpCl, clusterID, username, password)
		}
	}
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

	// Verify ACL propagation to prevent schema creation failures
	if err := s.verifyACLPropagation(ctx, client, &model); err != nil {
		response.Diagnostics.AddWarning(
			"ACL created but verification failed",
			fmt.Sprintf("The ACL was created but may not be immediately usable: %v", err),
		)
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
func (*SchemaRegistryACL) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var model models.SchemaRegistryACL
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
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

type importIDComponents struct {
	clusterID    string
	principal    string
	resourceType string
	resourceName string
	patternType  string
	host         string
	operation    string
	permission   string
	username     string
	password     string
}

func parseImportID(importID string) (*importIDComponents, error) {
	parts := strings.Split(importID, ":")
	if len(parts) < 10 {
		return nil, fmt.Errorf("expected format: cluster_id:principal:resource_type:resource_name:pattern_type:host:operation:permission:username:password, got %d parts (expected at least 10)", len(parts))
	}

	return &importIDComponents{
		clusterID:    parts[0],
		principal:    strings.Join(parts[1:len(parts)-8], ":"),
		resourceType: parts[len(parts)-8],
		resourceName: parts[len(parts)-7],
		patternType:  parts[len(parts)-6],
		host:         parts[len(parts)-5],
		operation:    parts[len(parts)-4],
		permission:   parts[len(parts)-3],
		username:     parts[len(parts)-2],
		password:     parts[len(parts)-1],
	}, nil
}

// ImportState imports a Schema Registry ACL resource from a colon-separated ID string
func (*SchemaRegistryACL) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	components, err := parseImportID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid import format", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("cluster_id"), components.clusterID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("principal"), components.principal)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("resource_type"), components.resourceType)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("resource_name"), components.resourceName)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("pattern_type"), components.patternType)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("host"), components.host)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("operation"), components.operation)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("permission"), components.permission)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("username"), components.username)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("password"), components.password)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), request.ID)...)
}

func (s *SchemaRegistryACL) getSchemaRegistryClient(ctx context.Context, model *models.SchemaRegistryACL) (kclients.SchemaRegistryACLClientInterface, error) {
	return s.clientFactory(ctx, s.CpCl, model.ClusterID.ValueString(), model.Username.ValueString(), model.Password.ValueString())
}

// verifyACLPropagation verifies that the ACL has been propagated and is ready for use.
// ACLs can report as created but not be immediately usable due to eventual consistency.
func (*SchemaRegistryACL) verifyACLPropagation(ctx context.Context, client kclients.SchemaRegistryACLClientInterface, model *models.SchemaRegistryACL) error {
	timeout := 30 * time.Second
	startTime := time.Now()
	attempt := 0

	tflog.Info(ctx, "Verifying Schema Registry ACL propagation", map[string]any{
		"principal": model.Principal.ValueString(),
		"resource":  model.ResourceName.ValueString(),
		"timeout":   timeout.String(),
	})

	return utils.Retry(ctx, timeout, func() *utils.RetryError {
		attempt++
		elapsed := time.Since(startTime)

		tflog.Debug(ctx, "ACL verification attempt", map[string]any{
			"attempt": attempt,
			"elapsed": elapsed.String(),
		})

		acls, err := client.ListACLs(ctx, model.ToSchemaRegistryACLFilter())
		if err != nil {
			// Permission denied errors during verification often indicate
			// the ACL hasn't propagated yet
			if utils.IsPermissionDenied(err) {
				tflog.Debug(ctx, "ACL verification permission denied, retrying", map[string]any{
					"error":   err.Error(),
					"attempt": attempt,
				})
				return utils.RetryableError(fmt.Errorf("ACL not yet propagated (permission denied): %w", err))
			}

			// Other errors are not retryable
			tflog.Error(ctx, "Non-retryable error during ACL verification", map[string]any{
				"error":   err.Error(),
				"attempt": attempt,
			})
			return utils.NonRetryableError(fmt.Errorf("failed to verify ACL: %w", err))
		}

		// Check if our ACL is in the list
		found := false
		for _, acl := range acls {
			if model.MatchesACLResponse(&acl) {
				found = true
				break
			}
		}

		if !found {
			tflog.Debug(ctx, "ACL not found in list, retrying", map[string]any{
				"acl_count": len(acls),
				"attempt":   attempt,
				"elapsed":   elapsed.String(),
			})
			return utils.RetryableError(fmt.Errorf("ACL not yet visible in list (found %d ACLs)", len(acls)))
		}

		tflog.Info(ctx, "ACL verification successful", map[string]any{
			"attempts":   attempt,
			"total_time": elapsed.String(),
			"acls_found": len(acls),
		})
		return nil
	})
}
