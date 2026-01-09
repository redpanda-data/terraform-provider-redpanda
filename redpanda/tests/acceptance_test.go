package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

const (
	awsDedicatedClusterDir             = "../../examples/cluster/aws"
	azureDedicatedClusterDir           = "../../examples/cluster/azure"
	gcpDedicatedClusterDir             = "../../examples/cluster/gcp"
	serverlessClusterDir               = "../../examples/cluster/serverless"
	serverlessPrivateLinkDir           = "../../examples/serverless_private_link/aws"
	awsByocClusterDir                  = "../../examples/byoc/aws"
	awsByocVpcClusterDir               = "infra/byovpc/aws"
	gcpByoVpcClusterDir                = "infra/byovpc/gcp"
	azureByocClusterDir                = "../../examples/byoc/azure"
	gcpByocClusterDir                  = "../../examples/byoc/gcp"
	dedicatedNetworkDir                = "../../examples/network"
	dataplaneDir                       = "../../examples/dataplane"
	dataSourcesTestDir                 = "../../examples/datasource/standard"
	bulkDataCreateDir                  = "../../examples/datasource/bulk"
	networkDataSourceDir               = "../../examples/datasource/network"
	serverlessRegionsDataSourceDir     = "../../examples/datasource/serverless_regions"
	resourceGroupName                  = "redpanda_resource_group.test"
	networkResourceName                = "redpanda_network.test"
	clusterResourceName                = "redpanda_cluster.test"
	userResourceName                   = "redpanda_user.test"
	testUserResourceName               = "redpanda_user.test_user"
	topicResourceName                  = "redpanda_topic.test"
	serverlessResourceName             = "redpanda_serverless_cluster.test"
	serverlessPrivateLinkResourceName  = "redpanda_serverless_private_link.example"
	networkDataSourceName              = "data.redpanda_network.test"
	serverlessRegionsAWSDataSourceName = "data.redpanda_serverless_regions.aws"
	serverlessRegionsGCPDataSourceName = "data.redpanda_serverless_regions.gcp"
	schemaResourceName                 = "redpanda_schema.user_schema"
	schemaEventResourceName            = "redpanda_schema.user_event_schema"
	schemaProductResourceName          = "redpanda_schema.product_schema"
	clusterAdminACLResourceName        = "redpanda_acl.cluster_admin"
	topicAccessACLResourceName         = "redpanda_acl.topic_access"
	schemaRegistryACLReadProductName   = "redpanda_schema_registry_acl.read_product"
	roleResourceName                   = "redpanda_role.developer"
	roleAssignmentResourceName         = "redpanda_role_assignment.developer_assignment"
	pipelineResourceName               = "redpanda_pipeline.test"
	allowDeletionFalseValue            = "false"
)

var (
	accNamePrepend             = "tfrp-acc-"
	runClusterTests            = os.Getenv("RUN_CLUSTER_TESTS")
	runByocTests               = os.Getenv("RUN_BYOC_TESTS")
	runByocVpcTests            = os.Getenv("RUN_BYOVPC_TESTS")
	runServerlessTests         = os.Getenv("RUN_SERVERLESS_TESTS")
	runBulkTests               = os.Getenv("RUN_BULK_TESTS")
	clientID                   = os.Getenv(redpanda.ClientIDEnv)
	clientSecret               = os.Getenv(redpanda.ClientSecretEnv)
	testAgainstExistingCluster = os.Getenv("TEST_AGAINST_EXISTING_CLUSTER")
	redpandaVersion            = os.Getenv("REDPANDA_VERSION")
	cloudEnv                   string
	throughputTier             string
	testaws                    = "testaws"
	testawsRename              = "testaws-rename"
	testazure                  = "testazure"
)

func init() {
	if v := os.Getenv("REDPANDA_CLOUD_ENVIRONMENT"); v != "" {
		cloudEnv = v
	} else {
		cloudEnv = "pre"
	}

	if cloudEnv == "ign" {
		if os.Getenv("GOOGLE_PROJECT") != "" {
			fmt.Println("cloud environment ign but provider gcp, setting throughput tier to nothing")
			throughputTier = ""
		} else {
			fmt.Println("cloud environment ign, setting throughput tier to test")
			throughputTier = "test"
		}
	} else if v := os.Getenv("THROUGHPUT_TIER"); v != "" {
		fmt.Println("setting throughput tier to", v)
		throughputTier = v
	}
}

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
				c, err = newTestClients(ctx, clientID, clientSecret, cloudEnv)
				if err != nil {
					t.Fatal(err)
				}
			}
		},
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(dedicatedNetworkDir),
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
				ConfigDirectory:          config.StaticDirectory(dedicatedNetworkDir),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", rename),
				),
			},
			{
				ResourceName:             networkResourceName,
				ConfigDirectory:          config.StaticDirectory(dedicatedNetworkDir),
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
				ConfigDirectory:          config.StaticDirectory(dedicatedNetworkDir),
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
	if throughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(bulkDataCreateDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigDirectory:          config.StaticDirectory(bulkDataCreateDir),
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
	testRunner(ctx, name, rename, redpandaVersion, awsDedicatedClusterDir, nil, t)
}

func TestAccResourcesClusterAzure(t *testing.T) {
	t.Skip("skipping azure tests - not currently working")
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testazure)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, redpandaVersion, azureDedicatedClusterDir, nil, t)
}

func TestAccResourcesClusterGCP(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "testgcp")
	rename := generateRandomName(accNamePrepend + "testgcp-rename")
	testRunner(ctx, name, rename, redpandaVersion, gcpDedicatedClusterDir, nil, t)
}

