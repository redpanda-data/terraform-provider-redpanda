//go:build integration

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

package serverlessregions_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// TestIntegration_ServerlessRegions exercises redpanda_serverless_regions datasource
// against the bufconn fake. ServerlessRegionFake is pre-seeded with one AWS
// region; the test reads it and asserts the seed attributes.
func TestIntegration_ServerlessRegions(t *testing.T) {
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")

	srv := mock.New(t)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": provider.NewMuxedServer(context.Background(), "pre", "test",
			provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...)),
			provider.WithProviderOption(redpanda.WithSkipAuth()),
		),
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "redpanda" {}
data "redpanda_serverless_regions" "aws" {
  cloud_provider = "aws"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.redpanda_serverless_regions.aws", "serverless_regions.#"),
					resource.TestCheckResourceAttr("data.redpanda_serverless_regions.aws", "cloud_provider", "aws"),
					resource.TestCheckResourceAttr("data.redpanda_serverless_regions.aws", "serverless_regions.0.name", "pro-us-east-1"),
					resource.TestCheckResourceAttr("data.redpanda_serverless_regions.aws", "serverless_regions.0.placement.enabled", "true"),
				),
			},
		},
	})
}
