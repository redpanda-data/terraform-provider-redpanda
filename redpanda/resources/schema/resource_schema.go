// Package schema contains the implementation of the Schema resource
package schema

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/kclients"
	schemamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/twmb/franz-go/pkg/sr"
	"golang.org/x/oauth2"
)

var (
	_ resource.Resource                = &Schema{}
	_ resource.ResourceWithConfigure   = &Schema{}
	_ resource.ResourceWithImportState = &Schema{}
)

// SRClienter defines the interface for Schema Registry client operations
type SRClienter interface {
	CreateSchema(ctx context.Context, subject string, schema sr.Schema) (sr.SubjectSchema, error)
	SchemaByVersion(ctx context.Context, subject string, version int) (sr.SubjectSchema, error)
	Schemas(ctx context.Context, subject string) ([]sr.SubjectSchema, error)
	DeleteSubject(ctx context.Context, subject string, how sr.DeleteHow) ([]int, error)
	SetCompatibility(ctx context.Context, c sr.SetCompatibility, subjects ...string) []sr.CompatibilityResult
	Compatibility(ctx context.Context, subjects ...string) []sr.CompatibilityResult
}

// Schema represents a schema managed resource
type Schema struct {
	base.ResourceBase

	resData       config.Resource
	clientFactory func(ctx context.Context, cpCl *cloud.ControlPlaneClientSet, clusterID string, ts oauth2.TokenSource, username, password string) (SRClienter, error)
}

// Schema returns the resource schema definition.
func (*Schema) Schema(ctx context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = ResourceSchemaSchema(ctx)
}

// NewSchema constructs a Schema resource.
func NewSchema() *Schema {
	s := &Schema{}
	s.ResourceBase = base.NewResourceBase(
		"redpanda_schema",
		ResourceSchemaSchema,
		func(p config.Resource) { s.resData = p },
	)
	return s
}

// importIDComponents holds the parsed components from an import ID string
type importIDComponents struct {
	clusterID string
	subject   string
	version   int64
	username  string
	password  string
}

// parseImportID parses the import ID string into its components. Two forms:
//
//	Bearer auth (default): cluster_id,subject,version
//	Basic auth (optional): cluster_id,subject,version,username,password
func parseImportID(importID string) (*importIDComponents, error) {
	parts := strings.Split(importID, ",")
	if len(parts) != 3 && len(parts) != 5 {
		return nil, fmt.Errorf(
			"expected one of:\n"+
				"  Bearer auth (default): cluster_id,subject,version\n"+
				"  Basic auth (optional): cluster_id,subject,version,username,password\n"+
				"got %d parts",
			len(parts),
		)
	}

	version, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("version must be a valid integer: %w", err)
	}

	ret := &importIDComponents{
		clusterID: parts[0],
		subject:   parts[1],
		version:   version,
	}
	if len(parts) == 5 {
		ret.username = parts[3]
		ret.password = parts[4]
	}
	return ret, nil
}

// ImportState imports an existing schema resource.
//
// Bearer auth (default): cluster_id,subject,version
// Basic auth (optional): same 3 fields + ,username,password
//
// For Basic auth, password can also be set via REDPANDA_IMPORT_PASSWORD env var.
func (*Schema) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	components, err := parseImportID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid import format", err.Error()+". Password can also be set via REDPANDA_IMPORT_PASSWORD env var.")
		return
	}

	password := components.password
	if envPassword := os.Getenv("REDPANDA_IMPORT_PASSWORD"); envPassword != "" {
		password = envPassword
	}

	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("cluster_id"), types.StringValue(components.clusterID))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("subject"), types.StringValue(components.subject))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("version"), types.Int64Value(components.version))...)
	if components.username != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("username"), types.StringValue(components.username))...)
	}
	if password != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("password"), types.StringValue(password))...)
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("allow_deletion"), types.BoolValue(false))...)
}

