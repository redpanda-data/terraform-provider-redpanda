package tests

import (
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIdSecretVars)
	origTestCaseVars["name"] = config.StringVariable(accNamePrepend + "testname")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, providerCfgIdSecretVars)
	updateTestCaseVars["name"] = config.StringVariable(accNamePrepend + "testname2")
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
				)},
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
			ClientId:       clientId,
			ClientSecret:   clientSecret,
			Version:        "ign",
		}.SweepNamespaces})
	resource.AddTestSweepers(accNamePrepend+"testname2", &resource.Sweeper{
		Name: accNamePrepend + "testname2",
		F: sweepNamespace{
			AccNamePrepend: accNamePrepend,
			NamespaceName:  "testname2",
			ClientId:       clientId,
			ClientSecret:   clientSecret,
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
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
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
			// TODO confirm i'm passing auth token right because the auth token tests aren't passing
		},
	})
}
