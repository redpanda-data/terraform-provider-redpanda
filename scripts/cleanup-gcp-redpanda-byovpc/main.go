// Copyright 2026 Redpanda Data, Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"context"
	"encoding/base64"
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
	CommonPrefix   string
	ProjectID      string
	Region         string
	DryRun         bool
	AutoApprove    bool
	UseCredsBase64 bool
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
	NetworkEndpointGroup *compute.NetworkEndpointGroupsClient
	IAM                  *iam.Service
	Storage              *storage.Client
}

// customRoleLimit is GCP's per-project cap on custom roles. It is a hard limit
// and cannot be raised. A deleted role keeps its slot for 7 days before GCP
// permanently removes it, so a project that churns BYOC clusters can exhaust
// the cap even with an aggressive sweeper.
const customRoleLimit = 300

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

	cleanupCreds, err := setupGCPCredentials(cfg)
	if err != nil {
		fmt.Printf("%s Failed to setup GCP credentials: %v\n", red("ERROR:"), err)
		os.Exit(1)
	}
	defer cleanupCreds()

	ctx := context.Background()

	clients, err := initializeClients(ctx)
	if err != nil {
		fmt.Printf("%s Failed to initialize GCP clients: %v\n", red("ERROR:"), err)
		os.Exit(1)
	}
	defer closeClients(clients)

	resourceCount, err := listResources(ctx, clients, cfg)
	if err != nil {
		fmt.Printf("%s Failed to list resources: %v\n", red("ERROR:"), err)
		os.Exit(1)
	}

	if resourceCount == 0 {
		fmt.Printf("\n%s No matching resources found to delete\n", yellow("INFO:"))
		os.Exit(0)
	}

	if !cfg.DryRun && !cfg.AutoApprove && !isCI() {
		if !confirmDeletion(resourceCount) {
			fmt.Println(yellow("Deletion cancelled by user"))
			os.Exit(0)
		}
	} else if cfg.AutoApprove || isCI() {
		fmt.Printf("%s Auto-approved deletion, skipping confirmation\n", yellow("INFO:"))
	}

	fmt.Printf("\n%s Starting cleanup for Redpanda BYOVPC resources\n", cyan("INFO:"))
	fmt.Printf("  Common Prefix: %s\n", cfg.CommonPrefix)
	fmt.Printf("  Project ID: %s\n", cfg.ProjectID)
	fmt.Printf("  Region: %s\n", cfg.Region)
	fmt.Printf("  Dry Run: %v\n\n", cfg.DryRun)

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

	// must precede subnetwork and network deletion: a leftover NEG holds the VPC
	if err := deleteNetworkEndpointGroups(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete network endpoint groups: %v\n", red("ERROR:"), err)
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

	if err := deleteCustomRoles(clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete custom IAM roles: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteStorageBuckets(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete storage buckets: %v\n", red("ERROR:"), err)
		errorCount++
	}

	reportCustomRoleQuota(clients, cfg)

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
	flag.BoolVar(&cfg.AutoApprove, "auto-approve", false, "Skip confirmation prompt (use with caution)")
	flag.BoolVar(&cfg.UseCredsBase64, "use-gcp-creds-base64", false, "Use GOOGLE_CREDENTIALS_BASE64 env var for authentication")

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

	negClient, err := compute.NewNetworkEndpointGroupsRESTClient(ctx)
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
		NetworkEndpointGroup: negClient,
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
	if clients.NetworkEndpointGroup != nil {
		clients.NetworkEndpointGroup.Close()
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

	negReq := &computepb.AggregatedListNetworkEndpointGroupsRequest{
		Project: cfg.ProjectID,
	}
	negIt := clients.NetworkEndpointGroup.AggregatedList(ctx, negReq)
	for {
		pair, err := negIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("%s Failed to list network endpoint groups: %v\n", yellow("WARNING:"), err)
			break
		}
		for _, neg := range pair.Value.NetworkEndpointGroups {
			network := getNameFromURL(neg.GetNetwork())
			if matchesRedpandaResource(network, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Network Endpoint Group: %s (zone: %s, network: %s)\n",
					neg.GetName(), getZoneFromURL(neg.GetZone()), network)
			}
		}
	}

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

	customRoles, err := listCustomRoles(clients, cfg, false)
	if err != nil {
		fmt.Printf("%s Failed to list custom roles: %v\n", yellow("WARNING:"), err)
	} else {
		for _, role := range customRoles {
			roleID := roleIDFromName(role.Name)
			if matchesRedpandaRole(roleID, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Custom IAM Role: %s\n", roleID)
			}
		}
	}

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

func isCI() bool {
	ci := os.Getenv("CI")
	buildkite := os.Getenv("BUILDKITE")
	return ci == "true" || buildkite == "true"
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	notFoundPatterns := []string{
		"NotFound",
		"not found",
		"notFound",
		"404",
	}
	for _, pattern := range notFoundPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

func setupGCPCredentials(cfg *CleanupConfig) (func(), error) {
	if !cfg.UseCredsBase64 {
		return func() {}, nil
	}

	credsBase64 := os.Getenv("GOOGLE_CREDENTIALS_BASE64")
	if credsBase64 == "" {
		return nil, fmt.Errorf("--use-gcp-creds-base64 flag is set but GOOGLE_CREDENTIALS_BASE64 environment variable is not set")
	}

	credsJSON, err := base64.StdEncoding.DecodeString(credsBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GOOGLE_CREDENTIALS_BASE64: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "gcp-credentials-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary credentials file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(credsJSON); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write credentials to temporary file: %w", err)
	}
	tmpFile.Close()

	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to set GOOGLE_APPLICATION_CREDENTIALS: %w", err)
	}

	fmt.Printf("%s Using base64-encoded GCP credentials from GOOGLE_CREDENTIALS_BASE64\n", cyan("INFO:"))

	cleanup := func() {
		os.Remove(tmpPath)
	}

	return cleanup, nil
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

func getNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func matchesRedpandaResource(name, commonPrefix string) bool {
	if strings.Contains(strings.ToLower(name), "devex") {
		return false
	}
	return strings.HasPrefix(name, commonPrefix) || strings.HasPrefix(name, "rp-")
}

// customRolePrefixes are the role-ID prefixes the BYOC agent and the BYOVPC
// module create. Matching is case-insensitive because the agent mixes cases
// across families (RedpandaAgent* vs redpandaUtility*), and policyMaterializer*
// carries no redpanda prefix at all.
var customRolePrefixes = []string{
	"redpanda",
	"rp-",
	"rp_",
	"policymaterializer",
	"byovpc_",
}

// matchesRedpandaRole decides whether a custom role ID was created by Redpanda
// test infrastructure. Roles are project-global with no network or cluster to
// scope them, so the ID is the only signal available.
func matchesRedpandaRole(roleID, commonPrefix string) bool {
	lower := strings.ToLower(roleID)
	if strings.Contains(lower, "devex") {
		return false
	}
	if strings.HasPrefix(lower, strings.ToLower(commonPrefix)) {
		return true
	}
	for _, p := range customRolePrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func roleIDFromName(name string) string {
	parts := strings.Split(name, "/roles/")
	if len(parts) == 2 {
		return parts[1]
	}
	return name
}

// listCustomRoles returns every custom role in the project. showDeleted
// includes soft-deleted roles, which still consume the per-project limit.
func listCustomRoles(clients *GCPClients, cfg *CleanupConfig, showDeleted bool) ([]*iam.Role, error) {
	parent := fmt.Sprintf("projects/%s", cfg.ProjectID)
	var roles []*iam.Role
	pageToken := ""
	for {
		call := clients.IAM.Projects.Roles.List(parent).ShowDeleted(showDeleted).PageSize(1000)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, err
		}
		roles = append(roles, resp.Roles...)
		if resp.NextPageToken == "" {
			return roles, nil
		}
		pageToken = resp.NextPageToken
	}
}

// reportCustomRoleQuota surfaces the per-project custom-role headroom. Soft
// deleted roles are called out separately because deleting more roles will not
// reclaim their slots (only the 7-day purge does) and a project sitting at the
// cap fails cluster creation with a 429 that reads like a transient rate limit.
func reportCustomRoleQuota(clients *GCPClients, cfg *CleanupConfig) {
	all, err := listCustomRoles(clients, cfg, true)
	if err != nil {
		fmt.Printf("%s Failed to list custom roles for quota check: %v\n", yellow("WARNING:"), err)
		return
	}

	var active, softDeleted int
	for _, r := range all {
		if r.Deleted {
			softDeleted++
			continue
		}
		active++
	}
	total := active + softDeleted

	fmt.Printf("\n%s Custom role usage: %d/%d (active: %d, soft-deleted: %d)\n",
		cyan("INFO:"), total, customRoleLimit, active, softDeleted)

	if softDeleted > 0 {
		fmt.Printf("  %s %d soft-deleted role(s) still hold a slot; GCP purges them 7 days after deletion\n",
			yellow("NOTE:"), softDeleted)
	}
	if total >= customRoleLimit {
		fmt.Printf("  %s Project is AT the hard limit of %d. Cluster creation will fail with\n",
			red("ERROR:"), customRoleLimit)
		fmt.Print("         'Error 429: Maximum number of roles reached'. The limit cannot be raised;\n")
		fmt.Print("         wait out the 7-day purge or run BYOC tests in a different project.\n")
	} else if total >= customRoleLimit*9/10 {
		fmt.Printf("  %s Within 10%% of the hard limit of %d\n", yellow("WARNING:"), customRoleLimit)
	}
}

// deleteCustomRoles removes the per-cluster IAM roles the BYOC agent and BYOVPC
// module leave behind. Nothing else in this sweeper deletes them, and because
// they are project-scoped they survive every network and cluster teardown.
func deleteCustomRoles(clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting custom IAM roles...\n", cyan("INFO:"))

	roles, err := listCustomRoles(clients, cfg, false)
	if err != nil {
		return err
	}

	for _, role := range roles {
		roleID := roleIDFromName(role.Name)
		if !matchesRedpandaRole(roleID, cfg.CommonPrefix) {
			continue
		}
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete custom role: %s\n", roleID)
			continue
		}
		if _, err := clients.IAM.Projects.Roles.Delete(role.Name).Do(); err != nil {
			if isNotFoundError(err) {
				fmt.Printf("  %s Custom role already deleted: %s\n", green("✓"), roleID)
			} else {
				fmt.Printf("%s Failed to delete custom role %s: %v\n", yellow("WARNING:"), roleID, err)
			}
			continue
		}
		fmt.Printf("  %s Deleted custom role: %s\n", green("✓"), roleID)
	}

	return nil
}

