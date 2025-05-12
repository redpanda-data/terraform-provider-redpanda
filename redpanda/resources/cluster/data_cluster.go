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

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource = &DataSourceCluster{}
)

// DataSourceCluster represents a cluster data source.
type DataSourceCluster struct {
	CpCl *cloud.ControlPlaneClientSet
}

// Metadata returns the metadata for the Cluster data source.
func (*DataSourceCluster) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "redpanda_cluster"
}

// Configure uses provider level data to configure DataSourceCluster's client.
func (d *DataSourceCluster) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(config.Datasource)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	d.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
}

// Read reads the Cluster data source's values and updates the state.
func (d *DataSourceCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model models.Cluster
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	cluster, err := d.CpCl.ClusterForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.Diagnostics.AddError(fmt.Sprintf("unable to find cluster %s", model.ID), utils.DeserializeGrpcError(err))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read cluster %s", model.ID), utils.DeserializeGrpcError(err))
		return
	}

	// Handle clusters in deleting states - add warning but still return the data
	if cluster.GetState() == controlplanev1.Cluster_STATE_DELETING || cluster.GetState() == controlplanev1.Cluster_STATE_DELETING_AGENT {
		resp.Diagnostics.AddWarning(fmt.Sprintf("cluster %s is in state %s", model.ID.ValueString(), cluster.GetState()), "")
	}

	// Convert cloud provider tags to Terraform map
	tags, err := utils.StringMapToTypesMap(cluster.GetCloudProviderTags())
	if err != nil {
		resp.Diagnostics.AddError("error converting tags to MapType", err.Error())
		return
	}

	persist, dg := generateModel(cluster, modelOrAPI{
		RedpandaVersion:       types.StringValue(cluster.GetCurrentRedpandaVersion()),
		Tags:                  tags,
		GcpGlobalAccessConfig: types.BoolValue(cluster.GetGcpGlobalAccessEnabled()),
	}, resp.Diagnostics)
	if dg.HasError() {
		resp.Diagnostics.AddError("error generating model", "failed to generate model in cluster datasource read")
		resp.Diagnostics.Append(dg...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Schema returns the schema for the Cluster data source.
func (*DataSourceCluster) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceClusterSchema() // Reuse the schema from the resource
}
