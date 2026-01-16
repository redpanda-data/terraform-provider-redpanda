package resourcegroup

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestResourceGroupSchema(t *testing.T) {
	n := &ResourceGroup{}
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	n.Schema(context.Background(), req, resp)

	if _, ok := resp.Schema.Attributes["name"]; !ok {
		t.Error("Expected 'name' attribute to be present in schema")
	}
}

func TestValidateSchema(t *testing.T) {
	if d := ResourceGroupSchema().ValidateImplementation(context.Background()); d.HasError() {
		t.Errorf("Unexpected error in schema: %s", d)
	}
}
