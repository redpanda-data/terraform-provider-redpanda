package roleassignment

import (
	"context"
	"fmt"
	"strings"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	consolev1alpha1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/console/v1alpha1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// RoleAssignment implements the resource interface for role assignments.
type RoleAssignment struct {
	resData        config.Resource
	SecurityClient consolev1alpha1grpc.SecurityServiceClient
	dataplaneConn  *grpc.ClientConn
}

// NewRoleAssignment creates a new instance of the role assignment resource.
func NewRoleAssignment() resource.Resource {
	return &RoleAssignment{}
}

// Metadata returns the metadata for the role assignment resource.
func (*RoleAssignment) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_assignment"
}

// Schema returns the schema for the role assignment resource.
func (*RoleAssignment) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
				MarkdownDescription: "The principal to assign the role to. Specify just the username (e.g., `\"john.doe\"`)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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

// Configure configures the role assignment resource.
func (r *RoleAssignment) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	resData, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected config.Resource, got: %T.", req.ProviderData))
		return
	}
	r.resData = resData
}

// Create creates a new role assignment.
func (r *RoleAssignment) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.RoleAssignment
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := model.RoleName.ValueString()
	principal := normalizePrincipal(model.Principal.ValueString())
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}
	defer r.dataplaneConn.Close()

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

	// Store the normalized principal and set the ID
	model.Principal = types.StringValue(principal)
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
	principal := model.Principal.ValueString() // Already normalized when stored
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	// Validate required fields
	if roleName == "" || principal == "" {
		resp.Diagnostics.AddError(
			"Invalid state data",
			fmt.Sprintf("Missing required fields - role_name: '%s', principal: '%s'", roleName, principal),
		)
		return
	}

	// If cluster_api_url is empty (e.g., during import), skip the API check
	// The next plan/apply will validate the resource exists
	if clusterAPIURL == "" {
		tflog.Info(ctx, "Skipping API check due to empty cluster_api_url (likely during import)")
		resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
		return
	}

	// Create SecurityService client and verify the role assignment exists
	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}
	defer r.dataplaneConn.Close()

	exists, err := r.roleAssignmentExists(ctx, roleName, principal)
	if err != nil {
		// Don't remove from state on API errors - just fail the read
		resp.Diagnostics.AddError(
			"Failed to verify role assignment",
			fmt.Sprintf("Could not check if role assignment %s:%s exists: %s", roleName, principal, utils.DeserializeGrpcError(err)),
		)
		return
	}

	if !exists {
		tflog.Warn(ctx, "Role assignment not found, removing from state", map[string]any{
			"role_name": roleName,
			"principal": principal,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Role assignment exists, keep current state
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
	principal := model.Principal.ValueString() // Already normalized when stored
	clusterAPIURL := model.ClusterAPIURL.ValueString()

	if err := r.createSecurityClient(ctx, clusterAPIURL); err != nil {
		resp.Diagnostics.AddError("Failed to create SecurityService client", utils.DeserializeGrpcError(err))
		return
	}
	defer r.dataplaneConn.Close()

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

// ImportState imports the role assignment state
func (*RoleAssignment) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: role_name:principal
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: role_name:principal",
		)
		return
	}

	roleName := parts[0]
	principal := normalizePrincipal(parts[1])

	// Set the imported state (cluster_api_url will be set from the resource configuration)
	model := models.RoleAssignment{
		RoleName:  types.StringValue(roleName),
		Principal: types.StringValue(principal), // Store normalized principal
		ID:        types.StringValue(fmt.Sprintf("%s:%s", roleName, principal)),
		// ClusterAPIURL will be set from the resource configuration
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// normalizePrincipal ensures the principal has the correct format for the API
func normalizePrincipal(principal string) string {
	// The gRPC API expects just the username, not "User:username"
	// Remove "User:" prefix if present
	return strings.TrimPrefix(principal, "User:")
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

	// Check if the principal is in the role members
	if resp.Response != nil && resp.Response.Members != nil {
		for _, member := range resp.Response.Members {
			if normalizePrincipal(member.Principal) == principal {
				return true, nil
			}
		}
	}

	return false, nil
}

// createSecurityClient creates a SecurityService client
func (r *RoleAssignment) createSecurityClient(_ context.Context, clusterURL string) error {
	if r.SecurityClient != nil {
		return nil // Client already exists
	}

	if r.dataplaneConn == nil {
		// Convert cluster API URL to console URL for SecurityService
		consoleURL := utils.ConvertToConsoleURL(clusterURL)

		conn, err := cloud.SpawnConn(consoleURL, r.resData.AuthToken, r.resData.ProviderVersion, r.resData.TerraformVersion)
		if err != nil {
			return fmt.Errorf("unable to open a connection with the console API at %s: %v", consoleURL, err)
		}
		r.dataplaneConn = conn
	}

	r.SecurityClient = consolev1alpha1grpc.NewSecurityServiceClient(r.dataplaneConn)
	return nil
}