// deleteNetworkEndpointGroups removes NEGs left behind by GKE ingress. They are
// matched by their attached network rather than their own name, which is a
// generated k8s2-* string with no redpanda prefix. A leftover NEG blocks VPC
// deletion with "already being used by ... networkEndpointGroups/...", so this
// must run before deleteSubnetworks and deleteNetworks.
func deleteNetworkEndpointGroups(ctx context.Context, clients *GCPClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting network endpoint groups...\n", cyan("INFO:"))

	req := &computepb.AggregatedListNetworkEndpointGroupsRequest{
		Project: cfg.ProjectID,
	}
	it := clients.NetworkEndpointGroup.AggregatedList(ctx, req)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			if isNotFoundError(err) {
				fmt.Printf("  %s No network endpoint groups found (already deleted)\n", green("✓"))
				break
			}
			return err
		}
		for _, neg := range pair.Value.NetworkEndpointGroups {
			network := getNameFromURL(neg.GetNetwork())
			if !matchesRedpandaResource(network, cfg.CommonPrefix) {
				continue
			}
			name := neg.GetName()
			zone := getZoneFromURL(neg.GetZone())
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete network endpoint group: %s (zone: %s, network: %s)\n", name, zone, network)
				continue
			}
			deleteReq := &computepb.DeleteNetworkEndpointGroupRequest{
				Project:              cfg.ProjectID,
				Zone:                 zone,
				NetworkEndpointGroup: name,
			}
			op, err := clients.NetworkEndpointGroup.Delete(ctx, deleteReq)
			if err != nil {
				if isNotFoundError(err) {
					fmt.Printf("  %s Network endpoint group already deleted: %s\n", green("✓"), name)
				} else {
					fmt.Printf("%s Failed to delete network endpoint group %s: %v\n", yellow("WARNING:"), name, err)
				}
				continue
			}
			if err := op.Wait(ctx); err != nil {
				fmt.Printf("%s Failed to wait for network endpoint group %s deletion: %v\n", yellow("WARNING:"), name, err)
				continue
			}
			fmt.Printf("  %s Deleted network endpoint group: %s (zone: %s)\n", green("✓"), name, zone)
		}
	}

	return nil
}

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
			if isNotFoundError(err) {
				fmt.Printf("  %s No instances found (already deleted)\n", green("✓"))
				break
			}
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
						if isNotFoundError(err) {
							fmt.Printf("  %s Instance already deleted: %s\n", green("✓"), name)
						} else {
							fmt.Printf("%s Failed to delete instance %s: %v\n", yellow("WARNING:"), name, err)
						}
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

func waitForGKEOperation(ctx context.Context, client *container.ClusterManagerClient, op *containerpb.Operation) error {
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

			if currentOp.Status == containerpb.Operation_DONE {
				if currentOp.Error != nil {
					return fmt.Errorf("operation failed: %s", currentOp.Error.Message)
				}
				return nil
			}
		}
	}
}

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
				if err := emptyBucket(ctx, clients.Storage, bucket.Name); err != nil {
					fmt.Printf("%s Failed to empty bucket %s: %v\n", yellow("WARNING:"), bucket.Name, err)
					continue
				}

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

func emptyBucket(ctx context.Context, client *storage.Client, bucketName string) error {
	bucket := client.Bucket(bucketName)

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

	const maxConcurrent = 50
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var deleteErrors []error
	deletedCount := 0

	for i, obj := range objects {
		wg.Add(1)
		sem <- struct{}{}

		go func(obj objectToDelete, index int) {
			defer wg.Done()
			defer func() { <-sem }()

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
				if deletedCount%100 == 0 {
					fmt.Printf("    Progress: %d/%d objects deleted\n", deletedCount, len(objects))
				}
				mu.Unlock()
			}
		}(obj, i)
	}

	wg.Wait()

	if len(deleteErrors) > 0 {
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
