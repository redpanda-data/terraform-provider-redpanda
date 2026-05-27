//go:build live_test && (all || byovpc_gcp)

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
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_Cluster_BYOVPC_GCP(t *testing.T) {
	ctx := context.Background()
	name := acc.RandomName(acc.NamePrefix + "testgcp")
	rename := acc.RandomName(acc.NamePrefix + "testgcp-rename")

	customVars := map[string]config.Variable{
		"region":                                 config.StringVariable(os.Getenv("GCP_REGION")),
		"network_project_id":                     config.StringVariable(os.Getenv("GCP_PROJECT_ID")),
		"vpc_network_name":                       config.StringVariable(os.Getenv("GCP_VPC_NETWORK_NAME")),
		"management_bucket_name":                 config.StringVariable(os.Getenv("GCP_MANAGEMENT_BUCKET_NAME")),
		"subnet_name":                            config.StringVariable(os.Getenv("GCP_SUBNET_NAME")),
		"secondary_ipv4_range_pods_name":         config.StringVariable(os.Getenv("GCP_SECONDARY_IPV4_RANGE_PODS_NAME")),
		"secondary_ipv4_range_services_name":     config.StringVariable(os.Getenv("GCP_SECONDARY_IPV4_RANGE_SERVICES_NAME")),
		"k8s_master_ipv4_range":                  config.StringVariable(os.Getenv("GCP_K8S_MASTER_IPV4_RANGE")),
		"agent_service_account_email":            config.StringVariable(os.Getenv("GCP_AGENT_SERVICE_ACCOUNT_EMAIL")),
		"console_service_account_email":          config.StringVariable(os.Getenv("GCP_CONSOLE_SERVICE_ACCOUNT_EMAIL")),
		"connector_service_account_email":        config.StringVariable(os.Getenv("GCP_CONNECTOR_SERVICE_ACCOUNT_EMAIL")),
		"redpanda_cluster_service_account_email": config.StringVariable(os.Getenv("GCP_REDPANDA_CLUSTER_SERVICE_ACCOUNT_EMAIL")),
		"gke_service_account_email":              config.StringVariable(os.Getenv("GCP_GKE_SERVICE_ACCOUNT_EMAIL")),
		"tiered_storage_bucket_name":             config.StringVariable(os.Getenv("GCP_TIERED_STORAGE_BUCKET_NAME")),
	}

	testRunnerCluster(ctx, name, rename, acc.RedpandaVersion, acc.GcpByoVpcClusterDir, customVars, t)
}
