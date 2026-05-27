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

// Package serverlesscluster contains the implementation of the ServerlessCluster resource
// following the Terraform framework interfaces.
package serverlesscluster

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	serverlessclustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlesscluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceServerlessCluster{}

// DataSourceServerlessCluster represents a serverless cluster data source.
type DataSourceServerlessCluster struct {
	base.DataSourceBase
}

// NewDataSourceServerlessCluster constructs a ServerlessCluster datasource.
func NewDataSourceServerlessCluster() *DataSourceServerlessCluster {
	d := &DataSourceServerlessCluster{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_serverless_cluster", DatasourceServerlessClusterSchema, nil)
	return d
}

// Read reads the ServerlessCluster data source's values and updates the state.
func (d *DataSourceServerlessCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model serverlessclustermodel.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	cluster, err := d.CpCl.ServerlessClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.Diagnostics.AddError(fmt.Sprintf("unable to find serverless cluster %s", model.ID), utils.DeserializeGrpcError(err))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read serverless cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := serverlessclustermodel.FlattenData(ctx, cluster, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
