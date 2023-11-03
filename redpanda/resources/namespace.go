package resources

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Namespace{}
var _ resource.ResourceWithImportState = &Namespace{}
var _ resource.ResourceWithConfigure = &Namespace{}

type Namespace struct {
	Client cloudv1beta1.NamespaceServiceClient
}

func (n Namespace) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "name"
}

func (n Namespace) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	p, ok := request.ProviderData.(utils.ResourceData)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}
	c := p.CloudV2Client
	if c == nil {
		response.Diagnostics.AddError(
			"CloudV2Client not configured",
			"Please report this issue to the provider developers.",
		)
		return
	}
	n.Client = p.CloudV2Client
}

func (n Namespace) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "name of the namespace",
			},
		},
		Description: "A Redpanda Cloud namespace",
	}
}

func (n Namespace) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data models.Namespace
	var config models.Namespace
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	spew.Dump(data)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	spew.Dump(config)
	if resp.Diagnostics.HasError() {
		return
	}
	namespace, err := n.Client.CreateNamespace(ctx, &cloudv1beta1.CreateNamespaceRequest{
		Namespace: &cloudv1beta1.Namespace{},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create namespace", err.Error())
		return
	}
	data.Id = types.StringValue(namespace.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
	return
}

func (n Namespace) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.Namespace
	req.State.Get(ctx, &model)
	_, err := n.Client.GetNamespace(ctx, &cloudv1beta1.GetNamespaceRequest{
		Id: model.Name.String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read namespace", err.Error())
	}
}

func (n Namespace) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

}

func (n Namespace) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (n Namespace) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
}
