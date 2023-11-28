package tests

import (
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

const (
	dedicatedClusterFile   = "../../examples/dedicated/main.tf"
	dedicatedNamespaceFile = "../../examples/namespace/main.tf"
)

func TestAccResourcesNamespace(t *testing.T) {
	resource.Test(t, resource.TestCase{
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

func TestAccResourcesNetwork(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile(dedicatedClusterFile),
				ConfigVariables: providerCfgIdSecretVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", "testname"),
					resource.TestCheckResourceAttr("redpanda_network.test", "name", "testname"),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:               config.StaticFile(dedicatedClusterFile),
				ConfigVariables:          providerCfgIdSecretVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
		},
	})
}
