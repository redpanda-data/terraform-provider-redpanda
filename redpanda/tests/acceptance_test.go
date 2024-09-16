package tests

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

const (
	awsDedicatedClusterFile   = "../../examples/cluster/aws/main.tf"
	azureDedicatedClusterFile = "../../examples/cluster/azure/main.tf"
	gcpDedicatedClusterFile   = "../../examples/cluster/gcp/main.tf"
	serverlessClusterFile     = "../../examples/cluster/serverless/main.tf"
	awsByocClusterFile        = "../../examples/byoc/aws/main.tf"
	azureByocClusterFile      = "../../examples/byoc/azure/main.tf"
	gcpByocClusterFile        = "../../examples/byoc/gcp/main.tf"
	dedicatedNetworkFile      = "../../examples/network/main.tf"
	dataSourcesTest           = "../../examples/datasource/standard/main.tf"
	bulkDataCreateFile        = "../../examples/datasource/bulk/main.tf"
	// These are the resource names as named in the TF files.
	resourceGroupName      = "redpanda_resource_group.test"
	networkResourceName    = "redpanda_network.test"
	clusterResourceName    = "redpanda_cluster.test"
	userResourceName       = "redpanda_user.test"
	topicResourceName      = "redpanda_topic.test"
	serverlessResourceName = "redpanda_serverless_cluster.test"
)

var (
	accNamePrepend             = "tfrp-acc-"
	runClusterTests            = os.Getenv("RUN_CLUSTER_TESTS")
	runByocTests               = os.Getenv("RUN_BYOC_TESTS")
	runServerlessTests         = os.Getenv("RUN_SERVERLESS_TESTS")
	runBulkTests               = os.Getenv("RUN_BULK_TESTS")
	clientID                   = os.Getenv(redpanda.ClientIDEnv)
	clientSecret               = os.Getenv(redpanda.ClientSecretEnv)
	testAgainstExistingCluster = os.Getenv("TEST_AGAINST_EXISTING_CLUSTER")
	redpandaVersion            = os.Getenv("REDPANDA_VERSION")
	testaws                    = "testaws"
	testawsRename              = "testaws-rename"
	testazure                  = "testazure"
)

func TestAccResourcesNetwork(t *testing.T) {
	ctx := context.Background()

	name := generateRandomName(accNamePrepend + "testnet")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)

	rename := generateRandomName(accNamePrepend + "testnet-rename")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["network_name"] = config.StringVariable(rename)

	var c *cloud.ControlPlaneClientSet
	var err error
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if c == nil {
				c, err = newTestClients(ctx, clientID, clientSecret, "pre")
				if err != nil {
					t.Fatal(err)
				}
			}
		},
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					func(_ *terraform.State) error {
						n, err := c.NetworkForName(ctx, name)
						if err != nil {
							return err
						}
						if n == nil {
							return fmt.Errorf("unable to find network %q after creation", name)
						}
						t.Logf("Successfully created network %v", name)
						return nil
					},
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", rename),
				),
			},
			{
				ResourceName:             networkResourceName,
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", rename),
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})
	resource.AddTestSweepers(generateRandomName("renamedNetworkSweeper"), &resource.Sweeper{
		Name: rename,
		F: sweepNetwork{
			NetworkName: rename,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}

func TestAccResourcesBulk(t *testing.T) {
	if !strings.Contains(runBulkTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()

	name := generateRandomName(accNamePrepend + "testbulk")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_id"] = config.StringVariable(os.Getenv("BULK_CLUSTER_ID"))

	c, err := newTestClients(ctx, clientID, clientSecret, "pre")
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(bulkDataCreateFile),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:               config.StaticFile(bulkDataCreateFile),
				ConfigVariables:          origTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	},
	)
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepCluster,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}

func TestAccResourcesClusterAWS(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testaws)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, "", awsDedicatedClusterFile, t)
}

func TestAccResourcesClusterAzure(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testazure)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, "", azureDedicatedClusterFile, t)
}

func TestAccResourcesClusterGCP(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "testgcp")
	rename := generateRandomName(accNamePrepend + "testgcp-rename")
	testRunner(ctx, name, rename, redpandaVersion, gcpDedicatedClusterFile, t)
}

