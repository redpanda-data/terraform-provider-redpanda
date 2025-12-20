package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fatih/color"
)

type CleanupConfig struct {
	CommonPrefix  string
	Region        string
	VpcID         string
	DryRun        bool
	AutoApprove   bool
	AutoDetectVPC bool
	AccountID     string
	ListVPCs      bool
}

type AWSClients struct {
	EC2         *ec2.Client
	IAM         *iam.Client
	S3          *s3.Client
	DynamoDB    *dynamodb.Client
	STS         *sts.Client
	AutoScaling *autoscaling.Client
	EKS         *eks.Client
	ELB         *elasticloadbalancing.Client
	ELBV2       *elasticloadbalancingv2.Client
}

type VPCInfo struct {
	ID   string
	Name string
}

var (
	red    = color.New(color.FgRed).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
)

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func main() {
	cfg := parseFlags()
	ctx := context.Background()

	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		fmt.Printf("%s Failed to load AWS config: %v\n", red("ERROR:"), err)
		os.Exit(1)
	}

	clients := &AWSClients{
		EC2:         ec2.NewFromConfig(awsCfg),
		IAM:         iam.NewFromConfig(awsCfg),
		S3:          s3.NewFromConfig(awsCfg),
		DynamoDB:    dynamodb.NewFromConfig(awsCfg),
		STS:         sts.NewFromConfig(awsCfg),
		AutoScaling: autoscaling.NewFromConfig(awsCfg),
		EKS:         eks.NewFromConfig(awsCfg),
		ELB:         elasticloadbalancing.NewFromConfig(awsCfg),
		ELBV2:       elasticloadbalancingv2.NewFromConfig(awsCfg),
	}

	// Get AWS account ID if not provided
	if cfg.AccountID == "" {
		accountID, err := getAccountID(ctx, clients.STS)
		if err != nil {
			fmt.Printf("%s %v\n", red("ERROR:"), err)
			fmt.Printf("%s Please check your AWS credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)\n", red("ERROR:"))
			os.Exit(1)
		}
		cfg.AccountID = accountID
	}

	// Handle --list-vpcs flag
	if cfg.ListVPCs {
		fmt.Printf("\n%s Querying all AWS regions for non-default VPCs with '%s' prefix...\n", cyan("INFO:"), cfg.CommonPrefix)

		// Get all regions
		regions, err := getAllRegions(ctx, clients.EC2)
		if err != nil {
			fmt.Printf("%s Failed to get regions: %v\n", red("ERROR:"), err)
			os.Exit(1)
		}

		fmt.Printf("%s Scanning %d region(s)...\n\n", cyan("INFO:"), len(regions))

		// Query each region
		type regionVPCs struct {
			region string
			vpcs   []VPCInfo
		}
		var results []regionVPCs
		totalVPCs := 0

		for _, region := range regions {
			// Create region-specific EC2 client
			regionalCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				fmt.Printf("%s Failed to load config for region %s: %v\n", yellow("WARNING:"), region, err)
				continue
			}
			regionalEC2 := ec2.NewFromConfig(regionalCfg)

			vpcs, err := getNonDefaultVPCs(ctx, regionalEC2, region, cfg.CommonPrefix)
			if err != nil {
				fmt.Printf("%s Failed to list VPCs in region %s: %v\n", yellow("WARNING:"), region, err)
				continue
			}

			if len(vpcs) > 0 {
				results = append(results, regionVPCs{region: region, vpcs: vpcs})
				totalVPCs += len(vpcs)
			}
		}

		// Display results grouped by region
		if totalVPCs == 0 {
			fmt.Printf("%s No non-default VPCs found with '%s' prefix in any region\n", yellow("INFO:"), cfg.CommonPrefix)
			os.Exit(0)
		}

		fmt.Printf("%s Found %d non-default VPC(s) with '%s' prefix across %d region(s):\n\n", green("SUCCESS:"), totalVPCs, cfg.CommonPrefix, len(results))

		for _, result := range results {
			fmt.Printf("%s Region: %s (%d VPC%s)\n", cyan("→"), result.region, len(result.vpcs), pluralize(len(result.vpcs)))
			for _, vpc := range result.vpcs {
				fmt.Printf("  - VPC ID: %s\n", vpc.ID)
				fmt.Printf("    Name:   %s\n", vpc.Name)
			}
			fmt.Println()
		}

		os.Exit(0)
	}

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

	// Confirm deletion (unless dry-run or auto-approved)
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
	fmt.Printf("  Region: %s\n", cfg.Region)
	fmt.Printf("  VPC ID: %s\n", cfg.VpcID)
	fmt.Printf("  Dry Run: %v\n\n", cfg.DryRun)

	// Delete resources in dependency order
	var errorCount int

	if err := deleteEKSClusters(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete EKS clusters: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteAutoScalingGroups(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete Auto Scaling Groups: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteEC2Instances(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete EC2 instances: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteLoadBalancers(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete Load Balancers: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteTargetGroups(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete Target Groups: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteLaunchTemplates(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete Launch Templates: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteIAMResources(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete IAM resources: %v\n", red("ERROR:"), err)
		errorCount++
	}

	// Handle VPC cleanup - either single VPC or auto-detected VPCs
	var vpcsToClean []string

	if cfg.VpcID != "" {
		// Single VPC specified explicitly
		vpcsToClean = []string{cfg.VpcID}
	} else if cfg.AutoDetectVPC {
		// Auto-detect VPCs matching prefix
		fmt.Printf("%s Auto-detecting VPCs with prefix '%s'...\n", cyan("INFO:"), cfg.CommonPrefix)
		detectedVPCs, err := detectVPCsByPrefix(ctx, clients.EC2, cfg.CommonPrefix)
		if err != nil {
			fmt.Printf("%s Failed to auto-detect VPCs: %v\n", red("ERROR:"), err)
			errorCount++
		} else if len(detectedVPCs) == 0 {
			fmt.Printf("%s No VPCs detected with prefix '%s'\n", yellow("INFO:"), cfg.CommonPrefix)
		} else {
			fmt.Printf("%s Found %d VPC(s) to clean up\n", green("INFO:"), len(detectedVPCs))
			vpcsToClean = detectedVPCs
		}
	}

	// Clean up VPCs (if any)
	for _, vpcID := range vpcsToClean {
		// Create temporary config with this VPC ID
		vpcCfg := *cfg
		vpcCfg.VpcID = vpcID

		fmt.Printf("\n%s Cleaning up VPC: %s\n", cyan("═══"), vpcID)

		if err := deleteVPCEndpoints(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete VPC endpoints for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		// Delete load balancers and target groups first - they create security groups and ENIs
		if err := deleteVPCLoadBalancers(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete load balancers for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		// Also delete Classic Load Balancers (ELBv1) - Kubernetes sometimes creates these
		if err := deleteVPCClassicLoadBalancers(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete classic load balancers for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteVPCTargetGroups(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete target groups for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		// Delete network interfaces BEFORE security groups (ENIs reference SGs)
		if err := deleteNetworkInterfaces(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete network interfaces for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteSecurityGroups(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete security groups for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteNATGateways(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete NAT gateways for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteElasticIPs(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete Elastic IPs for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteRouteTables(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete route tables for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteSubnets(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete subnets for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteNetworkInterfaces(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete network interfaces for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteInternetGatewaysWithRetry(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete internet gateways for VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		if err := deleteVPC(ctx, clients, &vpcCfg); err != nil {
			fmt.Printf("%s Failed to delete VPC %s: %v\n", red("ERROR:"), vpcID, err)
			errorCount++
		}

		fmt.Printf("%s Completed cleanup for VPC: %s\n", cyan("═══"), vpcID)
	}

	if err := deleteStorageResources(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete storage resources: %v\n", red("ERROR:"), err)
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
	flag.StringVar(&cfg.Region, "region", "us-east-1", "AWS region")
	flag.StringVar(&cfg.VpcID, "vpc-id", "", "VPC ID to delete (optional)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Preview actions without deleting")
	flag.BoolVar(&cfg.AutoApprove, "auto-approve", false, "Skip confirmation prompt (use with caution)")
	flag.BoolVar(&cfg.AutoDetectVPC, "auto-detect-vpc", false, "Auto-detect and cleanup all VPCs matching common prefix")
	flag.StringVar(&cfg.AccountID, "account-id", "", "AWS account ID (auto-detected if not provided)")
	flag.BoolVar(&cfg.ListVPCs, "list-vpcs", false, "List non-default VPCs with 'network-' prefix and exit")

	flag.Parse()

	return cfg
}

func listResources(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) (int, error) {
	fmt.Printf("\n%s Scanning for resources to delete...\n", cyan("INFO:"))
	fmt.Printf("  Common Prefix: %s\n", cfg.CommonPrefix)
	fmt.Printf("  Region: %s\n", cfg.Region)
	fmt.Printf("  VPC ID: %s\n\n", cfg.VpcID)

	totalCount := 0

	// List EKS clusters
	eksResult, err := clients.EKS.ListClusters(ctx, &eks.ListClustersInput{})
	if err == nil {
		for _, clusterName := range eksResult.Clusters {
			if strings.HasPrefix(clusterName, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - EKS Cluster: %s\n", clusterName)
			}
		}
	}

	// List Auto Scaling Groups
	asgResult, err := clients.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err == nil {
		for _, asg := range asgResult.AutoScalingGroups {
			asgName := aws.ToString(asg.AutoScalingGroupName)
			if strings.HasPrefix(asgName, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Auto Scaling Group: %s (%d instances)\n", asgName, len(asg.Instances))
			}
		}
	}

	// List EC2 instances
	ec2Result, err := clients.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{fmt.Sprintf("%s*", cfg.CommonPrefix)},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"pending", "running", "stopping", "stopped"},
			},
		},
	})
	if err == nil {
		for _, reservation := range ec2Result.Reservations {
			for _, instance := range reservation.Instances {
				totalCount++
				var name string
				for _, tag := range instance.Tags {
					if aws.ToString(tag.Key) == "Name" {
						name = aws.ToString(tag.Value)
						break
					}
				}
				fmt.Printf("  - EC2 Instance: %s (%s)\n", name, aws.ToString(instance.InstanceId))
			}
		}
	}

	// List Load Balancers
	lbResult, err := clients.ELBV2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	if err == nil {
		for _, lb := range lbResult.LoadBalancers {
			lbName := aws.ToString(lb.LoadBalancerName)
			if strings.HasPrefix(lbName, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Load Balancer: %s\n", lbName)
			}
		}
	}

	// List Target Groups
	tgResult, err := clients.ELBV2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	if err == nil {
		for _, tg := range tgResult.TargetGroups {
			tgName := aws.ToString(tg.TargetGroupName)
			if strings.HasPrefix(tgName, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Target Group: %s\n", tgName)
			}
		}
	}

	// List Launch Templates
	ltResult, err := clients.EC2.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
	if err == nil {
		for _, lt := range ltResult.LaunchTemplates {
			ltName := aws.ToString(lt.LaunchTemplateName)
			if strings.HasPrefix(ltName, cfg.CommonPrefix) {
				totalCount++
				fmt.Printf("  - Launch Template: %s\n", ltName)
			}
		}
	}

	// List IAM resources
	instanceProfilePrefixes := []string{
		fmt.Sprintf("%s-connectors-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-utility-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-agent-", cfg.CommonPrefix),
	}

	instanceProfileCount := 0
	profileResult, err := clients.IAM.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{})
	if err == nil {
		for _, profile := range profileResult.InstanceProfiles {
			for _, prefix := range instanceProfilePrefixes {
				if strings.HasPrefix(aws.ToString(profile.InstanceProfileName), prefix) {
					instanceProfileCount++
					fmt.Printf("  - IAM Instance Profile: %s\n", aws.ToString(profile.InstanceProfileName))
					break
				}
			}
		}
	}
	totalCount += instanceProfileCount

	rolePrefixes := []string{
		fmt.Sprintf("%s-cluster-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-agent-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-rpk-user-", cfg.CommonPrefix),
		fmt.Sprintf("%s-connectors-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-utility-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-connect-node-group-", cfg.CommonPrefix),
	}

	roleCount := 0
	roleResult, err := clients.IAM.ListRoles(ctx, &iam.ListRolesInput{})
	if err == nil {
		for _, role := range roleResult.Roles {
			for _, prefix := range rolePrefixes {
				if strings.HasPrefix(aws.ToString(role.RoleName), prefix) {
					roleCount++
					fmt.Printf("  - IAM Role: %s\n", aws.ToString(role.RoleName))
					break
				}
			}
		}
	}
	totalCount += roleCount

	// Determine which VPCs to list resources for
	var vpcsToList []string
	if cfg.VpcID != "" {
		vpcsToList = []string{cfg.VpcID}
	} else if cfg.AutoDetectVPC {
		// Auto-detect VPCs matching prefix
		detected, err := detectVPCsByPrefix(ctx, clients.EC2, cfg.CommonPrefix)
		if err != nil {
			fmt.Printf("%s Failed to auto-detect VPCs for listing: %v\n", yellow("WARNING:"), err)
		} else {
			vpcsToList = detected
		}
	}

	// List VPC resources for each VPC
	for _, vpcID := range vpcsToList {
		totalCount++ // Count the VPC itself
		fmt.Printf("  - VPC: %s\n", vpcID)

		// VPC Endpoints
		vpcEndpointResult, err := clients.EC2.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			},
		})
		if err == nil {
			for _, endpoint := range vpcEndpointResult.VpcEndpoints {
				totalCount++
				fmt.Printf("    - VPC Endpoint: %s\n", aws.ToString(endpoint.VpcEndpointId))
			}
		}

		// Load Balancers in this VPC (includes k8s-created load balancers)
		if lbResult != nil {
			for _, lb := range lbResult.LoadBalancers {
				if aws.ToString(lb.VpcId) == vpcID {
					lbName := aws.ToString(lb.LoadBalancerName)
					// Only count if not already counted by prefix match above
					if !strings.HasPrefix(lbName, cfg.CommonPrefix) {
						totalCount++
						fmt.Printf("    - Load Balancer: %s\n", lbName)
					}
				}
			}
		}

		// Target Groups in this VPC (includes k8s-created target groups)
		if tgResult != nil {
			for _, tg := range tgResult.TargetGroups {
				if aws.ToString(tg.VpcId) == vpcID {
					tgName := aws.ToString(tg.TargetGroupName)
					// Only count if not already counted by prefix match above
					if !strings.HasPrefix(tgName, cfg.CommonPrefix) {
						totalCount++
						fmt.Printf("    - Target Group: %s\n", tgName)
					}
				}
			}
		}

		// Classic Load Balancers in this VPC
		classicLBResult, err := clients.ELB.DescribeLoadBalancers(ctx, &elasticloadbalancing.DescribeLoadBalancersInput{})
		if err == nil {
			for _, lb := range classicLBResult.LoadBalancerDescriptions {
				if aws.ToString(lb.VPCId) == vpcID {
					totalCount++
					fmt.Printf("    - Classic Load Balancer: %s\n", aws.ToString(lb.LoadBalancerName))
				}
			}
		}

		// Security Groups
		sgResult, err := clients.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			},
		})
		if err == nil {
			for _, sg := range sgResult.SecurityGroups {
				name := aws.ToString(sg.GroupName)
				if name != "default" {
					totalCount++
					fmt.Printf("    - Security Group: %s (%s)\n", name, aws.ToString(sg.GroupId))
				}
			}
		}

		// NAT Gateways
		natResult, err := clients.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
			Filter: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			},
		})
		if err == nil {
			for _, natGw := range natResult.NatGateways {
				totalCount++
				fmt.Printf("    - NAT Gateway: %s\n", aws.ToString(natGw.NatGatewayId))
			}
		}

		// Route Tables
		rtResult, err := clients.EC2.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			},
		})
		if err == nil {
			for _, rt := range rtResult.RouteTables {
				isMain := false
				for _, assoc := range rt.Associations {
					if aws.ToBool(assoc.Main) {
						isMain = true
						break
					}
				}
				if !isMain {
					totalCount++
					fmt.Printf("    - Route Table: %s\n", aws.ToString(rt.RouteTableId))
				}
			}
		}

		// Subnets
		subnetResult, err := clients.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			},
		})
		if err == nil {
			for _, subnet := range subnetResult.Subnets {
				totalCount++
				fmt.Printf("    - Subnet: %s\n", aws.ToString(subnet.SubnetId))
			}
		}

		// Internet Gateways
		igwResult, err := clients.EC2.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
			Filters: []types.Filter{
				{Name: aws.String("attachment.vpc-id"), Values: []string{vpcID}},
			},
		})
		if err == nil {
			for _, igw := range igwResult.InternetGateways {
				totalCount++
				fmt.Printf("    - Internet Gateway: %s\n", aws.ToString(igw.InternetGatewayId))
			}
		}
	}

	// List S3 buckets
	bucketPrefixes := []string{
		"redpanda-cloud-storage-",
		fmt.Sprintf("rp-%s-%s-mgmt-", cfg.AccountID, cfg.Region),
	}

	listResult, err := clients.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err == nil {
		for _, bucket := range listResult.Buckets {
			bucketName := aws.ToString(bucket.Name)
			for _, prefix := range bucketPrefixes {
				if strings.HasPrefix(bucketName, prefix) {
					totalCount++
					fmt.Printf("  - S3 Bucket: %s\n", bucketName)
					break
				}
			}
		}
	}

	// List DynamoDB tables
	tablePrefix := fmt.Sprintf("rp-%s-%s-mgmt-tflock-", cfg.AccountID, cfg.Region)
	listTablesResult, err := clients.DynamoDB.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err == nil {
		for _, tableName := range listTablesResult.TableNames {
			if strings.HasPrefix(tableName, tablePrefix) {
				totalCount++
				fmt.Printf("  - DynamoDB Table: %s\n", tableName)
			}
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

// isNotFoundError checks if an error is a "not found" type error
// These errors indicate the resource is already deleted, which is the desired state
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common AWS "not found" error patterns
	notFoundPatterns := []string{
		"NotFound",
		"does not exist",
		"DoesNotExist",
		"NoSuch",
		"not found",
	}
	for _, pattern := range notFoundPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

func getAccountID(ctx context.Context, stsClient *sts.Client) (string, error) {
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("authentication failed - unable to get AWS account ID: %w", err)
	}

	return aws.ToString(result.Account), nil
}

// deleteEKSClusters deletes EKS clusters and their nodegroups
func deleteEKSClusters(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting EKS clusters...\n", cyan("INFO:"))

	result, err := clients.EKS.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return err
	}

	for _, clusterName := range result.Clusters {
		if strings.HasPrefix(clusterName, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete EKS cluster: %s\n", clusterName)
			} else {
				// First, list and delete all nodegroups
				nodegroups, err := clients.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
					ClusterName: aws.String(clusterName),
				})
				if err != nil {
					fmt.Printf("%s Failed to list nodegroups for cluster %s: %v\n", yellow("WARNING:"), clusterName, err)
				} else {
					// Delete each nodegroup
					for _, nodegroupName := range nodegroups.Nodegroups {
						fmt.Printf("  %s Deleting nodegroup: %s (cluster: %s)\n", cyan("INFO:"), nodegroupName, clusterName)
						_, err := clients.EKS.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{
							ClusterName:   aws.String(clusterName),
							NodegroupName: aws.String(nodegroupName),
						})
						if err != nil {
							fmt.Printf("%s Failed to delete nodegroup %s: %v\n", yellow("WARNING:"), nodegroupName, err)
						} else {
							fmt.Printf("  %s Deleted nodegroup: %s\n", green("✓"), nodegroupName)
						}
					}

					// Wait for nodegroups to be deleted
					if len(nodegroups.Nodegroups) > 0 {
						fmt.Printf("  %s Waiting for nodegroups to delete for cluster %s...\n", yellow("WAIT:"), clusterName)
						time.Sleep(60 * time.Second) // Wait 60 seconds for nodegroups to start deleting

						// Poll until all nodegroups are deleted (max 10 minutes)
						maxWaitTime := 10 * time.Minute
						pollInterval := 30 * time.Second
						startTime := time.Now()

						for time.Since(startTime) < maxWaitTime {
							remaining, err := clients.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
								ClusterName: aws.String(clusterName),
							})
							if err != nil || len(remaining.Nodegroups) == 0 {
								fmt.Printf("  %s All nodegroups deleted for cluster %s\n", green("✓"), clusterName)
								break
							}
							fmt.Printf("  %s Still waiting for %d nodegroup(s) to delete...\n", yellow("WAIT:"), len(remaining.Nodegroups))
							time.Sleep(pollInterval)
						}
					}
				}

				// Now delete the cluster
				_, err = clients.EKS.DeleteCluster(ctx, &eks.DeleteClusterInput{
					Name: aws.String(clusterName),
				})
				if err != nil {
					fmt.Printf("%s Failed to delete EKS cluster %s: %v\n", yellow("WARNING:"), clusterName, err)
				} else {
					fmt.Printf("  %s Deleted EKS cluster: %s\n", green("✓"), clusterName)
				}
			}
		}
	}

	return nil
}

