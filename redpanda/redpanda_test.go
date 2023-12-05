package redpanda

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"testing"
)

func TestProviderConfigure(t *testing.T) {
	ctx := context.Background()

	rp := New(ctx, "dev")()
	rp.Schema(ctx, provider.SchemaRequest{}, &provider.SchemaResponse{})

	if d := ProviderSchema().ValidateImplementation(ctx); d.HasError() {
		t.Fatalf("unexpected error in provider schema: %s", d)
	}

	request := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"client_id":      tftypes.String,
					"client_secret":  tftypes.String,
					"cloud_provider": tftypes.String,
					"region":         tftypes.String,
					"zones":          tftypes.List{ElementType: tftypes.String},
				},
			}, map[string]tftypes.Value{
				"client_id":      tftypes.NewValue(tftypes.String, "sampleClientID"),
				"client_secret":  tftypes.NewValue(tftypes.String, "sampleClientSecret"),
				"cloud_provider": tftypes.NewValue(tftypes.String, "sampleCloudProvider"),
				"region":         tftypes.NewValue(tftypes.String, "sampleRegion"),
				"zones": tftypes.NewValue(
					tftypes.List{
						ElementType: tftypes.String,
					},
					[]tftypes.Value{
						tftypes.NewValue(tftypes.String, "zone1"),
						tftypes.NewValue(tftypes.String, "zone2"),
					},
				),
			}),
			Schema: ProviderSchema(),
		},
	}

	resp := &provider.ConfigureResponse{
		Diagnostics: make(diag.Diagnostics, 0),
	}
	rp.Configure(ctx, request, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error in provider configuration: %s", resp.Diagnostics)
	}
}
