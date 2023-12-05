package tests

import (
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

const (
	awsDedicatedClusterFile = "../../examples/dedicated/aws/main.tf"
	gcpDedicatedClusterFile = "../../examples/dedicated/gcp/main.tf"
	dedicatedNamespaceFile  = "../../examples/namespace/main.tf"
)

func TestAccResourcesNamespace(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          providerCfgIdSecretVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:      config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables: providerCfgIdSecretVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", "testname"),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			// TODO work out what's going on with this one
			//{
			//	ResourceName:      "namespace.test",
			//  ConfigFile:      config.StaticFile("../../examples/namespace/main.tf"),
			// 	ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			//	ConfigVariables:   providerCfgVars,
			//	ImportState:       true,
			//	ImportStateVerify: true,
			//},
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
		},
	})
}

func TestAccResourcesClusterAWS(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile(awsDedicatedClusterFile),
				ConfigVariables: providerCfgIdSecretVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", "testname-aws"),
					resource.TestCheckResourceAttr("redpanda_network.test", "name", "testname-aws"),
					resource.TestCheckResourceAttr("redpanda_cluster.test", "name", "testname-aws"),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:               config.StaticFile(awsDedicatedClusterFile),
				ConfigVariables:          providerCfgIdSecretVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
		},
	})
}

func TestAccResourcesClusterGCP(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile(gcpDedicatedClusterFile),
				ConfigVariables: providerCfgIdSecretVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", "testname-gcp"),
					resource.TestCheckResourceAttr("redpanda_network.test", "name", "testname-gcp"),
					resource.TestCheckResourceAttr("redpanda_cluster.test", "name", "testname-gcp"),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:               config.StaticFile(gcpDedicatedClusterFile),
				ConfigVariables:          providerCfgIdSecretVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
		},
	})
}
