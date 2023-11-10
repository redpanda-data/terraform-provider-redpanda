package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type Namespace struct {
	Name types.String `tfsdk:"name"`
	Id   types.String `tfsdk:"id"`
}
