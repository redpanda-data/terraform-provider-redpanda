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

package resources

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/acl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/cluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/pipeline"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/region"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/regions"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/resourcegroup"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/role"
	rpschema "github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/schemaregistryacl"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlesscluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlessprivatelink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serverlessregions"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/serviceaccount"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/shadowlink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/throughputtiers"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/topic"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/user"
)

// TestSchemaGolden validates every resource and datasource schema against its
// golden file. Captures all structural details except descriptions.
//
// Flags:
//
//	-update           regenerate .golden (and .description if -descriptions) files
//	-descriptions     also compare field descriptions against .description files
//
// Examples:
//
//	go test ./redpanda/resources/ -run TestSchemaGolden                         # structural only
//	go test ./redpanda/resources/ -run TestSchemaGolden -descriptions           # structural + descriptions
//	go test ./redpanda/resources/ -run TestSchemaGolden -update                 # regenerate .golden
//	go test ./redpanda/resources/ -run TestSchemaGolden -update -descriptions   # regenerate both
//
// Add a new entry when adding a resource or datasource. A missing golden file
// causes the test to fail with a clear message.
func TestSchemaGolden(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		schema any
	}{
		{"acl_resource", acl.ResourceACLSchema(ctx)},
		{"cluster_resource", cluster.ResourceClusterSchema(ctx)},
		{"network_resource", network.ResourceNetworkSchema(ctx)},
		{"pipeline_resource", pipeline.ResourcePipelineSchema(ctx)},
		{"resourcegroup_resource", resourcegroup.ResourceGroupSchema(ctx)},
		{"role_resource", role.ResourceRoleSchema(ctx)},
		{"schema_resource", rpschema.ResourceSchemaSchema(ctx)},
		{"schemaregistryacl_resource", schemaregistryacl.ResourceSchemaRegistryACLSchema(ctx)},
		{"serverlesscluster_resource", serverlesscluster.ResourceServerlessClusterSchema(ctx)},
		{"serverlessprivatelink_resource", serverlessprivatelink.ResourceServerlessPrivateLinkSchema(ctx)},
		{"serviceaccount_resource", serviceaccount.ResourceServiceAccountSchema(ctx)},
		{"shadowlink_resource", shadowlink.ResourceShadowLinkSchema(ctx)},
		{"topic_resource", topic.ResourceTopicSchema(ctx)},
		{"user_resource", user.ResourceUserSchema(ctx)},

		{"cluster_datasource", cluster.DatasourceClusterSchema(ctx)},
		{"network_datasource", network.DatasourceNetworkSchema(ctx)},
		{"region_datasource", region.DataSourceRegionSchema(ctx)},
		{"regions_datasource", regions.DataSourceRegionsSchema(ctx)},
		{"resourcegroup_datasource", resourcegroup.DatasourceResourceGroupSchema(ctx)},
		{"serverlesscluster_datasource", serverlesscluster.DatasourceServerlessClusterSchema(ctx)},
		{"serverlessregions_datasource", serverlessregions.DataSourceServerlessRegionsSchema(ctx)},
		{"throughputtiers_datasource", throughputtiers.DataSourceThroughputTiersSchema(ctx)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testutil.DumpSchema(tt.schema)
			golden := filepath.Join("testdata", tt.name+"_schema.golden")
			testutil.AssertGolden(t, golden, got)

			if *testutil.Descriptions {
				descs := testutil.DumpDescriptions(tt.schema)
				descFile := filepath.Join("testdata", tt.name+"_schema.description")
				testutil.AssertGolden(t, descFile, descs)
			}
		})
	}
}
