// Package schema contains the implementation of the Schema resource
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// ImportState imports an existing schema resource using cluster_id:subject:version format.
func (*Schema) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	// Import format: "cluster_id:subject:version"
	parts := strings.Split(request.ID, ":")
	if len(parts) != 3 {
		response.Diagnostics.AddError(
			"Invalid import format",
			"Expected format: cluster_id:subject:version",
		)
		return
	}

	clusterID := parts[0]
	subject := parts[1]
	version := parts[2]

	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("cluster_id"), clusterID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("subject"), subject)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("version"), version)...)
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
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (s *Schema) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state schemamodel.ResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	client, err := kclients.GetSchemaRegistryClientForCluster(ctx, s.CpCl, state.ClusterID.ValueString(), state.Username.ValueString(), state.Password.ValueString())
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", state.ClusterID.ValueString(), err),
		)
		return
	}

	schemaResp, err := kclients.FetchSchema(ctx, client, state.GetSubject(), state.GetVersion())
	if err != nil {
		if utils.IsNotFound(err) {
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
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

// Update creates a new version of an existing schema.
func (s *Schema) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
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

	schemaReq := plan.ToSchemaRequest()
	schemaResp, err := client.CreateSchema(ctx, plan.Subject.ValueString(), schemaReq)
	if err != nil {
		response.Diagnostics.AddError(
			"Failed to update schema",
			fmt.Sprintf("Unable to create new version of schema for subject %s: %v", plan.Subject.ValueString(), err),
		)
		return
	}

	plan.UpdateFromSchema(schemaResp)
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
		response.Diagnostics.AddError(
			"Failed to create Schema Registry client",
			fmt.Sprintf("Unable to create client for cluster %s: %v", state.ClusterID.ValueString(), err),
		)
		return
	}

	_, err = client.DeleteSubject(ctx, state.GetSubject(), sr.SoftDelete)
	if err != nil {
		if !utils.IsNotFound(err) {
			response.Diagnostics.AddError(
				"Failed to delete schema",
				fmt.Sprintf("Unable to delete schema subject %s: %v", state.GetSubject(), err),
			)
			return
		}
	}
}
