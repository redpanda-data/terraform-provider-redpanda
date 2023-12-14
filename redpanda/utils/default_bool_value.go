package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DefaultBoolValue follows the TF framework bool type.
type DefaultBoolValue struct {
	Value    bool
	Desc     string
	MarkDesc string
}

// Description return the description string.
func (d *DefaultBoolValue) Description(_ context.Context) string {
	return d.Desc
}

// MarkdownDescription returns the markdown description string.
func (d *DefaultBoolValue) MarkdownDescription(_ context.Context) string {
	return d.MarkDesc
}

// DefaultBool sets the default bool value in resp.PlanValue.
func (d *DefaultBoolValue) DefaultBool(_ context.Context, _ defaults.BoolRequest, resp *defaults.BoolResponse) {
	resp.PlanValue = types.BoolValue(d.Value)
}
