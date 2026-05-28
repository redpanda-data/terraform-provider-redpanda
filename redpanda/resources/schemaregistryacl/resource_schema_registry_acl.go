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

package schemaregistryacl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	schemaregistryaclmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schemaregistryacl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// SchemaRegistryACLClientFactory is a function type for creating Schema Registry
// ACL clients. authToken is the provider's cloud-issued Bearer token, used when
// explicit Basic creds (username+password) are absent.
//
//nolint:revive // intentional naming for clarity
type SchemaRegistryACLClientFactory func(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID, authToken, username, password string) (kclients.SchemaRegistryACLClientInterface, error)

// SchemaRegistryACL represents the Schema Registry ACL Terraform resource.
type SchemaRegistryACL struct {
	base.ResourceBase

	resData       config.Resource
	clientFactory SchemaRegistryACLClientFactory
}

// NewSchemaRegistryACL constructs a SchemaRegistryACL resource.
func NewSchemaRegistryACL() *SchemaRegistryACL {
	s := &SchemaRegistryACL{}
	s.ResourceBase = base.NewResourceBase(
		"redpanda_schema_registry_acl",
		ResourceSchemaRegistryACLSchema,
		func(p config.Resource) {
			s.resData = p
			if s.clientFactory == nil {
				s.clientFactory = func(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID, authToken, username, password string) (kclients.SchemaRegistryACLClientInterface, error) {
					return kclients.NewSchemaRegistryACLClient(ctx, cpCl, clusterID, authToken, username, password)
				}
			}
		},
	)
	return s
}

var (
	_ resource.Resource                = &SchemaRegistryACL{}
	_ resource.ResourceWithConfigure   = &SchemaRegistryACL{}
	_ resource.ResourceWithImportState = &SchemaRegistryACL{}
)

// Create creates a new Schema Registry ACL resource.
func (s *SchemaRegistryACL) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model schemaregistryaclmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	var cfg schemaregistryaclmodel.ResourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &cfg)...)
	if response.Diagnostics.HasError() {
		return
	}
	model.PasswordWO = cfg.PasswordWO

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
	var model schemaregistryaclmodel.ResourceModel
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
		action, diags := utils.HandleGracefulRemoval(ctx, "schema registry ACL", model.GenerateID(), model.AllowDeletion, err, "create schema registry client")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	acls, err := client.ListACLs(ctx, model.ToSchemaRegistryACLFilter())
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "schema registry ACL", model.GenerateID(), model.AllowDeletion, err, "list schema registry ACLs")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
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
		action, diags := utils.HandleGracefulRemoval(ctx, "schema registry ACL", model.GenerateID(), model.AllowDeletion, utils.NotFoundError{Message: "schema registry ACL not found"}, "find schema registry ACL")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	if model.ID.IsNull() || model.ID.IsUnknown() {
		model.ID = types.StringValue(model.GenerateID())
	}

	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

// Update updates a Schema Registry ACL resource
func (*SchemaRegistryACL) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var model schemaregistryaclmodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

// Delete deletes a Schema Registry ACL resource
func (s *SchemaRegistryACL) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model schemaregistryaclmodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := s.getSchemaRegistryClient(ctx, &model)
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "schema registry ACL", model.GenerateID(), model.AllowDeletion, err, "create schema registry client")
		response.Diagnostics.Append(diags...)
		return
	}

	if err := client.DeleteACL(ctx, model.ToSchemaRegistryACLRequest()); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "schema registry ACL", model.GenerateID(), model.AllowDeletion, err, "delete schema registry ACL")
		response.Diagnostics.Append(diags...)
		return
	}
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
	parts := strings.Split(importID, ",")
	if len(parts) != 8 && len(parts) != 10 {
		return nil, fmt.Errorf(
			"expected one of:\n"+
				"  Bearer auth (default): cluster_id,principal,resource_type,resource_name,pattern_type,host,operation,permission\n"+
				"  Basic auth (optional): cluster_id,principal,resource_type,resource_name,pattern_type,host,operation,permission,username,password\n"+
				"got %d parts",
			len(parts),
		)
	}
	ret := &importIDComponents{
		clusterID:    parts[0],
		principal:    parts[1],
		resourceType: parts[2],
		resourceName: parts[3],
		patternType:  parts[4],
		host:         parts[5],
		operation:    parts[6],
		permission:   parts[7],
	}
	if len(parts) == 10 {
		ret.username = parts[8]
		ret.password = parts[9]
	}
	return ret, nil
}

// ImportState imports a Schema Registry ACL resource.
//
// Bearer auth (default): cluster_id,principal,resource_type,resource_name,pattern_type,host,operation,permission
// Basic auth (optional): same 8 fields + ,username,password
//
// For Basic auth, password can also be set via REDPANDA_IMPORT_PASSWORD env var.
func (*SchemaRegistryACL) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	components, err := parseImportID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid import format", err.Error()+". Password can also be set via REDPANDA_IMPORT_PASSWORD env var.")
		return
	}

	password := components.password
	if envPassword := os.Getenv("REDPANDA_IMPORT_PASSWORD"); envPassword != "" {
		password = envPassword
	}

	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("cluster_id"), components.clusterID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("principal"), components.principal)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("resource_type"), components.resourceType)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("resource_name"), components.resourceName)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("pattern_type"), components.patternType)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("host"), components.host)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("operation"), components.operation)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("permission"), components.permission)...)
	// Bearer-default: username and password stay null in state when the
	// import ID didn't include them. Only write to state when supplied
	// (Basic auth path).
	if components.username != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("username"), components.username)...)
	}
	if password != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("password"), password)...)
	}
	id := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s",
		components.clusterID,
		components.principal,
		components.resourceType,
		components.resourceName,
		components.patternType,
		components.host,
		components.operation,
		components.permission,
	)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), id)...)
	response.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceSchemaRegistryACLSchema(ctx), &response.State, "allow_deletion")...)
}

func (s *SchemaRegistryACL) getSchemaRegistryClient(ctx context.Context, model *schemaregistryaclmodel.ResourceModel) (kclients.SchemaRegistryACLClientInterface, error) {
	return s.clientFactory(ctx, s.CpCl, model.ClusterID.ValueString(), s.resData.AuthToken, model.Username.ValueString(), model.GetEffectivePassword())
}

// verifyACLPropagation verifies that the ACL has been propagated and is ready for use.
// ACLs can report as created but not be immediately usable due to eventual consistency.
func (*SchemaRegistryACL) verifyACLPropagation(ctx context.Context, client kclients.SchemaRegistryACLClientInterface, model *schemaregistryaclmodel.ResourceModel) error {
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
