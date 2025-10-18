package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"cloud.google.com/go/storage"
	"github.com/fatih/color"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/iterator"
)

type CleanupConfig struct {
	CommonPrefix string
	ProjectID    string
	Region       string
	DryRun       bool
}

type GCPClients struct {
	Compute              *compute.InstancesClient
	InstanceGroupManager *compute.InstanceGroupManagersClient
	ClusterManager       *container.ClusterManagerClient
	Firewall             *compute.FirewallsClient
	Router               *compute.RoutersClient
	Address              *compute.AddressesClient
	Subnetwork           *compute.SubnetworksClient
	Network              *compute.NetworksClient
	IAM                  *iam.Service
	Storage              *storage.Client
}

var (
	red    = color.New(color.FgRed).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
)

func main() {
	cfg := parseFlags()

	if cfg.ProjectID == "" {
		fmt.Printf("%s --project-id is required\n", red("ERROR:"))
		os.Exit(1)
	}

	ctx := context.Background()

	// Initialize GCP clients
	clients, err := initializeClients(ctx)
	if err != nil {
		fmt.Printf("%s Failed to initialize GCP clients: %v\n", red("ERROR:"), err)
		os.Exit(1)
	}
	defer closeClients(clients)

	// List resources that will be deleted
	resourceCount, err := listResources(ctx, clients, cfg)
	if err != nil {
		fmt.Printf("%s Failed to list resources: %v\n", red("ERROR:"), err)
		os.Exit(1)
	}

	if resourceCount == 0 {
		fmt.Printf("\n%s No matching resources found to delete\n", yellow("INFO:"))
		os.Exit(0)
	}

	// Confirm deletion (unless dry-run)
	if !cfg.DryRun {
		if !confirmDeletion(resourceCount) {
			fmt.Println(yellow("Deletion cancelled by user"))
			os.Exit(0)
		}
	}

	fmt.Printf("\n%s Starting cleanup for Redpanda BYOVPC resources\n", cyan("INFO:"))
	fmt.Printf("  Common Prefix: %s\n", cfg.CommonPrefix)
	fmt.Printf("  Project ID: %s\n", cfg.ProjectID)
	fmt.Printf("  Region: %s\n", cfg.Region)
	fmt.Printf("  Dry Run: %v\n\n", cfg.DryRun)

	// Delete resources in dependency order
	var errorCount int

	if err := deleteComputeInstances(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete compute instances: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteInstanceGroupManagers(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete instance group managers: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteGKEClusters(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete GKE clusters: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteFirewallRules(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete firewall rules: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteCloudRouters(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete cloud routers: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteAddresses(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete addresses: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteSubnetworks(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete subnetworks: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteNetworks(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete networks: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteServiceAccounts(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete service accounts: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteStorageBuckets(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete storage buckets: %v\n", red("ERROR:"), err)
		errorCount++
	}

	// Exit with error code if any errors occurred
	if errorCount > 0 {
		os.Exit(1)
	}
}

func parseFlags() *CleanupConfig {
	cfg := &CleanupConfig{}

	flag.StringVar(&cfg.CommonPrefix, "common-prefix", "redpanda", "Common prefix used for resource naming")
	flag.StringVar(&cfg.ProjectID, "project-id", "", "GCP Project ID (required)")
	flag.StringVar(&cfg.Region, "region", "us-central1", "GCP region")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Preview actions without deleting")

	flag.Parse()

	return cfg
}

func initializeClients(ctx context.Context) (*GCPClients, error) {
	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	igmClient, err := compute.NewInstanceGroupManagersRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	clusterManagerClient, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		return nil, err
	}

	firewallClient, err := compute.NewFirewallsRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	routerClient, err := compute.NewRoutersRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	addressClient, err := compute.NewAddressesRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	subnetworkClient, err := compute.NewSubnetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	networkClient, err := compute.NewNetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}

	iamService, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &GCPClients{
		Compute:              computeClient,
		InstanceGroupManager: igmClient,
		ClusterManager:       clusterManagerClient,
		Firewall:             firewallClient,
		Router:               routerClient,
		Address:              addressClient,
		Subnetwork:           subnetworkClient,
		Network:              networkClient,
		IAM:                  iamService,
		Storage:              storageClient,
	}, nil
}

func closeClients(clients *GCPClients) {
	if clients.Compute != nil {
		clients.Compute.Close()
	}
	if clients.InstanceGroupManager != nil {
		clients.InstanceGroupManager.Close()
	}
	if clients.ClusterManager != nil {
		clients.ClusterManager.Close()
	}
	if clients.Firewall != nil {
		clients.Firewall.Close()
	}
	if clients.Router != nil {
		clients.Router.Close()
	}
	if clients.Address != nil {
		clients.Address.Close()
	}
	if clients.Subnetwork != nil {
		clients.Subnetwork.Close()
	}
	if clients.Network != nil {
		clients.Network.Close()
	}
	if clients.Storage != nil {
		clients.Storage.Close()
	}
}

func listResources(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) (int, error) {
	fmt.Printf("\n%s Scanning for resources to delete...\n", cyan("INFO:"))
	fmt.Printf("  Common Prefix: %s\n", cfg.CommonPrefix)
	fmt.Printf("  Project ID: %s\n", cfg.ProjectID)
	fmt.Printf("  Region: %s\n\n", cfg.Region)

	totalCount := 0

	// List Compute Instances
	instanceReq := &computepb.AggregatedListInstancesRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Compute.AggregatedList(ctx, instanceReq)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list instances: %v\n", yellow("WARNING:"), err)
			break
		}
		for _, instance := range pair.Value.Instances {
			name := instance.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				totalCount++
				zone := getZoneFromURL(instance.GetZone())
				fmt.Printf("  - Compute Instance: %s (zone: %s)\n", name, zone)
			}
		}
	}

	// List Firewall Rules
	firewallReq := &computepb.ListFirewallsRequest{
		Project: cfg.ProjectID,
	}
	fwIt := clients.Firewall.List(ctx, firewallReq)
	for {
		fw, err := fwIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list firewalls: %v\n", yellow("WARNING:"), err)
			break
		}
		name := fw.GetName()
		if matchesRedpandaResource(name, cfg.CommonPrefix) {
			totalCount++
			fmt.Printf("  - Firewall Rule: %s\n", name)
		}
	}

	// List Cloud Routers
	routerReq := &computepb.AggregatedListRoutersRequest{
		Project: cfg.ProjectID,
	}
	routerIt := clients.Router.AggregatedList(ctx, routerReq)
	for {
		pair, err := routerIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list routers: %v\n", yellow("WARNING:"), err)
			break
		}
		for _, router := range pair.Value.Routers {
			name := router.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				totalCount++
				region := getRegionFromURL(router.GetRegion())
				fmt.Printf("  - Cloud Router: %s (region: %s)\n", name, region)
			}
		}
	}

	// List Addresses
	addrReq := &computepb.AggregatedListAddressesRequest{
		Project: cfg.ProjectID,
	}
	addrIt := clients.Address.AggregatedList(ctx, addrReq)
	for {
		pair, err := addrIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list addresses: %v\n", yellow("WARNING:"), err)
			break
		}
		for _, addr := range pair.Value.Addresses {
			name := addr.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				totalCount++
				region := getRegionFromURL(addr.GetRegion())
				fmt.Printf("  - Address: %s (region: %s)\n", name, region)
			}
		}
	}

	// List Subnetworks
	subnetReq := &computepb.AggregatedListSubnetworksRequest{
		Project: cfg.ProjectID,
	}
	subnetIt := clients.Subnetwork.AggregatedList(ctx, subnetReq)
	for {
		pair, err := subnetIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list subnetworks: %v\n", yellow("WARNING:"), err)
			break
		}
		for _, subnet := range pair.Value.Subnetworks {
			name := subnet.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				totalCount++
				region := getRegionFromURL(subnet.GetRegion())
				fmt.Printf("  - Subnetwork: %s (region: %s)\n", name, region)
			}
		}
	}

	// List Networks
	networkReq := &computepb.ListNetworksRequest{
		Project: cfg.ProjectID,
	}
	networkIt := clients.Network.List(ctx, networkReq)
	for {
		network, err := networkIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list networks: %v\n", yellow("WARNING:"), err)
			break
		}
		name := network.GetName()
		if matchesRedpandaResource(name, cfg.CommonPrefix) {
			totalCount++
			fmt.Printf("  - VPC Network: %s\n", name)
		}
	}

	// List GKE Clusters
	gkeReq := &containerpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/-", cfg.ProjectID),
	}
	gkeResp, err := clients.ClusterManager.ListClusters(ctx, gkeReq)
	if err != nil {
		fmt.Printf("%s Failed to list GKE clusters: %v\n", yellow("WARNING:"), err)
	} else {
		for _, cluster := range gkeResp.Clusters {
			if matchesRedpandaResource(cluster.Name, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - GKE Cluster: %s (location: %s)\n", cluster.Name, cluster.Location)
			}
		}
	}

	// List Service Accounts
	saList, err := clients.IAM.Projects.ServiceAccounts.List(fmt.Sprintf("projects/%s", cfg.ProjectID)).Do()
	if err != nil {
		fmt.Printf("%s Failed to list service accounts: %v\n", yellow("WARNING:"), err)
	} else {
		for _, sa := range saList.Accounts {
			if matchesRedpandaResource(sa.Email, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Service Account: %s\n", sa.Email)
			}
		}
	}

	// List Storage Buckets
	bucketIt := clients.Storage.Buckets(ctx, cfg.ProjectID)
	for {
		bucket, err := bucketIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list storage buckets: %v\n", yellow("WARNING:"), err)
			break
		}
		if matchesRedpandaResource(bucket.Name, cfg.CommonPrefix) {
			totalCount++
			fmt.Printf("  - Storage Bucket: %s\n", bucket.Name)
		}
	}

	fmt.Printf("\n%s Total resources found: %d\n", cyan("INFO:"), totalCount)
	return totalCount, nil
}

