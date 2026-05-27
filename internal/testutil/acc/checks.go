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

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// BuildTestCheckFuncs reads the Terraform config under testDir/main.tf and
// returns the resource.TestCheckFunc set appropriate for the resources
// declared there. hasPrivateNetworking optionally toggles serverless
// private-networking assertions on or off explicitly.
func BuildTestCheckFuncs(testDir, name string, hasPrivateNetworking ...bool) ([]resource.TestCheckFunc, error) {
	testFileContent, err := os.ReadFile(testDir + "/main.tf") // #nosec G304 -- testDir is controlled by test constants
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}
	testFileStr := string(testFileContent)

	var checkFuncs []resource.TestCheckFunc

	if strings.Contains(testFileStr, `resource "redpanda_resource_group" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(ResourceGroupName, "name", name),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_network" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(NetworkResourceName, "name", name),
			resource.TestCheckResourceAttrSet(NetworkResourceName, "state"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_cluster" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(ClusterResourceName, "name", name),
			resource.TestCheckResourceAttrSet(ClusterResourceName, "current_redpanda_version"),
			resource.TestCheckResourceAttrSet(ClusterResourceName, "api_gateway_access"),
		)
		if strings.Contains(testFileStr, "custom_properties_json") {
			checkFuncs = append(checkFuncs,
				resource.TestCheckResourceAttrSet(ClusterResourceName, "cluster_configuration.custom_properties_json"),
			)
		}
	}

	if strings.Contains(testFileStr, `resource "redpanda_serverless_cluster" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(ServerlessResourceName, "name", name),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "id"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "cluster_api_url"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "state"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "kafka_api.seed_brokers.#"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "kafka_api.seed_brokers.0"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "schema_registry.url"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "dataplane_api.url"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "console_url"),
			resource.TestCheckResourceAttrSet(ServerlessResourceName, "prometheus.url"),
		)

		privateNetworkingEnabled := false
		if len(hasPrivateNetworking) > 0 {
			privateNetworkingEnabled = hasPrivateNetworking[0]
		}

		if privateNetworkingEnabled {
			checkFuncs = append(checkFuncs,
				resource.TestCheckResourceAttrSet(ServerlessResourceName, "kafka_api.private_seed_brokers.#"),
				resource.TestCheckResourceAttrSet(ServerlessResourceName, "kafka_api.private_seed_brokers.0"),
				resource.TestCheckResourceAttrSet(ServerlessResourceName, "schema_registry.private_url"),
				resource.TestCheckResourceAttrSet(ServerlessResourceName, "dataplane_api.private_url"),
				resource.TestCheckResourceAttrSet(ServerlessResourceName, "console_private_url"),
				resource.TestCheckResourceAttrSet(ServerlessResourceName, "prometheus.private_url"),
			)
		}
	}

	if strings.Contains(testFileStr, `resource "redpanda_user" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(UserResourceName, "name", name))
	}

	if strings.Contains(testFileStr, `resource "redpanda_user" "test_user"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(TestUserResourceName, "name", name+"-test"))
	}

	if strings.Contains(testFileStr, `resource "redpanda_topic" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(TopicResourceName, "name", name))
		if strings.Contains(testFileStr, `"cleanup.policy"`) {
			checkFuncs = append(checkFuncs,
				resource.TestCheckResourceAttr(TopicResourceName, "configuration.cleanup.policy", "delete"),
				resource.TestCheckResourceAttr(TopicResourceName, "configuration.retention.ms", "604800000"),
			)
		}
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema" "user_schema"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(SchemaResourceName, "subject", name+"-value"),
			resource.TestCheckResourceAttr(SchemaResourceName, "schema_type", "AVRO"),
			resource.TestCheckResourceAttr(SchemaResourceName, "compatibility", "BACKWARD"),
			resource.TestCheckResourceAttrSet(SchemaResourceName, "id"),
			resource.TestCheckResourceAttrSet(SchemaResourceName, "version"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema" "user_event_schema"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(SchemaEventResourceName, "subject", name+"-events-value"),
			resource.TestCheckResourceAttr(SchemaEventResourceName, "references.#", "1"),
			resource.TestCheckResourceAttr(SchemaEventResourceName, "references.0.name", "User"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema" "product_schema"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(SchemaProductResourceName, "subject", name+"-product-value"),
			resource.TestCheckResourceAttr(SchemaProductResourceName, "compatibility", "FULL"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "read_product"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.read_product", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "resource_name", "product-"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "pattern_type", "PREFIXED"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "operation", "READ"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "write_orders"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.write_orders", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "resource_name", "orders-value"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "operation", "WRITE"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "all_test_topic"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.all_test_topic", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "resource_name", name+"-"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "pattern_type", "PREFIXED"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "operation", "ALL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "describe_test_topic"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.describe_test_topic", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "resource_name", name+"-"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "pattern_type", "PREFIXED"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "operation", "DESCRIBE"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "describe_registry"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.describe_registry", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "resource_type", "REGISTRY"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "resource_name", "*"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "operation", "DESCRIBE"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "alter_configs_registry"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.alter_configs_registry", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "resource_type", "REGISTRY"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "resource_name", "*"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "operation", "ALTER_CONFIGS"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_role" "developer"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(RoleResourceName, "id"),
			resource.TestCheckResourceAttr(RoleResourceName, "name", "developer"),
			resource.TestCheckResourceAttrSet(RoleResourceName, "cluster_api_url"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_acl" "role_topic_read"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(RoleTopicReadACLResourceName, "id"),
			resource.TestCheckResourceAttr(RoleTopicReadACLResourceName, "principal", "RedpandaRole:developer"),
			resource.TestCheckResourceAttr(RoleTopicReadACLResourceName, "resource_type", "TOPIC"),
			resource.TestCheckResourceAttr(RoleTopicReadACLResourceName, "resource_name", name),
			resource.TestCheckResourceAttr(RoleTopicReadACLResourceName, "resource_pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr(RoleTopicReadACLResourceName, "operation", "READ"),
			resource.TestCheckResourceAttr(RoleTopicReadACLResourceName, "permission_type", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_role_assignment" "developer_assignment"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(RoleAssignmentResourceName, "id"),
			resource.TestCheckResourceAttr(RoleAssignmentResourceName, "role_name", "developer"),
			resource.TestCheckResourceAttr(RoleAssignmentResourceName, "principal", "User:"+name),
			resource.TestCheckResourceAttrSet(RoleAssignmentResourceName, "cluster_api_url"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_pipeline" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(PipelineResourceName, "id"),
			resource.TestCheckResourceAttr(PipelineResourceName, "display_name", "test-pipeline"),
			resource.TestCheckResourceAttrSet(PipelineResourceName, "state"),
			resource.TestCheckResourceAttrSet(PipelineResourceName, "cluster_api_url"),
		)
	}

	return checkFuncs, nil
}
