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
	Name                     types.String              `tfsdk:"name"`
	ID                       types.String              `tfsdk:"id"`
	ConnectionType           types.String              `tfsdk:"connection_type"`
	CloudProvider            types.String              `tfsdk:"cloud_provider"`
	ClusterType              types.String              `tfsdk:"cluster_type"`
	RedpandaVersion          types.String              `tfsdk:"redpanda_version"`
	ThroughputTier           types.String              `tfsdk:"throughput_tier"`
	Region                   types.String              `tfsdk:"region"`
	Zones                    types.List                `tfsdk:"zones"`
	AllowDeletion            types.Bool                `tfsdk:"allow_deletion"`
	Tags                     types.Map                 `tfsdk:"tags"`
	ResourceGroupID          types.String              `tfsdk:"resource_group_id"`
	NetworkID                types.String              `tfsdk:"network_id"`
	ClusterAPIURL            types.String              `tfsdk:"cluster_api_url"`
	AwsPrivateLink           *AwsPrivateLink           `tfsdk:"aws_private_link"`
	GcpPrivateServiceConnect *GcpPrivateServiceConnect `tfsdk:"gcp_private_service_connect"`
	AzurePrivateLink         *AzurePrivateLink         `tfsdk:"azure_private_link"`
	KafkaAPI                 *KafkaAPI                 `tfsdk:"kafka_api"`
	HTTPProxy                *HTTPProxy                `tfsdk:"http_proxy"`
	SchemaRegistry           *SchemaRegistry           `tfsdk:"schema_registry"`
	ReadReplicaClusterIds    types.List                `tfsdk:"read_replica_cluster_ids"`
}

// AwsPrivateLink represents the Terraform schema for the AWS Private Link configuration.
type AwsPrivateLink struct {
	Enabled           types.Bool `tfsdk:"enabled"`
	ConnectConsole    types.Bool `tfsdk:"connect_console"`
	AllowedPrincipals types.List `tfsdk:"allowed_principals"`
}

// GcpPrivateServiceConnect represents the Terraform schema for the GCP Private Service Connect configuration.
type GcpPrivateServiceConnect struct {
	Enabled             types.Bool                          `tfsdk:"enabled"`
	GlobalAccessEnabled types.Bool                          `tfsdk:"global_access_enabled"`
	ConsumerAcceptList  []*GcpPrivateServiceConnectConsumer `tfsdk:"consumer_accept_list"`
}

// GcpPrivateServiceConnectConsumer represents the Terraform schema for the GCP Private Service Connect consumer configuration.
type GcpPrivateServiceConnectConsumer struct {
	Source string `tfsdk:"source"`
}

type AzurePrivateLink struct {
	AllowedSubscriptions types.List `tfsdk:"allowed_subscriptions"`
	ConnectConsole       types.Bool `tfsdk:"connect_console"`
	Enabled              types.Bool `tfsdk:"enabled"`
}

// KafkaAPI represents the Terraform schema for the Kafka API configuration.
type KafkaAPI struct {
	Mtls *Mtls `tfsdk:"mtls"`
}

// HTTPProxy represents the Terraform schema for the HTTP Proxy configuration.
type HTTPProxy struct {
	Mtls *Mtls `tfsdk:"mtls"`
}

// SchemaRegistry represents the Terraform schema for the Schema Registry configuration.
type SchemaRegistry struct {
	Mtls *Mtls `tfsdk:"mtls"`
}

// Mtls represents the Terraform schema for the mutual TLS configuration.
type Mtls struct {
	Enabled               types.Bool `tfsdk:"enabled"`
	CaCertificatesPem     types.List `tfsdk:"ca_certificates_pem"`
	PrincipalMappingRules types.List `tfsdk:"principal_mapping_rules"`
}
