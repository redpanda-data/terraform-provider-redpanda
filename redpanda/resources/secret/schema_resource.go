// Copyright 2025 Redpanda Data, Inc.
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

package secret

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// secretNamePattern: CreateSecretRequest.id validate.field rule from secret.proto.
var secretNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// secretLabelValuePattern: validate.field rule on Secret.labels values from secret.proto.
var secretLabelValuePattern = regexp.MustCompile(`^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`)

// ResourceSecretSchema returns the schema for the Secret resource.
func ResourceSecretSchema() schema.Schema {
	return schema.Schema{
		Description: "Defines the secret resource.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Secret identifier.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(secretNamePattern, "must match ^[A-Z][A-Z0-9_]*$ (uppercase letters, digits, underscores; must start with a letter)"),
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"secret_data": schema.StringAttribute{
				Description: "The secret data. Must be Base64-encoded.",
				Required:    true,
				WriteOnly:   true,
				Sensitive:   true,
			},
			"secret_data_version": schema.Int64Attribute{
				Description:   "Version counter for `secret_data`. Increment this each time you change `secret_data`; Terraform compares versions to decide when to call UpdateSecret (write-only values can't be diffed directly). TF-only construct — not part of the API.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"scopes": schema.ListAttribute{
				Description: "Secret scopes",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.UniqueValues(),
					listvalidator.ValueStringsAre(stringvalidator.OneOf(
						"SCOPE_REDPANDA_CONNECT",
						"SCOPE_REDPANDA_CLUSTER",
						"SCOPE_MCP_SERVER",
						"SCOPE_AI_AGENT",
						"SCOPE_AI_GATEWAY",
					)),
				},
			},
			"labels": schema.MapAttribute{
				Description:   "Secret labels.",
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace(), mapplanmodifier.UseStateForUnknown()},
				Validators: []validator.Map{
					mapvalidator.ValueStringsAre(stringvalidator.RegexMatches(secretLabelValuePattern, "must match ^([\\p{L}\\p{Z}\\p{N}_.:/=+\\-@]*)$")),
				},
			},
			"cluster_api_url": schema.StringAttribute{
				Required:      true,
				Description:   "Dataplane API URL of the cluster that owns this secret (`redpanda_cluster.<name>.cluster_api_url`). Immutable; changing this prevents deletion of the existing secret. Generally easier to recreate the resource than to change this.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Secret identifier.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Allows deletion of the secret. Defaults to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}