// deleteAutoScalingGroups deletes Auto Scaling Groups
func deleteAutoScalingGroups(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Auto Scaling Groups...\n", cyan("INFO:"))

	result, err := clients.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return err
	}

	for _, asg := range result.AutoScalingGroups {
		asgName := aws.ToString(asg.AutoScalingGroupName)
		if strings.HasPrefix(asgName, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete Auto Scaling Group: %s\n", asgName)
			} else {
				// Delete ASG with ForceDelete to terminate instances
				_, err := clients.AutoScaling.DeleteAutoScalingGroup(ctx, &autoscaling.DeleteAutoScalingGroupInput{
					AutoScalingGroupName: asg.AutoScalingGroupName,
					ForceDelete:          aws.Bool(true),
				})
				if err != nil {
					fmt.Printf("%s Failed to delete Auto Scaling Group %s: %v\n", yellow("WARNING:"), asgName, err)
				} else {
					fmt.Printf("  %s Deleted Auto Scaling Group: %s\n", green("✓"), asgName)
				}
			}
		}
	}

	// Wait for ASGs to finish terminating instances
	if !cfg.DryRun && len(result.AutoScalingGroups) > 0 {
		fmt.Printf("  %s Waiting for Auto Scaling Groups to delete...\n", yellow("WAIT:"))
		time.Sleep(30 * time.Second)
	}

	return nil
}

