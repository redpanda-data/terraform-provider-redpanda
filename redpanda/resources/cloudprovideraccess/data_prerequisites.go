// Copyright 2026 Redpanda Data, Inc.
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

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	cpamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cloudprovideraccess"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/validators"
)

var _ datasource.DataSource = &DataSourcePrerequisites{}

// DataSourcePrerequisites represents a data source for the values a cloud
// account must trust before a cloud provider access can use it.
type DataSourcePrerequisites struct {
	base.DataSourceBase
}

// NewDataSourcePrerequisites constructs a cloud provider access
// prerequisites datasource.
func NewDataSourcePrerequisites() *DataSourcePrerequisites {
	d := &DataSourcePrerequisites{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_cloud_provider_access_prerequisites", DataSourcePrerequisitesSchema, nil)
	return d
}

// DataSourcePrerequisitesSchema defines the schema for the cloud provider
// access prerequisites data source.
func DataSourcePrerequisitesSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Description: "Data source for the values an IAM role trust policy needs before a `redpanda_cloud_provider_access` can use the role",
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Required:    true,
				Description: "Cloud provider to get prerequisites for. Only `aws` is supported.",
				Validators:  validators.CloudProviderAccessProviders(),
			},
			"aws": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "AWS prerequisites. Set when cloud_provider is `aws`.",
				Attributes: map[string]schema.Attribute{
					"principal_arn": schema.StringAttribute{
						Computed:    true,
						Description: "ARN of the Redpanda principal that assumes the role. Allow it as the `Principal` in the IAM role trust policy.",
					},
					"external_id": schema.StringAttribute{
						Computed:    true,
						Description: "External ID to require in the IAM role trust policy's `sts:ExternalId` condition.",
					},
				},
			},
		},
	}
}

// Read reads the prerequisites for the configured cloud provider.
func (d *DataSourcePrerequisites) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model cpamodel.PrerequisitesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.CpCl.CloudProviderAccess.GetCloudProviderAccessPrerequisites(ctx, &controlplanev1.GetCloudProviderAccessPrerequisitesRequest{
		CloudProvider: enums.StringToCloudProvider(model.CloudProvider.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to read cloud provider access prerequisites", utils.DeserializeGrpcError(err))
		return
	}
	cpamodel.FlattenPrerequisites(&model, apiResp)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