func confirmDeletion(resourceCount int) bool {
	fmt.Printf("\n%s This action CANNOT be undone!\n", red("WARNING:"))
	fmt.Printf("%s You are about to delete %d resource(s)\n\n", yellow("WARNING:"), resourceCount)
	fmt.Print("Type 'yes' to confirm deletion: ")

	var response string
	fmt.Scanln(&response)

	return strings.ToLower(response) == "yes"
}

func getZoneFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func getRegionFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// matchesRedpandaResource checks if a resource name matches Redpanda naming patterns
// It matches both the configured prefix (default: "redpanda") and the "rp-" prefix
// used by Redpanda Cloud when deploying into BYOVPC
// Excludes any resources containing "devex" to protect development/testing resources
func matchesRedpandaResource(name, commonPrefix string) bool {
	// Exclude anything with "devex" in the name
	if strings.Contains(strings.ToLower(name), "devex") {
		return false
	}
	return strings.HasPrefix(name, commonPrefix) || strings.HasPrefix(name, "rp-")
}

// deleteComputeInstances deletes Compute Engine instances
func deleteComputeInstances(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Compute instances...\n", cyan("INFO:"))

	deletedCount := 0
	instanceReq := &computepb.AggregatedListInstancesRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Compute.AggregatedList(ctx, instanceReq)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		for _, instance := range pair.Value.Instances {
			name := instance.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				zone := getZoneFromURL(instance.GetZone())
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete instance: %s (zone: %s)\n", name, zone)
				} else {
					deleteReq := &computepb.DeleteInstanceRequest{
						Project:  cfg.ProjectID,
						Zone:     zone,
						Instance: name,
					}
					op, err := clients.Compute.Delete(ctx, deleteReq)
					if err != nil {
						fmt.Printf("%s Failed to delete instance %s: %v\n", yellow("WARNING:"), name, err)
					} else {
						// Wait for the operation to complete
						if err := op.Wait(ctx); err != nil {
							fmt.Printf("%s Failed to wait for instance %s deletion: %v\n", yellow("WARNING:"), name, err)
						} else {
							fmt.Printf("  %s Deleted instance: %s\n", green("✓"), name)
							deletedCount++
						}
					}
				}
			}
		}
	}

	return nil
}

