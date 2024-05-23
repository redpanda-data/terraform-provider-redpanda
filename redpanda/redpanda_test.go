package redpanda

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestProviderConfigure(t *testing.T) {
	ctx := context.Background()

	rp := New(ctx, "ign", "test")()
	rp.Schema(ctx, provider.SchemaRequest{}, &provider.SchemaResponse{})

	if d := providerSchema().ValidateImplementation(ctx); d.HasError() {
		t.Fatalf("unexpected error in provider schema: %s", d)
	}
}
