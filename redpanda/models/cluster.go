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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Cluster represents the Terraform schema for the cluster resource.
type Cluster struct {
	Name                     types.String        `tfsdk:"name"`
	ID                       types.String        `tfsdk:"id"`
	ConnectionType           types.String        `tfsdk:"connection_type"`
	CloudProvider            types.String        `tfsdk:"cloud_provider"`
	ClusterType              types.String        `tfsdk:"cluster_type"`
	RedpandaVersion          types.String        `tfsdk:"redpanda_version"`
	ThroughputTier           types.String        `tfsdk:"throughput_tier"`
	Region                   types.String        `tfsdk:"region"`
	Zones                    types.List          `tfsdk:"zones"`
	AllowDeletion            types.Bool          `tfsdk:"allow_deletion"`
	CreatedAt                types.String        `tfsdk:"created_at"`
	State                    types.String        `tfsdk:"state"`
	StateDescription         types.Object        `tfsdk:"state_description"`
	Tags                     types.Map           `tfsdk:"tags"`
	ResourceGroupID          types.String        `tfsdk:"resource_group_id"`
	NetworkID                types.String        `tfsdk:"network_id"`
	ClusterAPIURL            types.String        `tfsdk:"cluster_api_url"`
	AwsPrivateLink           types.Object        `tfsdk:"aws_private_link"`
	GcpPrivateServiceConnect types.Object        `tfsdk:"gcp_private_service_connect"`
	AzurePrivateLink         types.Object        `tfsdk:"azure_private_link"`
	KafkaAPI                 types.Object        `tfsdk:"kafka_api"`
	HTTPProxy                types.Object        `tfsdk:"http_proxy"`
	SchemaRegistry           types.Object        `tfsdk:"schema_registry"`
	KafkaConnect             types.Object        `tfsdk:"kafka_connect"`
	ReadReplicaClusterIDs    types.List          `tfsdk:"read_replica_cluster_ids"`
	CustomerManagedResources types.Object        `tfsdk:"customer_managed_resources"`
	Prometheus               types.Object        `tfsdk:"prometheus"`
	RedpandaConsole          types.Object        `tfsdk:"redpanda_console"`
	MaintenanceWindowConfig  types.Object        `tfsdk:"maintenance_window_config"`
	Timeouts                 types.Object        `tfsdk:"timeouts"`
	GCPGlobalAccessEnabled   basetypes.BoolValue `tfsdk:"gcp_global_access_enabled"`
}