func TestAccResourcesByocAWS(t *testing.T) {
	if !strings.Contains(runByocTests, "true") {
		t.Skip("skipping byoc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testaws)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, redpandaVersion, awsByocClusterDir, nil, t)
}

func TestAccResourcesByocAzure(t *testing.T) {
	t.Skip("skipping azure byoc tests - not currently working")
	if !strings.Contains(runByocTests, "true") {
		t.Skip("skipping byoc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testazure)
	rename := generateRandomName(accNamePrepend + testawsRename)
	testRunner(ctx, name, rename, redpandaVersion, azureByocClusterDir, nil, t)
}

func TestAccResourcesByocGCP(t *testing.T) {
	if !strings.Contains(runByocTests, "true") {
		t.Skip("skipping byoc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "testgcp")
	rename := generateRandomName(accNamePrepend + "testgcp-rename")
	testRunner(ctx, name, rename, redpandaVersion, gcpByocClusterDir, nil, t)
}

func TestAccResourcesByoVpcAWS(t *testing.T) {
	if !strings.Contains(runByocVpcTests, "true") {
		t.Skip("skipping byoc vpc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + testaws)
	rename := generateRandomName(accNamePrepend + testawsRename)

	var privateSubnetArns []string
	privateSubnetArnsEnv := os.Getenv("RP_PRIVATE_SUBNET_ARNS")
	if err := json.Unmarshal([]byte(privateSubnetArnsEnv), &privateSubnetArns); err != nil {
		t.Fatalf("Error parsing private subnet ARNs: %v", err)
	}
	var zones []string
	zonesEnv := os.Getenv("AWS_ZONES")
	if err := json.Unmarshal([]byte(zonesEnv), &zones); err != nil {
		t.Fatalf("Error parsing zones: %v", err)
	}
	customVars := map[string]config.Variable{
		"cloud_provider":                  config.StringVariable("aws"),
		"region":                          config.StringVariable(os.Getenv("AWS_REGION")),
		"aws_secret_key":                  config.StringVariable(os.Getenv("AWS_SECRET_ACCESS_KEY")),
		"aws_access_key":                  config.StringVariable(os.Getenv("AWS_ACCESS_KEY_ID")),
		"management_bucket_arn":           config.StringVariable(os.Getenv("RP_MANAGEMENT_BUCKET_ARN")),
		"dynamodb_table_arn":              config.StringVariable(os.Getenv("RP_DYNAMODB_TABLE_ARN")),
		"vpc_arn":                         config.StringVariable(os.Getenv("RP_VPC_ARN")),
		"permissions_boundary_policy_arn": config.StringVariable(os.Getenv("RP_PERMISSIONS_BOUNDARY_POLICY_ARN")),
		"agent_instance_profile_arn":      config.StringVariable(os.Getenv("RP_AGENT_INSTANCE_PROFILE_ARN")),
		"connectors_node_group_instance_profile_arn": config.StringVariable(os.Getenv("RP_CONNECTORS_NODE_GROUP_INSTANCE_PROFILE_ARN")),
		"utility_node_group_instance_profile_arn":    config.StringVariable(os.Getenv("RP_UTILITY_NODE_GROUP_INSTANCE_PROFILE_ARN")),
		"redpanda_node_group_instance_profile_arn":   config.StringVariable(os.Getenv("RP_REDPANDA_NODE_GROUP_INSTANCE_PROFILE_ARN")),
		"k8s_cluster_role_arn":                       config.StringVariable(os.Getenv("RP_K8S_CLUSTER_ROLE_ARN")),
		"redpanda_agent_security_group_arn":          config.StringVariable(os.Getenv("RP_REDPANDA_AGENT_SECURITY_GROUP_ARN")),
		"connectors_security_group_arn":              config.StringVariable(os.Getenv("RP_CONNECTORS_SECURITY_GROUP_ARN")),
		"redpanda_node_group_security_group_arn":     config.StringVariable(os.Getenv("RP_REDPANDA_NODE_GROUP_SECURITY_GROUP_ARN")),
		"utility_security_group_arn":                 config.StringVariable(os.Getenv("RP_UTILITY_SECURITY_GROUP_ARN")),
		"cluster_security_group_arn":                 config.StringVariable(os.Getenv("RP_CLUSTER_SECURITY_GROUP_ARN")),
		"node_security_group_arn":                    config.StringVariable(os.Getenv("RP_NODE_SECURITY_GROUP_ARN")),
		"cloud_storage_bucket_arn":                   config.StringVariable(os.Getenv("RP_CLOUD_STORAGE_BUCKET_ARN")),
	}

	if len(privateSubnetArns) > 0 {
		subnetVars := make([]config.Variable, len(privateSubnetArns))
		for i, arn := range privateSubnetArns {
			subnetVars[i] = config.StringVariable(arn)
		}
		customVars["private_subnet_arns"] = config.ListVariable(subnetVars...)
	}

	if len(zones) > 0 {
		zonesVars := make([]config.Variable, len(zones))
		for i, zone := range zones {
			zonesVars[i] = config.StringVariable(zone)
		}
		customVars["zones"] = config.ListVariable(zonesVars...)
	}

	testRunnerCluster(ctx, name, rename, redpandaVersion, awsByocVpcClusterDir, customVars, t)
}

func TestAccResourcesByoVpcGCP(t *testing.T) {
	if !strings.Contains(runByocVpcTests, "true") {
		t.Skip("skipping byoc vpc tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "testgcp")
	rename := generateRandomName(accNamePrepend + "testgcp-rename")

	customVars := map[string]config.Variable{
		"region":                                 config.StringVariable(os.Getenv("GCP_REGION")),
		"network_project_id":                     config.StringVariable(os.Getenv("GCP_PROJECT_ID")),
		"vpc_network_name":                       config.StringVariable(os.Getenv("GCP_VPC_NETWORK_NAME")),
		"management_bucket_name":                 config.StringVariable(os.Getenv("GCP_MANAGEMENT_BUCKET_NAME")),
		"subnet_name":                            config.StringVariable(os.Getenv("GCP_SUBNET_NAME")),
		"secondary_ipv4_range_pods_name":         config.StringVariable(os.Getenv("GCP_SECONDARY_IPV4_RANGE_PODS_NAME")),
		"secondary_ipv4_range_services_name":     config.StringVariable(os.Getenv("GCP_SECONDARY_IPV4_RANGE_SERVICES_NAME")),
		"k8s_master_ipv4_range":                  config.StringVariable(os.Getenv("GCP_K8S_MASTER_IPV4_RANGE")),
		"agent_service_account_email":            config.StringVariable(os.Getenv("GCP_AGENT_SERVICE_ACCOUNT_EMAIL")),
		"console_service_account_email":          config.StringVariable(os.Getenv("GCP_CONSOLE_SERVICE_ACCOUNT_EMAIL")),
		"connector_service_account_email":        config.StringVariable(os.Getenv("GCP_CONNECTOR_SERVICE_ACCOUNT_EMAIL")),
		"redpanda_cluster_service_account_email": config.StringVariable(os.Getenv("GCP_REDPANDA_CLUSTER_SERVICE_ACCOUNT_EMAIL")),
		"gke_service_account_email":              config.StringVariable(os.Getenv("GCP_GKE_SERVICE_ACCOUNT_EMAIL")),
		"tiered_storage_bucket_name":             config.StringVariable(os.Getenv("GCP_TIERED_STORAGE_BUCKET_NAME")),
	}

	testRunnerCluster(ctx, name, rename, redpandaVersion, gcpByoVpcClusterDir, customVars, t)
}

// buildTestCheckFuncs reads the test file and returns appropriate check functions based on resources present
func buildTestCheckFuncs(testDir, name string, hasPrivateNetworking ...bool) ([]resource.TestCheckFunc, error) {
	testFileContent, err := os.ReadFile(testDir + "/main.tf") // #nosec G304 -- testDir is controlled by test constants
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}
	testFileStr := string(testFileContent)

	var checkFuncs []resource.TestCheckFunc

	if strings.Contains(testFileStr, `resource "redpanda_resource_group" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(resourceGroupName, "name", name))
	}

	if strings.Contains(testFileStr, `resource "redpanda_network" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(networkResourceName, "name", name))
	}

	if strings.Contains(testFileStr, `resource "redpanda_cluster" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(clusterResourceName, "name", name))
	}

	if strings.Contains(testFileStr, `resource "redpanda_serverless_cluster" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(serverlessResourceName, "name", name),
			resource.TestCheckResourceAttrSet(serverlessResourceName, "id"),
			resource.TestCheckResourceAttrSet(serverlessResourceName, "cluster_api_url"),
			// Kafka API
			resource.TestCheckResourceAttrSet(serverlessResourceName, "kafka_api.seed_brokers.#"),
			resource.TestCheckResourceAttrSet(serverlessResourceName, "kafka_api.seed_brokers.0"),
			// Schema Registry
			resource.TestCheckResourceAttrSet(serverlessResourceName, "schema_registry.url"),
			// Dataplane API
			resource.TestCheckResourceAttrSet(serverlessResourceName, "dataplane_api.url"),
			// Console URL
			resource.TestCheckResourceAttrSet(serverlessResourceName, "console_url"),
			// Prometheus
			resource.TestCheckResourceAttrSet(serverlessResourceName, "prometheus.url"),
		)

		// Check for private networking fields if private networking is enabled
		// Use explicit parameter if provided, otherwise fall back to string detection
		privateNetworkingEnabled := false
		if len(hasPrivateNetworking) > 0 {
			privateNetworkingEnabled = hasPrivateNetworking[0]
		}

		if privateNetworkingEnabled {
			checkFuncs = append(checkFuncs,
				// Kafka API private
				resource.TestCheckResourceAttrSet(serverlessResourceName, "kafka_api.private_seed_brokers.#"),
				resource.TestCheckResourceAttrSet(serverlessResourceName, "kafka_api.private_seed_brokers.0"),
				// Schema Registry private
				resource.TestCheckResourceAttrSet(serverlessResourceName, "schema_registry.private_url"),
				// Dataplane API private
				resource.TestCheckResourceAttrSet(serverlessResourceName, "dataplane_api.private_url"),
				// Console private URL
				resource.TestCheckResourceAttrSet(serverlessResourceName, "console_private_url"),
				// Prometheus private
				resource.TestCheckResourceAttrSet(serverlessResourceName, "prometheus.private_url"),
			)
		}
	}

	if strings.Contains(testFileStr, `resource "redpanda_user" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(userResourceName, "name", name))
	}

	if strings.Contains(testFileStr, `resource "redpanda_user" "test_user"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(testUserResourceName, "name", name+"-test"))
	}

	if strings.Contains(testFileStr, `resource "redpanda_topic" "test"`) {
		checkFuncs = append(checkFuncs, resource.TestCheckResourceAttr(topicResourceName, "name", name))
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema" "user_schema"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(schemaResourceName, "subject", name+"-value"),
			resource.TestCheckResourceAttr(schemaResourceName, "schema_type", "AVRO"),
			resource.TestCheckResourceAttr(schemaResourceName, "compatibility", "BACKWARD"), // Default compatibility
			resource.TestCheckResourceAttrSet(schemaResourceName, "id"),
			resource.TestCheckResourceAttrSet(schemaResourceName, "version"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema" "user_event_schema"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(schemaEventResourceName, "subject", name+"-events-value"),
			resource.TestCheckResourceAttr(schemaEventResourceName, "references.#", "1"),
			resource.TestCheckResourceAttr(schemaEventResourceName, "references.0.name", "User"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema" "product_schema"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(schemaProductResourceName, "subject", name+"-product-value"),
			resource.TestCheckResourceAttr(schemaProductResourceName, "compatibility", "FULL"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "read_product"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.read_product", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "resource_name", "product-"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "pattern_type", "PREFIXED"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "operation", "READ"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.read_product", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "write_orders"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.write_orders", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "resource_name", "orders-value"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "operation", "WRITE"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.write_orders", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "all_test_topic"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.all_test_topic", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "resource_name", name+"-"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "pattern_type", "PREFIXED"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "operation", "ALL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.all_test_topic", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "describe_test_topic"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.describe_test_topic", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "resource_type", "SUBJECT"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "resource_name", name+"-"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "pattern_type", "PREFIXED"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "operation", "DESCRIBE"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_test_topic", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "describe_registry"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.describe_registry", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "resource_type", "REGISTRY"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "resource_name", "*"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "operation", "DESCRIBE"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.describe_registry", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_schema_registry_acl" "alter_configs_registry"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet("redpanda_schema_registry_acl.alter_configs_registry", "id"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "principal", "User:"+name),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "resource_type", "REGISTRY"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "resource_name", "*"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "pattern_type", "LITERAL"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "operation", "ALTER_CONFIGS"),
			resource.TestCheckResourceAttr("redpanda_schema_registry_acl.alter_configs_registry", "permission", "ALLOW"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_role" "developer"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(roleResourceName, "id"),
			resource.TestCheckResourceAttr(roleResourceName, "name", "developer"),
			resource.TestCheckResourceAttrSet(roleResourceName, "cluster_api_url"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_role_assignment" "developer_assignment"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(roleAssignmentResourceName, "id"),
			resource.TestCheckResourceAttr(roleAssignmentResourceName, "role_name", "developer"),
			resource.TestCheckResourceAttr(roleAssignmentResourceName, "principal", name),
			resource.TestCheckResourceAttrSet(roleAssignmentResourceName, "cluster_api_url"),
		)
	}

	if strings.Contains(testFileStr, `resource "redpanda_pipeline" "test"`) {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttrSet(pipelineResourceName, "id"),
			resource.TestCheckResourceAttr(pipelineResourceName, "display_name", "test-pipeline"),
			resource.TestCheckResourceAttrSet(pipelineResourceName, "state"),
			resource.TestCheckResourceAttrSet(pipelineResourceName, "cluster_api_url"),
		)
	}

	return checkFuncs, nil
}

// testRunner is a helper function that runs a series of tests on a given cluster in a given cloud provider.
func testRunner(ctx context.Context, name, rename, version, testFile string, customVars map[string]config.Variable, t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	if throughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	if len(customVars) > 0 {
		for k, v := range customVars {
			origTestCaseVars[k] = v
		}
	}
	if version != "" {
		origTestCaseVars["version"] = config.StringVariable(version)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)
	updateTestCaseVars["cluster_allow_deletion"] = config.BoolVariable(true)
	updateTestCaseVars["user_allow_deletion"] = config.BoolVariable(true)
	updateTestCaseVars["acl_allow_deletion"] = config.BoolVariable(true)

	compatibilityUpdateVars := make(map[string]config.Variable)
	maps.Copy(compatibilityUpdateVars, updateTestCaseVars)
	compatibilityUpdateVars["compatibility_level"] = config.StringVariable("FORWARD")

	userAllowDeletionFalseVars := make(map[string]config.Variable)
	maps.Copy(userAllowDeletionFalseVars, updateTestCaseVars)
	userAllowDeletionFalseVars["user_allow_deletion"] = config.BoolVariable(false)

	userAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(userAllowDeletionTrueVars, updateTestCaseVars)
	userAllowDeletionTrueVars["user_allow_deletion"] = config.BoolVariable(true)

	aclAllowDeletionFalseVars := make(map[string]config.Variable)
	maps.Copy(aclAllowDeletionFalseVars, updateTestCaseVars)
	aclAllowDeletionFalseVars["acl_allow_deletion"] = config.BoolVariable(false)

	aclAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(aclAllowDeletionTrueVars, updateTestCaseVars)
	aclAllowDeletionTrueVars["acl_allow_deletion"] = config.BoolVariable(true)

	srACLAllowDeletionFalseVars := make(map[string]config.Variable)
	maps.Copy(srACLAllowDeletionFalseVars, updateTestCaseVars)
	srACLAllowDeletionFalseVars["sr_acl_allow_deletion"] = config.BoolVariable(false)

	srACLAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(srACLAllowDeletionTrueVars, updateTestCaseVars)
	srACLAllowDeletionTrueVars["sr_acl_allow_deletion"] = config.BoolVariable(true)

	pipelineRunningVars := make(map[string]config.Variable)
	maps.Copy(pipelineRunningVars, updateTestCaseVars)
	pipelineRunningVars["pipeline_state"] = config.StringVariable("running")

	pipelineAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(pipelineAllowDeletionTrueVars, pipelineRunningVars)
	pipelineAllowDeletionTrueVars["pipeline_allow_deletion"] = config.BoolVariable(true)

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	checkFuncs, err := buildTestCheckFuncs(testFile, name)
	if err != nil {
		t.Fatal(err)
	}

	testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read test file: %w", err))
	}
	hasSchemaRegistryACL := strings.Contains(string(testFileContent), `resource "redpanda_schema_registry_acl" "read_product"`)
	hasSchema := strings.Contains(string(testFileContent), `resource "redpanda_schema" "user_schema"`)
	hasRole := strings.Contains(string(testFileContent), `resource "redpanda_role" "developer"`)
	hasTopic := strings.Contains(string(testFileContent), `resource "redpanda_topic" "test"`)
	hasPipeline := strings.Contains(string(testFileContent), `resource "redpanda_pipeline" "test"`)

	steps := []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          origTestCaseVars,
			Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		},
		{
			ResourceName:    userResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: origTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				i, err := c.ClusterForName(ctx, name)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
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
				if allowDeletion := attr["allow_deletion"]; allowDeletion != allowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		},
		{
			ResourceName:    userResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: origTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				i, err := c.ClusterForName(ctx, name)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("%v,%v,test-password,SCRAM-SHA-256", name, i.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != name {
					return fmt.Errorf("expected user name %q; got %q", name, attr["name"])
				}
				if attr["id"] != name {
					return fmt.Errorf("expected ID %q; got %q", name, attr["id"])
				}
				if attr["password"] != "test-password" {
					return fmt.Errorf("expected password 'test-password'; got %q", attr["password"])
				}
				if attr["mechanism"] != "SCRAM-SHA-256" {
					return fmt.Errorf("expected mechanism 'SCRAM-SHA-256'; got %q", attr["mechanism"])
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("unexpected empty cloud URL")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != allowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		},
		{
			ResourceName:             clusterResourceName,
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          origTestCaseVars,
			ImportState:              true,
			ImportStateVerify:        true,
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          updateTestCaseVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceGroupName, "name", name),
				resource.TestCheckResourceAttr(networkResourceName, "name", name),
				resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          userAllowDeletionFalseVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(userResourceName, "allow_deletion", "false"),
				resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          userAllowDeletionTrueVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(userResourceName, "allow_deletion", "true"),
				resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          aclAllowDeletionFalseVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					aclResourceName := clusterAdminACLResourceName
					if strings.Contains(string(testFileContent), `resource "redpanda_acl" "topic_access"`) {
						aclResourceName = topicAccessACLResourceName
					}
					return resource.TestCheckResourceAttr(aclResourceName, "allow_deletion", "false")
				}(),
				resource.TestCheckResourceAttr(userResourceName, "allow_deletion", "true"),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          aclAllowDeletionTrueVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					aclResourceName := clusterAdminACLResourceName
					if strings.Contains(string(testFileContent), `resource "redpanda_acl" "topic_access"`) {
						aclResourceName = topicAccessACLResourceName
					}
					return resource.TestCheckResourceAttr(aclResourceName, "allow_deletion", "true")
				}(),
				resource.TestCheckResourceAttr(userResourceName, "allow_deletion", "true"),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          srACLAllowDeletionFalseVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema_registry_acl" "read_product"`) {
						return resource.TestCheckResourceAttr(schemaRegistryACLReadProductName, "allow_deletion", "false")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          srACLAllowDeletionTrueVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema_registry_acl" "read_product"`) {
						return resource.TestCheckResourceAttr(schemaRegistryACLReadProductName, "allow_deletion", "true")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          compatibilityUpdateVars,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceGroupName, "name", name),
				resource.TestCheckResourceAttr(networkResourceName, "name", name),
				resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema" "product_schema"`) {
						return resource.TestCheckResourceAttr(schemaProductResourceName, "compatibility", "FORWARD")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
	}

	if hasSchemaRegistryACL {
		steps = append(steps, resource.TestStep{
			ResourceName:    schemaRegistryACLReadProductName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[schemaRegistryACLReadProductName]
				if !ok {
					return "", errors.New("schema registry ACL resource not found in state")
				}

				clusterID := rs.Primary.Attributes["cluster_id"]
				principal := rs.Primary.Attributes["principal"]
				resourceType := rs.Primary.Attributes["resource_type"]
				resourceName := rs.Primary.Attributes["resource_name"]
				patternType := rs.Primary.Attributes["pattern_type"]
				host := rs.Primary.Attributes["host"]
				operation := rs.Primary.Attributes["operation"]
				permission := rs.Primary.Attributes["permission"]
				username := rs.Primary.Attributes["username"]
				password := rs.Primary.Attributes["password"]

				importID := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s:%s:%s",
					clusterID, principal, resourceType, resourceName, patternType, host, operation, permission, username, password)
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["cluster_id"] == "" {
					return errors.New("expected non-empty cluster_id")
				}
				if attr["principal"] == "" {
					return errors.New("expected non-empty principal")
				}
				if attr["resource_type"] != "SUBJECT" {
					return fmt.Errorf("expected resource_type SUBJECT; got %q", attr["resource_type"])
				}
				if attr["resource_name"] != "product-" {
					return fmt.Errorf("expected resource_name 'product-'; got %q", attr["resource_name"])
				}
				if attr["pattern_type"] != "PREFIXED" {
					return fmt.Errorf("expected pattern_type PREFIXED; got %q", attr["pattern_type"])
				}
				if attr["host"] == "" {
					return errors.New("expected non-empty host")
				}
				if attr["operation"] != "READ" {
					return fmt.Errorf("expected operation READ; got %q", attr["operation"])
				}
				if attr["permission"] != "ALLOW" {
					return fmt.Errorf("expected permission ALLOW; got %q", attr["permission"])
				}
				if attr["username"] == "" {
					return errors.New("expected non-empty username")
				}
				if attr["password"] == "" {
					return errors.New("expected non-empty password")
				}
				if attr["id"] == "" {
					return errors.New("expected non-empty id")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != allowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		})
	}

	if hasSchema {
		steps = append(steps, resource.TestStep{
			ResourceName:    schemaResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[schemaResourceName]
				if !ok {
					return "", errors.New("schema resource not found in state")
				}

				clusterID := rs.Primary.Attributes["cluster_id"]
				subject := rs.Primary.Attributes["subject"]
				version := rs.Primary.Attributes["version"]
				username := rs.Primary.Attributes["username"]
				password := rs.Primary.Attributes["password"]

				importID := fmt.Sprintf("%s:%s:%s:%s:%s",
					clusterID, subject, version, username, password)
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["subject"] != name+"-value" {
					return fmt.Errorf("expected subject %q; got %q", name+"-value", attr["subject"])
				}
				if attr["schema_type"] != "AVRO" {
					return fmt.Errorf("expected schema_type AVRO; got %q", attr["schema_type"])
				}
				if attr["version"] == "" {
					return errors.New("expected non-empty version")
				}
				if attr["id"] == "" {
					return errors.New("expected non-empty id")
				}
				if attr["cluster_id"] == "" {
					return errors.New("expected non-empty cluster_id")
				}
				if attr["username"] == "" {
					return errors.New("expected non-empty username")
				}
				if attr["password"] == "" {
					return errors.New("expected non-empty password")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != allowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		})
	}

	if hasRole {
		steps = append(steps, resource.TestStep{
			ResourceName:    roleResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				cluster, err := c.ClusterForName(ctx, rename)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("developer,%v", cluster.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != "developer" {
					return fmt.Errorf("expected role name 'developer'; got %q", attr["name"])
				}
				if attr["id"] != "developer" {
					return fmt.Errorf("expected ID 'developer'; got %q", attr["id"])
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("expected cluster_api_url to be set after import")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != allowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		})
	}

	if hasTopic {
		steps = append(steps, resource.TestStep{
			ResourceName:    topicResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				cluster, err := c.ClusterForName(ctx, rename)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("%s,%v", name, cluster.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != name {
					return fmt.Errorf("expected topic name %q; got %q", name, attr["name"])
				}
				if attr["id"] != name {
					return fmt.Errorf("expected ID %q; got %q", name, attr["id"])
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("expected cluster_api_url to be set after import")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != allowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		})
	}

	if hasPipeline {
		steps = append(steps,
			resource.TestStep{
				ResourceName:    pipelineResourceName,
				ConfigDirectory: config.StaticDirectory(testFile),
				ConfigVariables: updateTestCaseVars,
				ImportState:     true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					rs, ok := state.RootModule().Resources[pipelineResourceName]
					if !ok {
						return "", errors.New("pipeline resource not found in state")
					}
					pipelineID := rs.Primary.Attributes["id"]
					cluster, err := c.ClusterForName(ctx, rename)
					if err != nil {
						return "", errors.New("test error: unable to get cluster by name")
					}
					importID := fmt.Sprintf("%s,%v", pipelineID, cluster.GetId())
					return importID, nil
				},
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					attr := state[0].Attributes
					if attr["id"] == "" {
						return errors.New("expected non-empty id")
					}
					if attr["display_name"] != "test-pipeline" {
						return fmt.Errorf("expected display_name 'test-pipeline'; got %q", attr["display_name"])
					}
					if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
						return errors.New("expected cluster_api_url to be set after import")
					}
					if attr["state"] == "" {
						return errors.New("expected non-empty state")
					}
					return nil
				},
				ImportStateVerifyIgnore:  []string{"config_yaml", "description", "resources", "tags"},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			resource.TestStep{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          pipelineRunningVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(pipelineResourceName, "state", "running"),
				),
			},
			// Enable deletion for cleanup - pipeline defaults to allow_deletion=false
			resource.TestStep{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          pipelineAllowDeletionTrueVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(pipelineResourceName, "allow_deletion", "true"),
				),
			},
		)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps:    steps,
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

// testRunnerCluster is a helper function that runs a series of tests on a given cluster in a given cloud provider. Does not test for user or topic
func testRunnerCluster(ctx context.Context, name, rename, version, testFile string, customVars map[string]config.Variable, t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	if throughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	if len(customVars) > 0 {
		for k, v := range customVars {
			origTestCaseVars[k] = v
		}
	}
	if version != "" {
		origTestCaseVars["version"] = config.StringVariable(version)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)
	updateTestCaseVars["cluster_allow_deletion"] = config.BoolVariable(true)

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory(testFile),
				ConfigVariables: origTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", name),
				),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ResourceName:             clusterResourceName,
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          origTestCaseVars,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"tags"},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigDirectory: config.StaticDirectory(testFile),
				ConfigVariables: updateTestCaseVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceGroupName, "name", name),
					resource.TestCheckResourceAttr(networkResourceName, "name", name),
					resource.TestCheckResourceAttr(clusterResourceName, "name", rename),
				),
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

func TestAccDataSourceNetwork(t *testing.T) {
	if !strings.Contains(testAgainstExistingCluster, "true") {
		t.Skip("skipping network datasource test")
	}

	networkIDEnv := os.Getenv("REDPANDA_NETWORK_ID")
	if networkIDEnv == "" {
		t.Skip("skipping network data source test: REDPANDA_NETWORK_ID not set")
	}

	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["network_id"] = config.StringVariable(networkIDEnv)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(networkDataSourceDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(networkDataSourceName, "id"),
					resource.TestCheckResourceAttrSet(networkDataSourceName, "name"),
					resource.TestCheckResourceAttrSet(networkDataSourceName, "cloud_provider"),
					resource.TestCheckResourceAttrSet(networkDataSourceName, "region"),
					resource.TestCheckResourceAttrSet(networkDataSourceName, "resource_group_id"),
					resource.TestCheckResourceAttrSet(networkDataSourceName, "cluster_type"),
					resource.TestCheckResourceAttr(networkDataSourceName, "id", networkIDEnv),
				),
			},
		},
	})
}

