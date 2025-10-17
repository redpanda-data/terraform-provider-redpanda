package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	ctx := context.Background()

	clientID := os.Getenv("REDPANDA_CLIENT_ID")
	clientSecret := os.Getenv("REDPANDA_CLIENT_SECRET")
	cloudEnv := os.Getenv("REDPANDA_CLOUD_ENVIRONMENT")

	if clientID == "" || clientSecret == "" {
		log.Fatal("REDPANDA_CLIENT_ID and REDPANDA_CLIENT_SECRET must be set")
	}

	if cloudEnv == "" {
		cloudEnv = "prod"
	}

	fmt.Printf("Connecting to Redpanda Cloud (%s)...\n", cloudEnv)

	endpoint, err := cloud.EndpointForEnv(cloudEnv)
	if err != nil {
		log.Fatalf("Failed to get endpoint: %v", err)
	}

	token, err := cloud.RequestToken(ctx, endpoint, clientID, clientSecret)
	if err != nil {
		log.Fatalf("Failed to request auth token: %v", err)
	}

	conn, err := cloud.SpawnConn(endpoint.APIURL, token, "dev", "")
	if err != nil {
		log.Fatalf("Failed to spawn connection: %v", err)
	}

	client := cloud.NewControlPlaneClientSet(conn)

	prefix := "tfrp-"
	dryRun := false
	statusOnly := false

	// Parse arguments
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--status", "-s":
			statusOnly = true
		default:
			if !strings.HasPrefix(arg, "-") {
				prefix = arg
			}
		}
	}

	switch {
	case statusOnly:
		fmt.Println("STATUS MODE: Showing current resources")
	case dryRun:
		fmt.Println("DRY RUN MODE: No resources will be deleted")
	}
	fmt.Printf("Resources with prefix: %s\n\n", prefix)

	// Step 1: List and delete clusters
	if statusOnly {
		fmt.Println("=== Clusters ===")
	} else {
		fmt.Println("=== Cleaning up Clusters ===")
	}
	clustersResp, err := client.Cluster.ListClusters(ctx, &controlplanev1.ListClustersRequest{
		Filter: &controlplanev1.ListClustersRequest_Filter{NameContains: prefix},
	})
	if err != nil {
		log.Fatalf("Failed to list clusters: %v", err)
	}

	clusterCount := 0
	clustersDeleted := 0
	clustersFailed := 0
	for _, cluster := range clustersResp.Clusters {
		switch {
		case statusOnly:
			fmt.Printf("Cluster: %s\n", cluster.Name)
			fmt.Printf("  ID: %s\n", cluster.Id)
			fmt.Printf("  State: %s\n", cluster.State)
			fmt.Printf("  Cloud Provider: %s\n", cluster.CloudProvider)
			fmt.Printf("  Region: %s\n", cluster.Region)
			if cluster.CreatedAt != nil {
				fmt.Printf("  Created: %s\n", cluster.CreatedAt.AsTime().Format(time.RFC3339))
			}
			clusterCount++
		case dryRun:
			fmt.Printf("Would delete cluster: %s (ID: %s, State: %s)\n", cluster.Name, cluster.Id, cluster.State)
			clusterCount++
		default:
			fmt.Printf("Deleting cluster: %s (ID: %s)\n", cluster.Name, cluster.Id)
			_, err := client.Cluster.DeleteCluster(ctx, &controlplanev1.DeleteClusterRequest{
				Id: cluster.Id,
			})
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
					fmt.Println("  ⚠ Cluster already deleted")
				} else {
					fmt.Printf("  ✗ Error deleting cluster: %v\n", err)
					clustersFailed++
				}
			} else {
				fmt.Println("  ✓ Cluster deletion initiated")
				clustersDeleted++
			}
		}
	}

	// Step 2: List and delete networks
	if statusOnly {
		fmt.Println("\n=== Networks ===")
	} else {
		fmt.Println("\n=== Cleaning up Networks ===")
	}
	networksResp, err := client.Network.ListNetworks(ctx, &controlplanev1.ListNetworksRequest{
		Filter: &controlplanev1.ListNetworksRequest_Filter{NameContains: prefix},
	})
	if err != nil {
		log.Fatalf("Failed to list networks: %v", err)
	}

	networkCount := 0
	networksDeleted := 0
	networksFailed := 0
	for _, network := range networksResp.Networks {
		switch {
		case statusOnly:
			fmt.Printf("Network: %s\n", network.Name)
			fmt.Printf("  ID: %s\n", network.Id)
			fmt.Printf("  State: %s\n", network.State)
			fmt.Printf("  Cloud Provider: %s\n", network.CloudProvider)
			fmt.Printf("  Region: %s\n", network.Region)
			fmt.Printf("  Cluster Type: %s\n", network.ClusterType)
			if network.CreatedAt != nil {
				fmt.Printf("  Created: %s\n", network.CreatedAt.AsTime().Format(time.RFC3339))
			}
			networkCount++
		case dryRun:
			fmt.Printf("Would delete network: %s (ID: %s, State: %s)\n", network.Name, network.Id, network.State)
			networkCount++
		default:
			fmt.Printf("Deleting network: %s (ID: %s)\n", network.Name, network.Id)
			_, err := client.Network.DeleteNetwork(ctx, &controlplanev1.DeleteNetworkRequest{
				Id: network.Id,
			})
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
					fmt.Println("  ⚠ Network already deleted")
				} else {
					fmt.Printf("  ✗ Error deleting network: %v\n", err)
					networksFailed++
				}
			} else {
				fmt.Println("  ✓ Network deletion initiated")
				networksDeleted++
			}
		}
	}

	// Step 3: List and delete resource groups
	if statusOnly {
		fmt.Println("\n=== Resource Groups ===")
	} else {
		fmt.Println("\n=== Cleaning up Resource Groups ===")
	}

	resourceGroupsResp, err := client.ResourceGroup.ListResourceGroups(ctx, &controlplanev1.ListResourceGroupsRequest{
		Filter: &controlplanev1.ListResourceGroupsRequest_Filter{NameContains: prefix},
	})
	if err != nil {
		log.Fatalf("Failed to list resource groups: %v", err)
	}

	rgCount := 0
	resourceGroupsDeleted := 0
	resourceGroupsFailed := 0
	for _, rg := range resourceGroupsResp.ResourceGroups {
		switch {
		case statusOnly:
			fmt.Printf("Resource Group: %s\n", rg.Name)
			fmt.Printf("  ID: %s\n", rg.Id)
			if rg.CreatedAt != nil {
				fmt.Printf("  Created: %s\n", rg.CreatedAt.AsTime().Format(time.RFC3339))
			}
			rgCount++
		case dryRun:
			fmt.Printf("Would delete resource group: %s (ID: %s)\n", rg.Name, rg.Id)
			rgCount++
		default:
			fmt.Printf("Deleting resource group: %s (ID: %s)\n", rg.Name, rg.Id)
			_, err := client.ResourceGroup.DeleteResourceGroup(ctx, &controlplanev1.DeleteResourceGroupRequest{
				Id: rg.Id,
			})
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
					fmt.Println("  ⚠ Resource group already deleted")
				} else {
					fmt.Printf("  ✗ Error deleting resource group: %v\n", err)
					resourceGroupsFailed++
				}
			} else {
				fmt.Println("  ✓ Resource group deleted")
				resourceGroupsDeleted++
			}
		}
	}

	fmt.Println("\n=== Summary ===")
	switch {
	case statusOnly:
		fmt.Printf("Total clusters: %d\n", clusterCount)
		fmt.Printf("Total networks: %d\n", networkCount)
		fmt.Printf("Total resource groups: %d\n", rgCount)
	case dryRun:
		fmt.Printf("Would delete clusters: %d\n", clusterCount)
		fmt.Printf("Would delete networks: %d\n", networkCount)
		fmt.Printf("Would delete resource groups: %d\n", rgCount)
	default:
		fmt.Printf("Clusters successfully deleted: %d\n", clustersDeleted)
		fmt.Printf("Clusters failed to delete: %d\n", clustersFailed)
		fmt.Printf("Networks successfully deleted: %d\n", networksDeleted)
		fmt.Printf("Networks failed to delete: %d\n", networksFailed)
		fmt.Printf("Resource groups successfully deleted: %d\n", resourceGroupsDeleted)
		fmt.Printf("Resource groups failed to delete: %d\n", resourceGroupsFailed)

		totalDeleted := clustersDeleted + networksDeleted + resourceGroupsDeleted
		totalFailed := clustersFailed + networksFailed + resourceGroupsFailed

		fmt.Printf("\nTotal successfully deleted: %d\n", totalDeleted)
		fmt.Printf("Total failed: %d\n", totalFailed)

		if totalFailed > 0 {
			fmt.Println("\n⚠ Some resources failed to delete. Review errors above.")
		}
		if totalDeleted > 0 {
			fmt.Println("\nNote: Successfully deleted resources may take time to fully remove.")
		}
	}
}
