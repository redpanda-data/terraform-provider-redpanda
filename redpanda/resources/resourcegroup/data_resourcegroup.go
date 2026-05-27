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

package resourcegroup

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	resourcegroupmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/resourcegroup"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceResourceGroup{}

// DataSourceResourceGroup represents a data source for a Redpanda Cloud resource group.
type DataSourceResourceGroup struct {
	base.DataSourceBase
}

// NewDataSourceResourceGroup constructs a ResourceGroup datasource.
func NewDataSourceResourceGroup() *DataSourceResourceGroup {
	d := &DataSourceResourceGroup{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_resource_group", DatasourceResourceGroupSchema, nil)
	return d
}

// Read reads the ResourceGroup data source's values and updates the state.
func (n *DataSourceResourceGroup) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model resourcegroupmodel.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	rg, err := n.CpCl.ResourceGroupForIDOrName(ctx, model.ID.ValueString(), model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read resource group", utils.DeserializeGrpcError(err))
		return
	}

	persist, diags := resourcegroupmodel.FlattenData(ctx, rg, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}
