// Copyright 2023 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package user

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// resourceUserSchema returns the schema for the User resource.
func resourceUserSchema() schema.Schema {
	return schema.Schema{
		Description: "User is a user that can be created in Redpanda",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Name of the user, must be unique",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				Description: "Password of the user",
				Required:    true,
				Sensitive:   true,
			},
			"mechanism": schema.StringAttribute{
				Description: "Which authentication method to use, see https://docs.redpanda.com/current/manage/security/authentication/ for more information",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("", "scram-sha-256", "scram-sha-512"),
				},
			},
			"cluster_api_url": schema.StringAttribute{
				Required: true,
				Description: "The cluster API URL. Changing this will prevent deletion of the resource on the existing " +
					"cluster. It is generally a better idea to delete an existing resource and create a new one than to " +
					"change this value unless you are planning to do state imports",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Allows deletion of the user. If false, the user cannot be deleted and the resource will be removed from the state on destruction. Defaults to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}