// deleteInstanceGroupManagers deletes instance group managers (and their managed instance groups)
func deleteInstanceGroupManagers(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting instance group managers...\n", cyan("INFO:"))

	igmReq := &computepb.AggregatedListInstanceGroupManagersRequest{
		Project: cfg.ProjectID,
	}
	it := clients.InstanceGroupManager.AggregatedList(ctx, igmReq)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		for _, igm := range pair.Value.InstanceGroupManagers {
			name := igm.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				zone := getZoneFromURL(igm.GetZone())
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete instance group manager: %s (zone: %s)\n", name, zone)
				} else {
					deleteReq := &computepb.DeleteInstanceGroupManagerRequest{
						Project:              cfg.ProjectID,
						Zone:                 zone,
						InstanceGroupManager: name,
					}
					op, err := clients.InstanceGroupManager.Delete(ctx, deleteReq)
					if err != nil {
						fmt.Printf("%s Failed to delete instance group manager %s: %v\n", yellow("WARNING:"), name, err)
					} else {
						// Wait for the operation to complete
						if err := op.Wait(ctx); err != nil {
							fmt.Printf("%s Failed to wait for instance group manager %s deletion: %v\n", yellow("WARNING:"), name, err)
						} else {
							fmt.Printf("  %s Deleted instance group manager: %s\n", green("✓"), name)
						}
					}
				}
			}
		}
	}

	return nil
}

