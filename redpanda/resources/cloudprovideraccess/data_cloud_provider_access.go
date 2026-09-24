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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	cpamodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cloudprovideraccess"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceCloudProviderAccess{}

// DataSourceCloudProviderAccess represents a data source for a Redpanda Cloud
// cloud provider access.
type DataSourceCloudProviderAccess struct {
	base.DataSourceBase
}

// NewDataSourceCloudProviderAccess constructs a CloudProviderAccess datasource.
func NewDataSourceCloudProviderAccess() *DataSourceCloudProviderAccess {
	d := &DataSourceCloudProviderAccess{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_cloud_provider_access", DatasourceCloudProviderAccessSchema, nil)
	return d
}

// Read reads the CloudProviderAccess data source's values and updates the state.
func (d *DataSourceCloudProviderAccess) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model cpamodel.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cpa, err := d.CpCl.CloudProviderAccessForID(ctx, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cloud provider access %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	state, diags := cpamodel.FlattenData(ctx, cpa, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
