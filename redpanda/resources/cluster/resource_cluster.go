package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
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

var (
	_ resource.Resource                = &Cluster{}
	_ resource.ResourceWithConfigure   = &Cluster{}
	_ resource.ResourceWithImportState = &Cluster{}
)

type Cluster struct {
	CluClient cloudv1beta1.ClusterServiceClient
	OpsClient cloudv1beta1.OperationServiceClient
}

func (c *Cluster) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_cluster"
}

func (c *Cluster) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		resp.Diagnostics.AddWarning("provider data not set", "provider data not set at cluster.Configure")
		return
	}

	p, ok := req.ProviderData.(utils.ResourceData)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	client, err := clients.NewClusterServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create cluster client", err.Error())
		return
	}
	c.CluClient = client

	ops, err := clients.NewOperationServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create ops client", err.Error())
		return
	}
	c.OpsClient = ops
}

func (c *Cluster) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceClusterSchema()
}

func ResourceClusterSchema() schema.Schema {
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
				Description:   "Version of redpanda to deploy",
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
				Optional:      true,
				Description:   "allows deletion of the cluster. defaults to true. should probably be set to false for production use",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
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
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (c *Cluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	op, err := c.CluClient.CreateCluster(ctx, &cloudv1beta1.CreateClusterRequest{
		Cluster: GenerateClusterRequest(model),
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

	resp.Diagnostics.Append(resp.State.Set(ctx, models.Cluster{
		Name:            model.Name,
		ConnectionType:  model.ConnectionType,
		CloudProvider:   model.CloudProvider,
		ClusterType:     model.ClusterType,
		RedpandaVersion: model.RedpandaVersion,
		ThroughputTier:  model.ThroughputTier,
		Region:          model.Region,
		Zones:           model.Zones,
		AllowDeletion:   model.AllowDeletion,
		Tags:            model.Tags,
		NamespaceID:     model.NamespaceID,
		NetworkID:       model.NetworkID,
		ID:              utils.TrimmedStringValue(metadata.GetClusterId()),
	})...)
}

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
		} else {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), err.Error())
			return
		}
	}
	lv, d := types.ListValueFrom(ctx, types.StringType, cluster.Zones)
	if d.HasError() {
		resp.Diagnostics.Append(d...)
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
		Zones:           lv,
		AllowDeletion:   model.AllowDeletion,
		Tags:            model.Tags,
		NamespaceID:     types.StringValue(cluster.NamespaceId),
		NetworkID:       types.StringValue(cluster.NetworkId),
		ID:              types.StringValue(cluster.Id),
	})...)
}

// Update all cluster updates are currently delete and recreate
func (c *Cluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

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

	if err := utils.AreWeDoneYet(ctx, op, 45*time.Minute, c.OpsClient); err != nil {
		resp.Diagnostics.AddError("failed to delete cluster", err.Error())
		return
	}
}

func (c *Cluster) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.Set(ctx, &models.Cluster{
		ID: types.StringValue(req.ID),
	})...)
}

// GenerateClusterRequest was pulled out to enable unit testing
func GenerateClusterRequest(model models.Cluster) *cloudv1beta1.Cluster {
	return &cloudv1beta1.Cluster{
		Name:            model.Name.ValueString(),
		ConnectionType:  utils.StringToConnectionType(model.ConnectionType.ValueString()),
		CloudProvider:   utils.StringToCloudProvider(model.CloudProvider.ValueString()),
		RedpandaVersion: model.RedpandaVersion.ValueString(),
		ThroughputTier:  model.ThroughputTier.ValueString(),
		Region:          model.Region.ValueString(),
		Zones:           utils.TypeListToStringSlice(model.Zones),
		NamespaceId:     model.NamespaceID.ValueString(),
		NetworkId:       model.NetworkID.ValueString(),
		Type:            utils.StringToClusterType(model.ClusterType.ValueString()),
	}
}
