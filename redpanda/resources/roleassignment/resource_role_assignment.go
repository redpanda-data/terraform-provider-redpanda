package roleassignment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// RoleAssignment implements the resource interface for role assignments.
type RoleAssignment struct {
	base.ResourceBase

	resData        config.Resource
	SecurityClient consolev1alpha1grpc.SecurityServiceClient
}

// NewRoleAssignment creates a new instance of the role assignment resource.
func NewRoleAssignment() *RoleAssignment {
	r := &RoleAssignment{}
	r.ResourceBase = base.NewResourceBase(
		"redpanda_role_assignment",
		ResourceRoleAssignmentSchema,
		func(p config.Resource) { r.resData = p },
	)
	return r
}

// ResourceRoleAssignmentSchema returns the schema for the role assignment resource.
func ResourceRoleAssignmentSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Assigns existing Redpanda roles to principals. Requires an existing role and user.",
		Attributes: map[string]schema.Attribute{
			"role_name": schema.StringAttribute{
				MarkdownDescription: "The name of the role to assign",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal": schema.StringAttribute{
				MarkdownDescription: "The principal to assign the role to. Use the Kafka-style prefixed form: `\"User:<name>\"` for an end user or `\"Group:<name>\"` for an IdP group. The value is preserved verbatim in state.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.PrincipalPrefix(),
				},
			},
			"cluster_api_url": schema.StringAttribute{
				MarkdownDescription: "The cluster API URL. Changing this will prevent deletion of the resource on the existing cluster",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of this resource. Format: `{role_name}:{principal}`",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create creates a new role assignment.
func (r *RoleAssignment) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.RoleAssignment
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := model.RoleName.ValueString()
	principal := model.Principal.ValueString()
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}

	// Add the principal to the role
	dataplaneReq := &dataplanev1.UpdateRoleMembershipRequest{
		RoleName: roleName,
		Add:      []*dataplanev1.RoleMembership{{Principal: principal}},
	}
	consoleReq := &consolev1alpha1.UpdateRoleMembershipRequest{
		Request: dataplaneReq,
	}

	_, err := r.SecurityClient.UpdateRoleMembership(ctx, consoleReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to assign role", utils.DeserializeGrpcError(err))
		return
	}

	// Principal is preserved as the user supplied it (the API accepts the
	// "User:" and "Group:" prefixed forms verbatim).
	model.ID = types.StringValue(fmt.Sprintf("%s:%s", roleName, principal))
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Read reads the role assignment state
func (r *RoleAssignment) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.RoleAssignment
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := model.RoleName.ValueString()
	principal := model.Principal.ValueString()
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	// Validate required fields
	if roleName == "" || principal == "" {
		resp.Diagnostics.AddError(
			"Invalid state data",
			fmt.Sprintf("Missing required fields - role_name: '%s', principal: '%s'", roleName, principal),
		)
		return
	}

	// Self-heal legacy state: a pre-validator provider may have written a
	// bare principal (e.g. "alice") into state. The backend canonicalizes
	// to "User:alice", so verbatim comparison would falsely report the
	// membership missing. Canonicalize once here, use it for both lookup
	// and the state writeback so the next refresh sees a canonical value.
	canonical := validators.CanonicalizePrincipal(principal)

	// If cluster_api_url is empty (e.g., during import), skip the API check
	// The next plan/apply will validate the resource exists
	if clusterAPIURL == "" {
		tflog.Debug(ctx, "Skipping API check due to empty cluster_api_url (likely during import)")
		if canonical != principal {
			model.Principal = types.StringValue(canonical)
			model.ID = types.StringValue(fmt.Sprintf("%s:%s", roleName, canonical))
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
		return
	}

	// Create SecurityService client and verify the role assignment exists
	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}

	exists, err := r.roleAssignmentExists(ctx, roleName, canonical)
	if err != nil {
		// Don't remove from state on API errors - just fail the read
		resp.Diagnostics.AddError(
			"Failed to verify role assignment",
			fmt.Sprintf("Could not check if role assignment %s:%s exists: %s", roleName, canonical, utils.DeserializeGrpcError(err)),
		)
		return
	}

	if !exists {
		tflog.Warn(ctx, "Role assignment not found, removing from state", map[string]any{
			"role_name": roleName,
			"principal": canonical,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	if canonical != principal {
		model.Principal = types.StringValue(canonical)
		model.ID = types.StringValue(fmt.Sprintf("%s:%s", roleName, canonical))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Update updates the role assignment (not supported - requires replace)
func (*RoleAssignment) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// Role assignments are immutable - updates require replacement
}

// Delete deletes a role assignment.
func (r *RoleAssignment) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.RoleAssignment
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := model.RoleName.ValueString()
	// Canonicalize for the same reason as Read: legacy bare-form state
	// would send a bare principal to UpdateRoleMembership.Remove, which
	// the backend may reject because membership is keyed by the canonical
	// "User:<name>" form.
	principal := validators.CanonicalizePrincipal(model.Principal.ValueString())
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}

	// Remove the principal from the role
	dataplaneReq := &dataplanev1.UpdateRoleMembershipRequest{
		RoleName: roleName,
		Remove:   []*dataplanev1.RoleMembership{{Principal: principal}},
	}
	consoleReq := &consolev1alpha1.UpdateRoleMembershipRequest{
		Request: dataplaneReq,
	}

	_, err := r.SecurityClient.UpdateRoleMembership(ctx, consoleReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to unassign role", utils.DeserializeGrpcError(err))
		return
	}

	// Remove resource from state
	resp.State.RemoveResource(ctx)
}

// ImportState parses a composite import ID of the form
// "<role_name>:<principal>[|<cluster_api_url>]". The trailing |URL is optional
// but recommended: cluster_api_url is RequiresReplace and is not recoverable
// from the server-side membership response. "|" avoids collision with ":" in
// prefixed principals (e.g. "User:alice").
func (*RoleAssignment) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	rest, clusterAPIURL, _ := strings.Cut(req.ID, "|")
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: <role_name>:<principal>[|<cluster_api_url>] — e.g. developer:User:alice|https://api.region.redpanda.com",
		)
		return
	}

	roleName := parts[0]
	principal := parts[1]

	model := models.RoleAssignment{
		RoleName:  types.StringValue(roleName),
		Principal: types.StringValue(principal),
		ID:        types.StringValue(fmt.Sprintf("%s:%s", roleName, principal)),
	}
	if clusterAPIURL != "" {
		model.ClusterAPIURL = types.StringValue(clusterAPIURL)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// roleAssignmentExists checks if a role assignment exists
func (r *RoleAssignment) roleAssignmentExists(ctx context.Context, roleName, principal string) (bool, error) {
	// Create request to list role members
	dataplaneReq := &dataplanev1.ListRoleMembersRequest{
		RoleName: roleName,
	}
	consoleReq := &consolev1alpha1.ListRoleMembersRequest{
		Request: dataplaneReq,
	}

	// Execute the request
	resp, err := r.SecurityClient.ListRoleMembers(ctx, consoleReq)
	if err != nil {
		// Check for common "not found" errors
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "notfound") ||
			strings.Contains(errStr, "does not exist") ||
			strings.Contains(errStr, "unknown role") {
			return false, nil
		}
		// For other errors, we should fail rather than assume the assignment doesn't exist
		return false, fmt.Errorf("failed to list role members for role '%s': %w", roleName, err)
	}

	// Check if the principal is in the role members.
	// Principal comparison is verbatim — both client and server use the same
	// "User:" / "Group:" prefixed form.
	if resp.Response != nil && resp.Response.Members != nil {
		for _, member := range resp.Response.Members {
			if member.Principal == principal {
				return true, nil
			}
		}
	}

	return false, nil
}

// createSecurityClient creates a SecurityService client
func (r *RoleAssignment) createSecurityClient(ctx context.Context, clusterURL string) error {
	if r.SecurityClient != nil {
		return nil
	}

	if r.resData.DataplaneConnPool == nil {
		return errors.New("provider not configured: dataplane connection pool is nil")
	}
	consoleURL := utils.ConvertToConsoleURL(clusterURL)
	conn, err := r.resData.DataplaneConnPool.GetConnection(ctx, consoleURL)
	if err != nil {
		return fmt.Errorf("unable to open a connection with the console API at %s: %v", consoleURL, err)
	}

	r.SecurityClient = consolev1alpha1grpc.NewSecurityServiceClient(conn)
	return nil
}