// deleteGKEClusters deletes GKE clusters (including all nodes and resources)
func deleteGKEClusters(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting GKE clusters...\n", cyan("INFO:"))

	gkeReq := &containerpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/-", cfg.ProjectID),
	}
	gkeResp, err := clients.ClusterManager.ListClusters(ctx, gkeReq)
	if err != nil {
		return err
	}

	for _, cluster := range gkeResp.Clusters {
		if matchesRedpandaResource(cluster.Name, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete GKE cluster: %s (location: %s)\n", cluster.Name, cluster.Location)
			} else {
				// Build the cluster name in the format: projects/{project}/locations/{location}/clusters/{cluster}
				clusterName := fmt.Sprintf("projects/%s/locations/%s/clusters/%s",
					cfg.ProjectID, cluster.Location, cluster.Name)

				deleteReq := &containerpb.DeleteClusterRequest{
					Name: clusterName,
				}

				fmt.Printf("  Deleting GKE cluster: %s (this may take 5-10 minutes)...\n", cluster.Name)
				op, err := clients.ClusterManager.DeleteCluster(ctx, deleteReq)
				if err != nil {
					fmt.Printf("%s Failed to delete GKE cluster %s: %v\n", yellow("WARNING:"), cluster.Name, err)
				} else {
					// Wait for the operation to complete by polling
					fmt.Printf("  Waiting for GKE cluster %s deletion to complete...\n", cluster.Name)
					if err := waitForGKEOperation(ctx, clients.ClusterManager, op); err != nil {
						fmt.Printf("%s Failed to wait for GKE cluster %s deletion: %v\n", yellow("WARNING:"), cluster.Name, err)
					} else {
						fmt.Printf("  %s Deleted GKE cluster: %s\n", green("✓"), cluster.Name)
					}
				}
			}
		}
	}

	return nil
}

// waitForGKEOperation polls a GKE operation until it completes
func waitForGKEOperation(ctx context.Context, client *container.ClusterManagerClient, op *containerpb.Operation) error {
	// Poll every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			getOpReq := &containerpb.GetOperationRequest{
				Name: op.Name,
			}
			currentOp, err := client.GetOperation(ctx, getOpReq)
			if err != nil {
				return fmt.Errorf("failed to get operation status: %w", err)
			}

			// Check if operation is done
			if currentOp.Status == containerpb.Operation_DONE {
				// Check if there was an error
				if currentOp.Error != nil {
					return fmt.Errorf("operation failed: %s", currentOp.Error.Message)
				}
				return nil
			}
			// Operation still running, continue polling
		}
	}
}