// deleteEC2Instances deletes EC2 instances
func deleteEC2Instances(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting EC2 instances...\n", cyan("INFO:"))

	result, err := clients.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{fmt.Sprintf("%s*", cfg.CommonPrefix)},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"pending", "running", "stopping", "stopped"},
			},
		},
	})
	if err != nil {
		return err
	}

	var instanceIDs []string
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceIDs = append(instanceIDs, aws.ToString(instance.InstanceId))
		}
	}

	if len(instanceIDs) > 0 {
		if cfg.DryRun {
			for _, id := range instanceIDs {
				fmt.Printf("  [DRY RUN] Would terminate EC2 instance: %s\n", id)
			}
		} else {
			_, err := clients.EC2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
				InstanceIds: instanceIDs,
			})
			if err != nil {
				fmt.Printf("%s Failed to terminate instances: %v\n", yellow("WARNING:"), err)
			} else {
				for _, id := range instanceIDs {
					fmt.Printf("  %s Terminated EC2 instance: %s\n", green("✓"), id)
				}
			}

			// Wait for instances to terminate
			fmt.Printf("  %s Waiting for EC2 instances to terminate...\n", yellow("WAIT:"))
			time.Sleep(30 * time.Second)
		}
	}

	return nil
}

// deleteLoadBalancers deletes Application and Network Load Balancers
func deleteLoadBalancers(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Load Balancers...\n", cyan("INFO:"))

	result, err := clients.ELBV2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	if err != nil {
		return err
	}

	for _, lb := range result.LoadBalancers {
		lbName := aws.ToString(lb.LoadBalancerName)
		if strings.HasPrefix(lbName, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete Load Balancer: %s\n", lbName)
			} else {
				_, err := clients.ELBV2.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
					LoadBalancerArn: lb.LoadBalancerArn,
				})
				if err != nil {
					fmt.Printf("%s Failed to delete Load Balancer %s: %v\n", yellow("WARNING:"), lbName, err)
				} else {
					fmt.Printf("  %s Deleted Load Balancer: %s\n", green("✓"), lbName)
				}
			}
		}
	}

	// Wait for load balancers to be deleted
	if !cfg.DryRun && len(result.LoadBalancers) > 0 {
		fmt.Printf("  %s Waiting for Load Balancers to delete...\n", yellow("WAIT:"))
		time.Sleep(10 * time.Second)
	}

	return nil
}

