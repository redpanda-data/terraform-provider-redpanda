// Copyright 2023 Redpanda Data, Inc.
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

// Package cluster contains the implementation of the Cluster resource
// following the Terraform framework interfaces.
package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &Cluster{}
	_ resource.ResourceWithConfigure   = &Cluster{}
	_ resource.ResourceWithImportState = &Cluster{}
)

// Cluster represents a cluster managed resource.
type Cluster struct {
	CluClient cloudv1beta1.ClusterServiceClient
	OpsClient cloudv1beta1.OperationServiceClient
}

// Metadata returns the full name of the Cluster resource.
func (*Cluster) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_cluster"
}

// Configure uses provider level data to configure Cluster's clients.
func (c *Cluster) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(utils.ResourceData)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	client, err := clients.NewClusterServiceClient(ctx, p.CloudEnv, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster client", err.Error())
		return
	}
	c.CluClient = client

	ops, err := clients.NewOperationServiceClient(ctx, p.CloudEnv, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create ops client", err.Error())
		return
	}
	c.OpsClient = ops
}

// Schema returns the schema for the Cluster resource.
func (*Cluster) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceClusterSchema()
}

func resourceClusterSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cluster_type": schema.StringAttribute{
				Required:      true,
				Description:   "Type of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connection_type": schema.StringAttribute{
				Required:      true,
				Description:   "Connection type of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_provider": schema.StringAttribute{
				Optional:      true,
				Description:   "Must be one of aws or gcp",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"redpanda_version": schema.StringAttribute{
				Optional:      true,
				Description:   "Version of Redpanda to deploy",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"throughput_tier": schema.StringAttribute{
				Required:      true,
				Description:   "Throughput tier of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				Optional:      true,
				Description:   "Cloud provider specific region of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"zones": schema.ListAttribute{
				Optional:      true,
				Description:   "Cloud provider specific zones of the cluster",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"allow_deletion": schema.BoolAttribute{
				Optional:    true,
				Description: "allows deletion of the cluster. defaults to true. should probably be set to false for production use",
			},
			"tags": schema.MapAttribute{
				Optional:      true,
				Description:   "Tags to apply to the cluster",
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"namespace_id": schema.StringAttribute{
				Required:      true,
				Description:   "The id of the namespace in which to create the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"network_id": schema.StringAttribute{
				Required:      true,
				Description:   "The id of the network in which to create the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The id of the cluster",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_api_url": schema.StringAttribute{
				Computed:      true,
				Description:   "The URL of the cluster API",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// Create creates a new Cluster resource. It updates the state if the resource
// is successfully created.
func (c *Cluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	clusterReq, err := GenerateClusterRequest(model)
	if err != nil {
		resp.Diagnostics.AddError("unable to parse CreateCluster request", err.Error())
		return
	}
	op, err := c.CluClient.CreateCluster(ctx, &cloudv1beta1.CreateClusterRequest{
		Cluster: clusterReq,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster", err.Error())
		return
	}
	var metadata cloudv1beta1.CreateClusterMetadata
	if err := op.Metadata.UnmarshalTo(&metadata); err != nil {
		resp.Diagnostics.AddError("failed to unmarshal cluster metadata", err.Error())
		return
	}
	if err := utils.AreWeDoneYet(ctx, op, 45*time.Minute, c.OpsClient); err != nil {
		resp.Diagnostics.AddError("failed to create cluster", err.Error())
		return
	}
	cluster, err := c.CluClient.GetCluster(ctx, &cloudv1beta1.GetClusterRequest{
		Id: metadata.GetClusterId(),
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("successfully created the cluster with ID %q, but failed to read the cluster configuration: %v", model.ID.ValueString(), err), err.Error())
		return
	}
	clusterZones, d := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if d.HasError() {
		resp.Diagnostics.Append(d...)
		return
	}
	clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
	if err != nil {
		resp.Diagnostics.AddError("unable to parse Cluster API URL", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.Cluster{
		Name:            types.StringValue(cluster.Name),
		ConnectionType:  types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion: model.RedpandaVersion,
		ThroughputTier:  types.StringValue(cluster.ThroughputTier),
		Region:          types.StringValue(cluster.Region),
		Zones:           clusterZones,
		AllowDeletion:   model.AllowDeletion,
		Tags:            model.Tags,
		NamespaceID:     types.StringValue(cluster.NamespaceId),
		NetworkID:       types.StringValue(cluster.NetworkId),
		ID:              types.StringValue(cluster.Id),
		ClusterAPIURL:   types.StringValue(clusterURL),
	})...)
}

// Read reads Cluster resource's values and updates the state.
func (c *Cluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	cluster, err := c.CluClient.GetCluster(ctx, &cloudv1beta1.GetClusterRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			// Treat HTTP 404 Not Found status as a signal to recreate resource and return early
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), err.Error())
		return
	}
	clusterZones, d := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if d.HasError() {
		resp.Diagnostics.Append(d...)
		return
	}
	clusterURL, err := utils.SplitSchemeDefPort(cluster.DataplaneApi.Url, "443")
	if err != nil {
		resp.Diagnostics.AddError("unable to parse Cluster API URL", err.Error())
		return
	}

	// Re: RedpandaVersion, I chose to not set it using the return value from the API because the user leaving the field blank
	// is a valid choice that causes the API to select the latest value. If we then persist the value provided by the API to state
	// we end up in a situation where on refresh TF will attempt to remove the RP_VER from state. This will cause a diff and a run
	// even though that is neither user intent nor a change in the cluster.
	resp.Diagnostics.Append(resp.State.Set(ctx, models.Cluster{
		Name:            types.StringValue(cluster.Name),
		ConnectionType:  types.StringValue(utils.ConnectionTypeToString(cluster.ConnectionType)),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(cluster.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(cluster.Type)),
		RedpandaVersion: model.RedpandaVersion,
		ThroughputTier:  types.StringValue(cluster.ThroughputTier),
		Region:          types.StringValue(cluster.Region),
		Zones:           clusterZones,
		AllowDeletion:   model.AllowDeletion,
		Tags:            model.Tags,
		NamespaceID:     types.StringValue(cluster.NamespaceId),
		NetworkID:       types.StringValue(cluster.NetworkId),
		ID:              types.StringValue(cluster.Id),
		ClusterAPIURL:   types.StringValue(clusterURL),
	})...)
}

// Update all cluster updates are currently delete and recreate.
func (*Cluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.Cluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// We pass through the plan to state. Currently, every cluster change needs
	// a resource replacement except for allow_deletion.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the Cluster resource.
func (c *Cluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("cluster deletion not allowed", "allow_deletion is set to false")
		return
	}
	op, err := c.CluClient.DeleteCluster(ctx, &cloudv1beta1.DeleteClusterRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to delete cluster", err.Error())
		return
	}

	if err := utils.AreWeDoneYet(ctx, op, 90*time.Minute, c.OpsClient); err != nil {
		resp.Diagnostics.AddError("failed to delete cluster", err.Error())
		return
	}
}

// ImportState imports and update the state of the cluster resource.
func (*Cluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// GenerateClusterRequest was pulled out to enable unit testing
func GenerateClusterRequest(model models.Cluster) (*cloudv1beta1.Cluster, error) {
	provider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse cloud provider: %v", err)
	}
	clusterType, err := utils.StringToClusterType(model.ClusterType.ValueString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse cluster type: %v", err)
	}
	return &cloudv1beta1.Cluster{
		Name:            model.Name.ValueString(),
		ConnectionType:  utils.StringToConnectionType(model.ConnectionType.ValueString()),
		CloudProvider:   provider,
		RedpandaVersion: model.RedpandaVersion.ValueString(),
		ThroughputTier:  model.ThroughputTier.ValueString(),
		Region:          model.Region.ValueString(),
		Zones:           utils.TypeListToStringSlice(model.Zones),
		NamespaceId:     model.NamespaceID.ValueString(),
		NetworkId:       model.NetworkID.ValueString(),
		Type:            clusterType,
	}, nil
}