// deleteFirewallRules deletes firewall rules
func deleteFirewallRules(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting firewall rules...\n", cyan("INFO:"))

	firewallReq := &computepb.ListFirewallsRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Firewall.List(ctx, firewallReq)
	for {
		fw, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		name := fw.GetName()
		if matchesRedpandaResource(name, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete firewall: %s\n", name)
			} else {
				deleteReq := &computepb.DeleteFirewallRequest{
					Project:  cfg.ProjectID,
					Firewall: name,
				}
				_, err := clients.Firewall.Delete(ctx, deleteReq)
				if err != nil {
					fmt.Printf("%s Failed to delete firewall %s: %v\n", yellow("WARNING:"), name, err)
				} else {
					fmt.Printf("  %s Deleted firewall: %s\n", green("✓"), name)
				}
			}
		}
	}

	return nil
}

// deleteCloudRouters deletes Cloud Routers and their NAT configurations
func deleteCloudRouters(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Cloud Routers...\n", cyan("INFO:"))

	routerReq := &computepb.AggregatedListRoutersRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Router.AggregatedList(ctx, routerReq)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		for _, router := range pair.Value.Routers {
			name := router.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				region := getRegionFromURL(router.GetRegion())
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete router: %s (region: %s)\n", name, region)
				} else {
					deleteReq := &computepb.DeleteRouterRequest{
						Project: cfg.ProjectID,
						Region:  region,
						Router:  name,
					}
					_, err := clients.Router.Delete(ctx, deleteReq)
					if err != nil {
						fmt.Printf("%s Failed to delete router %s: %v\n", yellow("WARNING:"), name, err)
					} else {
						fmt.Printf("  %s Deleted router: %s\n", green("✓"), name)
					}
				}
			}
		}
	}

	return nil
}

// deleteAddresses deletes static IP addresses
func deleteAddresses(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting addresses...\n", cyan("INFO:"))

	addrReq := &computepb.AggregatedListAddressesRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Address.AggregatedList(ctx, addrReq)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		for _, addr := range pair.Value.Addresses {
			name := addr.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				region := getRegionFromURL(addr.GetRegion())
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete address: %s (region: %s)\n", name, region)
				} else {
					deleteReq := &computepb.DeleteAddressRequest{
						Project: cfg.ProjectID,
						Region:  region,
						Address: name,
					}
					_, err := clients.Address.Delete(ctx, deleteReq)
					if err != nil {
						fmt.Printf("%s Failed to delete address %s: %v\n", yellow("WARNING:"), name, err)
					} else {
						fmt.Printf("  %s Deleted address: %s\n", green("✓"), name)
					}
				}
			}
		}
	}

	return nil
}

// deleteSubnetworks deletes subnetworks
func deleteSubnetworks(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting subnetworks...\n", cyan("INFO:"))

	subnetReq := &computepb.AggregatedListSubnetworksRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Subnetwork.AggregatedList(ctx, subnetReq)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		for _, subnet := range pair.Value.Subnetworks {
			name := subnet.GetName()
			if matchesRedpandaResource(name, cfg.CommonPrefix) {
				region := getRegionFromURL(subnet.GetRegion())
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete subnetwork: %s (region: %s)\n", name, region)
				} else {
					deleteReq := &computepb.DeleteSubnetworkRequest{
						Project:    cfg.ProjectID,
						Region:     region,
						Subnetwork: name,
					}
					_, err := clients.Subnetwork.Delete(ctx, deleteReq)
					if err != nil {
						fmt.Printf("%s Failed to delete subnetwork %s: %v\n", yellow("WARNING:"), name, err)
					} else {
						fmt.Printf("  %s Deleted subnetwork: %s\n", green("✓"), name)
					}
				}
			}
		}
	}

	return nil
}

// deleteNetworks deletes VPC networks
func deleteNetworks(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting VPC networks...\n", cyan("INFO:"))

	networkReq := &computepb.ListNetworksRequest{
		Project: cfg.ProjectID,
	}
	it := clients.Network.List(ctx, networkReq)
	for {
		network, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		name := network.GetName()
		if matchesRedpandaResource(name, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete network: %s\n", name)
			} else {
				deleteReq := &computepb.DeleteNetworkRequest{
					Project: cfg.ProjectID,
					Network: name,
				}
				_, err := clients.Network.Delete(ctx, deleteReq)
				if err != nil {
					fmt.Printf("%s Failed to delete network %s: %v\n", yellow("WARNING:"), name, err)
				} else {
					fmt.Printf("  %s Deleted network: %s\n", green("✓"), name)
				}
			}
		}
	}

	return nil
}

