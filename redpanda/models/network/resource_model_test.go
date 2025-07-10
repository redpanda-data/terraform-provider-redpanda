package network

import (
	"context"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper functions for safe type assertions
func requireObjectFromAttrs(t *testing.T, attrs map[string]attr.Value, key string) types.Object {
	val, ok := attrs[key]
	require.True(t, ok, "Expected key '%s' to exist in attributes", key)
	obj, ok := val.(types.Object)
	require.True(t, ok, "Expected value at key '%s' to be types.Object", key)
	return obj
}

func requireStringFromAttrs(t *testing.T, attrs map[string]attr.Value, key string) types.String {
	val, ok := attrs[key]
	require.True(t, ok, "Expected key '%s' to exist in attributes", key)
	str, ok := val.(types.String)
	require.True(t, ok, "Expected value at key '%s' to be types.String", key)
	return str
}

func requireListFromAttrs(t *testing.T, attrs map[string]attr.Value, key string) types.List {
	val, ok := attrs[key]
	require.True(t, ok, "Expected key '%s' to exist in attributes", key)
	list, ok := val.(types.List)
	require.True(t, ok, "Expected value at key '%s' to be types.List", key)
	return list
}

func requireStringFromListElement(t *testing.T, elements []attr.Value, index int) types.String {
	require.True(t, index < len(elements), "Expected list to have at least %d elements", index+1)
	str, ok := elements[index].(types.String)
	require.True(t, ok, "Expected element at index %d to be types.String", index)
	return str
}

func TestResourceModel_GetUpdatedModel(t *testing.T) {
	ctx := context.Background()
	createdTime := time.Now()

	tests := []struct {
		name     string
		network  *controlplanev1.Network
		validate func(t *testing.T, result *ResourceModel)
	}{
		{
			name: "aws_byovpc_cmr",
			network: &controlplanev1.Network{
				Id:              "net-aws123full",
				Name:            "aws-full-cmr-test",
				ResourceGroupId: "rg-aws123",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				Region:          "us-west-2",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Aws{
						Aws: &controlplanev1.Network_CustomerManagedResources_AWS{
							ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
								Arn: "arn:aws:s3:::my-management-bucket",
							},
							DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
								Arn: "arn:aws:dynamodb:us-west-2:123456789012:table/my-dynamodb-table",
							},
							Vpc: &controlplanev1.CustomerManagedAWSVPC{
								Arn: "arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345678",
							},
							PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
								Arns: []string{
									"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345678",
									"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-87654321",
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-aws123full", result.ID.ValueString())
				require.Equal(t, "aws-full-cmr-test", result.Name.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.False(t, result.CustomerManagedResources.IsNull())

				cmrAttrs := result.CustomerManagedResources.Attributes()
				awsObj := requireObjectFromAttrs(t, cmrAttrs, "aws")
				require.False(t, awsObj.IsNull())

				awsAttrs := awsObj.Attributes()

				mgmtBucket := requireObjectFromAttrs(t, awsAttrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtBucketAttrs := mgmtBucket.Attributes()
				require.Equal(t, "arn:aws:s3:::my-management-bucket", requireStringFromAttrs(t, mgmtBucketAttrs, "arn").ValueString())

				dynamoTable := requireObjectFromAttrs(t, awsAttrs, "dynamodb_table")
				require.False(t, dynamoTable.IsNull())
				dynamoAttrs := dynamoTable.Attributes()
				arnStr := requireStringFromAttrs(t, dynamoAttrs, "arn")
				require.Equal(t, "arn:aws:dynamodb:us-west-2:123456789012:table/my-dynamodb-table", arnStr.ValueString())

				vpc := requireObjectFromAttrs(t, awsAttrs, "vpc")
				require.False(t, vpc.IsNull())
				vpcAttrs := vpc.Attributes()
				vpcArn := requireStringFromAttrs(t, vpcAttrs, "arn")
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345678", vpcArn.ValueString())

				privateSubnets := requireObjectFromAttrs(t, awsAttrs, "private_subnets")
				require.False(t, privateSubnets.IsNull())
				subnetAttrs := privateSubnets.Attributes()
				arnsList := requireListFromAttrs(t, subnetAttrs, "arns")
				require.Equal(t, 2, len(arnsList.Elements()))
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345678", arnsList.Elements()[0].(types.String).ValueString())
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:subnet/subnet-87654321", arnsList.Elements()[1].(types.String).ValueString())
			},
		},
		{
			name: "gcp_byovpc_cmr",
			network: &controlplanev1.Network{
				Id:              "net-gcp456full",
				Name:            "gcp-full-cmr-test",
				ResourceGroupId: "rg-gcp456",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				Region:          "us-central1",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{
							NetworkName:      "my-gcp-network",
							NetworkProjectId: "my-gcp-project-12345",
							ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "my-gcp-management-bucket",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-gcp456full", result.ID.ValueString())
				require.Equal(t, "gcp-full-cmr-test", result.Name.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.False(t, result.CustomerManagedResources.IsNull())

				cmrAttrs := result.CustomerManagedResources.Attributes()
				gcpObj := requireObjectFromAttrs(t, cmrAttrs, "gcp")
				require.False(t, gcpObj.IsNull())

				gcpAttrs := gcpObj.Attributes()

				networkName := requireStringFromAttrs(t, gcpAttrs, "network_name")
				require.False(t, networkName.IsNull())
				require.Equal(t, "my-gcp-network", networkName.ValueString())

				networkProjectId := requireStringFromAttrs(t, gcpAttrs, "network_project_id")
				require.False(t, networkProjectId.IsNull())
				require.Equal(t, "my-gcp-project-12345", networkProjectId.ValueString())

				mgmtBucket := requireObjectFromAttrs(t, gcpAttrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtBucketAttrs := mgmtBucket.Attributes()
				bucketName := requireStringFromAttrs(t, mgmtBucketAttrs, "name")
				require.Equal(t, "my-gcp-management-bucket", bucketName.ValueString())
			},
		},
		{
			name: "aws_byovpc_network_partial_cmr",
			network: &controlplanev1.Network{
				Id:              "net-aws789partial",
				Name:            "aws-partial-cmr-test",
				ResourceGroupId: "rg-aws789",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				Region:          "eu-west-1",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Aws{
						Aws: &controlplanev1.Network_CustomerManagedResources_AWS{
							ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
								Arn: "",
							},
							DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
								Arn: "",
							},
							Vpc: &controlplanev1.CustomerManagedAWSVPC{
								Arn: "",
							},
							PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
								Arns: []string{},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-aws789partial", result.ID.ValueString())
				require.Equal(t, "aws-partial-cmr-test", result.Name.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.False(t, result.CustomerManagedResources.IsNull())

				// Extract and validate AWS CMR fields with empty values
				cmrAttrs := result.CustomerManagedResources.Attributes()
				awsObj := requireObjectFromAttrs(t, cmrAttrs, "aws")
				require.False(t, awsObj.IsNull())

				awsAttrs := awsObj.Attributes()

				// Nested objects are created even when ARNs are empty (due to HasXXX() protobuf checks)
				mgmtBucket := requireObjectFromAttrs(t, awsAttrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtBucketAttrs := mgmtBucket.Attributes()
				mgmtArn := requireStringFromAttrs(t, mgmtBucketAttrs, "arn")
				require.Equal(t, "", mgmtArn.ValueString())

				dynamoTable := requireObjectFromAttrs(t, awsAttrs, "dynamodb_table")
				require.False(t, dynamoTable.IsNull())
				dynamoAttrs := dynamoTable.Attributes()
				dynamoArn := requireStringFromAttrs(t, dynamoAttrs, "arn")
				require.Equal(t, "", dynamoArn.ValueString())

				vpc := requireObjectFromAttrs(t, awsAttrs, "vpc")
				require.False(t, vpc.IsNull())
				vpcAttrs := vpc.Attributes()
				vpcArn := requireStringFromAttrs(t, vpcAttrs, "arn")
				require.Equal(t, "", vpcArn.ValueString())

				privateSubnets := requireObjectFromAttrs(t, awsAttrs, "private_subnets")
				require.False(t, privateSubnets.IsNull())
				subnetAttrs := privateSubnets.Attributes()
				arnsList := requireListFromAttrs(t, subnetAttrs, "arns")
				require.Equal(t, 0, len(arnsList.Elements()))
			},
		},
		{
			name: "gcp_byovpc_network_with_partial_cmr_empty_fields",
			network: &controlplanev1.Network{
				Id:              "net-gcp101partial",
				Name:            "gcp-partial-cmr-test",
				ResourceGroupId: "rg-gcp101",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				Region:          "europe-west1",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{
							NetworkName:      "",
							NetworkProjectId: "",
							ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-gcp101partial", result.ID.ValueString())
				require.Equal(t, "gcp-partial-cmr-test", result.Name.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.False(t, result.CustomerManagedResources.IsNull())

				// Extract and validate GCP CMR fields with empty values
				cmrAttrs := result.CustomerManagedResources.Attributes()
				gcpObj := requireObjectFromAttrs(t, cmrAttrs, "gcp")
				require.False(t, gcpObj.IsNull())

				gcpAttrs := gcpObj.Attributes()

				// String fields should be null when empty
				networkName := requireStringFromAttrs(t, gcpAttrs, "network_name")
				require.True(t, networkName.IsNull())

				networkProjectId := requireStringFromAttrs(t, gcpAttrs, "network_project_id")
				require.True(t, networkProjectId.IsNull())

				// Management bucket object is created even when name is empty (due to HasManagementBucket() check)
				mgmtBucket := requireObjectFromAttrs(t, gcpAttrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtBucketAttrs := mgmtBucket.Attributes()
				bucketName := requireStringFromAttrs(t, mgmtBucketAttrs, "name")
				require.Equal(t, "", bucketName.ValueString())
			},
		},
		{
			name: "network_without_cmr",
			network: &controlplanev1.Network{
				Id:              "net-no-cmr",
				Name:            "no-cmr-test",
				ResourceGroupId: "rg-no-cmr",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				Region:          "us-east-1",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-no-cmr", result.ID.ValueString())
				require.Equal(t, "no-cmr-test", result.Name.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &ResourceModel{}
			result, diags := model.GetUpdatedModel(ctx, tt.network)

			require.False(t, diags.HasError())
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestResourceModel_GetNetworkCreate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		model    *ResourceModel
		validate func(t *testing.T, result *controlplanev1.NetworkCreate)
	}{
		{
			name: "aws_dedicated_network_create",
			model: &ResourceModel{
				Name:            types.StringValue("testname"),
				ResourceGroupID: types.StringValue("rg-123456"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("dedicated"),
				Region:          types.StringValue("us-east-2"),
				CidrBlock:       types.StringValue("10.0.0.0/20"),
			},
			validate: func(t *testing.T, result *controlplanev1.NetworkCreate) {
				require.Equal(t, "testname", result.GetName())
				require.Equal(t, "rg-123456", result.GetResourceGroupId())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_DEDICATED, result.GetClusterType())
				require.Equal(t, "us-east-2", result.GetRegion())
				require.Equal(t, "10.0.0.0/20", result.GetCidrBlock())
			},
		},
		{
			name: "gcp_byovpc_network_create",
			model: &ResourceModel{
				Name:            types.StringValue("testname"),
				ResourceGroupID: types.StringValue("rg-456789"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("us-central1"),
				CustomerManagedResources: types.ObjectValueMust(
					getCMRType(),
					map[string]attr.Value{
						"aws": types.ObjectNull(getAwsType()),
						"gcp": types.ObjectValueMust(
							getGcpType(),
							map[string]attr.Value{
								"network_name":       types.StringValue("redpanda-network-vpc"),
								"network_project_id": types.StringValue("hallowed-ray-376320"),
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{"name": types.StringType},
									map[string]attr.Value{
										"name": types.StringValue("redpanda-management-bucket-testname"),
									},
								),
							},
						),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.NetworkCreate) {
				require.Equal(t, "testname", result.GetName())
				require.Equal(t, "rg-456789", result.GetResourceGroupId())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetClusterType())
				require.Equal(t, "us-central1", result.GetRegion())
				require.True(t, result.HasCustomerManagedResources())
				require.True(t, result.GetCustomerManagedResources().HasGcp())
			},
		},
		{
			name: "aws_byovpc_network_create_with_full_cmr",
			model: &ResourceModel{
				Name:            types.StringValue("aws-full-cmr-create"),
				ResourceGroupID: types.StringValue("rg-aws-full"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("us-west-2"),
				CustomerManagedResources: types.ObjectValueMust(
					getCMRType(),
					map[string]attr.Value{
						"gcp": types.ObjectNull(getGcpType()),
						"aws": types.ObjectValueMust(
							getAwsType(),
							map[string]attr.Value{
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{"arn": types.StringType},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:s3:::test-management-bucket"),
									},
								),
								"dynamodb_table": types.ObjectValueMust(
									map[string]attr.Type{"arn": types.StringType},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:dynamodb:us-west-2:123456789012:table/test-dynamodb"),
									},
								),
								"vpc": types.ObjectValueMust(
									map[string]attr.Type{"arn": types.StringType},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:ec2:us-west-2:123456789012:vpc/vpc-test123"),
									},
								),
								"private_subnets": types.ObjectValueMust(
									map[string]attr.Type{
										"arns": types.ListType{ElemType: types.StringType},
									},
									map[string]attr.Value{
										"arns": types.ListValueMust(types.StringType, []attr.Value{
											types.StringValue("arn:aws:ec2:us-west-2:123456789012:subnet/subnet-test1"),
											types.StringValue("arn:aws:ec2:us-west-2:123456789012:subnet/subnet-test2"),
											types.StringValue("arn:aws:ec2:us-west-2:123456789012:subnet/subnet-test3"),
										}),
									},
								),
							},
						),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.NetworkCreate) {
				require.Equal(t, "aws-full-cmr-create", result.GetName())
				require.Equal(t, "rg-aws-full", result.GetResourceGroupId())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetClusterType())
				require.Equal(t, "us-west-2", result.GetRegion())
				require.True(t, result.HasCustomerManagedResources())
				require.True(t, result.GetCustomerManagedResources().HasAws())

				// Validate all AWS CMR fields
				awsCMR := result.GetCustomerManagedResources().GetAws()
				require.Equal(t, "arn:aws:s3:::test-management-bucket", awsCMR.GetManagementBucket().GetArn())
				require.Equal(t, "arn:aws:dynamodb:us-west-2:123456789012:table/test-dynamodb", awsCMR.GetDynamodbTable().GetArn())
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:vpc/vpc-test123", awsCMR.GetVpc().GetArn())

				privateSubnets := awsCMR.GetPrivateSubnets().GetArns()
				require.Equal(t, 3, len(privateSubnets))
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:subnet/subnet-test1", privateSubnets[0])
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:subnet/subnet-test2", privateSubnets[1])
				require.Equal(t, "arn:aws:ec2:us-west-2:123456789012:subnet/subnet-test3", privateSubnets[2])
			},
		},
		{
			name: "gcp_byovpc_network_create_with_full_cmr",
			model: &ResourceModel{
				Name:            types.StringValue("gcp-full-cmr-create"),
				ResourceGroupID: types.StringValue("rg-gcp-full"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("europe-west1"),
				CustomerManagedResources: types.ObjectValueMust(
					getCMRType(),
					map[string]attr.Value{
						"aws": types.ObjectNull(getAwsType()),
						"gcp": types.ObjectValueMust(
							getGcpType(),
							map[string]attr.Value{
								"network_name":       types.StringValue("test-gcp-network"),
								"network_project_id": types.StringValue("test-gcp-project-789"),
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{"name": types.StringType},
									map[string]attr.Value{
										"name": types.StringValue("test-gcp-management-bucket"),
									},
								),
							},
						),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.NetworkCreate) {
				require.Equal(t, "gcp-full-cmr-create", result.GetName())
				require.Equal(t, "rg-gcp-full", result.GetResourceGroupId())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetClusterType())
				require.Equal(t, "europe-west1", result.GetRegion())
				require.True(t, result.HasCustomerManagedResources())
				require.True(t, result.GetCustomerManagedResources().HasGcp())

				// Validate all GCP CMR fields
				gcpCMR := result.GetCustomerManagedResources().GetGcp()
				require.Equal(t, "test-gcp-network", gcpCMR.GetNetworkName())
				require.Equal(t, "test-gcp-project-789", gcpCMR.GetNetworkProjectId())
				require.Equal(t, "test-gcp-management-bucket", gcpCMR.GetManagementBucket().GetName())
			},
		},
		{
			name: "aws_byovpc_network_create_with_null_cmr",
			model: &ResourceModel{
				Name:                     types.StringValue("aws-null-cmr"),
				ResourceGroupID:          types.StringValue("rg-aws-null"),
				CloudProvider:            types.StringValue("aws"),
				ClusterType:              types.StringValue("byoc"),
				Region:                   types.StringValue("us-east-1"),
				CustomerManagedResources: getCMRNull(),
			},
			validate: func(t *testing.T, result *controlplanev1.NetworkCreate) {
				require.Equal(t, "aws-null-cmr", result.GetName())
				require.Equal(t, "rg-aws-null", result.GetResourceGroupId())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetClusterType())
				require.Equal(t, "us-east-1", result.GetRegion())
				require.False(t, result.HasCustomerManagedResources())
				require.Nil(t, result.GetCustomerManagedResources())
			},
		},
		{
			name: "gcp_byovpc_network_create_with_null_cmr",
			model: &ResourceModel{
				Name:                     types.StringValue("gcp-null-cmr"),
				ResourceGroupID:          types.StringValue("rg-gcp-null"),
				CloudProvider:            types.StringValue("gcp"),
				ClusterType:              types.StringValue("byoc"),
				Region:                   types.StringValue("asia-east1"),
				CustomerManagedResources: getCMRNull(),
			},
			validate: func(t *testing.T, result *controlplanev1.NetworkCreate) {
				require.Equal(t, "gcp-null-cmr", result.GetName())
				require.Equal(t, "rg-gcp-null", result.GetResourceGroupId())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetClusterType())
				require.Equal(t, "asia-east1", result.GetRegion())
				require.False(t, result.HasCustomerManagedResources())
				require.Nil(t, result.GetCustomerManagedResources())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := tt.model.GetNetworkCreate(ctx)

			require.False(t, diags.HasError())
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestResourceModel_GetID(t *testing.T) {
	tests := []struct {
		name     string
		model    *ResourceModel
		expected string
	}{
		{
			name: "valid_id",
			model: &ResourceModel{
				ID: types.StringValue("net-abc123def"),
			},
			expected: "net-abc123def",
		},
		{
			name: "null_id",
			model: &ResourceModel{
				ID: types.StringNull(),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.GetID()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateNetworkAWSCMR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		cmrObj    types.Object
		expectErr bool
		validate  func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS)
	}{
		{
			name: "aws_cmr_with_all_fields",
			cmrObj: types.ObjectValueMust(
				getCMRType(),
				map[string]attr.Value{
					"gcp": types.ObjectNull(getGcpType()),
					"aws": types.ObjectValueMust(
						getAwsType(),
						map[string]attr.Value{
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:s3:::test-bucket"),
								},
							),
							"dynamodb_table": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
								},
							),
							"vpc": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-abc123"),
								},
							),
							"private_subnets": types.ObjectValueMust(
								map[string]attr.Type{
									"arns": types.ListType{ElemType: types.StringType},
								},
								map[string]attr.Value{
									"arns": types.ListValueMust(types.StringType, []attr.Value{
										types.StringValue("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-1"),
										types.StringValue("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-2"),
									}),
								},
							),
						},
					),
				},
			),
			expectErr: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS) {
				require.Equal(t, "arn:aws:s3:::test-bucket", result.GetManagementBucket().GetArn())
				require.Equal(t, "arn:aws:dynamodb:us-east-1:123456789012:table/test-table", result.GetDynamodbTable().GetArn())
				require.Equal(t, "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-abc123", result.GetVpc().GetArn())
				require.Equal(t, 2, len(result.GetPrivateSubnets().GetArns()))
				require.Equal(t, "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-1", result.GetPrivateSubnets().GetArns()[0])
				require.Equal(t, "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-2", result.GetPrivateSubnets().GetArns()[1])
			},
		},
		{
			name: "aws_cmr_with_empty_private_subnets",
			cmrObj: types.ObjectValueMust(
				getCMRType(),
				map[string]attr.Value{
					"gcp": types.ObjectNull(getGcpType()),
					"aws": types.ObjectValueMust(
						getAwsType(),
						map[string]attr.Value{
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:s3:::test-bucket"),
								},
							),
							"dynamodb_table": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
								},
							),
							"vpc": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-abc123"),
								},
							),
							"private_subnets": types.ObjectValueMust(
								map[string]attr.Type{
									"arns": types.ListType{ElemType: types.StringType},
								},
								map[string]attr.Value{
									"arns": types.ListValueMust(types.StringType, []attr.Value{}),
								},
							),
						},
					),
				},
			),
			expectErr: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS) {
				require.Equal(t, "arn:aws:s3:::test-bucket", result.GetManagementBucket().GetArn())
				require.Equal(t, "arn:aws:dynamodb:us-east-1:123456789012:table/test-table", result.GetDynamodbTable().GetArn())
				require.Equal(t, "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-abc123", result.GetVpc().GetArn())
				require.Equal(t, 0, len(result.GetPrivateSubnets().GetArns()))
			},
		},
		{
			name: "aws_cmr_with_single_private_subnet",
			cmrObj: types.ObjectValueMust(
				getCMRType(),
				map[string]attr.Value{
					"gcp": types.ObjectNull(getGcpType()),
					"aws": types.ObjectValueMust(
						getAwsType(),
						map[string]attr.Value{
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:s3:::single-test-bucket"),
								},
							),
							"dynamodb_table": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:dynamodb:eu-west-1:999888777666:table/single-table"),
								},
							),
							"vpc": types.ObjectValueMust(
								map[string]attr.Type{"arn": types.StringType},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:ec2:eu-west-1:999888777666:vpc/vpc-single123"),
								},
							),
							"private_subnets": types.ObjectValueMust(
								map[string]attr.Type{
									"arns": types.ListType{ElemType: types.StringType},
								},
								map[string]attr.Value{
									"arns": types.ListValueMust(types.StringType, []attr.Value{
										types.StringValue("arn:aws:ec2:eu-west-1:999888777666:subnet/subnet-single"),
									}),
								},
							),
						},
					),
				},
			),
			expectErr: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS) {
				require.Equal(t, "arn:aws:s3:::single-test-bucket", result.GetManagementBucket().GetArn())
				require.Equal(t, "arn:aws:dynamodb:eu-west-1:999888777666:table/single-table", result.GetDynamodbTable().GetArn())
				require.Equal(t, "arn:aws:ec2:eu-west-1:999888777666:vpc/vpc-single123", result.GetVpc().GetArn())
				require.Equal(t, 1, len(result.GetPrivateSubnets().GetArns()))
				require.Equal(t, "arn:aws:ec2:eu-west-1:999888777666:subnet/subnet-single", result.GetPrivateSubnets().GetArns()[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateNetworkAWSCMR(ctx, tt.cmrObj)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestGenerateNetworkGCPCMR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		cmrObj    types.Object
		expectErr bool
		validate  func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_GCP)
	}{
		{
			name: "gcp_cmr_with_all_fields",
			cmrObj: types.ObjectValueMust(
				getCMRType(),
				map[string]attr.Value{
					"aws": types.ObjectNull(getAwsType()),
					"gcp": types.ObjectValueMust(
						getGcpType(),
						map[string]attr.Value{
							"network_name":       types.StringValue("test-gcp-network"),
							"network_project_id": types.StringValue("test-project-12345"),
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{"name": types.StringType},
								map[string]attr.Value{
									"name": types.StringValue("test-gcp-bucket"),
								},
							),
						},
					),
				},
			),
			expectErr: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_GCP) {
				require.Equal(t, "test-gcp-network", result.GetNetworkName())
				require.Equal(t, "test-project-12345", result.GetNetworkProjectId())
				require.Equal(t, "test-gcp-bucket", result.GetManagementBucket().GetName())
			},
		},
		{
			name: "gcp_cmr_with_minimal_fields",
			cmrObj: types.ObjectValueMust(
				getCMRType(),
				map[string]attr.Value{
					"aws": types.ObjectNull(getAwsType()),
					"gcp": types.ObjectValueMust(
						getGcpType(),
						map[string]attr.Value{
							"network_name":       types.StringValue("minimal-network"),
							"network_project_id": types.StringValue("minimal-project"),
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{"name": types.StringType},
								map[string]attr.Value{
									"name": types.StringValue("minimal-bucket"),
								},
							),
						},
					),
				},
			),
			expectErr: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_GCP) {
				require.Equal(t, "minimal-network", result.GetNetworkName())
				require.Equal(t, "minimal-project", result.GetNetworkProjectId())
				require.Equal(t, "minimal-bucket", result.GetManagementBucket().GetName())
			},
		},
		{
			name: "gcp_cmr_with_long_names",
			cmrObj: types.ObjectValueMust(
				getCMRType(),
				map[string]attr.Value{
					"aws": types.ObjectNull(getAwsType()),
					"gcp": types.ObjectValueMust(
						getGcpType(),
						map[string]attr.Value{
							"network_name":       types.StringValue("very-long-gcp-network-name-for-testing-purposes"),
							"network_project_id": types.StringValue("very-long-project-id-12345-67890-abcdef"),
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{"name": types.StringType},
								map[string]attr.Value{
									"name": types.StringValue("very-long-gcp-management-bucket-name-for-testing"),
								},
							),
						},
					),
				},
			),
			expectErr: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_GCP) {
				require.Equal(t, "very-long-gcp-network-name-for-testing-purposes", result.GetNetworkName())
				require.Equal(t, "very-long-project-id-12345-67890-abcdef", result.GetNetworkProjectId())
				require.Equal(t, "very-long-gcp-management-bucket-name-for-testing", result.GetManagementBucket().GetName())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateNetworkGCPCMR(ctx, tt.cmrObj)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestGenerateModelAWSCMR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		awsData  *controlplanev1.Network_CustomerManagedResources_AWS
		validate func(t *testing.T, result types.Object)
	}{
		{
			name: "aws_model_cmr_with_all_fields",
			awsData: &controlplanev1.Network_CustomerManagedResources_AWS{
				ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
					Arn: "arn:aws:s3:::model-test-bucket",
				},
				DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
					Arn: "arn:aws:dynamodb:us-west-2:111222333444:table/model-test-table",
				},
				Vpc: &controlplanev1.CustomerManagedAWSVPC{
					Arn: "arn:aws:ec2:us-west-2:111222333444:vpc/vpc-model123",
				},
				PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
					Arns: []string{
						"arn:aws:ec2:us-west-2:111222333444:subnet/subnet-model1",
						"arn:aws:ec2:us-west-2:111222333444:subnet/subnet-model2",
						"arn:aws:ec2:us-west-2:111222333444:subnet/subnet-model3",
					},
				},
			},
			validate: func(t *testing.T, result types.Object) {
				require.False(t, result.IsNull())

				attrs := result.Attributes()

				mgmtBucket := requireObjectFromAttrs(t, attrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtAttrs := mgmtBucket.Attributes()
				mgmtArn := requireStringFromAttrs(t, mgmtAttrs, "arn")
				require.Equal(t, "arn:aws:s3:::model-test-bucket", mgmtArn.ValueString())

				dynamoTable := requireObjectFromAttrs(t, attrs, "dynamodb_table")
				require.False(t, dynamoTable.IsNull())
				dynamoAttrs := dynamoTable.Attributes()
				dynamoArn := requireStringFromAttrs(t, dynamoAttrs, "arn")
				require.Equal(t, "arn:aws:dynamodb:us-west-2:111222333444:table/model-test-table", dynamoArn.ValueString())

				vpc := requireObjectFromAttrs(t, attrs, "vpc")
				require.False(t, vpc.IsNull())
				vpcAttrs := vpc.Attributes()
				vpcArn := requireStringFromAttrs(t, vpcAttrs, "arn")
				require.Equal(t, "arn:aws:ec2:us-west-2:111222333444:vpc/vpc-model123", vpcArn.ValueString())

				privateSubnets := requireObjectFromAttrs(t, attrs, "private_subnets")
				require.False(t, privateSubnets.IsNull())
				subnetAttrs := privateSubnets.Attributes()
				arnsList := requireListFromAttrs(t, subnetAttrs, "arns")
				elements := arnsList.Elements()
				require.Equal(t, 3, len(elements))
				arn1 := requireStringFromListElement(t, elements, 0)
				require.Equal(t, "arn:aws:ec2:us-west-2:111222333444:subnet/subnet-model1", arn1.ValueString())
				arn2 := requireStringFromListElement(t, elements, 1)
				require.Equal(t, "arn:aws:ec2:us-west-2:111222333444:subnet/subnet-model2", arn2.ValueString())
				arn3 := requireStringFromListElement(t, elements, 2)
				require.Equal(t, "arn:aws:ec2:us-west-2:111222333444:subnet/subnet-model3", arn3.ValueString())
			},
		},
		{
			name: "aws_model_cmr_with_null_fields",
			awsData: &controlplanev1.Network_CustomerManagedResources_AWS{
				ManagementBucket: nil,
				DynamodbTable:    nil,
				Vpc:              nil,
				PrivateSubnets:   nil,
			},
			validate: func(t *testing.T, result types.Object) {
				require.False(t, result.IsNull())

				attrs := result.Attributes()

				// All nested objects should be null when protobuf fields are nil
				mgmtBucket := requireObjectFromAttrs(t, attrs, "management_bucket")
				require.True(t, mgmtBucket.IsNull())

				dynamoTable := requireObjectFromAttrs(t, attrs, "dynamodb_table")
				require.True(t, dynamoTable.IsNull())

				vpc := requireObjectFromAttrs(t, attrs, "vpc")
				require.True(t, vpc.IsNull())

				privateSubnets := requireObjectFromAttrs(t, attrs, "private_subnets")
				require.True(t, privateSubnets.IsNull())
			},
		},
		{
			name: "aws_model_cmr_with_empty_private_subnets",
			awsData: &controlplanev1.Network_CustomerManagedResources_AWS{
				ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
					Arn: "arn:aws:s3:::empty-subnets-bucket",
				},
				DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
					Arn: "arn:aws:dynamodb:ap-southeast-1:555666777888:table/empty-subnets-table",
				},
				Vpc: &controlplanev1.CustomerManagedAWSVPC{
					Arn: "arn:aws:ec2:ap-southeast-1:555666777888:vpc/vpc-empty123",
				},
				PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
					Arns: []string{},
				},
			},
			validate: func(t *testing.T, result types.Object) {
				require.False(t, result.IsNull())

				attrs := result.Attributes()

				// Validate that empty private subnets list is properly handled
				privateSubnets := requireObjectFromAttrs(t, attrs, "private_subnets")
				require.False(t, privateSubnets.IsNull())
				subnetAttrs := privateSubnets.Attributes()
				arnsList := requireListFromAttrs(t, subnetAttrs, "arns")
				require.Equal(t, 0, len(arnsList.Elements()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := generateModelAWSCMR(ctx, tt.awsData)

			require.False(t, diags.HasError())
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestGenerateModelGCPCMR(t *testing.T) {
	tests := []struct {
		name     string
		gcpData  *controlplanev1.Network_CustomerManagedResources_GCP
		validate func(t *testing.T, result types.Object)
	}{
		{
			name: "gcp_model_cmr_with_all_fields",
			gcpData: &controlplanev1.Network_CustomerManagedResources_GCP{
				NetworkName:      "model-test-gcp-network",
				NetworkProjectId: "model-test-project-54321",
				ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
					Name: "model-test-gcp-bucket",
				},
			},
			validate: func(t *testing.T, result types.Object) {
				require.False(t, result.IsNull())

				attrs := result.Attributes()

				networkName := requireStringFromAttrs(t, attrs, "network_name")
				require.False(t, networkName.IsNull())
				require.Equal(t, "model-test-gcp-network", networkName.ValueString())

				networkProjectId := requireStringFromAttrs(t, attrs, "network_project_id")
				require.False(t, networkProjectId.IsNull())
				require.Equal(t, "model-test-project-54321", networkProjectId.ValueString())

				mgmtBucket := requireObjectFromAttrs(t, attrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtAttrs := mgmtBucket.Attributes()
				bucketName := requireStringFromAttrs(t, mgmtAttrs, "name")
				require.Equal(t, "model-test-gcp-bucket", bucketName.ValueString())
			},
		},
		{
			name: "gcp_model_cmr_with_empty_strings",
			gcpData: &controlplanev1.Network_CustomerManagedResources_GCP{
				NetworkName:      "",
				NetworkProjectId: "",
				ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
					Name: "",
				},
			},
			validate: func(t *testing.T, result types.Object) {
				require.False(t, result.IsNull())

				attrs := result.Attributes()

				// All string fields should be null when empty
				networkName := requireStringFromAttrs(t, attrs, "network_name")
				require.True(t, networkName.IsNull())

				networkProjectId := requireStringFromAttrs(t, attrs, "network_project_id")
				require.True(t, networkProjectId.IsNull())

				// Management bucket object is created even when name is empty (due to HasManagementBucket() check)
				mgmtBucket := requireObjectFromAttrs(t, attrs, "management_bucket")
				require.False(t, mgmtBucket.IsNull())
				mgmtAttrs := mgmtBucket.Attributes()
				bucketName := requireStringFromAttrs(t, mgmtAttrs, "name")
				require.Equal(t, "", bucketName.ValueString())
			},
		},
		{
			name: "gcp_model_cmr_with_null_bucket",
			gcpData: &controlplanev1.Network_CustomerManagedResources_GCP{
				NetworkName:      "null-bucket-network",
				NetworkProjectId: "null-bucket-project",
				ManagementBucket: nil,
			},
			validate: func(t *testing.T, result types.Object) {
				require.False(t, result.IsNull())

				attrs := result.Attributes()

				// Network fields should be populated
				networkName := requireStringFromAttrs(t, attrs, "network_name")
				require.False(t, networkName.IsNull())
				require.Equal(t, "null-bucket-network", networkName.ValueString())

				networkProjectId := requireStringFromAttrs(t, attrs, "network_project_id")
				require.False(t, networkProjectId.IsNull())
				require.Equal(t, "null-bucket-project", networkProjectId.ValueString())

				// Management bucket should be null when protobuf field is nil
				mgmtBucket := requireObjectFromAttrs(t, attrs, "management_bucket")
				require.True(t, mgmtBucket.IsNull())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := generateModelGCPCMR(tt.gcpData)

			require.False(t, diags.HasError())
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
