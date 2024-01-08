package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type Acl struct {
	ResourceType        types.String `tfsdk:"resource_type"`
	ResourceName        types.String `tfsdk:"resource_name"`
	ResourcePatternType types.String `tfsdk:"resource_pattern_type"`
	Principal           types.String `tfsdk:"principal"`
	Host                types.String `tfsdk:"host"`
	Operation           types.String `tfsdk:"operation"`
	PermissionType      types.String `tfsdk:"permission_type"`
}
