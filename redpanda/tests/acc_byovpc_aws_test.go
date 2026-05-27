//go:build live_test && (all || byovpc_aws)

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

package tests

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_Cluster_BYOVPC_AWS(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + acc.CloudLabelAWS)
	rename := acc.RandomName(acc.NamePrefix + acc.CloudLabelAWSRename)

	var privateSubnetArns []string
	privateSubnetArnsEnv := os.Getenv("RP_PRIVATE_SUBNET_ARNS")
	if err := json.Unmarshal([]byte(privateSubnetArnsEnv), &privateSubnetArns); err != nil {
		t.Fatalf("Error parsing private subnet ARNs: %v", err)
	}
	var zones []string
	zonesEnv := os.Getenv("AWS_ZONES")
	if err := json.Unmarshal([]byte(zonesEnv), &zones); err != nil {
		t.Fatalf("Error parsing zones: %v", err)
	}
	customVars := map[string]config.Variable{
		"cloud_provider":                  config.StringVariable("aws"),
		"region":                          config.StringVariable(os.Getenv("AWS_REGION")),
		"aws_secret_key":                  config.StringVariable(os.Getenv("AWS_SECRET_ACCESS_KEY")),
		"aws_access_key":                  config.StringVariable(os.Getenv("AWS_ACCESS_KEY_ID")),
		"management_bucket_arn":           config.StringVariable(os.Getenv("RP_MANAGEMENT_BUCKET_ARN")),
		"dynamodb_table_arn":              config.StringVariable(os.Getenv("RP_DYNAMODB_TABLE_ARN")),
		"vpc_arn":                         config.StringVariable(os.Getenv("RP_VPC_ARN")),
		"permissions_boundary_policy_arn": config.StringVariable(os.Getenv("RP_PERMISSIONS_BOUNDARY_POLICY_ARN")),
		"agent_instance_profile_arn":      config.StringVariable(os.Getenv("RP_AGENT_INSTANCE_PROFILE_ARN")),
		"connectors_node_group_instance_profile_arn": config.StringVariable(os.Getenv("RP_CONNECTORS_NODE_GROUP_INSTANCE_PROFILE_ARN")),
		"utility_node_group_instance_profile_arn":    config.StringVariable(os.Getenv("RP_UTILITY_NODE_GROUP_INSTANCE_PROFILE_ARN")),
		"redpanda_node_group_instance_profile_arn":   config.StringVariable(os.Getenv("RP_REDPANDA_NODE_GROUP_INSTANCE_PROFILE_ARN")),
		"k8s_cluster_role_arn":                       config.StringVariable(os.Getenv("RP_K8S_CLUSTER_ROLE_ARN")),
		"redpanda_agent_security_group_arn":          config.StringVariable(os.Getenv("RP_REDPANDA_AGENT_SECURITY_GROUP_ARN")),
		"connectors_security_group_arn":              config.StringVariable(os.Getenv("RP_CONNECTORS_SECURITY_GROUP_ARN")),
		"redpanda_node_group_security_group_arn":     config.StringVariable(os.Getenv("RP_REDPANDA_NODE_GROUP_SECURITY_GROUP_ARN")),
		"utility_security_group_arn":                 config.StringVariable(os.Getenv("RP_UTILITY_SECURITY_GROUP_ARN")),
		"cluster_security_group_arn":                 config.StringVariable(os.Getenv("RP_CLUSTER_SECURITY_GROUP_ARN")),
		"node_security_group_arn":                    config.StringVariable(os.Getenv("RP_NODE_SECURITY_GROUP_ARN")),
		"cloud_storage_bucket_arn":                   config.StringVariable(os.Getenv("RP_CLOUD_STORAGE_BUCKET_ARN")),
	}

	if len(privateSubnetArns) > 0 {
		subnetVars := make([]config.Variable, len(privateSubnetArns))
		for i, arn := range privateSubnetArns {
			subnetVars[i] = config.StringVariable(arn)
		}
		customVars["private_subnet_arns"] = config.ListVariable(subnetVars...)
	}

	if len(zones) > 0 {
		zonesVars := make([]config.Variable, len(zones))
		for i, zone := range zones {
			zonesVars[i] = config.StringVariable(zone)
		}
		customVars["zones"] = config.ListVariable(zonesVars...)
	}

	testRunnerClusterWithAwsPrivateLinkToggle(ctx, name, rename, acc.RedpandaVersion, acc.AwsByocVpcClusterDir, customVars, t)
}
