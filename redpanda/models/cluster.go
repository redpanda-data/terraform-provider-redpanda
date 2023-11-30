package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Cluster struct {
	Name            types.String `tfsdk:"name"`
	Id              types.String `tfsdk:"id"`
	ConnectionType  types.String `tfsdk:"connection_type"`
	CloudProvider   types.String `tfsdk:"cloud_provider"`
	ClusterType     types.String `tfsdk:"cluster_type"`
	RedpandaVersion types.String `tfsdk:"redpanda_version"`
	ThroughputTier  types.String `tfsdk:"throughput_tier"`
	Region          types.String `tfsdk:"region"`
	Zones           types.List   `tfsdk:"zones"`
	AllowDeletion   types.Bool   `tfsdk:"allow_deletion"`
	Tags            types.Map    `tfsdk:"tags"`
	NamespaceId     types.String `tfsdk:"namespace_id"`
	NetworkId       types.String `tfsdk:"network_id"`
}
