// Copyright 2023 Redpanda Data, Inc.
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

// Package region contains the implementation of the Region data
// source following the Terraform framework interfaces.
package region

import (
	"context"
	"fmt"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	regionmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/region"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

var _ datasource.DataSource = &DataSourceRegion{}

// DataSourceRegion represents a data source for a Redpanda Cloud region.
type DataSourceRegion struct {
	base.DataSourceBase
}

// NewDataSourceRegion constructs a Region datasource.
func NewDataSourceRegion() *DataSourceRegion {
	d := &DataSourceRegion{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_region", DataSourceRegionSchema, nil)
	return d
}

// Read reads the Region data source's values and updates the state.
func (r *DataSourceRegion) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model regionmodel.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudProvider := enums.StringToCloudProvider(model.CloudProvider.ValueString())
	if cloudProvider == controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED {
		resp.Diagnostics.AddError("unsupported cloud provider", fmt.Sprintf("unknown cloud provider %q", model.CloudProvider.ValueString()))
		return
	}

	region, err := r.CpCl.Region.GetRegion(ctx, &controlplanev1.GetRegionRequest{
		Name:          model.Name.ValueString(),
		CloudProvider: cloudProvider,
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read region %v", model.Name), utils.DeserializeGrpcError(err))
		return
	}
	if region.Region == nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read region %v; please report this bug to Redpanda Support", model.Name), "")
		return
	}

	persist, diags := regionmodel.FlattenData(ctx, region.Region, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}
