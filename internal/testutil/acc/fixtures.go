// Copyright 2026 Redpanda Data, Inc.
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

package acc

import "path/filepath"

// Fixture directories resolved against the repo root so callers at any
// package depth see the same absolute path.
var (
	AwsDedicatedClusterDir         = filepath.Join(RepoRoot(), "examples", "cluster", "aws")
	GcpDedicatedClusterDir         = filepath.Join(RepoRoot(), "examples", "cluster", "gcp")
	ServerlessClusterDir           = filepath.Join(RepoRoot(), "examples", "cluster", "serverless")
	ServerlessClusterPrivateDir    = filepath.Join(RepoRoot(), "redpanda", "tests", "testdata", "serverless_private")
	ServerlessPrivateLinkDir       = filepath.Join(RepoRoot(), "examples", "serverless_private_link", "aws")
	AwsByocClusterDir              = filepath.Join(RepoRoot(), "examples", "byoc", "aws")
	AwsByocVpcClusterDir           = filepath.Join(RepoRoot(), "redpanda", "tests", "testdata", "byovpc", "aws")
	GcpByoVpcClusterDir            = filepath.Join(RepoRoot(), "redpanda", "tests", "testdata", "byovpc", "gcp")
	GcpByocClusterDir              = filepath.Join(RepoRoot(), "examples", "byoc", "gcp")
	DedicatedNetworkDir            = filepath.Join(RepoRoot(), "examples", "network")
	DataplaneDir                   = filepath.Join(RepoRoot(), "examples", "dataplane")
	DataSourcesTestDir             = filepath.Join(RepoRoot(), "examples", "datasource", "standard")
	NetworkDataSourceDir           = filepath.Join(RepoRoot(), "examples", "datasource", "network")
	ServerlessRegionsDataSourceDir = filepath.Join(RepoRoot(), "examples", "datasource", "serverless_regions")
	ClusterDatasourceInfraDir      = filepath.Join(RepoRoot(), "redpanda", "tests", "testdata", "datasource", "cluster")
	ShadowLinkDir                  = filepath.Join(RepoRoot(), "examples", "shadow_link")
	ServiceAccountDir              = filepath.Join(RepoRoot(), "examples", "service_account")
)

// Terraform resource-address constants used as the second argument to
// resource.TestCheckResourceAttr and friends.
const (
	ResourceGroupName                  = "redpanda_resource_group.test"
	NetworkResourceName                = "redpanda_network.test"
	ClusterResourceName                = "redpanda_cluster.test"
	UserResourceName                   = "redpanda_user.test"
	TestUserResourceName               = "redpanda_user.test_user"
	TopicResourceName                  = "redpanda_topic.test"
	ServerlessResourceName             = "redpanda_serverless_cluster.test"
	ServerlessPrivateLinkResourceName  = "redpanda_serverless_private_link.example"
	NetworkDataSourceName              = "data.redpanda_network.test"
	ClusterDataSourceName              = "data.redpanda_cluster.test"
	ServerlessRegionsAWSDataSourceName = "data.redpanda_serverless_regions.aws"
	ServerlessRegionsGCPDataSourceName = "data.redpanda_serverless_regions.gcp"
	SchemaResourceName                 = "redpanda_schema.user_schema"
	SchemaEventResourceName            = "redpanda_schema.user_event_schema"
	SchemaProductResourceName          = "redpanda_schema.product_schema"
	ClusterAdminACLResourceName        = "redpanda_acl.cluster_admin"
	TopicAccessACLResourceName         = "redpanda_acl.topic_access"
	RoleTopicReadACLResourceName       = "redpanda_acl.role_topic_read"
	SchemaRegistryACLReadProductName   = "redpanda_schema_registry_acl.read_product"
	RoleResourceName                   = "redpanda_role.developer"
	RoleAssignmentResourceName         = "redpanda_role_assignment.developer_assignment"
	PipelineResourceName               = "redpanda_pipeline.test"
	ShadowLinkResourceName             = "redpanda_shadow_link.test"
	ShadowLinkSecretResourceName       = "redpanda_secret.source_password"
	ServiceAccountResourceName         = "redpanda_service_account.test"
	AllowDeletionFalseValue            = "false"
)