// Create creates a new schema in the Schema Registry.
func (s *Schema) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan schemamodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)

	var cfg schemamodel.ResourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &cfg)...)
	if response.Diagnostics.HasError() {
		return
	}
	plan.PasswordWO = cfg.PasswordWO

	client, err := s.getClient(ctx, plan.ClusterID.ValueString(), s.resData.TokenSource, plan.Username.ValueString(), plan.GetEffectivePassword())
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
		err = setSubjectCompatibility(ctx, client, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			response.Diagnostics.AddError(
				"Failed to set compatibility level",
				fmt.Sprintf("Unable to set compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
			)
			return
		}
	} else {
		// If compatibility is not specified, get the current compatibility level
		compat, err := getCompatibility(ctx, client, plan.Subject.ValueString())
		if err != nil {
			response.Diagnostics.AddError(
				"Failed to get compatibility level",
				fmt.Sprintf("Unable to get compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
			)
			return
		}
		plan.Compatibility = compat
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

	subject := state.GetSubject()
	client, err := s.getClient(ctx, state.ClusterID.ValueString(), s.resData.TokenSource, state.Username.ValueString(), state.GetEffectivePassword())
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "schema", subject, state.AllowDeletion, err, "create schema registry client")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	schemaResp, err := fetchSchema(ctx, client, subject, state.GetVersion())
	if err != nil {
		tflog.Debug(ctx, "Schema read error encountered", map[string]any{
			"subject":              subject,
			"error":                err.Error(),
			"is_not_found":         utils.IsNotFound(err),
			"is_permission_denied": utils.IsPermissionDenied(err),
		})

		action, diags := utils.HandleGracefulRemoval(ctx, "schema", subject, state.AllowDeletion, err, "read schema")
		response.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			response.State.RemoveResource(ctx)
		}
		return
	}

	password := state.Password
	passwordWOVersion := state.PasswordWOVersion
	state.UpdateFromSchema(schemaResp)
	state.Password = password
	state.PasswordWOVersion = passwordWOVersion

	// Get compatibility level for the subject
	compat, err := getCompatibility(ctx, client, state.Subject.ValueString())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to get compatibility level",
			fmt.Sprintf("Unable to get compatibility level for subject %s: %v", state.Subject.ValueString(), err),
		)
		return
	}
	state.Compatibility = compat

	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

// Update creates a new version of an existing schema.
func (s *Schema) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan schemamodel.ResourceModel
	var state schemamodel.ResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)

	var cfg schemamodel.ResourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &cfg)...)
	if response.Diagnostics.HasError() {
		return
	}
	plan.PasswordWO = cfg.PasswordWO

	client, err := s.getClient(ctx, plan.ClusterID.ValueString(), s.resData.TokenSource, plan.Username.ValueString(), plan.GetEffectivePassword())
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

	// Body equivalence: strict-equal first, then Avro-canonical for the
	// FQN-vs-namespace-relative case (Schema Registry collapses
	// in-namespace type references on write; user's FQN form persists in
	// plan). Without the Avro fallback, every plan after the first apply
	// sees plan.Schema != state.Schema and Update fires unnecessarily —
	// eventually tripping the framework's post-apply consistency check.
	bodiesEqual := planReq.Schema == stateReq.Schema
	if !bodiesEqual && strings.EqualFold(plan.SchemaType.ValueString(), "AVRO") {
		bodiesEqual = schemamodel.AvroBodiesEquivalent(planReq.Schema, stateReq.Schema)
	}

	// Compare schema content, type, and references
	if bodiesEqual &&
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
				err = setSubjectCompatibility(ctx, client, plan.Subject.ValueString(), plan.Compatibility.ValueString())
				if err != nil {
					response.Diagnostics.AddError(
						"Failed to update compatibility level",
						fmt.Sprintf("Unable to update compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
					)
					return
				}
			}
			// Retrieve the current compatibility value
			compat, err := getCompatibility(ctx, client, plan.Subject.ValueString())
			if err != nil {
				response.Diagnostics.AddError(
					"Failed to get compatibility level",
					fmt.Sprintf("Unable to get compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
				)
				return
			}
			plan.Compatibility = compat

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
		err = setSubjectCompatibility(ctx, client, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			response.Diagnostics.AddError(
				"Failed to update compatibility level",
				fmt.Sprintf("Unable to update compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
			)
			return
		}
	}
	// Retrieve the current compatibility value
	compat, err := getCompatibility(ctx, client, plan.Subject.ValueString())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to get compatibility level",
			fmt.Sprintf("Unable to get compatibility level for subject %s: %v", plan.Subject.ValueString(), err),
		)
		return
	}
	plan.Compatibility = compat

	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