func TestAccResourcesByocAWS(t *testing.T) {
	if !strings.Contains(runByocTests, "true") {
		t.Skip("skipping byoc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testaws)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, "", awsByocClusterFile, t)
}

func TestAccResourcesByocAzure(t *testing.T) {
	if !strings.Contains(runByocTests, "true") {
		t.Skip("skipping byoc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testazure)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, "", azureByocClusterFile, t)
}

func TestAccResourcesByocGCP(t *testing.T) {
	if !strings.Contains(runByocTests, "true") {
		t.Skip("skipping byoc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "testgcp")
	rename := generateRandomName(accNamePrepend + "testgcp-rename")
	testRunner(ctx, name, rename, redpandaVersion, gcpByocClusterFile, t)
}

// testRunner is a helper function that runs a series of tests on a given cluster in a given cloud provider.
func testRunner(ctx context.Context, name, rename, version, testFile string, t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	if version != "" {
		// version is only necessary to resolve a GCP install pack issue. we should generally use latest (nil)
		origTestCaseVars["version"] = config.StringVariable(version)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	c, err := newTestClients(ctx, clientID, clientSecret, "pre")
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile(testFile),
				ConfigVariables: origTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", name),
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(topicResourceName, "name", name),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ResourceName:    userResourceName,
				ConfigFile:      config.StaticFile(testFile),
				ConfigVariables: origTestCaseVars,
				ImportState:     true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					i, err := c.ClusterForName(ctx, name)
					if err != nil {
						return "", fmt.Errorf("test error: unable to get cluster by name")
					}
					importID := fmt.Sprintf("%v,%v", name, i.GetId())
					return importID, nil
				},
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					attr := state[0].Attributes
					id, user := attr["id"], attr["name"]
					if user != name {
						return fmt.Errorf("expected user %q; got %q", name, user)
					}
					if id != name {
						return fmt.Errorf("expected ID %q; got %q", name, id)
					}
					if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
						return errors.New("unexpected empty cloud URL")
					}
					if pw, ok := attr["password"]; ok {
						return fmt.Errorf("expected empty password; got %q", pw)
					}
					return nil
				},
				ImportStateVerifyIgnore:  []string{"tags", "allow_deletion"},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", name),
					resource.TestCheckResourceAttr(userResourceName, "name", name),
				),
			},
			{
				ConfigFile:               config.StaticFile(testFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
				),
			},
			{
				ResourceName:      clusterResourceName,
				ConfigFile:        config.StaticFile(testFile),
				ConfigVariables:   updateTestCaseVars,
				ImportState:       true,
				ImportStateVerify: true,
				//  These two only matter on apply; On apply the user will be
				//  getting Plan, not State, and have correct values for both.
				ImportStateVerifyIgnore:  []string{"tags", "allow_deletion"},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
				),
			},
			{
				ConfigFile:               config.StaticFile(testFile),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	},
	)
	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepCluster,
	})
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
		Name: rename,
		F: sweepCluster{
			ClusterName: rename,
			Client:      c,
		}.SweepCluster,
	})
}

func TestAccResourcesWithDataSources(t *testing.T) {
	if !strings.Contains(testAgainstExistingCluster, "true") {
		t.Skip("skipping cluster user-acl-topic tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "test-with-data-sources")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["cluster_id"] = config.StringVariable(os.Getenv("CLUSTER_ID"))
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	// Change 1, remove other
	updateTestCaseVars["topic_config"] = config.MapVariable(map[string]config.Variable{
		"compression.type": config.StringVariable("gzip"),
		"flush.ms":         config.StringVariable("100"),
	})

	c, err := newTestClients(ctx, clientID, clientSecret, "pre")
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:               config.StaticFile(dataSourcesTest),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(topicResourceName, "name", name),
				),
			},
			{
				ConfigFile:               config.StaticFile(dataSourcesTest),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(topicResourceName, "configuration.compression.type", "gzip"),
				),
			},
			{
				ConfigFile:               config.StaticFile(dataSourcesTest),
				ConfigVariables:          origTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	},
	)

	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepCluster,
	})
}

func TestAccResourcesStrippedDownServerlessCluster(t *testing.T) {
	if !strings.Contains(runServerlessTests, "true") {
		t.Skip("skipping serverless tests")
	}
	ctx := context.Background()

	name := generateRandomName(accNamePrepend + testaws)
	region := "int-eu-west-1"
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["region"] = config.StringVariable(region)

	rename := generateRandomName(accNamePrepend + testawsRename)
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)
	updateTestCaseVars["region"] = config.StringVariable(region)

	c, err := newTestClients(ctx, clientID, clientSecret, "pre")
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile(serverlessClusterFile),
				ConfigVariables: origTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(serverlessResourceName, "name", name),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:               config.StaticFile(serverlessClusterFile),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	},
	)
	resource.AddTestSweepers(generateRandomName("renameClusterSweeper"), &resource.Sweeper{
		Name: rename,
		F: sweepCluster{
			ClusterName: rename,
			Client:      c,
		}.SweepServerlessCluster,
	})
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepServerlessCluster,
	})
	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}