// deleteVPCLoadBalancers deletes all load balancers in a specific VPC
// This is used for VPC cleanup where we want to delete everything in the VPC,
// regardless of name prefix (the VPC was already identified by prefix)
func deleteVPCLoadBalancers(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	if cfg.VpcID == "" {
		return nil
	}

	fmt.Printf("%s Deleting load balancers in VPC...\n", cyan("INFO:"))

	result, err := clients.ELBV2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	if err != nil {
		return err
	}

	var lbsToDelete []elbv2types.LoadBalancer
	for _, lb := range result.LoadBalancers {
		if aws.ToString(lb.VpcId) == cfg.VpcID {
			lbsToDelete = append(lbsToDelete, lb)
		}
	}

	if len(lbsToDelete) == 0 {
		return nil
	}

	for _, lb := range lbsToDelete {
		lbName := aws.ToString(lb.LoadBalancerName)
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete Load Balancer: %s\n", lbName)
		} else {
			_, err := clients.ELBV2.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
				LoadBalancerArn: lb.LoadBalancerArn,
			})
			if err != nil {
				fmt.Printf("%s Failed to delete Load Balancer %s: %v\n", yellow("WARNING:"), lbName, err)
			} else {
				fmt.Printf("  %s Deleted Load Balancer: %s\n", green("✓"), lbName)
			}
		}
	}

	// Wait for load balancers to be fully deleted (AWS needs time to release ENIs)
	if !cfg.DryRun {
		fmt.Printf("  %s Waiting for Load Balancers to be fully deleted...\n", yellow("WAIT:"))
		time.Sleep(30 * time.Second)
	}

	return nil
}

