package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

const (
	awsDedicatedClusterFile = "../../examples/cluster/aws/main.tf"
	gcpDedicatedClusterFile = "../../examples/cluster/gcp/main.tf"
	dedicatedNamespaceFile  = "../../examples/namespace/main.tf"
	dedicatedNetworkFile    = "../../examples/network/main.tf"

	// These are the resource names as named in the TF files.
	namespaceResourceName = "redpanda_namespace.test"
	networkResourceName   = "redpanda_network.test"
	clusterResourceName   = "redpanda_cluster.test"
)

var (
	runClusterTests = os.Getenv("RUN_CLUSTER_TESTS")
	accNamePrepend  = "tfrp-acc-"
	clientID        = os.Getenv(redpanda.ClientIDEnv)
	clientSecret    = os.Getenv(redpanda.ClientSecretEnv)
)

func TestAccResourcesNamespace(t *testing.T) {
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "testns")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)

	rename := generateRandomName(accNamePrepend + "testns-rename")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
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
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
				),
			},
			{
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(namespaceResourceName, "name", rename),
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
				ResourceName:             namespaceResourceName,
				ConfigFile:               config.StaticFile(dedicatedNamespaceFile),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateId:            importID,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(namespaceResourceName, "name", rename),
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

	resource.AddTestSweepers(generateRandomName("namespaceSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(generateRandomName("namespaceRenameSweeper"), &resource.Sweeper{
		Name: rename,
		F: sweepNamespace{
			NamespaceName: rename,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
}

func TestAccResourcesNetwork(t *testing.T) {
	ctx := context.Background()

	name := generateRandomName(accNamePrepend + "testnet")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)

	rename := generateRandomName(accNamePrepend + "testnet-rename")
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
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					func(s *terraform.State) error {
						n, err := utils.FindNetworkByName(ctx, name, c.NetClient)
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
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", rename),
					func(s *terraform.State) error {
						n, err := utils.FindNetworkByName(ctx, rename, c.NetClient)
						if err != nil {
							return err
						}
						if n == nil {
							return fmt.Errorf("unable to find network %q after updating", rename)
						}
						importID = n.GetId()
						t.Logf("Successfully created network %v, with ID: %v", rename, importID)
						return nil
					}),
			},
			{
				ResourceName:             networkResourceName,
				ConfigFile:               config.StaticFile(dedicatedNetworkFile),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateId:            importID,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
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
	resource.AddTestSweepers(generateRandomName("namespaceSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("renamedNetworkSweeper"), &resource.Sweeper{
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

	name := generateRandomName(accNamePrepend + "testaws")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)

	rename := generateRandomName(accNamePrepend + "testaws-rename")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	c, err := newClients(ctx, clientID, clientSecret, "ign")
	if err != nil {
		t.Fatal(err)
	}
	var importID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigFile:      config.StaticFile(awsDedicatedClusterFile),
				ConfigVariables: origTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", name),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigFile:               config.StaticFile(awsDedicatedClusterFile),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
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
				ResourceName:      clusterResourceName,
				ConfigFile:        config.StaticFile(awsDedicatedClusterFile),
				ConfigVariables:   updateTestCaseVars,
				ImportState:       true,
				ImportStateId:     importID,
				ImportStateVerify: true,
				//  These two only matter on apply; On apply the user will be
				//  getting Plan, not State, and have correct values for both.
				ImportStateVerifyIgnore:  []string{"tags", "allow_deletion"},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
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
	resource.AddTestSweepers(generateRandomName("namespaceSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
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

	name := generateRandomName(accNamePrepend + "testgcp")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["namespace_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)

	rename := generateRandomName(accNamePrepend + "testgcp-rename")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	c, err := newClients(ctx, clientID, clientSecret, "ign")
	if err != nil {
		t.Fatal(err)
	}
	var importID string
	resource.ParallelTest(
		t,
		resource.TestCase{
			PreCheck: func() { testAccPreCheck(t) },
			Steps: []resource.TestStep{
				{
					ConfigFile:      config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables: origTestCaseVars,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
						resource.TestCheckResourceAttr(networkResourceName, "name", name),
						resource.TestCheckResourceAttr(clusterResourceName, "name", name),
					),
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				},
				{
					ConfigFile:               config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
						resource.TestCheckResourceAttr(networkResourceName, "name", name),
						resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
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
					ResourceName:             clusterResourceName,
					ConfigFile:               config.StaticFile(gcpDedicatedClusterFile),
					ConfigVariables:          updateTestCaseVars,
					ImportState:              true,
					ImportStateId:            importID,
					ImportStateVerify:        true,
					ImportStateVerifyIgnore:  []string{"tags", "allow_deletion"},
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(namespaceResourceName, "name", name),
						resource.TestCheckResourceAttr(networkResourceName, "name", name),
						resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
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

	resource.AddTestSweepers(generateRandomName("namespaceSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNamespace{
			NamespaceName: name,
			Client:        c.NsClient,
		}.SweepNamespaces,
	})
	resource.AddTestSweepers(generateRandomName("networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			NetClient:   c.NetClient,
			OpsClient:   c.OpsClient,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("clusterSweeper"), &resource.Sweeper{
		Name: rename,
		F: sweepCluster{
			ClusterName: rename,
			CluClient:   c.ClusterClient,
			OpsClient:   c.OpsClient,
		}.SweepCluster,
	})
}

// generateRandomName generates a random name with a given prefix. The name will
// have the form of '<prefix>-<random>' where random is any 4 alphanumeric
// characters.
func generateRandomName(prefix string) string {
	baseChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	randomLength := 4 // Should be good, this is 62^4 = 14M combinations.

	var randStr bytes.Buffer
	r, _ := rand.Int(rand.Reader, big.NewInt(int64(len(baseChars))))
	for i := 0; i < randomLength; i++ {
		randStr.WriteByte(baseChars[r.Int64()])
	}

	return fmt.Sprintf("%v-%v", prefix, randStr.String())
}
