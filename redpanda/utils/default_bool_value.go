package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type DefaultBoolValue struct {
	Value    bool
	Desc     string
	MarkDesc string
}

func (d *DefaultBoolValue) Description(_ context.Context) string {
	return d.Desc
}

func (d *DefaultBoolValue) MarkdownDescription(_ context.Context) string {
	return d.MarkDesc
}

func (d *DefaultBoolValue) DefaultBool(_ context.Context, _ defaults.BoolRequest, resp *defaults.BoolResponse) {
	resp.PlanValue = types.BoolValue(d.Value)
}
