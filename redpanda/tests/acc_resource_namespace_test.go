package tests

import (
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccNamespaceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile("../../examples/namespace/main.tf"),
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
			{
				ConfigFile:               config.StaticFile("../../examples/namespace/main.tf"),
				ConfigVariables:          providerCfgIdSecretVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
		},
	})
}
