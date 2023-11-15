package redpanda

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"os"
	"testing"
)

var (
	cfgProvider = `
provider "redpanda" {
	client_id = "` + os.Getenv("CLIENT_ID") + `"
	client_secret = "` + os.Getenv("CLIENT_SECRET") + `"
}
`
	cfgAccResourceNamespaceSuccess = `
resource "redpanda_namespace" "test" {
	name = "testname"
}
`
)

func TestAccExampleResource(t *testing.T) {
	fmt.Println(cfgProvider + cfgAccResourceNamespaceSuccess)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgProvider + cfgAccResourceNamespaceSuccess,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", "testname"),
				),
			},
			{
				Config:  cfgProvider + cfgAccResourceNamespaceSuccess,
				Destroy: true,
			},
		},
	})
}
