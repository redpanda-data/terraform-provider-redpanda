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

// Package cluster contains the implementation of the Cluster resource
// following the Terraform framework interfaces.
package cluster

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var _ datasource.DataSource = &DataSourceCluster{}

// DataSourceCluster represents a cluster data source.
type DataSourceCluster struct {
	base.DataSourceBase
}

// NewDataSourceCluster constructs a Cluster datasource.
func NewDataSourceCluster() *DataSourceCluster {
	d := &DataSourceCluster{}
	d.DataSourceBase = base.NewDataSourceBase("redpanda_cluster", DatasourceClusterSchema, nil)
	return d
}

// Read reads the Cluster data source's values and updates the state.
func (d *DataSourceCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model clustermodel.DataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	readTimeout, diags := model.Timeouts.Read(ctx, 10*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	cl, err := d.CpCl.ClusterForID(timeoutCtx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.Diagnostics.AddError(fmt.Sprintf("unable to find cluster %s", model.ID), utils.DeserializeGrpcError(err))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	if cl.GetState() == controlplanev1.Cluster_STATE_DELETING || cl.GetState() == controlplanev1.Cluster_STATE_DELETING_AGENT {
		resp.Diagnostics.AddWarning(fmt.Sprintf("cluster %s is in state %s", model.ID.ValueString(), cl.GetState()), "")
	}

	state, flatDiags := clustermodel.FlattenData(ctx, cl, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
