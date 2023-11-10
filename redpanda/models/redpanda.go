package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Redpanda struct {
	AuthToken     types.String   `tfsdk:"auth_token"`
	ClientID      types.String   `tfsdk:"client_id"`
	ClientSecret  types.String   `tfsdk:"client_secret"`
	CloudProvider types.String   `tfsdk:"cloud_provider"`
	Region        types.String   `tfsdk:"region"`
	Zones         []types.String `tfsdk:"zones"`
}
