package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type User struct {
	Name      types.String `tfsdk:"name"`
	Password  types.String `tfsdk:"password"`
	Mechanism types.String `tfsdk:"mechanism"`
}
