package resourcegroup

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestUnit_ResourceGroup_Schema(t *testing.T) {
	n := NewResourceGroup()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	n.Schema(context.Background(), req, resp)

	if _, ok := resp.Schema.Attributes["name"]; !ok {
		t.Error("Expected 'name' attribute to be present in schema")
	}
}

func TestUnit_ResourceGroup_ValidateSchema(t *testing.T) {
	if d := ResourceGroupSchema(context.Background()).ValidateImplementation(context.Background()); d.HasError() {
		t.Errorf("Unexpected error in schema: %s", d)
	}
}