// deleteVPCClassicLoadBalancers deletes all Classic Load Balancers in a specific VPC
// This handles ELBs created by Kubernetes that use the older Classic LB type
func deleteVPCClassicLoadBalancers(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	if cfg.VpcID == "" {
		return nil
	}

	fmt.Printf("%s Deleting classic load balancers in VPC...\n", cyan("INFO:"))

	result, err := clients.ELB.DescribeLoadBalancers(ctx, &elasticloadbalancing.DescribeLoadBalancersInput{})
	if err != nil {
		return err
	}

	var lbsToDelete []string
	for _, lb := range result.LoadBalancerDescriptions {
		if aws.ToString(lb.VPCId) == cfg.VpcID {
			lbsToDelete = append(lbsToDelete, aws.ToString(lb.LoadBalancerName))
		}
	}

	if len(lbsToDelete) == 0 {
		return nil
	}

	for _, lbName := range lbsToDelete {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete Classic Load Balancer: %s\n", lbName)
		} else {
			_, err := clients.ELB.DeleteLoadBalancer(ctx, &elasticloadbalancing.DeleteLoadBalancerInput{
				LoadBalancerName: aws.String(lbName),
			})
			if err != nil {
				fmt.Printf("%s Failed to delete Classic Load Balancer %s: %v\n", yellow("WARNING:"), lbName, err)
			} else {
				fmt.Printf("  %s Deleted Classic Load Balancer: %s\n", green("✓"), lbName)
			}
		}
	}

	// Wait for classic load balancers to be deleted
	if !cfg.DryRun {
		fmt.Printf("  %s Waiting for Classic Load Balancers to be deleted...\n", yellow("WAIT:"))
		time.Sleep(30 * time.Second)
	}

	return nil
}

// deleteVPCTargetGroups deletes all target groups in a specific VPC
// This is used for VPC cleanup where we want to delete everything in the VPC,
// regardless of name prefix (the VPC was already identified by prefix)
func deleteVPCTargetGroups(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	if cfg.VpcID == "" {
		return nil
	}

	fmt.Printf("%s Deleting target groups in VPC...\n", cyan("INFO:"))

	result, err := clients.ELBV2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	if err != nil {
		return err
	}

	for _, tg := range result.TargetGroups {
		if aws.ToString(tg.VpcId) == cfg.VpcID {
			tgName := aws.ToString(tg.TargetGroupName)
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete Target Group: %s\n", tgName)
			} else {
				_, err := clients.ELBV2.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
					TargetGroupArn: tg.TargetGroupArn,
				})
				if err != nil {
					fmt.Printf("%s Failed to delete Target Group %s: %v\n", yellow("WARNING:"), tgName, err)
				} else {
					fmt.Printf("  %s Deleted Target Group: %s\n", green("✓"), tgName)
				}
			}
		}
	}

	return nil
}

// deleteTargetGroups deletes Target Groups
func deleteTargetGroups(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Target Groups...\n", cyan("INFO:"))

	result, err := clients.ELBV2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	if err != nil {
		return err
	}

	for _, tg := range result.TargetGroups {
		tgName := aws.ToString(tg.TargetGroupName)
		if strings.HasPrefix(tgName, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete Target Group: %s\n", tgName)
			} else {
				_, err := clients.ELBV2.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
					TargetGroupArn: tg.TargetGroupArn,
				})
				if err != nil {
					fmt.Printf("%s Failed to delete Target Group %s: %v\n", yellow("WARNING:"), tgName, err)
				} else {
					fmt.Printf("  %s Deleted Target Group: %s\n", green("✓"), tgName)
				}
			}
		}
	}

	return nil
}

// deleteLaunchTemplates deletes Launch Templates
func deleteLaunchTemplates(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Launch Templates...\n", cyan("INFO:"))

	result, err := clients.EC2.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
	if err != nil {
		return err
	}

	for _, lt := range result.LaunchTemplates {
		ltName := aws.ToString(lt.LaunchTemplateName)
		if strings.HasPrefix(ltName, cfg.CommonPrefix) {
			if cfg.DryRun {
				fmt.Printf("  [DRY RUN] Would delete Launch Template: %s\n", ltName)
			} else {
				_, err := clients.EC2.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
					LaunchTemplateId: lt.LaunchTemplateId,
				})
				if err != nil {
					fmt.Printf("%s Failed to delete Launch Template %s: %v\n", yellow("WARNING:"), ltName, err)
				} else {
					fmt.Printf("  %s Deleted Launch Template: %s\n", green("✓"), ltName)
				}
			}
		}
	}

	return nil
}

// deleteIAMResources deletes IAM instance profiles, policies, and roles
func deleteIAMResources(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting IAM resources...\n", cyan("INFO:"))

	// List and delete instance profiles
	instanceProfiles := []string{
		fmt.Sprintf("%s-connectors-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-utility-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-agent-", cfg.CommonPrefix),
	}

	for _, profilePrefix := range instanceProfiles {
		if err := deleteInstanceProfilesByPrefix(ctx, clients.IAM, profilePrefix, cfg.DryRun); err != nil {
			fmt.Printf("%s Failed to delete instance profile %s: %v\n", yellow("WARNING:"), profilePrefix, err)
		}
	}

	// List and delete IAM roles
	rolePrefixes := []string{
		fmt.Sprintf("%s-cluster-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-agent-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-rpk-user-", cfg.CommonPrefix),
		fmt.Sprintf("%s-connectors-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-utility-node-group-", cfg.CommonPrefix),
		fmt.Sprintf("%s-redpanda-connect-node-group-", cfg.CommonPrefix),
	}

	for _, rolePrefix := range rolePrefixes {
		if err := deleteRolesByPrefix(ctx, clients.IAM, rolePrefix, cfg.DryRun); err != nil {
			fmt.Printf("%s Failed to delete role %s: %v\n", yellow("WARNING:"), rolePrefix, err)
		}
	}

	return nil
}

func deleteInstanceProfilesByPrefix(ctx context.Context, iamClient *iam.Client, prefix string, dryRun bool) error {
	result, err := iamClient.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{})
	if err != nil {
		return err
	}

	for _, profile := range result.InstanceProfiles {
		if strings.HasPrefix(aws.ToString(profile.InstanceProfileName), prefix) {
			if dryRun {
				fmt.Printf("  [DRY RUN] Would delete instance profile: %s\n", aws.ToString(profile.InstanceProfileName))
			} else {
				// Remove roles from instance profile first
				for _, role := range profile.Roles {
					_, err := iamClient.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
						InstanceProfileName: profile.InstanceProfileName,
						RoleName:            role.RoleName,
					})
					if err != nil {
						fmt.Printf("%s Failed to remove role from instance profile: %v\n", yellow("WARNING:"), err)
					}
				}

				_, err := iamClient.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{
					InstanceProfileName: profile.InstanceProfileName,
				})
				if err != nil {
					return err
				}
				fmt.Printf("  %s Deleted instance profile: %s\n", green("✓"), aws.ToString(profile.InstanceProfileName))
			}
		}
	}

	return nil
}

