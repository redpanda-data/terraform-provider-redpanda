package redpanda

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProviderConfigure(t *testing.T) {
	ctx := context.Background()

	rp := New(ctx, "ign", "test")()
	rp.Schema(ctx, provider.SchemaRequest{}, &provider.SchemaResponse{})

	if d := providerSchema().ValidateImplementation(ctx); d.HasError() {
		t.Fatalf("unexpected error in provider schema: %s", d)
	}

	request := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"client_id":     tftypes.String,
					"client_secret": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"client_id":     tftypes.NewValue(tftypes.String, "sampleClientID"),
				"client_secret": tftypes.NewValue(tftypes.String, "sampleClientSecret"),
			}),
			Schema: providerSchema(),
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
