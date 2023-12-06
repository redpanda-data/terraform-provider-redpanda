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

func (d *DefaultBoolValue) Description(ctx context.Context) string {
	return d.Desc
}

func (d *DefaultBoolValue) MarkdownDescription(ctx context.Context) string {
	return d.MarkDesc
}

func (d *DefaultBoolValue) DefaultBool(ctx context.Context, req defaults.BoolRequest, resp *defaults.BoolResponse) {
	resp.PlanValue = types.BoolValue(d.Value)
}
