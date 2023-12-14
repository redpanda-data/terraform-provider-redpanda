package tests

import (
	"context"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

const (
	awsDedicatedClusterFile = "../../examples/dedicated/aws/main.tf"
	gcpDedicatedClusterFile = "../../examples/dedicated/gcp/main.tf"
	dedicatedNamespaceFile  = "../../examples/namespace/main.tf"
	dedicatedNetworkFile    = "../../examples/network/main.tf"
)

var (
	runClusterTests = os.Getenv("RUN_CLUSTER_TESTS")
	accNamePrepend  = "tfrp-acc-"
	clientID        = os.Getenv("CLIENT_ID")
	clientSecret    = os.Getenv("CLIENT_SECRET")
)

func TestAccResourcesNamespace(t *testing.T) {
	ctx := context.Background()
	name := accNamePrepend + "testns"
	rename := accNamePrepend + "testns2"
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, providerCfgIDSecretVars)
	updateTestCaseVars["namespace_name"] = config.StringVariable(rename)

	c, err := newClients(ctx, clientID, clientSecret, "ign")
	if err != nil {
		t.Fatal(err)
	}
	var importID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", rename),
					func(s *terraform.State) error {
						i, err := utils.FindNamespaceByName(ctx, rename, c.NsClient)
						if err != nil {
							return err
						}
						importID = i.GetId()
						return nil
					}),
			},
			{
				ResourceName:             "redpanda_namespace.test",
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateId:            importID,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", rename),
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

	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(rename, &resource.Sweeper{
		Name: rename,
		F: sweepNamespace{
			NamespaceName: rename,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
}

func TestAccResourcesNetwork(t *testing.T) {
	ctx := context.Background()
	name := accNamePrepend + "testnet"
	rename := accNamePrepend + "testnet2"
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["network_name"] = config.StringVariable(rename)

	c, err := newClients(ctx, clientID, clientSecret, "ign")
	if err != nil {
		t.Fatal(err)
	}
	var importID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
					resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
					resource.TestCheckResourceAttr("redpanda_network.test", "name", rename),
					func(s *terraform.State) error {
						i, err := utils.FindNetworkByName(ctx, rename, c.NetClient)
						if err != nil {
							return err
						}
						importID = i.GetId()
						return nil
					}),
			},
			{
				ResourceName:             "redpanda_namespace.test",
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateId:            importID,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
					resource.TestCheckResourceAttr("redpanda_network.test", "name", rename)),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})

	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(rename, &resource.Sweeper{
		Name: rename,
		F: sweepNetwork{
			NetworkName: rename,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
}

func TestAccResourcesClusterAWS(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := accNamePrepend + "testaws"
	rename := accNamePrepend + "testaws2"
	var importID string

	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	c, err := newClients(ctx, clientID, clientSecret, "ign")
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(
		t,
		resource.TestCase{
			PreCheck: func() { testAccPreCheck(t) },
			Steps: []resource.TestStep{
				{
					ConfigFile:      config.StaticFile(awsDedicatedClusterFile),
					ConfigVariables: origTestCaseVars,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_cluster.test", "name", name),
					),
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				},
				{
					ConfigFile:               config.StaticFile(awsDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_cluster.test", "name", rename),
						func(s *terraform.State) error {
							i, err := utils.FindClusterByName(ctx, rename, c.ClusterClient)
							if err != nil {
								return err
							}
							importID = i.GetId()
							return nil
						}),
				},
				{
					ResourceName:             "redpanda_cluster.test",
					ConfigFile:               config.StaticFile(awsDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					ImportState:              true,
					ImportStateId:            importID,
					ImportStateVerify:        true,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_cluster.test", "name", rename),
					),
				},
				{
					ConfigFile:               config.StaticFile(awsDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					Destroy:                  true,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				},
			},
		},
	)
	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(rename, &resource.Sweeper{
		Name: rename,
		F: sweepCluster{
			ClusterName: rename,
			CluClient:   c.ClusterClient,
			OpsClient:   c.OpsClient,
		}.SweepCluster,
	})
}

func TestAccResourcesClusterGCP(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}

	ctx := context.Background()
	name := accNamePrepend + "testgcp"
	rename := accNamePrepend + "testgcp2"
	var importID string

	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	c, err := newClients(ctx, clientID, clientSecret, "ign")
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(
		t,
		resource.TestCase{
			PreCheck: func() { testAccPreCheck(t) },
			Steps: []resource.TestStep{
				{
					ConfigFile:      config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables: origTestCaseVars,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_cluster.test", "name", name),
					),
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				},
				{
					ConfigFile:               config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_cluster.test", "name", rename),
						func(s *terraform.State) error {
							i, err := utils.FindClusterByName(ctx, rename, c.ClusterClient)
							if err != nil {
								return err
							}
							importID = i.GetId()
							return nil
						}),
				},
				{
					ResourceName:             "redpanda_cluster.test",
					ConfigFile:               config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					ImportState:              true,
					ImportStateId:            importID,
					ImportStateVerify:        true,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("redpanda_namespace.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_network.test", "name", name),
						resource.TestCheckResourceAttr("redpanda_cluster.test", "name", rename),
					),
				},
				{
					ConfigFile:               config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					Destroy:                  true,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				},
			},
		},
	)

	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(name, &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(rename, &resource.Sweeper{
		Name: rename,
		F: sweepCluster{
			ClusterName: rename,
			CluClient:   c.ClusterClient,
			OpsClient:   c.OpsClient,
		}.SweepCluster,
	})
}
