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

package network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	networkmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceNetwork{}

// DataSourceNetwork represents a data source for a Redpanda Cloud network.
type DataSourceNetwork struct {
	base.DataSourceBase
}

// NewDataSourceNetwork constructs a Network datasource.
func NewDataSourceNetwork() *DataSourceNetwork {
	d := &DataSourceNetwork{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_network", DatasourceNetworkSchema, nil)
	return d
}

// Read reads the Network data source's values and updates the state.
func (n *DataSourceNetwork) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model networkmodel.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	nw, err := n.CpCl.NetworkForID(ctx, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read network %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := networkmodel.FlattenData(ctx, nw, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
