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

package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Cluster represents the Terraform schema for the cluster resource.
type Cluster struct {
	Name            types.String `tfsdk:"name"`
	ID              types.String `tfsdk:"id"`
	ConnectionType  types.String `tfsdk:"connection_type"`
	CloudProvider   types.String `tfsdk:"cloud_provider"`
	ClusterType     types.String `tfsdk:"cluster_type"`
	RedpandaVersion types.String `tfsdk:"redpanda_version"`
	ThroughputTier  types.String `tfsdk:"throughput_tier"`
	Region          types.String `tfsdk:"region"`
	Zones           types.List   `tfsdk:"zones"`
	AllowDeletion   types.Bool   `tfsdk:"allow_deletion"`
	Tags            types.Map    `tfsdk:"tags"`
	ResourceGroupID types.String `tfsdk:"resource_group_id"`
	NetworkID       types.String `tfsdk:"network_id"`
	ClusterAPIURL   types.String `tfsdk:"cluster_api_url"`
}
