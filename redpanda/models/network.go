package models

import "github.com/hashicorp/terraform-plugin-framework/types"

// Network represents the Terraform schema for the network resource.
type Network struct {
	Name          types.String `tfsdk:"name"`
	NamespaceID   types.String `tfsdk:"namespace_id"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	Region        types.String `tfsdk:"region"`
	CidrBlock     types.String `tfsdk:"cidr_block"`
	ID            types.String `tfsdk:"id"`
	ClusterType   types.String `tfsdk:"cluster_type"`
}
