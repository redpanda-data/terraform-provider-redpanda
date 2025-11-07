// Package schema contains the implementation of the Schema resource
package schema

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	schemamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/twmb/franz-go/pkg/sr"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Schema{}
	_ resource.ResourceWithConfigure   = &Schema{}
	_ resource.ResourceWithImportState = &Schema{}
)

// Schema represents a schema managed resource
type Schema struct {
	CpCl    *cloud.ControlPlaneClientSet
	resData config.Resource
}

// Configure configures the schema resource with provider data.
func (s *Schema) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	cc, ok := request.ProviderData.(config.Resource)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	s.resData = cc
	s.CpCl = cloud.NewControlPlaneClientSet(cc.ControlPlaneConnection)
}

// importIDComponents holds the parsed components from an import ID string
type importIDComponents struct {
	clusterID string
	subject   string
	version   int64
	username  string
	password  string
}

// parseImportID parses the import ID string into its components
// Expected format: "cluster_id:subject:version:username:password"
func parseImportID(importID string) (*importIDComponents, error) {
	parts := strings.Split(importID, ":")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid import ID format: expected cluster_id:subject:version:username:password, got %d parts (expected 5)", len(parts))
	}

	clusterID := parts[0]
	subject := parts[1]
	versionStr := parts[2]
	username := parts[3]
	password := parts[4]

	// Parse version to int64
	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("version must be a valid integer: %w", err)
	}

	return &importIDComponents{
		clusterID: clusterID,
		subject:   subject,
		version:   version,
		username:  username,
		password:  password,
	}, nil
}

// ImportState imports an existing schema resource using cluster_id:subject:version:username:password format.
func (*Schema) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	components, err := parseImportID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid import format", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("cluster_id"), types.StringValue(components.clusterID))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("subject"), types.StringValue(components.subject))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("version"), types.Int64Value(components.version))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("username"), types.StringValue(components.username))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("password"), types.StringValue(components.password))...)
}

// Metadata returns the resource metadata.
func (*Schema) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_schema"
}

// Schema returns the resource schema definition.
func (*Schema) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceSchemaSchema()
}

