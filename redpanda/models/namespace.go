package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type Namespace struct {
	Name types.String `tfsdk:"name"`
	ID   types.String `tfsdk:"id"`
}