func TestAccResourcesDataSource(t *testing.T) {
	if !strings.Contains(testAgainstExistingCluster, "true") {
		t.Skip("skipping cluster user-acl-topic tests")
	}
	name := generateRandomName(accNamePrepend + "datasource")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["cluster_id"] = config.StringVariable(os.Getenv("CLUSTER_ID"))
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	if throughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["topic_config"] = config.MapVariable(map[string]config.Variable{
		"compression.type": config.StringVariable("gzip"),
		"flush.ms":         config.StringVariable("100"),
	})

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(dataSourcesTestDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(topicResourceName, "name", name),
				),
			},
			{
				ConfigDirectory:          config.StaticDirectory(dataSourcesTestDir),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(topicResourceName, "configuration.compression.type", "gzip"),
				),
			},
			{
				ConfigDirectory:          config.StaticDirectory(dataSourcesTestDir),
				ConfigVariables:          origTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	},
	)
}

// runServerlessClusterVariantTest is a helper that runs a serverless cluster test with specific configuration
func runServerlessClusterVariantTest(t *testing.T, testSuffix, region string, publicNetworking, privateNetworking bool) {
	if !strings.Contains(runServerlessTests, "true") {
		t.Skip("skipping serverless tests")
	}
	ctx := context.Background()

	name := generateRandomName(accNamePrepend + testSuffix)
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["region"] = config.StringVariable(region)

	// Convert booleans to state strings
	publicState := "STATE_DISABLED"
	if publicNetworking {
		publicState = "STATE_ENABLED"
	}
	privateState := "STATE_DISABLED"
	if privateNetworking {
		privateState = "STATE_ENABLED"
	}

	// Add networking config if not using defaults (public enabled, private disabled)
	if !publicNetworking || privateNetworking {
		origTestCaseVars["public_networking"] = config.StringVariable(publicState)
		origTestCaseVars["private_networking"] = config.StringVariable(privateState)
	}

	// Add private link config if private networking is enabled
	if privateNetworking {
		origTestCaseVars["allowed_principals"] = config.ListVariable(
			config.StringVariable("arn:aws:iam::123456789012:root"),
		)
		origTestCaseVars["allow_private_link_deletion"] = config.BoolVariable(true)
	}

	rename := generateRandomName(accNamePrepend + testSuffix + "-rename")
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)

	checkFuncs, err := buildTestCheckFuncs(serverlessClusterDir, name, privateNetworking)
	if err != nil {
		t.Fatal(err)
	}

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(serverlessClusterDir),
				ConfigVariables:          origTestCaseVars,
				Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigDirectory:          config.StaticDirectory(serverlessClusterDir),
				ConfigVariables:          updateTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})

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
	if privateNetworking {
		resource.AddTestSweepers(generateRandomName("privateLinkSweeper"), &resource.Sweeper{
			Name: name,
			F: sweepServerlessPrivateLink{
				PrivateLinkName: name + "-private-link",
				Client:          c,
			}.SweepServerlessPrivateLink,
		})
	}
	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}

