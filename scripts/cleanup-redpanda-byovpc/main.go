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
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fatih/color"
)

type CleanupConfig struct {
	CommonPrefix string
	Region       string
	VpcID        string
	DryRun       bool
	AccountID    string
}

type AWSClients struct {
	EC2         *ec2.Client
	IAM         *iam.Client
	S3          *s3.Client
	DynamoDB    *dynamodb.Client
	STS         *sts.Client
	AutoScaling *autoscaling.Client
	EKS         *eks.Client
	ELBV2       *elasticloadbalancingv2.Client
}

var (
	red    = color.New(color.FgRed).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
)

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
		ELBV2:       elasticloadbalancingv2.NewFromConfig(awsCfg),
	}

	// Get AWS account ID if not provided
	if cfg.AccountID == "" {
		cfg.AccountID = getAccountID(ctx, clients.STS)
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

	// Confirm deletion (unless dry-run)
	if !cfg.DryRun {
		if !confirmDeletion(resourceCount) {
			fmt.Println(yellow("Deletion cancelled by user"))
			os.Exit(0)
		}
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

	if err := deleteVPCEndpoints(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete VPC endpoints: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteSecurityGroups(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete security groups: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteNATGateways(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete NAT gateways: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteRouteTables(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete route tables: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteSubnets(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete subnets: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteInternetGateways(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete internet gateways: %v\n", red("ERROR:"), err)
		errorCount++
	}

	if err := deleteVPC(ctx, clients, cfg); err != nil {
		fmt.Printf("%s Failed to delete VPC: %v\n", red("ERROR:"), err)
		errorCount++
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
	flag.StringVar(&cfg.AccountID, "account-id", "", "AWS account ID (auto-detected if not provided)")

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

	// List VPC resources only if VPC ID is provided
	if cfg.VpcID != "" {
		// VPC Endpoints
		vpcEndpointResult, err := clients.EC2.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{cfg.VpcID}},
			},
		})
		if err == nil {
			for _, endpoint := range vpcEndpointResult.VpcEndpoints {
				totalCount++
				fmt.Printf("  - VPC Endpoint: %s\n", aws.ToString(endpoint.VpcEndpointId))
			}
		}

		// Security Groups
		sgResult, err := clients.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{cfg.VpcID}},
			},
		})
		if err == nil {
			for _, sg := range sgResult.SecurityGroups {
				name := aws.ToString(sg.GroupName)
				if strings.HasPrefix(name, cfg.CommonPrefix) && name != "default" {
					totalCount++
					fmt.Printf("  - Security Group: %s (%s)\n", name, aws.ToString(sg.GroupId))
				}
			}
		}

		// NAT Gateways
		natResult, err := clients.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
			Filter: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{cfg.VpcID}},
			},
		})
		if err == nil {
			for _, natGw := range natResult.NatGateways {
				totalCount++
				fmt.Printf("  - NAT Gateway: %s\n", aws.ToString(natGw.NatGatewayId))
			}
		}

		// Route Tables
		rtResult, err := clients.EC2.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{cfg.VpcID}},
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
					fmt.Printf("  - Route Table: %s\n", aws.ToString(rt.RouteTableId))
				}
			}
		}

		// Subnets
		subnetResult, err := clients.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{cfg.VpcID}},
			},
		})
		if err == nil {
			for _, subnet := range subnetResult.Subnets {
				totalCount++
				fmt.Printf("  - Subnet: %s\n", aws.ToString(subnet.SubnetId))
			}
		}

		// Internet Gateways
		igwResult, err := clients.EC2.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
			Filters: []types.Filter{
				{Name: aws.String("attachment.vpc-id"), Values: []string{cfg.VpcID}},
			},
		})
		if err == nil {
			for _, igw := range igwResult.InternetGateways {
				totalCount++
				fmt.Printf("  - Internet Gateway: %s\n", aws.ToString(igw.InternetGatewayId))
			}
		}

		// VPC itself
		totalCount++
		fmt.Printf("  - VPC: %s\n", cfg.VpcID)
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

func getAccountID(ctx context.Context, stsClient *sts.Client) string {
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Printf("%s Failed to get account ID: %v\n", yellow("WARNING:"), err)
		return ""
	}

	return aws.ToString(result.Account)
}

// deleteEKSClusters deletes EKS clusters
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
				_, err := clients.EKS.DeleteCluster(ctx, &eks.DeleteClusterInput{
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
		return err
	}

	// Filter to only security groups matching our prefix
	var sgToDelete []types.SecurityGroup
	for _, sg := range result.SecurityGroups {
		name := aws.ToString(sg.GroupName)
		if strings.HasPrefix(name, cfg.CommonPrefix) && name != "default" {
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
			if err != nil {
				fmt.Printf("%s Failed to detach internet gateway: %v\n", yellow("WARNING:"), err)
			}

			_, err = clients.EC2.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
				InternetGatewayId: igw.InternetGatewayId,
			})
			if err != nil {
				fmt.Printf("%s Failed to delete internet gateway %s: %v\n", yellow("WARNING:"), aws.ToString(igw.InternetGatewayId), err)
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
			return err
		}
		fmt.Printf("  %s Deleted VPC: %s\n", green("✓"), cfg.VpcID)
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
