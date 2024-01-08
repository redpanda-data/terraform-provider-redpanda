package user

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &User{}
var _ resource.ResourceWithConfigure = &User{}
var _ resource.ResourceWithImportState = &User{}

type User struct {
	UserClient dataplanev1alpha1.UserServiceClient
}

func (u *User) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "redpanda_user"
}

func (u *User) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		response.Diagnostics.AddWarning("provider data not set", "provider data not set at user.Configure")
	}
	p, ok := request.ProviderData.(utils.ResourceData)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
	}
	client, err := clients.NewUserServiceClient(ctx, p.Version, clients.ClientRequest{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create user client", err.Error())
		return
	}
	u.UserClient = client
}

func (u *User) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = ResourceUserSchema()
}

func ResourceUserSchema() schema.Schema {
	return schema.Schema{
		Description: "User is a user that can be created in Redpanda",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Name of the user, must be unique",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				Description:   "Password of the user",
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mechanism": schema.StringAttribute{
				Description:   "Which authentication method to use, see https://docs.redpanda.com/current/manage/security/authentication/ for more information",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (u *User) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var model models.User
	response.Diagnostics.Append(request.Plan.Get(ctx, &model)...)

	user, err := u.UserClient.CreateUser(ctx, &dataplanev1alpha1.CreateUserRequest{
		User: &dataplanev1alpha1.CreateUserRequest_User{
			Name:      model.Name.ValueString(),
			Password:  model.Password.ValueString(), // this seems wrong and bad
			Mechanism: utils.StringToUserMechanism(model.Mechanism.ValueString()),
		},
	})
	if err != nil {
		response.Diagnostics.AddError("failed to create user", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, models.User{
		Name:      types.StringValue(user.User.Name),
		Password:  model.Password,
		Mechanism: types.StringValue(utils.UserMechanismToString(user.User.Mechanism)),
	})...)
}

func (u *User) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var model models.User
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	usr, err := utils.FindUserByName(ctx, model.Name.ValueString(), u.UserClient)
	if err != nil {
		if utils.IsNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		} else {
			response.Diagnostics.AddError(fmt.Sprintf("failed receive response from user api for user %s", model.Name), err.Error())
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(ctx, models.User{
		Name:      types.StringValue(usr.Name),
		Password:  model.Password,
		Mechanism: types.StringValue(utils.UserMechanismToString(usr.Mechanism)),
	})...)
}

func (u *User) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// TODO implement me
}

func (u *User) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var model models.User
	response.Diagnostics.Append(request.State.Get(ctx, &model)...)
	_, err := u.UserClient.DeleteUser(ctx, &dataplanev1alpha1.DeleteUserRequest{
		Name: model.Name.ValueString(),
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("failed to delete user %s", model.Name), err.Error())
		return
	}

}

func (u *User) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	response.Diagnostics.Append(response.State.Set(ctx, models.User{
		Name: types.StringValue(request.ID),
	})...)
}
