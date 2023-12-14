package namespace

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
)

func TestNamespaceSchema(t *testing.T) {
	n := &Namespace{}
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	n.Schema(context.Background(), req, resp)

	if _, ok := resp.Schema.Attributes["name"]; !ok {
		t.Errorf("Expected 'name' attribute to be present in schema")
	}
}

func TestValidateSchema(t *testing.T) {
	if d := resourceNamespaceSchema().ValidateImplementation(context.Background()); d.HasError() {
		t.Errorf("Unexpected error in schema: %s", d)
	}
}

func TestNamespaceCreate(t *testing.T) {
	n := &Namespace{
		Client: mocks.MockNamespaceServiceClient{},
	}

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"name": tftypes.String,
					"id":   tftypes.String,
				},
			},
				map[string]tftypes.Value{
					"name": tftypes.NewValue(tftypes.String, "sample"),
					"id":   tftypes.NewValue(tftypes.String, ""),
				}),
			Schema: resourceNamespaceSchema(),
		},
	}
	resp := resource.CreateResponse{
		State: tfsdk.State{
			Schema: resourceNamespaceSchema(),
		},
	}
	n.Create(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Create function failed: %v", resp.Diagnostics)
	}
}

func TestNamespaceRead(t *testing.T) {
	n := &Namespace{
		Client: mocks.MockNamespaceServiceClient{},
	}
	req := resource.ReadRequest{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"name": tftypes.String,
					"id":   tftypes.String,
				},
			},
				map[string]tftypes.Value{
					"name": tftypes.NewValue(tftypes.String, "sample"),
					"id":   tftypes.NewValue(tftypes.String, ""),
				}),
			Schema: resourceNamespaceSchema(),
		},
	}
	resp := resource.ReadResponse{
		State: tfsdk.State{
			Schema: resourceNamespaceSchema(),
		},
	}
	n.Read(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Read function failed: %v", resp.Diagnostics)
	}
}

func TestNamespace_Delete(t *testing.T) {
	n := &Namespace{
		Client: mocks.MockNamespaceServiceClient{},
	}
	req := resource.DeleteRequest{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"name": tftypes.String,
					"id":   tftypes.String,
				},
			},
				map[string]tftypes.Value{
					"name": tftypes.NewValue(tftypes.String, "sample"),
					"id":   tftypes.NewValue(tftypes.String, ""),
				}),
			Schema: resourceNamespaceSchema(),
		},
	}
	resp := resource.DeleteResponse{
		State: tfsdk.State{
			Schema: resourceNamespaceSchema(),
		},
	}
	n.Delete(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Read function failed: %v", resp.Diagnostics)
	}
}

func TestNamespace_ImportState(t *testing.T) {
	n := &Namespace{
		Client: mocks.MockNamespaceServiceClient{},
	}
	resp := resource.ImportStateResponse{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"name": tftypes.String,
					"id":   tftypes.String,
				},
			},
				map[string]tftypes.Value{
					"name": tftypes.NewValue(tftypes.String, "sample"),
					"id":   tftypes.NewValue(tftypes.String, ""),
				}),
			Schema: resourceNamespaceSchema(),
		},
	}
	n.ImportState(context.Background(), resource.ImportStateRequest{
		ID: "1234",
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("ImportState function failed: %v", resp.Diagnostics)
	}
}