func TestAccResourcesServerlessClusterAWSPublic(t *testing.T) {
	runServerlessClusterVariantTest(t, testaws, "eu-west-1", true, false)
}

func TestAccResourcesServerlessClusterGCP(t *testing.T) {
	runServerlessClusterVariantTest(t, "testgcp", "us-central1", true, false)
}

func TestAccResourcesServerlessClusterAWSPrivate(t *testing.T) {
	runServerlessClusterVariantTest(t, "testprivate", "eu-west-1", false, true)
}

func TestAccResourcesServerlessClusterAWSBoth(t *testing.T) {
	runServerlessClusterVariantTest(t, "testboth", "eu-west-1", true, true)
}

func TestAccServerlessPrivateLink(t *testing.T) {
	if !strings.Contains(runServerlessTests, "true") {
		t.Skip("skipping serverless tests")
	}
	ctx := context.Background()

	name := generateRandomName(accNamePrepend + "spl-test")
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["private_link_name"] = config.StringVariable(name)
	origTestCaseVars["serverless_region"] = config.StringVariable("eu-west-1")
	origTestCaseVars["allow_deletion"] = config.BoolVariable(true)
	// Use a dummy AWS principal ARN for testing
	origTestCaseVars["allowed_principals"] = config.ListVariable(
		config.StringVariable("arn:aws:iam::123456789012:root"),
	)
	origTestCaseVars["allow_deletion"] = config.BoolVariable(true)

	// Update test case with different allowed_principals
	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["allowed_principals"] = config.ListVariable(
		config.StringVariable("arn:aws:iam::123456789012:root"),
		config.StringVariable("arn:aws:iam::987654321098:root"),
	)

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(serverlessPrivateLinkDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(serverlessPrivateLinkResourceName, "name", name),
					resource.TestCheckResourceAttr(serverlessPrivateLinkResourceName, "cloud_provider", "aws"),
					resource.TestCheckResourceAttr(serverlessPrivateLinkResourceName, "serverless_region", "eu-west-1"),
					resource.TestCheckResourceAttrSet(serverlessPrivateLinkResourceName, "id"),
					resource.TestCheckResourceAttrSet(serverlessPrivateLinkResourceName, "resource_group_id"),
					resource.TestCheckResourceAttrSet(serverlessPrivateLinkResourceName, "state"),
					resource.TestCheckResourceAttrSet(serverlessPrivateLinkResourceName, "status.aws.vpc_endpoint_service_name"),
					resource.TestCheckResourceAttrSet(serverlessPrivateLinkResourceName, "status.aws.availability_zones.#"),
					resource.TestCheckResourceAttrSet(serverlessPrivateLinkResourceName, "status.aws.availability_zones.0"),
					resource.TestCheckResourceAttr(serverlessPrivateLinkResourceName, "cloud_provider_config.aws.allowed_principals.#", "1"),
				),
			},
			{
				ConfigDirectory:          config.StaticDirectory(serverlessPrivateLinkDir),
				ConfigVariables:          updateTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(serverlessPrivateLinkResourceName, "name", name),
					resource.TestCheckResourceAttr(serverlessPrivateLinkResourceName, "cloud_provider_config.aws.allowed_principals.#", "2"),
				),
			},
			{
				ResourceName:             serverlessPrivateLinkResourceName,
				ConfigDirectory:          config.StaticDirectory(serverlessPrivateLinkDir),
				ConfigVariables:          updateTestCaseVars,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"updated_at", "allow_deletion"},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ConfigDirectory:          config.StaticDirectory(serverlessPrivateLinkDir),
				ConfigVariables:          origTestCaseVars,
				Destroy:                  true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})

	resource.AddTestSweepers(generateRandomName("privateLinkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepServerlessPrivateLink{
			PrivateLinkName: name,
			Client:          c,
		}.SweepServerlessPrivateLink,
	})
	resource.AddTestSweepers(generateRandomName("resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}

func TestAccDataSourceServerlessRegions(t *testing.T) {
	if !strings.Contains(runServerlessTests, "true") {
		t.Skip("skipping serverless tests")
	}

	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, providerCfgIDSecretVars)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ConfigDirectory:          config.StaticDirectory(serverlessRegionsDataSourceDir),
				ConfigVariables:          origTestCaseVars,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(serverlessRegionsAWSDataSourceName, "serverless_regions.#"),
					resource.TestMatchResourceAttr(serverlessRegionsAWSDataSourceName, "serverless_regions.#", regexp.MustCompile(`^[1-9]\d*$`)),
					resource.TestCheckResourceAttrSet(serverlessRegionsAWSDataSourceName, "serverless_regions.0.name"),
					resource.TestCheckResourceAttrSet(serverlessRegionsAWSDataSourceName, "serverless_regions.0.time_zone"),
					resource.TestCheckResourceAttrSet(serverlessRegionsAWSDataSourceName, "serverless_regions.0.placement.enabled"),
					resource.TestCheckResourceAttrSet(serverlessRegionsGCPDataSourceName, "serverless_regions.#"),
					resource.TestMatchResourceAttr(serverlessRegionsGCPDataSourceName, "serverless_regions.#", regexp.MustCompile(`^[1-9]\d*$`)),
					resource.TestCheckResourceAttrSet(serverlessRegionsGCPDataSourceName, "serverless_regions.0.name"),
					resource.TestCheckResourceAttrSet(serverlessRegionsGCPDataSourceName, "serverless_regions.0.time_zone"),
					resource.TestCheckResourceAttrSet(serverlessRegionsGCPDataSourceName, "serverless_regions.0.placement.enabled"),
				),
			},
		},
	})
}

