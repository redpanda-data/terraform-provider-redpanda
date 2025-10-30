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

type stats struct {
	count, deleted, failed int
}

type resourceHandler[T any] struct {
	name       string
	pluralName string
	list       func() ([]*T, error)
	delete     func(id string) error
	getID      func(*T) string
	getName    func(*T) string
	display    func(*T)
}

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

	// Parse arguments
	prefix := "tfrp-"
	dryRun := false
	statusOnly := false

	for i := 1; i < len(os.Args); i++ {
		switch arg := os.Args[i]; arg {
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
	fmt.Printf("Resources with prefix: %s\n", prefix)

	// Cluster handler
	clusterHandler := resourceHandler[controlplanev1.Cluster]{
		name:       "cluster",
		pluralName: "Clusters",
		list: func() ([]*controlplanev1.Cluster, error) {
			resp, err := client.Cluster.ListClusters(ctx, &controlplanev1.ListClustersRequest{
				Filter: &controlplanev1.ListClustersRequest_Filter{NameContains: prefix},
			})
			if err != nil {
				return nil, err
			}
			return resp.Clusters, nil
		},
		delete: func(id string) error {
			_, err := client.Cluster.DeleteCluster(ctx, &controlplanev1.DeleteClusterRequest{Id: id})
			return err
		},
		getID: func(c *controlplanev1.Cluster) string {
			return c.Id
		},
		getName: func(c *controlplanev1.Cluster) string {
			return c.Name
		},
		display: func(c *controlplanev1.Cluster) {
			fmt.Printf("Cluster: %s\n  ID: %s\n  State: %s\n  Cloud: %s\n  Region: %s\n",
				c.Name, c.Id, c.State, c.CloudProvider, c.Region)
			if c.CreatedAt != nil {
				fmt.Printf("  Created: %s\n", c.CreatedAt.AsTime().Format(time.RFC3339))
			}
		},
	}

	// Network handler
	networkHandler := resourceHandler[controlplanev1.Network]{
		name:       "network",
		pluralName: "Networks",
		list: func() ([]*controlplanev1.Network, error) {
			resp, err := client.Network.ListNetworks(ctx, &controlplanev1.ListNetworksRequest{
				Filter: &controlplanev1.ListNetworksRequest_Filter{NameContains: prefix},
			})
			if err != nil {
				return nil, err
			}
			return resp.Networks, nil
		},
		delete: func(id string) error {
			_, err := client.Network.DeleteNetwork(ctx, &controlplanev1.DeleteNetworkRequest{Id: id})
			return err
		},
		getID: func(n *controlplanev1.Network) string {
			return n.Id
		},
		getName: func(n *controlplanev1.Network) string {
			return n.Name
		},
		display: func(n *controlplanev1.Network) {
			fmt.Printf("Network: %s\n  ID: %s\n  State: %s\n  Cloud: %s\n  Region: %s\n  Type: %s\n",
				n.Name, n.Id, n.State, n.CloudProvider, n.Region, n.ClusterType)
			if n.CreatedAt != nil {
				fmt.Printf("  Created: %s\n", n.CreatedAt.AsTime().Format(time.RFC3339))
			}
		},
	}

	rgHandler := resourceHandler[controlplanev1.ResourceGroup]{
		name:       "resource group",
		pluralName: "Resource Groups",
		list: func() ([]*controlplanev1.ResourceGroup, error) {
			resp, err := client.ResourceGroup.ListResourceGroups(ctx, &controlplanev1.ListResourceGroupsRequest{
				Filter: &controlplanev1.ListResourceGroupsRequest_Filter{NameContains: prefix},
			})
			if err != nil {
				return nil, err
			}
			return resp.ResourceGroups, nil
		},
		delete: func(id string) error {
			_, err := client.ResourceGroup.DeleteResourceGroup(ctx, &controlplanev1.DeleteResourceGroupRequest{Id: id})
			return err
		},
		getID: func(rg *controlplanev1.ResourceGroup) string {
			return rg.Id
		},
		getName: func(rg *controlplanev1.ResourceGroup) string {
			return rg.Name
		},
		display: func(rg *controlplanev1.ResourceGroup) {
			fmt.Printf("Resource Group: %s\n  ID: %s\n", rg.Name, rg.Id)
			if rg.CreatedAt != nil {
				fmt.Printf("  Created: %s\n", rg.CreatedAt.AsTime().Format(time.RFC3339))
			}
		},
	}

	clusterStats, err := processResources(clusterHandler, statusOnly, dryRun)
	if err != nil {
		log.Fatalf("Failed to process clusters: %v", err)
	}
	networkStats, err := processResources(networkHandler, statusOnly, dryRun)
	if err != nil {
		log.Fatalf("Failed to process networks: %v", err)
	}
	rgStats, err := processResources(rgHandler, statusOnly, dryRun)
	if err != nil {
		log.Fatalf("Failed to process resource groups: %v", err)
	}

	fmt.Println("\n=== Summary ===")
	var totalDeleted, totalFailed int

	printStat := func(name string, s stats) {
		switch {
		case statusOnly:
			fmt.Printf("Total %s: %d\n", strings.ToLower(name), s.count)
		case dryRun:
			fmt.Printf("Would delete %s: %d\n", strings.ToLower(name), s.count)
		default:
			fmt.Printf("%s deleted: %d, failed: %d\n", name, s.deleted, s.failed)
			totalDeleted += s.deleted
			totalFailed += s.failed
		}
	}

	printStat(clusterHandler.pluralName, clusterStats)
	printStat(networkHandler.pluralName, networkStats)
	printStat(rgHandler.pluralName, rgStats)

	if !statusOnly && !dryRun {
		fmt.Printf("\nTotal deleted: %d, failed: %d\n", totalDeleted, totalFailed)
		if totalFailed > 0 {
			fmt.Println("\n⚠ Some resources failed to delete. Review errors above.")
		}
		if totalDeleted > 0 {
			fmt.Println("\nNote: Deleted resources may take time to fully remove.")
		}
	}
}

func processResources[T any](h resourceHandler[T], statusOnly, dryRun bool) (stats, error) {
	var s stats

	if statusOnly {
		fmt.Printf("\n=== %s ===\n", h.pluralName)
	} else {
		fmt.Printf("\n=== Cleaning up %s ===\n", h.pluralName)
	}

	items, err := h.list()
	if err != nil {
		return s, fmt.Errorf("failed to list %s: %w", h.name, err)
	}

	for _, item := range items {
		id := h.getID(item)
		name := h.getName(item)

		switch {
		case statusOnly:
			h.display(item)
			s.count++
		case dryRun:
			fmt.Printf("Would delete %s: %s (ID: %s)\n", h.name, name, id)
			s.count++
		default:
			fmt.Printf("Deleting %s: %s (ID: %s)\n", h.name, name, id)
			if err := h.delete(id); err != nil {
				if isNotFound(err) {
					fmt.Println("  ⚠ Already deleted")
				} else {
					fmt.Printf("  ✗ Error: %v\n", err)
					s.failed++
				}
			} else {
				fmt.Println("  ✓ Deletion initiated")
				s.deleted++
			}
		}
	}

	return s, nil
}

func isNotFound(err error) bool {
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.NotFound
}