func deleteRolesByPrefix(ctx context.Context, iamClient *iam.Client, prefix string, dryRun bool) error {
	result, err := iamClient.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		return err
	}

	for _, role := range result.Roles {
		if strings.HasPrefix(aws.ToString(role.RoleName), prefix) {
			if dryRun {
				fmt.Printf("  [DRY RUN] Would delete role: %s\n", aws.ToString(role.RoleName))
			} else {
				// Detach all managed policies
				policies, err := iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
					RoleName: role.RoleName,
				})
				if err == nil {
					for _, policy := range policies.AttachedPolicies {
						_, err := iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
							RoleName:  role.RoleName,
							PolicyArn: policy.PolicyArn,
						})
						if err != nil {
							fmt.Printf("%s Failed to detach policy: %v\n", yellow("WARNING:"), err)
						}
					}
				}

				// Delete inline policies
				inlinePolicies, err := iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
					RoleName: role.RoleName,
				})
				if err == nil {
					for _, policyName := range inlinePolicies.PolicyNames {
						_, err := iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
							RoleName:   role.RoleName,
							PolicyName: aws.String(policyName),
						})
						if err != nil {
							fmt.Printf("%s Failed to delete inline policy: %v\n", yellow("WARNING:"), err)
						}
					}
				}

				_, err = iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
					RoleName: role.RoleName,
				})
				if err != nil {
					return err
				}
				fmt.Printf("  %s Deleted role: %s\n", green("✓"), aws.ToString(role.RoleName))
			}
		}
	}

	return nil
}

// deleteVPCEndpoints deletes VPC endpoints
func deleteVPCEndpoints(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting VPC endpoints...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping VPC endpoints (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeVpcEndpointsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeVpcEndpoints(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No VPC endpoints found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	for _, endpoint := range result.VpcEndpoints {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete VPC endpoint: %s\n", aws.ToString(endpoint.VpcEndpointId))
		} else {
			_, err := clients.EC2.DeleteVpcEndpoints(ctx, &ec2.DeleteVpcEndpointsInput{
				VpcEndpointIds: []string{aws.ToString(endpoint.VpcEndpointId)},
			})
			if err != nil {
				fmt.Printf("%s Failed to delete VPC endpoint %s: %v\n", yellow("WARNING:"), aws.ToString(endpoint.VpcEndpointId), err)
			} else {
				fmt.Printf("  %s Deleted VPC endpoint: %s\n", green("✓"), aws.ToString(endpoint.VpcEndpointId))
			}
		}
	}

	return nil
}

// deleteSecurityGroups deletes security groups
func deleteSecurityGroups(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting security groups...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping security groups (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeSecurityGroups(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No security groups found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	// Delete all non-default security groups in this VPC
	// (The VPC was already identified by prefix - clean up everything inside it)
	var sgToDelete []types.SecurityGroup
	for _, sg := range result.SecurityGroups {
		name := aws.ToString(sg.GroupName)
		if name != "default" {
			sgToDelete = append(sgToDelete, sg)
		}
	}

	// First pass: remove all ingress/egress rules
	for _, sg := range sgToDelete {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would revoke rules for security group: %s (%s)\n", aws.ToString(sg.GroupName), aws.ToString(sg.GroupId))
		} else {
			// Revoke ingress rules
			if len(sg.IpPermissions) > 0 {
				_, err := clients.EC2.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
					GroupId:       sg.GroupId,
					IpPermissions: sg.IpPermissions,
				})
				if err != nil {
					fmt.Printf("%s Failed to revoke ingress rules: %v\n", yellow("WARNING:"), err)
				}
			}

			// Revoke egress rules
			if len(sg.IpPermissionsEgress) > 0 {
				_, err := clients.EC2.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
					GroupId:       sg.GroupId,
					IpPermissions: sg.IpPermissionsEgress,
				})
				if err != nil {
					fmt.Printf("%s Failed to revoke egress rules: %v\n", yellow("WARNING:"), err)
				}
			}
		}
	}

	// Second pass: delete security groups
	for _, sg := range sgToDelete {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete security group: %s (%s)\n", aws.ToString(sg.GroupName), aws.ToString(sg.GroupId))
		} else {
			_, err := clients.EC2.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
				GroupId: sg.GroupId,
			})
			if err != nil {
				fmt.Printf("%s Failed to delete security group %s: %v\n", yellow("WARNING:"), aws.ToString(sg.GroupName), err)
			} else {
				fmt.Printf("  %s Deleted security group: %s (%s)\n", green("✓"), aws.ToString(sg.GroupName), aws.ToString(sg.GroupId))
			}
		}
	}

	return nil
}

// deleteElasticIPs finds and releases all Elastic IPs associated with the VPC
func deleteElasticIPs(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting Elastic IPs...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping Elastic IPs (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	// Get all addresses
	addressResult, err := clients.EC2.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return err
	}

	// Filter to addresses associated with network interfaces in our VPC
	for _, addr := range addressResult.Addresses {
		// Skip if not associated with a network interface
		if addr.NetworkInterfaceId == nil {
			continue
		}

		// Check if the network interface belongs to our VPC
		eniResult, err := clients.EC2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{aws.ToString(addr.NetworkInterfaceId)},
		})
		if err != nil {
			continue
		}

		if len(eniResult.NetworkInterfaces) == 0 {
			continue
		}

		eni := eniResult.NetworkInterfaces[0]
		if aws.ToString(eni.VpcId) != cfg.VpcID {
			continue
		}

		// Release the Elastic IP
		allocationID := aws.ToString(addr.AllocationId)
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would release Elastic IP: %s\n", allocationID)
		} else {
			_, err := clients.EC2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
				AllocationId: aws.String(allocationID),
			})
			if err != nil {
				fmt.Printf("%s Failed to release Elastic IP %s: %v\n", yellow("WARNING:"), allocationID, err)
			} else {
				fmt.Printf("  %s Released Elastic IP: %s\n", green("✓"), allocationID)
			}
		}
	}

	return nil
}

// deleteNATGateways deletes NAT gateways and associated Elastic IPs
func deleteNATGateways(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting NAT gateways...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping NAT gateways (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeNatGatewaysInput{
		Filter: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeNatGateways(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No NAT gateways found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	var eips []string
	for _, natGw := range result.NatGateways {
		// Collect allocation IDs for Elastic IPs
		for _, addr := range natGw.NatGatewayAddresses {
			if addr.AllocationId != nil {
				eips = append(eips, aws.ToString(addr.AllocationId))
			}
		}

		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete NAT gateway: %s\n", aws.ToString(natGw.NatGatewayId))
		} else {
			_, err := clients.EC2.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{
				NatGatewayId: natGw.NatGatewayId,
			})
			if err != nil {
				fmt.Printf("%s Failed to delete NAT gateway %s: %v\n", yellow("WARNING:"), aws.ToString(natGw.NatGatewayId), err)
			} else {
				fmt.Printf("  %s Deleted NAT gateway: %s\n", green("✓"), aws.ToString(natGw.NatGatewayId))
			}
		}
	}

	// Wait for NAT gateways to be deleted before releasing EIPs
	if !cfg.DryRun && len(result.NatGateways) > 0 {
		fmt.Printf("  %s Waiting for NAT gateways to be deleted...\n", yellow("WAIT:"))
		time.Sleep(10 * time.Second)
	}

	// Release Elastic IPs
	for _, allocationID := range eips {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would release Elastic IP: %s\n", allocationID)
		} else {
			_, err := clients.EC2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
				AllocationId: aws.String(allocationID),
			})
			if err != nil {
				fmt.Printf("%s Failed to release Elastic IP %s: %v\n", yellow("WARNING:"), allocationID, err)
			} else {
				fmt.Printf("  %s Released Elastic IP: %s\n", green("✓"), allocationID)
			}
		}
	}

	return nil
}

