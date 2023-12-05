package tests

import (
	"context"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"maps"
	"os"
	"strings"
	"testing"
)

const (
	awsDedicatedClusterFile = "../../examples/dedicated/aws/main.tf"
	gcpDedicatedClusterFile = "../../examples/dedicated/gcp/main.tf"
	dedicatedNamespaceFile  = "../../examples/namespace/main.tf"
)

var runClusterTests = os.Getenv("RUN_CLUSTER_TESTS")
var accNamePrepend = "tfrp-acc-"
var clientId = os.Getenv("CLIENT_ID")
var clientSecret = os.Getenv("CLIENT_SECRET")

func TestAccResourcesNamespace(t *testing.T) {
	ctx := context.Background()
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIdSecretVars)
	origTestCaseVars["name"] = config.StringVariable(accNamePrepend + "testname")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, providerCfgIdSecretVars)
	updateTestCaseVars["name"] = config.StringVariable(accNamePrepend + "testname2")

	nsClient, err := clients.NewNamespaceServiceClient(ctx, "ign", clients.ClientRequest{
		ClientID:     clientId,
		ClientSecret: clientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	var importId string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", accNamePrepend+"testname"),
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", accNamePrepend+"testname2"),
					func(s *terraform.State) error {
						i, err := utils.FindNamespaceByName(ctx, accNamePrepend+"testname2", nsClient)
						if err != nil {
							return err
						}
						importId = i.GetId()
						return nil
					}),
			},
			{
				ResourceName:             "redpanda_namespace.test",
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateId:            importId,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", accNamePrepend+"testname2"),
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})

	resource.AddTestSweepers(accNamePrepend+"testname", &resource.Sweeper{
		Name: accNamePrepend + "testname",
		F: sweepNamespace{
			AccNamePrepend: accNamePrepend,
			NamespaceName:  "testname",
			Client:         nsClient,
			Version:        "ign",
		}.SweepNamespaces})
	resource.AddTestSweepers(accNamePrepend+"testname2", &resource.Sweeper{
		Name: accNamePrepend + "testname2",
		F: sweepNamespace{
			AccNamePrepend: accNamePrepend,
			NamespaceName:  "testname2",
			Client:         nsClient,
			Version:        "ign",
		}.SweepNamespaces})

}

func TestAccResourcesClusterAWS(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
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
		},
	})
}

func TestAccResourcesClusterGCP(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
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
		},
	})
}