// TestAccUserWriteOnlyPassword tests the write-only password field for the user resource.
// Tests create/update with password_wo and bidirectional migration between password and password_wo.
func TestAccUserWriteOnlyPassword(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "wo-user")

	// Step 1: Create with password_wo
	createVars := make(map[string]config.Variable)
	maps.Copy(createVars, providerCfgIDSecretVars)
	createVars["resource_group_name"] = config.StringVariable(name)
	createVars["network_name"] = config.StringVariable(name)
	createVars["cluster_name"] = config.StringVariable(name)
	createVars["user_name"] = config.StringVariable(name)
	createVars["topic_name"] = config.StringVariable(name)
	createVars["user_password_wo"] = config.StringVariable("initial-secret-123")
	createVars["user_password_wo_version"] = config.IntegerVariable(1)
	createVars["cluster_allow_deletion"] = config.BoolVariable(true)
	createVars["user_allow_deletion"] = config.BoolVariable(true)
	createVars["acl_allow_deletion"] = config.BoolVariable(true)
	createVars["sr_acl_allow_deletion"] = config.BoolVariable(true)
	if throughputTier != "" {
		createVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	// Step 2: Update password via version bump
	updateVars := make(map[string]config.Variable)
	maps.Copy(updateVars, createVars)
	updateVars["user_password_wo"] = config.StringVariable("updated-secret-456")
	updateVars["user_password_wo_version"] = config.IntegerVariable(2)

	// Step 3: Migrate to legacy password field
	migrateToLegacyVars := make(map[string]config.Variable)
	maps.Copy(migrateToLegacyVars, createVars)
	delete(migrateToLegacyVars, "user_password_wo")
	delete(migrateToLegacyVars, "user_password_wo_version")
	migrateToLegacyVars["user_pw"] = config.StringVariable("legacy-secret-789")

	// Step 4: Migrate back to password_wo
	migrateBackToWOVars := make(map[string]config.Variable)
	maps.Copy(migrateBackToWOVars, createVars)
	migrateBackToWOVars["user_password_wo"] = config.StringVariable("final-secret-000")
	migrateBackToWOVars["user_password_wo_version"] = config.IntegerVariable(3)

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.11.0"))),
		},
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with password_wo
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: createVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(userResourceName, "password_wo_version", "1"),
				),
			},
			// Step 2: Update via version bump
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: updateVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(userResourceName, "password_wo_version", "2"),
				),
			},
			// Step 3: Migrate to legacy password
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: migrateToLegacyVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(userResourceName, "password", "legacy-secret-789"),
				),
			},
			// Step 4: Migrate back to password_wo
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: migrateBackToWOVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name", name),
					resource.TestCheckResourceAttr(userResourceName, "password_wo_version", "3"),
				),
			},
		},
	})

	resource.AddTestSweepers(generateRandomName("wo-user-clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepCluster,
	})
	resource.AddTestSweepers(generateRandomName("wo-user-networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("wo-user-resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}

// TestAccSchemaWriteOnlyPassword tests the write-only password field for the schema resource.
// Tests create/update with password_wo and bidirectional migration between password and password_wo.
func TestAccSchemaWriteOnlyPassword(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "wo-schema")

	// Step 1: Create with password_wo
	createVars := make(map[string]config.Variable)
	maps.Copy(createVars, providerCfgIDSecretVars)
	createVars["resource_group_name"] = config.StringVariable(name)
	createVars["network_name"] = config.StringVariable(name)
	createVars["cluster_name"] = config.StringVariable(name)
	createVars["user_name"] = config.StringVariable(name)
	createVars["topic_name"] = config.StringVariable(name)
	createVars["user_pw"] = config.StringVariable("user-password-123")
	createVars["schema_password_wo"] = config.StringVariable("initial-schema-secret")
	createVars["schema_password_wo_version"] = config.IntegerVariable(1)
	createVars["cluster_allow_deletion"] = config.BoolVariable(true)
	createVars["user_allow_deletion"] = config.BoolVariable(true)
	createVars["acl_allow_deletion"] = config.BoolVariable(true)
	createVars["sr_acl_allow_deletion"] = config.BoolVariable(true)
	if throughputTier != "" {
		createVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	// Step 2: Update password via version bump
	updateVars := make(map[string]config.Variable)
	maps.Copy(updateVars, createVars)
	updateVars["schema_password_wo"] = config.StringVariable("updated-schema-secret")
	updateVars["schema_password_wo_version"] = config.IntegerVariable(2)

	// Step 3: Migrate to legacy password field (uses user_pw for schemas)
	migrateToLegacyVars := make(map[string]config.Variable)
	maps.Copy(migrateToLegacyVars, createVars)
	delete(migrateToLegacyVars, "schema_password_wo")
	delete(migrateToLegacyVars, "schema_password_wo_version")
	// Schema uses user_pw as fallback password

	// Step 4: Migrate back to password_wo
	migrateBackToWOVars := make(map[string]config.Variable)
	maps.Copy(migrateBackToWOVars, createVars)
	migrateBackToWOVars["schema_password_wo"] = config.StringVariable("final-schema-secret")
	migrateBackToWOVars["schema_password_wo_version"] = config.IntegerVariable(3)

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.11.0"))),
		},
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with password_wo
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: createVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(schemaResourceName, "subject", name+"-value"),
					resource.TestCheckResourceAttr(schemaResourceName, "password_wo_version", "1"),
				),
			},
			// Step 2: Update via version bump
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: updateVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(schemaResourceName, "subject", name+"-value"),
					resource.TestCheckResourceAttr(schemaResourceName, "password_wo_version", "2"),
				),
			},
			// Step 3: Migrate to legacy password (fallback to user_pw)
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: migrateToLegacyVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(schemaResourceName, "subject", name+"-value"),
					resource.TestCheckResourceAttr(schemaResourceName, "password", "user-password-123"),
				),
			},
			// Step 4: Migrate back to password_wo
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: migrateBackToWOVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(schemaResourceName, "subject", name+"-value"),
					resource.TestCheckResourceAttr(schemaResourceName, "password_wo_version", "3"),
				),
			},
		},
	})

	resource.AddTestSweepers(generateRandomName("wo-schema-clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepCluster,
	})
	resource.AddTestSweepers(generateRandomName("wo-schema-networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("wo-schema-resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}

// TestAccSchemaRegistryACLWriteOnlyPassword tests the write-only password field for the schema registry ACL resource.
// Tests create/update with password_wo and bidirectional migration between password and password_wo.
func TestAccSchemaRegistryACLWriteOnlyPassword(t *testing.T) {
	if !strings.Contains(runClusterTests, "true") {
		t.Skip("skipping cluster tests")
	}
	ctx := context.Background()
	name := generateRandomName(accNamePrepend + "wo-sr-acl")

	// Step 1: Create with password_wo
	createVars := make(map[string]config.Variable)
	maps.Copy(createVars, providerCfgIDSecretVars)
	createVars["resource_group_name"] = config.StringVariable(name)
	createVars["network_name"] = config.StringVariable(name)
	createVars["cluster_name"] = config.StringVariable(name)
	createVars["user_name"] = config.StringVariable(name)
	createVars["topic_name"] = config.StringVariable(name)
	createVars["user_pw"] = config.StringVariable("user-password-123")
	createVars["sr_acl_password_wo"] = config.StringVariable("initial-sr-acl-secret")
	createVars["sr_acl_password_wo_version"] = config.IntegerVariable(1)
	createVars["cluster_allow_deletion"] = config.BoolVariable(true)
	createVars["user_allow_deletion"] = config.BoolVariable(true)
	createVars["acl_allow_deletion"] = config.BoolVariable(true)
	createVars["sr_acl_allow_deletion"] = config.BoolVariable(true)
	if throughputTier != "" {
		createVars["throughput_tier"] = config.StringVariable(throughputTier)
	}

	// Step 2: Update password via version bump
	updateVars := make(map[string]config.Variable)
	maps.Copy(updateVars, createVars)
	updateVars["sr_acl_password_wo"] = config.StringVariable("updated-sr-acl-secret")
	updateVars["sr_acl_password_wo_version"] = config.IntegerVariable(2)

	// Step 3: Migrate to legacy password field (uses user_pw for SR ACLs)
	migrateToLegacyVars := make(map[string]config.Variable)
	maps.Copy(migrateToLegacyVars, createVars)
	delete(migrateToLegacyVars, "sr_acl_password_wo")
	delete(migrateToLegacyVars, "sr_acl_password_wo_version")
	// SR ACL uses user_pw as fallback password

	// Step 4: Migrate back to password_wo
	migrateBackToWOVars := make(map[string]config.Variable)
	maps.Copy(migrateBackToWOVars, createVars)
	migrateBackToWOVars["sr_acl_password_wo"] = config.StringVariable("final-sr-acl-secret")
	migrateBackToWOVars["sr_acl_password_wo_version"] = config.IntegerVariable(3)

	c, err := newTestClients(ctx, clientID, clientSecret, cloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.11.0"))),
		},
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with password_wo
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: createVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(schemaRegistryACLReadProductName, "id"),
					resource.TestCheckResourceAttr(schemaRegistryACLReadProductName, "password_wo_version", "1"),
				),
			},
			// Step 2: Update via version bump
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: updateVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(schemaRegistryACLReadProductName, "id"),
					resource.TestCheckResourceAttr(schemaRegistryACLReadProductName, "password_wo_version", "2"),
				),
			},
			// Step 3: Migrate to legacy password (fallback to user_pw)
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: migrateToLegacyVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(schemaRegistryACLReadProductName, "id"),
					resource.TestCheckResourceAttr(schemaRegistryACLReadProductName, "password", "user-password-123"),
				),
			},
			// Step 4: Migrate back to password_wo
			{
				ConfigDirectory: config.StaticDirectory(awsDedicatedClusterDir),
				ConfigVariables: migrateBackToWOVars,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(schemaRegistryACLReadProductName, "id"),
					resource.TestCheckResourceAttr(schemaRegistryACLReadProductName, "password_wo_version", "3"),
				),
			},
		},
	})

	resource.AddTestSweepers(generateRandomName("wo-sr-acl-clusterSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepCluster{
			ClusterName: name,
			Client:      c,
		}.SweepCluster,
	})
	resource.AddTestSweepers(generateRandomName("wo-sr-acl-networkSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepNetwork{
			NetworkName: name,
			Client:      c,
		}.SweepNetworks,
	})
	resource.AddTestSweepers(generateRandomName("wo-sr-acl-resourcegroupSweeper"), &resource.Sweeper{
		Name: name,
		F: sweepResourceGroup{
			ResourceGroupName: name,
			Client:            c,
		}.SweepResourceGroup,
	})
}