// deleteNetworkInterfaces deletes orphaned network interfaces in the VPC
func deleteNetworkInterfaces(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting network interfaces...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping network interfaces (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No network interfaces found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	var availableCount, inUseCount int
	for _, eni := range result.NetworkInterfaces {
		eniID := aws.ToString(eni.NetworkInterfaceId)
		status := string(eni.Status)
		requester := aws.ToString(eni.RequesterId)
		description := aws.ToString(eni.Description)

		// Skip network interfaces that are attached to instances
		if eni.Attachment != nil && aws.ToString(eni.Attachment.InstanceId) != "" {
			continue
		}

		// Only delete ENIs that are in "available" status
		// After load balancers are deleted, their ENIs become "available" and can be cleaned up
		if eni.Status != types.NetworkInterfaceStatusAvailable {
			inUseCount++
			// Show debug info for in-use ENIs so we can understand what's blocking
			fmt.Printf("  %s ENI %s is %s (requester: %s, desc: %s)\n",
				yellow("SKIP:"), eniID, status, requester, truncateString(description, 50))
			continue
		}
		availableCount++

		eniID = aws.ToString(eni.NetworkInterfaceId)
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete network interface: %s\n", eniID)
		} else {
			_, err := clients.EC2.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: aws.String(eniID),
			})
			if err != nil {
				fmt.Printf("%s Failed to delete network interface %s: %v\n", yellow("WARNING:"), eniID, err)
			} else {
				fmt.Printf("  %s Deleted network interface: %s\n", green("✓"), eniID)
			}
		}
	}

	return nil
}

// deleteRouteTables deletes route tables
func deleteRouteTables(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting route tables...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping route tables (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeRouteTables(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No route tables found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	for _, rt := range result.RouteTables {
		// Skip main route table (it will be deleted with VPC)
		isMain := false
		for _, assoc := range rt.Associations {
			if aws.ToBool(assoc.Main) {
				isMain = true
				break
			}
		}

		if isMain {
			continue
		}

		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete route table: %s\n", aws.ToString(rt.RouteTableId))
		} else {
			// Disassociate from subnets first
			for _, assoc := range rt.Associations {
				if assoc.RouteTableAssociationId != nil {
					_, err := clients.EC2.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{
						AssociationId: assoc.RouteTableAssociationId,
					})
					if err != nil {
						fmt.Printf("%s Failed to disassociate route table: %v\n", yellow("WARNING:"), err)
					}
				}
			}

			_, err := clients.EC2.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
				RouteTableId: rt.RouteTableId,
			})
			if err != nil {
				fmt.Printf("%s Failed to delete route table %s: %v\n", yellow("WARNING:"), aws.ToString(rt.RouteTableId), err)
			} else {
				fmt.Printf("  %s Deleted route table: %s\n", green("✓"), aws.ToString(rt.RouteTableId))
			}
		}
	}

	return nil
}

// deleteSubnets deletes subnets
func deleteSubnets(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting subnets...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping subnets (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeSubnets(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No subnets found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	for _, subnet := range result.Subnets {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete subnet: %s\n", aws.ToString(subnet.SubnetId))
		} else {
			_, err := clients.EC2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
				SubnetId: subnet.SubnetId,
			})
			if err != nil {
				fmt.Printf("%s Failed to delete subnet %s: %v\n", yellow("WARNING:"), aws.ToString(subnet.SubnetId), err)
			} else {
				fmt.Printf("  %s Deleted subnet: %s\n", green("✓"), aws.ToString(subnet.SubnetId))
			}
		}
	}

	return nil
}

// deleteInternetGatewaysWithRetry deletes internet gateways with retry logic for dependencies
func deleteInternetGatewaysWithRetry(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	maxRetries := 3
	retryDelay := 10 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := deleteInternetGateways(ctx, clients, cfg)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < maxRetries {
			fmt.Printf("  %s Internet Gateway deletion failed (attempt %d/%d), retrying in %v...\n",
				yellow("RETRY:"), attempt, maxRetries, retryDelay)
			time.Sleep(retryDelay)
		}
	}

	return lastErr
}

// deleteInternetGateways deletes internet gateways
func deleteInternetGateways(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting internet gateways...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping internet gateways (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	input := &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []string{cfg.VpcID},
			},
		},
	}

	result, err := clients.EC2.DescribeInternetGateways(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Printf("  %s No internet gateways found (already deleted)\n", green("✓"))
			return nil
		}
		return err
	}

	for _, igw := range result.InternetGateways {
		if cfg.DryRun {
			fmt.Printf("  [DRY RUN] Would delete internet gateway: %s\n", aws.ToString(igw.InternetGatewayId))
		} else {
			// Detach from VPC first
			_, err := clients.EC2.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
				InternetGatewayId: igw.InternetGatewayId,
				VpcId:             aws.String(cfg.VpcID),
			})
			if err != nil && !isNotFoundError(err) {
				fmt.Printf("%s Failed to detach internet gateway: %v\n", yellow("WARNING:"), err)
			}

			_, err = clients.EC2.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
				InternetGatewayId: igw.InternetGatewayId,
			})
			if err != nil {
				if isNotFoundError(err) {
					fmt.Printf("  %s Internet gateway already deleted: %s\n", green("✓"), aws.ToString(igw.InternetGatewayId))
				} else {
					fmt.Printf("%s Failed to delete internet gateway %s: %v\n", yellow("WARNING:"), aws.ToString(igw.InternetGatewayId), err)
				}
			} else {
				fmt.Printf("  %s Deleted internet gateway: %s\n", green("✓"), aws.ToString(igw.InternetGatewayId))
			}
		}
	}

	return nil
}

// deleteVPC deletes the VPC
func deleteVPC(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting VPC...\n", cyan("INFO:"))

	if cfg.VpcID == "" {
		fmt.Printf("  %s Skipping VPC (no VPC ID provided)\n", yellow("SKIP:"))
		return nil
	}

	if cfg.DryRun {
		fmt.Printf("  [DRY RUN] Would delete VPC: %s\n", cfg.VpcID)
	} else {
		_, err := clients.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(cfg.VpcID),
		})
		if err != nil {
			if isNotFoundError(err) {
				fmt.Printf("  %s VPC already deleted: %s\n", green("✓"), cfg.VpcID)
			} else {
				return err
			}
		} else {
			fmt.Printf("  %s Deleted VPC: %s\n", green("✓"), cfg.VpcID)
		}
	}

	return nil
}

