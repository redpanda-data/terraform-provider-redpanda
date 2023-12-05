package namespace

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Namespace{}
var _ resource.ResourceWithConfigure = &Namespace{}
var _ resource.ResourceWithImportState = &Namespace{}

type Namespace struct {
	Client cloudv1beta1.NamespaceServiceClient
}

func (n *Namespace) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "redpanda_namespace"
}

func (n *Namespace) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		// we can't add a diagnostic for an unset providerdata here because during the early part of the terraform
		// lifecycle, the provider data is not set and this is valid
		// but we also can't do anything until it is set
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at namespace.Configure")
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
	client, err := clients.NewNamespaceServiceClient(ctx, p.Version, clients.ClientRequest{
		AuthToken:    p.AuthToken,
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create namespace client", err.Error())
		return
	}
	n.Client = client
}

func (n *Namespace) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceNamespaceSchema()
}

// ResourceNamespaceSchema defines the schema for a namespace. Not used directly by TF but very helpful for tests
func ResourceNamespaceSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the namespace. Changing the name of a namespace will result in a new namespace being created and the old one being destroyed",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the namespace",
			},
		},
		Description: "A Redpanda Cloud namespace",
		Version:     1,
	}
}

func (n *Namespace) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model models.Namespace
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	ns, err := n.Client.CreateNamespace(ctx, &cloudv1beta1.CreateNamespaceRequest{
		Namespace: &cloudv1beta1.Namespace{
			Name: model.Name.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create namespace", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, models.Namespace{
		Name: types.StringValue(ns.Name),
		Id:   types.StringValue(ns.Id),
	})...)
	return
}

func (n *Namespace) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model models.Namespace
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	ns, err := n.Client.GetNamespace(ctx, &cloudv1beta1.GetNamespaceRequest{
		Id: model.Id.ValueString(),
	})

	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("failed to read namespace", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.Namespace{
		Name: types.StringValue(ns.Name),
		Id:   types.StringValue(ns.Id),
	})...)
}

func (n *Namespace) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model models.Namespace
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	ns, err := n.Client.UpdateNamespace(ctx, &cloudv1beta1.UpdateNamespaceRequest{
		Namespace: &cloudv1beta1.Namespace{
			Name: model.Name.ValueString(),
			Id:   model.Id.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update namespace", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, models.Namespace{
		Name: types.StringValue(ns.Name),
		Id:   types.StringValue(ns.Id),
	})...)
}

func (n *Namespace) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model models.Namespace
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	_, err := n.Client.DeleteNamespace(ctx, &cloudv1beta1.DeleteNamespaceRequest{
		Id: model.Id.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete namespace", err.Error())
		return
	}
}

// ImportState refreshes the state with the correct ID for the namespace, allowing TF to use Read to get the correct Namespace name into state
// see https://developer.hashicorp.com/terraform/plugin/framework/resources/import for more details
func (n *Namespace) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(ctx, models.Namespace{
		Id: types.StringValue(request.ID),
	})...)
}