// Create creates a new schema in the Schema Registry.
func (s *Schema) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan schemamodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, s.CpCl, plan.ClusterID.ValueString(), plan.Username.ValueString(), plan.Password.ValueString())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", plan.ClusterID.ValueString(), err),
		)
		return
	}

	schemaResp, err := client.CreateSchema(ctx, plan.Subject.ValueString(), plan.ToSchemaRequest())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to create schema",
			fmt.Sprintf("Unable to create schema for subject %s: %v", plan.Subject.ValueString(), err),
		)
		return
	}

	plan.ID = types.Int64Value(int64(schemaResp.ID))
	plan.Version = types.Int64Value(int64(schemaResp.Version))

	// Set compatibility level if specified
	if !plan.Compatibility.IsNull() && !plan.Compatibility.IsUnknown() {
		err = kclients.SetSubjectCompatibility(ctx, client, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			response.Diagnostics.AddError(
				"Failed to set compatibility level",
				fmt.Sprintf("Unable to set compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
			)
			return
		}
	} else {
		// If compatibility is not specified, get the current compatibility level
		compatibility, err := kclients.GetSubjectCompatibility(ctx, client, plan.Subject.ValueString())
		if err != nil {
			// Log warning but don't fail the resource creation
			tflog.Warn(ctx, "Failed to get compatibility level", map[string]any{
				"subject": plan.Subject.ValueString(),
				"error":   err.Error(),
			})
			plan.Compatibility = types.StringValue(kclients.DefaultCompatibilityLevel)
		} else {
			plan.Compatibility = types.StringValue(compatibility)
		}
	}

	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (s *Schema) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state schemamodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	if state.ClusterID.IsNull() || state.ClusterID.IsUnknown() || state.ClusterID.ValueString() == "" {
		response.State.RemoveResource(ctx)
		return
	}

	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, s.CpCl, state.ClusterID.ValueString(), state.Username.ValueString(), state.Password.ValueString())
	if err != nil {
		if utils.IsClusterUnreachable(err) || utils.IsPermissionDenied(err) {
			if state.AllowDeletion.IsNull() || state.AllowDeletion.ValueBool() {
				response.State.RemoveResource(ctx)
				return
			}
		}
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", state.ClusterID.ValueString(), err),
		)
		return
	}

	schemaResp, err := kclients.FetchSchema(ctx, client, state.GetSubject(), state.GetVersion())
	if err != nil {
		tflog.Debug(ctx, "Schema read error encountered", map[string]any{
			"subject":              state.GetSubject(),
			"error":                err.Error(),
			"is_not_found":         utils.IsNotFound(err),
			"is_permission_denied": utils.IsPermissionDenied(err),
		})

		if utils.IsNotFound(err) {
			tflog.Debug(ctx, "Schema read failed due to not found, removing from state")
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(
			"Failed to read schema",
			fmt.Sprintf("Unable to read schema for subject %s: %v", state.GetSubject(), err),
		)
		return
	}

	state.UpdateFromSchema(schemaResp)

	// Get compatibility level for the subject
	compatibility, err := kclients.GetSubjectCompatibility(ctx, client, state.Subject.ValueString())
	if err != nil {
		tflog.Warn(ctx, "Failed to get compatibility level", map[string]any{
			"subject": state.Subject.ValueString(),
			"error":   err.Error(),
		})
		// Don't fail the read, just use the existing value or default
		if state.Compatibility.IsNull() {
			state.Compatibility = types.StringValue(kclients.DefaultCompatibilityLevel)
		}
	} else {
		state.Compatibility = types.StringValue(compatibility)
	}

	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

// Update creates a new version of an existing schema.
func (s *Schema) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan schemamodel.ResourceModel
	var state schemamodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, s.CpCl, plan.ClusterID.ValueString(), plan.Username.ValueString(), plan.Password.ValueString())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", plan.ClusterID.ValueString(), err),
		)
		return
	}

	// Check if the schema has actually changed by comparing the normalized versions
	planReq := plan.ToSchemaRequest()
	stateReq := state.ToSchemaRequest()

	// Compare schema content, type, and references
	if planReq.Schema == stateReq.Schema &&
		planReq.Type == stateReq.Type &&
		len(planReq.References) == len(stateReq.References) {
		// Check if references are identical
		referencesEqual := true
		for i, planRef := range planReq.References {
			if i >= len(stateReq.References) ||
				planRef.Name != stateReq.References[i].Name ||
				planRef.Subject != stateReq.References[i].Subject ||
				planRef.Version != stateReq.References[i].Version {
				referencesEqual = false
				break
			}
		}

		if referencesEqual {
			// Schema hasn't changed, just update state with current values
			plan.UpdateFromSchema(sr.SubjectSchema{
				ID:      int(state.ID.ValueInt64()),
				Version: int(state.Version.ValueInt64()),
				Schema:  planReq,
				Subject: state.Subject.ValueString(),
			})

			// Check if only compatibility changed
			if plan.Compatibility.ValueString() != state.Compatibility.ValueString() && !plan.Compatibility.IsNull() && !plan.Compatibility.IsUnknown() {
				err = kclients.SetSubjectCompatibility(ctx, client, plan.Subject.ValueString(), plan.Compatibility.ValueString())
				if err != nil {
					response.Diagnostics.AddError(
						"Failed to update compatibility level",
						fmt.Sprintf("Unable to update compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
					)
					return
				}
			}

			response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
			return
		}
	}

	// Schema has changed, create new version
	schemaResp, err := client.CreateSchema(ctx, plan.Subject.ValueString(), planReq)
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to update schema",
			fmt.Sprintf("Unable to create new version of schema for subject %s: %v", plan.Subject.ValueString(), err),
		)
		return
	}

	plan.UpdateFromSchema(schemaResp)

	// Update compatibility level if it changed
	if plan.Compatibility.ValueString() != state.Compatibility.ValueString() && !plan.Compatibility.IsNull() && !plan.Compatibility.IsUnknown() {
		err = kclients.SetSubjectCompatibility(ctx, client, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			response.Diagnostics.AddError(
				"Failed to update compatibility level",
				fmt.Sprintf("Unable to update compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
			)
			return
		}
	}

	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

// Delete removes a schema from the Schema Registry.
func (s *Schema) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state schemamodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, s.CpCl, state.ClusterID.ValueString(), state.Username.ValueString(), state.Password.ValueString())
	if err != nil {
		if utils.IsPermissionDenied(err) || utils.IsClusterUnreachable(err) {
			if !state.AllowDeletion.IsNull() && !state.AllowDeletion.ValueBool() {
				response.Diagnostics.AddError(
					"Cannot delete schema - permission denied or cluster unreachable",
					fmt.Sprintf("Unable to delete schema because of permission error or cluster is unreachable. Set allow_deletion=true to force removal from state. Error: %v", err),
				)
				return
			}
			tflog.Warn(ctx, "Schema deletion failed due to permission/cluster error during client creation, removing from state", map[string]any{
				"subject":        state.GetSubject(),
				"allow_deletion": state.AllowDeletion.ValueBool(),
				"error":          err.Error(),
			})
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", state.ClusterID.ValueString(), err),
		)
		return
	}

	_, err = client.DeleteSubject(ctx, state.GetSubject(), sr.SoftDelete)
	if err != nil {
		if !utils.IsNotFound(err) {
			if utils.IsClusterUnreachable(err) || utils.IsPermissionDenied(err) {
				if !state.AllowDeletion.IsNull() && !state.AllowDeletion.ValueBool() {
					response.Diagnostics.AddError(
						"Cannot delete schema - cluster unreachable or permission denied",
						fmt.Sprintf("Unable to delete schema subject %s. Set allow_deletion=true to force removal from state. Error: %v", state.GetSubject(), err),
					)
					return
				}
				tflog.Warn(ctx, "Schema deletion failed but removing from state", map[string]any{
					"subject":        state.GetSubject(),
					"allow_deletion": state.AllowDeletion.ValueBool(),
					"error":          err.Error(),
				})
				response.State.RemoveResource(ctx)
				return
			}
			response.Diagnostics.AddError(
				"Failed to delete schema",
				fmt.Sprintf("Unable to delete schema subject %s: %v", state.GetSubject(), err),
			)
			return
		}
	}
	response.State.RemoveResource(ctx)
}
