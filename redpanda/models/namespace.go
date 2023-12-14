package models

import "github.com/hashicorp/terraform-plugin-framework/types"

// Namespace represents the Terraform schema for the namespace resource.
type Namespace struct {
	Name types.String `tfsdk:"name"`
	ID   types.String `tfsdk:"id"`
}
