// Copyright 2026 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package cloudprovideraccess

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

// ResourceCloudProviderAccessSchema returns the Terraform schema for the cloud_provider_access resource.
func ResourceCloudProviderAccessSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "A Redpanda Cloud cross-account cloud provider access configuration",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Display name for the cloud provider access configuration.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},

			"cloud_provider": schema.StringAttribute{
				Description:   "Cloud provider for this access configuration. Currently only `aws` is supported.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    validators.CloudProviders(),
			},

			"aws": schema.SingleNestedAttribute{
				Description:   "AWS-specific configuration for the cross-account IAM role.",
				Optional:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"role_arn": schema.StringAttribute{
						Description: "The full ARN of the customer IAM role that Redpanda will assume for cross-account provisioning.",
						Required:    true,
					},
					"external_id": schema.StringAttribute{
						Description: "External ID for the IAM trust policy's sts:ExternalId condition. Derived from the organization ID by the server.",
						Computed:    true,
						// The server derives external_id from the organization
						// ID, so it is stable across replacements of this
						// resource within the same organization — safe to
						// carry the prior value through replace plans.
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},

			"id": schema.StringAttribute{
				Description:   "Unique identifier of the cloud provider access configuration.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			"state": schema.StringAttribute{
				Description: "Current state of the configuration (e.g. PENDING, ACTIVE, FAILED).",
				Computed:    true,
			},

			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}