// deleteStorageResources deletes S3 buckets and DynamoDB tables
func deleteStorageResources(ctx context.Context, clients *AWSClients, cfg *CleanupConfig) error {
	fmt.Printf("%s Deleting storage resources...\n", cyan("INFO:"))

	// Delete S3 buckets
	bucketPrefixes := []string{
		"redpanda-cloud-storage-",
		fmt.Sprintf("rp-%s-%s-mgmt-", cfg.AccountID, cfg.Region),
	}

	listResult, err := clients.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("%s Failed to list S3 buckets: %v\n", yellow("WARNING:"), err)
	} else {
		for _, bucket := range listResult.Buckets {
			bucketName := aws.ToString(bucket.Name)
			shouldDelete := false

			for _, prefix := range bucketPrefixes {
				if strings.HasPrefix(bucketName, prefix) {
					shouldDelete = true
					break
				}
			}

			if shouldDelete {
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete S3 bucket: %s\n", bucketName)
				} else {
					if err := emptyAndDeleteBucketWithRegion(ctx, clients.S3, bucketName); err != nil {
						fmt.Printf("%s Failed to delete S3 bucket %s: %v\n", yellow("WARNING:"), bucketName, err)
					} else {
						fmt.Printf("  %s Deleted S3 bucket: %s\n", green("✓"), bucketName)
					}
				}
			}
		}
	}

	// Delete DynamoDB tables
	tablePrefix := fmt.Sprintf("rp-%s-%s-mgmt-tflock-", cfg.AccountID, cfg.Region)

	listTablesResult, err := clients.DynamoDB.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		fmt.Printf("%s Failed to list DynamoDB tables: %v\n", yellow("WARNING:"), err)
	} else {
		for _, tableName := range listTablesResult.TableNames {
			if strings.HasPrefix(tableName, tablePrefix) {
				if cfg.DryRun {
					fmt.Printf("  [DRY RUN] Would delete DynamoDB table: %s\n", tableName)
				} else {
					_, err := clients.DynamoDB.DeleteTable(ctx, &dynamodb.DeleteTableInput{
						TableName: aws.String(tableName),
					})
					if err != nil {
						fmt.Printf("%s Failed to delete DynamoDB table %s: %v\n", yellow("WARNING:"), tableName, err)
					} else {
						fmt.Printf("  %s Deleted DynamoDB table: %s\n", green("✓"), tableName)
					}
				}
			}
		}
	}

	return nil
}

// emptyAndDeleteBucketWithRegion deletes all objects in a bucket and then deletes the bucket
// It automatically determines the bucket's region and uses the correct endpoint
func emptyAndDeleteBucketWithRegion(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	// Get bucket region
	locationResult, err := s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to get bucket location: %w", err)
	}

	// Handle region - empty location means us-east-1
	bucketRegion := string(locationResult.LocationConstraint)
	if bucketRegion == "" {
		bucketRegion = "us-east-1"
	}

	// Create a region-specific S3 client
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(bucketRegion))
	if err != nil {
		return fmt.Errorf("failed to load config for region %s: %w", bucketRegion, err)
	}
	regionalS3Client := s3.NewFromConfig(awsCfg)

	// List and delete all object versions
	versionInput := &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucketName),
	}

	for {
		versionResult, err := regionalS3Client.ListObjectVersions(ctx, versionInput)
		if err != nil {
			return err
		}

		if len(versionResult.Versions) == 0 && len(versionResult.DeleteMarkers) == 0 {
			break
		}

		// Delete versions
		var objectsToDelete []s3types.ObjectIdentifier
		for _, version := range versionResult.Versions {
			objectsToDelete = append(objectsToDelete, s3types.ObjectIdentifier{
				Key:       version.Key,
				VersionId: version.VersionId,
			})
		}

		// Delete markers
		for _, marker := range versionResult.DeleteMarkers {
			objectsToDelete = append(objectsToDelete, s3types.ObjectIdentifier{
				Key:       marker.Key,
				VersionId: marker.VersionId,
			})
		}

		if len(objectsToDelete) > 0 {
			_, err := regionalS3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucketName),
				Delete: &s3types.Delete{
					Objects: objectsToDelete,
					Quiet:   aws.Bool(true),
				},
			})
			if err != nil {
				return err
			}
		}

		if !aws.ToBool(versionResult.IsTruncated) {
			break
		}

		versionInput.KeyMarker = versionResult.NextKeyMarker
		versionInput.VersionIdMarker = versionResult.NextVersionIdMarker
	}

	// Delete the bucket
	_, err = regionalS3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})

	return err
}

// getAllRegions returns a list of all enabled AWS regions
func getAllRegions(ctx context.Context, ec2Client *ec2.Client) ([]string, error) {
	input := &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(false), // Only enabled regions
	}

	result, err := ec2Client.DescribeRegions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe regions: %w", err)
	}

	var regions []string
	for _, region := range result.Regions {
		regions = append(regions, aws.ToString(region.RegionName))
	}

	return regions, nil
}

// getNonDefaultVPCs returns all non-default VPCs in the region that have a Name tag starting with the given prefix
func getNonDefaultVPCs(ctx context.Context, ec2Client *ec2.Client, region, commonPrefix string) ([]VPCInfo, error) {
	input := &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("isDefault"),
				Values: []string{"false"},
			},
		},
	}

	result, err := ec2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	var vpcs []VPCInfo
	for _, vpc := range result.Vpcs {
		// Extract the Name tag
		var name string
		for _, tag := range vpc.Tags {
			if aws.ToString(tag.Key) == "Name" {
				name = aws.ToString(tag.Value)
				break
			}
		}

		// Filter by commonPrefix
		if strings.HasPrefix(name, commonPrefix) {
			vpcs = append(vpcs, VPCInfo{
				ID:   aws.ToString(vpc.VpcId),
				Name: name,
			})
		}
	}

	return vpcs, nil
}

// detectVPCsByPrefix finds all non-default VPCs with Name tag matching the common prefix
func detectVPCsByPrefix(ctx context.Context, ec2Client *ec2.Client, commonPrefix string) ([]string, error) {
	input := &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("isDefault"),
				Values: []string{"false"},
			},
		},
	}

	result, err := ec2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	var vpcIDs []string
	for _, vpc := range result.Vpcs {
		// Extract Name tag
		var name string
		for _, tag := range vpc.Tags {
			if aws.ToString(tag.Key) == "Name" {
				name = aws.ToString(tag.Value)
				break
			}
		}

		// Check if name starts with common prefix
		if strings.HasPrefix(name, commonPrefix) {
			vpcIDs = append(vpcIDs, aws.ToString(vpc.VpcId))
			fmt.Printf("  %s Detected VPC: %s (Name: %s)\n", cyan("INFO:"), aws.ToString(vpc.VpcId), name)
		}
	}

	return vpcIDs, nil
}