// deleteServiceAccounts deletes service accounts
func deleteServiceAccounts(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting service accounts...\n", cyan("INFO:"))

	saList, err := clients.IAM.Projects.ServiceAccounts.List(fmt.Sprintf("projects/%s", cfg.ProjectID)).Do()
	if err != nil {
		return err
	}

	for _, sa := range saList.Accounts {
		if matchesRedpandaResource(sa.Email, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete service account: %s\n", sa.Email)
			} else {
				_, err := clients.IAM.Projects.ServiceAccounts.Delete(sa.Name).Do()
				if err != nil {
					fmt.Printf("%s Failed to delete service account %s: %v\n", yellow("WARNING:"), sa.Email, err)
				} else {
					fmt.Printf("  %s Deleted service account: %s\n", green("✓"), sa.Email)
				}
			}
		}
	}

	return nil
}

// deleteStorageBuckets deletes Cloud Storage buckets
func deleteStorageBuckets(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting storage buckets...\n", cyan("INFO:"))

	it := clients.Storage.Buckets(ctx, cfg.ProjectID)
	for {
		bucket, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if matchesRedpandaResource(bucket.Name, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete bucket: %s\n", bucket.Name)
			} else {
				// Delete all objects in bucket first
				if err := emptyBucket(ctx, clients.Storage, bucket.Name); err != nil {
					fmt.Printf("%s Failed to empty bucket %s: %v\n", yellow("WARNING:"), bucket.Name, err)
					continue
				}

				// Delete the bucket
				if err := clients.Storage.Bucket(bucket.Name).Delete(ctx); err != nil {
					fmt.Printf("%s Failed to delete bucket %s: %v\n", yellow("WARNING:"), bucket.Name, err)
				} else {
					fmt.Printf("  %s Deleted bucket: %s\n", green("✓"), bucket.Name)
				}
			}
		}
	}

	return nil
}

// emptyBucket deletes all objects in a bucket, including all versions
// Uses parallel deletion for better performance
func emptyBucket(ctx context.Context, client *storage.Client, bucketName string) error {
	bucket := client.Bucket(bucketName)

	// First, collect all objects to delete
	it := bucket.Objects(ctx, &storage.Query{
		Versions: true,
	})

	type objectToDelete struct {
		name       string
		generation int64
	}

	var objects []objectToDelete
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}
		objects = append(objects, objectToDelete{
			name:       objAttrs.Name,
			generation: objAttrs.Generation,
		})
	}

	if len(objects) == 0 {
		return nil
	}

	fmt.Printf("    Found %d object(s) to delete from bucket %s\n", len(objects), bucketName)
	fmt.Printf("    Deleting objects in parallel...\n")

	// Delete objects in parallel with a semaphore to limit concurrency
	const maxConcurrent = 50
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var deleteErrors []error
	deletedCount := 0

	for i, obj := range objects {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(obj objectToDelete, index int) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// Delete the object
			objHandle := bucket.Object(obj.name)
			if obj.generation > 0 {
				objHandle = objHandle.Generation(obj.generation)
			}

			if err := objHandle.Delete(ctx); err != nil {
				mu.Lock()
				deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete %s (gen %d): %w", obj.name, obj.generation, err))
				mu.Unlock()
			} else {
				mu.Lock()
				deletedCount++
				// Show progress every 100 objects
				if deletedCount%100 == 0 {
					fmt.Printf("    Progress: %d/%d objects deleted\n", deletedCount, len(objects))
				}
				mu.Unlock()
			}
		}(obj, i)
	}

	wg.Wait()

	if len(deleteErrors) > 0 {
		// Show first few errors
		fmt.Printf("%s Failed to delete some objects:\n", yellow("WARNING:"))
		for i, err := range deleteErrors {
			if i >= 5 {
				fmt.Printf("  ... and %d more errors\n", len(deleteErrors)-5)
				break
			}
			fmt.Printf("  - %v\n", err)
		}
		return fmt.Errorf("failed to delete %d objects", len(deleteErrors))
	}

	fmt.Printf("    %s Deleted %d object(s) from bucket %s\n", green("✓"), deletedCount, bucketName)
	return nil
}