// Delete removes a schema from the Schema Registry.
func (s *Schema) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state schemamodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := s.getClient(ctx, state.ClusterID.ValueString(), s.resData.TokenSource, state.Username.ValueString(), state.GetEffectivePassword())
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

// getClient returns a Schema Registry client, using the factory if available or
// the default implementation. ts is the provider's cloud-issued token source
// used when explicit Basic creds (username+password) are absent; the bearer
// header is stamped per HTTP request.
func (s *Schema) getClient(ctx context.Context, clusterID string, ts oauth2.TokenSource, username, password string) (SRClienter, error) {
	if s.clientFactory != nil {
		return s.clientFactory(ctx, s.CpCl, clusterID, ts, username, password)
	}
	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, s.CpCl, clusterID, ts, username, password)
	if err != nil {
		return nil, err
	}
	return newSchemaRegistryClientWrapper(client), nil
}

// fetchSchema fetches a schema by subject and optional version
func fetchSchema(ctx context.Context, client SRClienter, subject string, version *int) (sr.SubjectSchema, error) {
	if version != nil {
		return client.SchemaByVersion(ctx, subject, *version)
	}

	schemas, err := client.Schemas(ctx, subject)
	if err != nil {
		return sr.SubjectSchema{}, err
	}

	if len(schemas) == 0 {
		return sr.SubjectSchema{}, fmt.Errorf("no schemas found for subject %s", subject)
	}

	return schemas[len(schemas)-1], nil
}

// setSubjectCompatibility sets the compatibility level for a subject
func setSubjectCompatibility(ctx context.Context, client SRClienter, subject, compatibility string) error {
	if compatibility == "" {
		return nil // No compatibility to set
	}

	var level sr.CompatibilityLevel
	if err := level.UnmarshalText([]byte(strings.ToUpper(compatibility))); err != nil {
		return fmt.Errorf("invalid compatibility level %q: %w", compatibility, err)
	}

	setCompat := sr.SetCompatibility{
		Level: level,
	}

	results := client.SetCompatibility(ctx, setCompat, subject)
	for _, result := range results {
		if result.Err != nil {
			return result.Err
		}
	}
	return nil
}

// getSubjectCompatibility gets the compatibility level for a subject.
// With DefaultToGlobal set in the wrapper, this returns either the subject-level
// compatibility or the global default.
func getSubjectCompatibility(ctx context.Context, client SRClienter, subject string) (string, error) {
	results := client.Compatibility(ctx, subject)

	if len(results) == 0 {
		return "", fmt.Errorf("no compatibility results returned for subject %s", subject)
	}

	// With DefaultToGlobal, we should get exactly one result for the subject
	// containing either subject-level or global compatibility
	result := results[0]
	if result.Err != nil {
		return "", fmt.Errorf("failed to get compatibility for subject %s: %w", subject, result.Err)
	}

	return result.Level.String(), nil
}

func getCompatibility(ctx context.Context, client SRClienter, subject string) (types.String, error) {
	compatibility, err := getSubjectCompatibility(ctx, client, subject)
	if err != nil {
		return types.StringNull(), err
	}
	return types.StringValue(compatibility), nil
}
