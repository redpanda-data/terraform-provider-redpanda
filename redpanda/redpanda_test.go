package redpanda

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"testing"
)

// Providers map for testing
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"redpanda": providerserver.NewProtocol6WithError(New(context.Background(), "dev")()),
}

func testAccPreCheck(t *testing.T) {
}

func TestProviderConfigure(t *testing.T) {
	ctx := context.Background()

	rp := New(ctx, "dev")()
	rp.Schema(ctx, tfprovider.SchemaRequest{}, &tfprovider.SchemaResponse{})

	if d := Provider().ValidateImplementation(ctx); d.HasError() {
		t.Fatalf("unexpected error in provider schema: %s", d)
	}

	request := tfprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"auth_token":     tftypes.String,
					"client_id":      tftypes.String,
					"client_secret":  tftypes.String,
					"cloud_provider": tftypes.String,
					"region":         tftypes.String,
					"zones":          tftypes.List{ElementType: tftypes.String},
				},
			}, map[string]tftypes.Value{
				"auth_token":     tftypes.NewValue(tftypes.String, "sampleAuthToken"),
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
			Schema: Provider(),
		},
	}

	resp := &tfprovider.ConfigureResponse{
		Diagnostics: make(diag.Diagnostics, 0),
	}
	rp.Configure(ctx, request, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error in provider configuration: %s", resp.Diagnostics)
	}
}
