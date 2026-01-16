package cluster

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestResourceModel_GetUpdatedModel(t *testing.T) {
	ctx := context.Background()
	createdTime := time.Now()

	tests := []struct {
		name       string
		cluster    *controlplanev1.Cluster
		contingent ContingentFields
		validate   func(t *testing.T, result *ResourceModel)
	}{
		{
			name: "aws_dedicated_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-abc123def",
				Name:            "testname",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-1-aws-v2-arm",
				Region:          "us-east-2",
				Zones:           []string{"use2-az1", "use2-az2", "use2-az3"},
				ResourceGroupId: "rg-123456",
				NetworkId:       "net-789012",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
					Url: "https://testname.us-east-2.aws.redpanda.cloud:443",
				},
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"key": types.StringValue("value"),
				}),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "rp-abc123def", result.ID.ValueString())
				require.Equal(t, "testname", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-aws-v2-arm", result.ThroughputTier.ValueString())
				require.Equal(t, "us-east-2", result.Region.ValueString())
				require.True(t, result.AllowDeletion.ValueBool())
				// Dedicated clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "gcp_byovpc_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-gcp789xyz",
				Name:            "testname",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier:  "tier-1-gcp-um4g",
				Region:          "us-central1",
				Zones:           []string{"us-central1-a", "us-central1-b", "us-central1-c"},
				ResourceGroupId: "rg-456789",
				NetworkId:       "net-345678",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.CustomerManagedResources_GCP{
							Subnet: &controlplanev1.CustomerManagedResources_GCP_Subnet{
								Name: "redpanda-subnet-testname",
								SecondaryIpv4RangePods: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "redpanda-pods-testname",
								},
								SecondaryIpv4RangeServices: &controlplanev1.CustomerManagedResources_GCP_Subnet_SecondaryIPv4Range{
									Name: "redpanda-services-testname",
								},
								K8SMasterIpv4Range: "10.0.7.240/28",
							},
							AgentServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-agent-testname@project.iam.gserviceaccount.com",
							},
							ConsoleServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-console-testname@project.iam.gserviceaccount.com",
							},
							ConnectorServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-connector-testname@project.iam.gserviceaccount.com",
							},
							RedpandaClusterServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-cluster-testname@project.iam.gserviceaccount.com",
							},
							GkeServiceAccount: &controlplanev1.GCPServiceAccount{
								Email: "redpanda-gke-testname@project.iam.gserviceaccount.com",
							},
							TieredStorageBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "redpanda-storage-testname",
							},
						},
					},
				},
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"environment": types.StringValue("dev"),
					"managed-by":  types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "rp-gcp789xyz", result.ID.ValueString())
				require.Equal(t, "testname", result.Name.ValueString())
				require.Equal(t, "private", result.ConnectionType.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-gcp-um4g", result.ThroughputTier.ValueString())
				require.Equal(t, "us-central1", result.Region.ValueString())
				require.True(t, result.AllowDeletion.ValueBool())
				// BYOC clusters with private connection should have CustomerManagedResources
				require.False(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "azure_dedicated_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-azure123def",
				Name:            "azure-testname",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-1-azure-v3-x86",
				Region:          "eastus",
				Zones:           []string{"eastus-az1", "eastus-az2", "eastus-az3"},
				ResourceGroupId: "rg-azure123",
				NetworkId:       "net-azure123",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"environment": types.StringValue("production"),
				}),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "rp-azure123def", result.ID.ValueString())
				require.Equal(t, "azure-testname", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "azure", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-azure-v3-x86", result.ThroughputTier.ValueString())
				require.Equal(t, "eastus", result.Region.ValueString())
				require.True(t, result.AllowDeletion.ValueBool())
				// Dedicated clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "aws_byoc_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-awsbyoc123",
				Name:            "aws-byoc-cluster",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier:  "tier-1-aws-v2-x86",
				Region:          "us-east-1",
				Zones:           []string{"use1-az2", "use1-az4", "use1-az6"},
				ResourceGroupId: "rg-byoc123",
				NetworkId:       "net-byoc123",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(false),
				Tags:            types.MapNull(types.StringType),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "rp-awsbyoc123", result.ID.ValueString())
				require.Equal(t, "aws-byoc-cluster", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-aws-v2-x86", result.ThroughputTier.ValueString())
				require.Equal(t, "us-east-1", result.Region.ValueString())
				require.False(t, result.AllowDeletion.ValueBool())
				require.True(t, result.Tags.IsNull())
				// Public BYOC clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "gcp_dedicated_public_cluster_example",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-gcpdedicated",
				Name:            "gcp-dedicated-cluster",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-1-gcp-um4g",
				Region:          "us-central1",
				Zones:           []string{"us-central1-a", "us-central1-b", "us-central1-c"},
				ResourceGroupId: "rg-gcpdedicated",
				NetworkId:       "net-gcpdedicated",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"team":        types.StringValue("data-platform"),
					"cost-center": types.StringValue("engineering"),
				}),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "rp-gcpdedicated", result.ID.ValueString())
				require.Equal(t, "gcp-dedicated-cluster", result.Name.ValueString())
				require.Equal(t, "public", result.ConnectionType.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-gcp-um4g", result.ThroughputTier.ValueString())
				require.Equal(t, "us-central1", result.Region.ValueString())
				require.True(t, result.AllowDeletion.ValueBool())
				// Dedicated clusters should not have CustomerManagedResources
				require.True(t, result.CustomerManagedResources.IsNull())
			},
		},
		{
			name: "aws_byovpc_private_cluster_with_full_cmr",
			cluster: &controlplanev1.Cluster{
				Id:              "rp-awsbyovpc123",
				Name:            "aws-byovpc-cluster",
				ConnectionType:  controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:            controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier:  "tier-1-aws-v2-arm",
				Region:          "us-east-2",
				Zones:           []string{"use2-az1", "use2-az2", "use2-az3"},
				ResourceGroupId: "rg-awsbyovpc",
				NetworkId:       "net-awsbyovpc",
				State:           controlplanev1.Cluster_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.CustomerManagedResources{
					CloudProvider: &controlplanev1.CustomerManagedResources_Aws{
						Aws: &controlplanev1.CustomerManagedResources_AWS{
							AgentInstanceProfile: &controlplanev1.AWSInstanceProfile{
								Arn: "arn:aws:iam::123456789012:instance-profile/redpanda-byovpc-agent-instance-profile",
							},
							CloudStorageBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
								Arn: "arn:aws:s3:::redpanda-byovpc-cloud-storage-bucket",
							},
							K8SClusterRole: &controlplanev1.CustomerManagedResources_AWS_Role{
								Arn: "arn:aws:iam::123456789012:role/redpanda-byovpc-k8s-cluster-role",
							},
							RedpandaAgentSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:us-east-2:123456789012:security-group/sg-agent123",
							},
							ClusterSecurityGroup: &controlplanev1.AWSSecurityGroup{
								Arn: "arn:aws:ec2:us-east-2:123456789012:security-group/sg-cluster123",
							},
						},
					},
				},
			},
			contingent: ContingentFields{
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"deployment": types.StringValue("byovpc"),
					"managed-by": types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "rp-awsbyovpc123", result.ID.ValueString())
				require.Equal(t, "aws-byovpc-cluster", result.Name.ValueString())
				require.Equal(t, "private", result.ConnectionType.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "tier-1-aws-v2-arm", result.ThroughputTier.ValueString())
				require.Equal(t, "us-east-2", result.Region.ValueString())
				require.True(t, result.AllowDeletion.ValueBool())
				// Private BYOC clusters should have CustomerManagedResources
				require.False(t, result.CustomerManagedResources.IsNull())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &ResourceModel{}
			result, diags := model.GetUpdatedModel(ctx, tt.cluster, tt.contingent)

			require.False(t, diags.HasError())
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestResourceModel_GetClusterCreate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		model    *ResourceModel
		validate func(t *testing.T, result *controlplanev1.ClusterCreate)
	}{
		{
			name: "aws_dedicated_cluster_create",
			model: &ResourceModel{
				Name:            types.StringValue("testname"),
				ConnectionType:  types.StringValue("public"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("dedicated"),
				ThroughputTier:  types.StringValue("tier-1-aws-v2-arm"),
				Region:          types.StringValue("us-east-2"),
				Zones:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("use2-az1"), types.StringValue("use2-az2"), types.StringValue("use2-az3")}),
				ResourceGroupID: types.StringValue("rg-123456"),
				NetworkID:       types.StringValue("net-789012"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"key": types.StringValue("value"),
				}),
			},
			validate: func(t *testing.T, result *controlplanev1.ClusterCreate) {
				require.Equal(t, "testname", result.GetName())
				require.Equal(t, controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, result.GetConnectionType())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_DEDICATED, result.GetType())
				require.Equal(t, "tier-1-aws-v2-arm", result.GetThroughputTier())
				require.Equal(t, "us-east-2", result.GetRegion())
				require.Equal(t, []string{"use2-az1", "use2-az2", "use2-az3"}, result.GetZones())
				require.Equal(t, "rg-123456", result.GetResourceGroupId())
				require.Equal(t, "net-789012", result.GetNetworkId())
				require.Equal(t, "v24.3.1", result.GetRedpandaVersion())
				// Dedicated clusters should not have CustomerManagedResources
				require.False(t, result.HasCustomerManagedResources())
			},
		},
		{
			name: "gcp_byovpc_cluster_create",
			model: &ResourceModel{
				Name:            types.StringValue("testname"),
				ConnectionType:  types.StringValue("private"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("byoc"),
				ThroughputTier:  types.StringValue("tier-1-gcp-um4g"),
				Region:          types.StringValue("us-central1"),
				Zones:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("us-central1-a"), types.StringValue("us-central1-b"), types.StringValue("us-central1-c")}),
				ResourceGroupID: types.StringValue("rg-456789"),
				NetworkID:       types.StringValue("net-345678"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				CustomerManagedResources: types.ObjectValueMust(
					getCustomerManagedResourcesType(),
					map[string]attr.Value{
						"aws": types.ObjectNull(getAwsCmrType()),
						"gcp": types.ObjectValueMust(
							getGcpCmrType(),
							map[string]attr.Value{
								"subnet": types.ObjectValueMust(
									getGcpSubnetType(),
									map[string]attr.Value{
										"name": types.StringValue("redpanda-subnet-testname"),
										"secondary_ipv4_range_pods": types.ObjectValueMust(
											getGcpSecondaryIPv4RangeType(),
											map[string]attr.Value{
												"name": types.StringValue("redpanda-pods-testname"),
											},
										),
										"secondary_ipv4_range_services": types.ObjectValueMust(
											getGcpSecondaryIPv4RangeType(),
											map[string]attr.Value{
												"name": types.StringValue("redpanda-services-testname"),
											},
										),
										"k8s_master_ipv4_range": types.StringValue("10.0.7.240/28"),
									},
								),
								"agent_service_account": types.ObjectValueMust(
									getGcpServiceAccountType(),
									map[string]attr.Value{
										"email": types.StringValue("redpanda-agent-testname@project.iam.gserviceaccount.com"),
									},
								),
								"console_service_account": types.ObjectValueMust(
									getGcpServiceAccountType(),
									map[string]attr.Value{
										"email": types.StringValue("redpanda-console-testname@project.iam.gserviceaccount.com"),
									},
								),
								"connector_service_account": types.ObjectValueMust(
									getGcpServiceAccountType(),
									map[string]attr.Value{
										"email": types.StringValue("redpanda-connector-testname@project.iam.gserviceaccount.com"),
									},
								),
								"redpanda_cluster_service_account": types.ObjectValueMust(
									getGcpServiceAccountType(),
									map[string]attr.Value{
										"email": types.StringValue("redpanda-cluster-testname@project.iam.gserviceaccount.com"),
									},
								),
								"gke_service_account": types.ObjectValueMust(
									getGcpServiceAccountType(),
									map[string]attr.Value{
										"email": types.StringValue("redpanda-gke-testname@project.iam.gserviceaccount.com"),
									},
								),
								"tiered_storage_bucket": types.ObjectValueMust(
									getGcpBucketType(),
									map[string]attr.Value{
										"name": types.StringValue("redpanda-storage-testname"),
									},
								),
								"psc_nat_subnet_name": types.StringNull(),
							},
						),
					},
				),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"environment": types.StringValue("dev"),
					"managed-by":  types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *controlplanev1.ClusterCreate) {
				require.Equal(t, "testname", result.GetName())
				require.Equal(t, controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE, result.GetConnectionType())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetType())
				require.Equal(t, "tier-1-gcp-um4g", result.GetThroughputTier())
				require.Equal(t, "us-central1", result.GetRegion())
				require.True(t, result.HasCustomerManagedResources())
				require.True(t, result.GetCustomerManagedResources().HasGcp())
			},
		},
		{
			name: "azure_dedicated_cluster_create",
			model: &ResourceModel{
				Name:            types.StringValue("azure-cluster"),
				ConnectionType:  types.StringValue("public"),
				CloudProvider:   types.StringValue("azure"),
				ClusterType:     types.StringValue("dedicated"),
				ThroughputTier:  types.StringValue("tier-1-azure-v3-x86"),
				Region:          types.StringValue("eastus"),
				Zones:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("eastus-az1"), types.StringValue("eastus-az2"), types.StringValue("eastus-az3")}),
				ResourceGroupID: types.StringValue("rg-azure"),
				NetworkID:       types.StringValue("net-azure"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"environment": types.StringValue("production"),
					"team":        types.StringValue("platform"),
				}),
			},
			validate: func(t *testing.T, result *controlplanev1.ClusterCreate) {
				require.Equal(t, "azure-cluster", result.GetName())
				require.Equal(t, controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, result.GetConnectionType())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_DEDICATED, result.GetType())
				require.Equal(t, "tier-1-azure-v3-x86", result.GetThroughputTier())
				require.Equal(t, "eastus", result.GetRegion())
				// Dedicated clusters should not have CustomerManagedResources
				require.False(t, result.HasCustomerManagedResources())
			},
		},
		{
			name: "aws_byoc_cluster_create",
			model: &ResourceModel{
				Name:            types.StringValue("aws-byoc-cluster"),
				ConnectionType:  types.StringValue("public"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("byoc"),
				ThroughputTier:  types.StringValue("tier-1-aws-v2-x86"),
				Region:          types.StringValue("us-east-1"),
				Zones:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("use1-az2"), types.StringValue("use1-az4"), types.StringValue("use1-az6")}),
				ResourceGroupID: types.StringValue("rg-byoc"),
				NetworkID:       types.StringValue("net-byoc"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(false),
				Tags:            types.MapNull(types.StringType),
			},
			validate: func(t *testing.T, result *controlplanev1.ClusterCreate) {
				require.Equal(t, "aws-byoc-cluster", result.GetName())
				require.Equal(t, controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, result.GetConnectionType())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetType())
				require.Equal(t, "tier-1-aws-v2-x86", result.GetThroughputTier())
				require.Equal(t, "us-east-1", result.GetRegion())
				// Public BYOC clusters should not have CustomerManagedResources
				require.False(t, result.HasCustomerManagedResources())
			},
		},
		{
			name: "gcp_dedicated_cluster_create",
			model: &ResourceModel{
				Name:            types.StringValue("gcp-dedicated-cluster"),
				ConnectionType:  types.StringValue("public"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("dedicated"),
				ThroughputTier:  types.StringValue("tier-1-gcp-um4g"),
				Region:          types.StringValue("us-central1"),
				Zones:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("us-central1-a"), types.StringValue("us-central1-b"), types.StringValue("us-central1-c")}),
				ResourceGroupID: types.StringValue("rg-gcp-dedicated"),
				NetworkID:       types.StringValue("net-gcp-dedicated"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"cost-center": types.StringValue("engineering"),
					"managed-by":  types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *controlplanev1.ClusterCreate) {
				require.Equal(t, "gcp-dedicated-cluster", result.GetName())
				require.Equal(t, controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, result.GetConnectionType())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_DEDICATED, result.GetType())
				require.Equal(t, "tier-1-gcp-um4g", result.GetThroughputTier())
				require.Equal(t, "us-central1", result.GetRegion())
				// Dedicated clusters should not have CustomerManagedResources
				require.False(t, result.HasCustomerManagedResources())
			},
		},
		{
			name: "aws_byovpc_cluster_create_with_cmr",
			model: &ResourceModel{
				Name:            types.StringValue("aws-byovpc-cluster"),
				ConnectionType:  types.StringValue("private"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("byoc"),
				ThroughputTier:  types.StringValue("tier-1-aws-v2-arm"),
				Region:          types.StringValue("us-east-2"),
				Zones:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("use2-az1"), types.StringValue("use2-az2"), types.StringValue("use2-az3")}),
				ResourceGroupID: types.StringValue("rg-awsbyovpc"),
				NetworkID:       types.StringValue("net-awsbyovpc"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				AllowDeletion:   types.BoolValue(true),
				CustomerManagedResources: types.ObjectValueMust(
					getCustomerManagedResourcesType(),
					map[string]attr.Value{
						"gcp": types.ObjectNull(getGcpCmrType()),
						"aws": types.ObjectValueMust(
							getAwsCmrType(),
							map[string]attr.Value{
								"agent_instance_profile": types.ObjectValueMust(
									getArnContainerType(),
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:iam::123456789012:instance-profile/redpanda-byovpc-agent-instance-profile"),
									},
								),
								"cloud_storage_bucket": types.ObjectValueMust(
									getArnContainerType(),
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:s3:::redpanda-byovpc-cloud-storage-bucket"),
									},
								),
								"k8s_cluster_role": types.ObjectValueMust(
									getArnContainerType(),
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:iam::123456789012:role/redpanda-byovpc-k8s-cluster-role"),
									},
								),
								"redpanda_agent_security_group": types.ObjectValueMust(
									getArnContainerType(),
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:ec2:us-east-2:123456789012:security-group/sg-agent123"),
									},
								),
								"cluster_security_group": types.ObjectValueMust(
									getArnContainerType(),
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:ec2:us-east-2:123456789012:security-group/sg-cluster123"),
									},
								),
								"connectors_node_group_instance_profile": types.ObjectNull(getArnContainerType()),
								"utility_node_group_instance_profile":    types.ObjectNull(getArnContainerType()),
								"redpanda_node_group_instance_profile":   types.ObjectNull(getArnContainerType()),
								"connectors_security_group":              types.ObjectNull(getArnContainerType()),
								"redpanda_node_group_security_group":     types.ObjectNull(getArnContainerType()),
								"utility_security_group":                 types.ObjectNull(getArnContainerType()),
								"node_security_group":                    types.ObjectNull(getArnContainerType()),
								"permissions_boundary_policy":            types.ObjectNull(getArnContainerType()),
							},
						),
					},
				),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"deployment": types.StringValue("byovpc"),
					"managed-by": types.StringValue("terraform"),
				}),
			},
			validate: func(t *testing.T, result *controlplanev1.ClusterCreate) {
				require.Equal(t, "aws-byovpc-cluster", result.GetName())
				require.Equal(t, controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE, result.GetConnectionType())
				require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, result.GetCloudProvider())
				require.Equal(t, controlplanev1.Cluster_TYPE_BYOC, result.GetType())
				require.Equal(t, "tier-1-aws-v2-arm", result.GetThroughputTier())
				require.Equal(t, "us-east-2", result.GetRegion())
				require.True(t, result.HasCustomerManagedResources())
				require.True(t, result.GetCustomerManagedResources().HasAws())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := tt.model.GetClusterCreate(ctx)

			require.False(t, diags.HasError())
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestResourceModel_GetClusterUpdateRequest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		model         *ResourceModel
		previousState *ResourceModel
		validate      func(t *testing.T, result *controlplanev1.UpdateClusterRequest)
	}{
		{
			name: "update_tags_and_version",
			model: &ResourceModel{
				ID:              types.StringValue("rp-abc123def"),
				RedpandaVersion: types.StringValue("v24.3.2"),
				ThroughputTier:  types.StringValue("tier2"),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"key":         types.StringValue("new-value"),
					"environment": types.StringValue("production"),
				}),
			},
			previousState: &ResourceModel{
				ID:              types.StringValue("rp-abc123def"),
				RedpandaVersion: types.StringValue("v24.3.1"),
				ThroughputTier:  types.StringValue("tier1"),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"key": types.StringValue("value"),
				}),
			},
			validate: func(t *testing.T, result *controlplanev1.UpdateClusterRequest) {
				require.NotNil(t, result.GetCluster())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := tt.model.GetClusterUpdateRequest(ctx, tt.previousState)

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
				ID: types.StringValue("rp-abc123def"),
			},
			expected: "rp-abc123def",
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

func TestGenerateMinimalResourceModel(t *testing.T) {
	clusterID := "rp-test123"
	result := GenerateMinimalResourceModel(clusterID, timeouts.Value{})

	require.NotNil(t, result)
	require.Equal(t, clusterID, result.ID.ValueString())
	require.True(t, result.Name.IsNull())
	require.True(t, result.KafkaAPI.IsNull())
	require.True(t, result.HTTPProxy.IsNull())
	require.True(t, result.SchemaRegistry.IsNull())
}

func TestResourceModel_GenerateClusterConfiguration(t *testing.T) {
	model := &ResourceModel{}

	t.Run("with cluster configuration and custom properties", func(t *testing.T) {
		customProps, err := structpb.NewStruct(map[string]any{
			"auto.create.topics.enable": true,
			"log.segment.bytes":         "1073741824",
			"retention.ms":              "604800000",
			"compaction.type":           "cleanup",
		})
		require.NoError(t, err)

		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: customProps,
			},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		require.False(t, diags.HasError())
		require.False(t, result.IsNull())

		attrs := result.Attributes()
		customPropsJSONAttr, ok := attrs["custom_properties_json"].(types.String)
		require.True(t, ok)

		jsonStr := customPropsJSONAttr.ValueString()
		require.Contains(t, jsonStr, "auto.create.topics.enable")
		require.Contains(t, jsonStr, "log.segment.bytes")
		require.Contains(t, jsonStr, "retention.ms")
		require.Contains(t, jsonStr, "compaction.type")

		var parsedJSON map[string]any
		err = json.Unmarshal([]byte(jsonStr), &parsedJSON)
		require.NoError(t, err)
		require.Equal(t, true, parsedJSON["auto.create.topics.enable"])
		require.Equal(t, "1073741824", parsedJSON["log.segment.bytes"])
		require.Equal(t, "604800000", parsedJSON["retention.ms"])
		require.Equal(t, "cleanup", parsedJSON["compaction.type"])
	})

	t.Run("with cluster configuration but no custom properties", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		require.False(t, diags.HasError())
		require.True(t, result.IsNull())
	})

	t.Run("with cluster configuration and empty custom properties", func(t *testing.T) {
		customProps, err := structpb.NewStruct(map[string]any{})
		require.NoError(t, err)

		cluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: customProps,
			},
		}

		result, diags := model.generateModelClusterConfiguration(cluster)

		require.False(t, diags.HasError())
		require.True(t, result.IsNull(), "Empty custom properties should return null to avoid plan/apply consistency issues")
	})

	t.Run("without cluster configuration", func(t *testing.T) {
		cluster := &controlplanev1.Cluster{}

		result, diags := model.generateModelClusterConfiguration(cluster)

		require.False(t, diags.HasError())
		require.True(t, result.IsNull())
	})

	t.Run("consistency test - simulate plan vs apply scenario", func(t *testing.T) {
		planCluster := &controlplanev1.Cluster{}
		planResult, planDiags := model.generateModelClusterConfiguration(planCluster)
		require.False(t, planDiags.HasError())
		require.True(t, planResult.IsNull(), "Plan phase should return null when no cluster configuration")

		applyCluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{},
		}
		applyResult, applyDiags := model.generateModelClusterConfiguration(applyCluster)
		require.False(t, applyDiags.HasError())
		require.True(t, applyResult.IsNull(), "Apply phase should also return null when cluster configuration has no custom properties")
		require.Equal(t, planResult.IsNull(), applyResult.IsNull(), "Plan and Apply phases should be consistent")
	})
}

func TestClusterConfigurationConsistency(t *testing.T) {
	t.Run("plan_apply_consistency_empty_config", func(t *testing.T) {
		planModel := &ResourceModel{
			ClusterConfiguration: types.ObjectNull(getClusterConfigurationType()),
		}

		applyCluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{},
		}

		applyResult, diags := planModel.generateModelClusterConfiguration(applyCluster)
		require.False(t, diags.HasError())
		require.True(t, applyResult.IsNull(),
			"When cluster has empty ClusterConfiguration, result should be null to match plan")
	})

	t.Run("plan_apply_consistency_with_empty_json", func(t *testing.T) {
		planModel := &ResourceModel{
			ClusterConfiguration: types.ObjectNull(getClusterConfigurationType()),
		}

		emptyProps, err := structpb.NewStruct(map[string]any{})
		require.NoError(t, err)

		applyCluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: emptyProps,
			},
		}

		applyResult, diags := planModel.generateModelClusterConfiguration(applyCluster)
		require.False(t, diags.HasError())
		require.True(t, applyResult.IsNull(),
			"When cluster has empty custom properties, result should be null to match plan")
	})

	t.Run("plan_apply_consistency_object_with_null_field", func(t *testing.T) {
		planModel := &ResourceModel{
			ClusterConfiguration: types.ObjectValueMust(
				getClusterConfigurationType(),
				map[string]attr.Value{
					"custom_properties_json": types.StringNull(),
				},
			),
		}

		applyCluster := &controlplanev1.Cluster{}

		applyResult, diags := planModel.generateModelClusterConfiguration(applyCluster)
		require.False(t, diags.HasError())
		require.True(t, applyResult.IsNull(),
			"When cluster has no configuration, result should be null")
	})

	t.Run("plan_apply_consistency_with_actual_properties", func(t *testing.T) {
		planModel := &ResourceModel{
			ClusterConfiguration: types.ObjectValueMust(
				getClusterConfigurationType(),
				map[string]attr.Value{
					"custom_properties_json": types.StringValue(`{"log.retention.ms":"604800000"}`),
				},
			),
		}

		customProps, err := structpb.NewStruct(map[string]any{
			"log.retention.ms": "604800000",
		})
		require.NoError(t, err)

		applyCluster := &controlplanev1.Cluster{
			ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{
				CustomProperties: customProps,
			},
		}

		applyResult, diags := planModel.generateModelClusterConfiguration(applyCluster)
		require.False(t, diags.HasError())
		require.False(t, applyResult.IsNull(),
			"When cluster has actual custom properties, result should not be null")

		attrs := applyResult.Attributes()
		customPropsJSON, ok := attrs["custom_properties_json"].(types.String)
		require.True(t, ok)
		require.Contains(t, customPropsJSON.ValueString(), "log.retention.ms")
	})
}

func TestResourceModel_ComprehensiveConsistencyTest(t *testing.T) {
	ctx := context.Background()

	compareModels := func(t *testing.T, planModel, applyModel *ResourceModel, fieldName string) {
		t.Helper()

		switch fieldName {
		case "Name":
			require.Equal(t, planModel.Name.IsNull(), applyModel.Name.IsNull(), "Name null status should be consistent")
			if !planModel.Name.IsNull() && !applyModel.Name.IsNull() {
				require.Equal(t, planModel.Name.ValueString(), applyModel.Name.ValueString(), "Name value should be consistent")
			}
		case "ID":
			require.Equal(t, planModel.ID.IsNull(), applyModel.ID.IsNull(), "ID null status should be consistent")
			if !planModel.ID.IsNull() && !applyModel.ID.IsNull() {
				require.Equal(t, planModel.ID.ValueString(), applyModel.ID.ValueString(), "ID value should be consistent")
			}
		case "ConnectionType":
			require.Equal(t, planModel.ConnectionType.IsNull(), applyModel.ConnectionType.IsNull(), "ConnectionType null status should be consistent")
			if !planModel.ConnectionType.IsNull() && !applyModel.ConnectionType.IsNull() {
				require.Equal(t, planModel.ConnectionType.ValueString(), applyModel.ConnectionType.ValueString(), "ConnectionType value should be consistent")
			}
		case "CloudProvider":
			require.Equal(t, planModel.CloudProvider.IsNull(), applyModel.CloudProvider.IsNull(), "CloudProvider null status should be consistent")
			if !planModel.CloudProvider.IsNull() && !applyModel.CloudProvider.IsNull() {
				require.Equal(t, planModel.CloudProvider.ValueString(), applyModel.CloudProvider.ValueString(), "CloudProvider value should be consistent")
			}
		case "ClusterType":
			require.Equal(t, planModel.ClusterType.IsNull(), applyModel.ClusterType.IsNull(), "ClusterType null status should be consistent")
			if !planModel.ClusterType.IsNull() && !applyModel.ClusterType.IsNull() {
				require.Equal(t, planModel.ClusterType.ValueString(), applyModel.ClusterType.ValueString(), "ClusterType value should be consistent")
			}
		case "RedpandaVersion":
			require.Equal(t, planModel.RedpandaVersion.IsNull(), applyModel.RedpandaVersion.IsNull(), "RedpandaVersion null status should be consistent")
			if !planModel.RedpandaVersion.IsNull() && !applyModel.RedpandaVersion.IsNull() {
				require.Equal(t, planModel.RedpandaVersion.ValueString(), applyModel.RedpandaVersion.ValueString(), "RedpandaVersion value should be consistent")
			}
		case "ThroughputTier":
			require.Equal(t, planModel.ThroughputTier.IsNull(), applyModel.ThroughputTier.IsNull(), "ThroughputTier null status should be consistent")
			if !planModel.ThroughputTier.IsNull() && !applyModel.ThroughputTier.IsNull() {
				require.Equal(t, planModel.ThroughputTier.ValueString(), applyModel.ThroughputTier.ValueString(), "ThroughputTier value should be consistent")
			}
		case "Region":
			require.Equal(t, planModel.Region.IsNull(), applyModel.Region.IsNull(), "Region null status should be consistent")
			if !planModel.Region.IsNull() && !applyModel.Region.IsNull() {
				require.Equal(t, planModel.Region.ValueString(), applyModel.Region.ValueString(), "Region value should be consistent")
			}
		case "Zones":
			require.Equal(t, planModel.Zones.IsNull(), applyModel.Zones.IsNull(), "Zones null status should be consistent")
			if !planModel.Zones.IsNull() && !applyModel.Zones.IsNull() {
				require.Equal(t, len(planModel.Zones.Elements()), len(applyModel.Zones.Elements()), "Zones length should be consistent")
			}
		case "AllowDeletion":
			require.Equal(t, planModel.AllowDeletion.IsNull(), applyModel.AllowDeletion.IsNull(), "AllowDeletion null status should be consistent")
			if !planModel.AllowDeletion.IsNull() && !applyModel.AllowDeletion.IsNull() {
				require.Equal(t, planModel.AllowDeletion.ValueBool(), applyModel.AllowDeletion.ValueBool(), "AllowDeletion value should be consistent")
			}
		case "CreatedAt":
			if !planModel.CreatedAt.IsNull() || applyModel.CreatedAt.IsNull() {
				require.Equal(t, planModel.CreatedAt.IsNull(), applyModel.CreatedAt.IsNull(), "CreatedAt null status should be consistent when both are set")
				if !planModel.CreatedAt.IsNull() && !applyModel.CreatedAt.IsNull() {
					require.Equal(t, planModel.CreatedAt.ValueString(), applyModel.CreatedAt.ValueString(), "CreatedAt value should be consistent")
				}
			}
		case "State":
			require.False(t, planModel.State.IsNull(), "Plan state should not be null")
			require.False(t, applyModel.State.IsNull(), "Apply state should not be null")
		case "StateDescription":
			require.Equal(t, planModel.StateDescription.IsNull(), applyModel.StateDescription.IsNull(), "StateDescription null status should be consistent")
		case "Tags":
			require.Equal(t, planModel.Tags.IsNull(), applyModel.Tags.IsNull(), "Tags null status should be consistent")
		case "ResourceGroupID":
			require.Equal(t, planModel.ResourceGroupID.IsNull(), applyModel.ResourceGroupID.IsNull(), "ResourceGroupID null status should be consistent")
			if !planModel.ResourceGroupID.IsNull() && !applyModel.ResourceGroupID.IsNull() {
				require.Equal(t, planModel.ResourceGroupID.ValueString(), applyModel.ResourceGroupID.ValueString(), "ResourceGroupID value should be consistent")
			}
		case "NetworkID":
			require.Equal(t, planModel.NetworkID.IsNull(), applyModel.NetworkID.IsNull(), "NetworkID null status should be consistent")
			if !planModel.NetworkID.IsNull() && !applyModel.NetworkID.IsNull() {
				require.Equal(t, planModel.NetworkID.ValueString(), applyModel.NetworkID.ValueString(), "NetworkID value should be consistent")
			}
		case "ClusterAPIURL":
			if !planModel.ClusterAPIURL.IsNull() || applyModel.ClusterAPIURL.IsNull() {
				require.Equal(t, planModel.ClusterAPIURL.IsNull(), applyModel.ClusterAPIURL.IsNull(), "ClusterAPIURL null status should be consistent when both are set")
				if !planModel.ClusterAPIURL.IsNull() && !applyModel.ClusterAPIURL.IsNull() {
					require.Equal(t, planModel.ClusterAPIURL.ValueString(), applyModel.ClusterAPIURL.ValueString(), "ClusterAPIURL value should be consistent")
				}
			}
		case "AWSPrivateLink":
			require.Equal(t, planModel.AWSPrivateLink.IsNull(), applyModel.AWSPrivateLink.IsNull(), "AWSPrivateLink null status should be consistent")
		case "GCPPrivateServiceConnect":
			require.Equal(t, planModel.GCPPrivateServiceConnect.IsNull(), applyModel.GCPPrivateServiceConnect.IsNull(), "GCPPrivateServiceConnect null status should be consistent")
		case "AzurePrivateLink":
			require.Equal(t, planModel.AzurePrivateLink.IsNull(), applyModel.AzurePrivateLink.IsNull(), "AzurePrivateLink null status should be consistent")
		case "KafkaAPI":
			require.Equal(t, planModel.KafkaAPI.IsNull(), applyModel.KafkaAPI.IsNull(), "KafkaAPI null status should be consistent")
		case "HTTPProxy":
			require.Equal(t, planModel.HTTPProxy.IsNull(), applyModel.HTTPProxy.IsNull(), "HTTPProxy null status should be consistent")
		case "SchemaRegistry":
			require.Equal(t, planModel.SchemaRegistry.IsNull(), applyModel.SchemaRegistry.IsNull(), "SchemaRegistry null status should be consistent")
		case "KafkaConnect":
			require.Equal(t, planModel.KafkaConnect.IsNull(), applyModel.KafkaConnect.IsNull(), "KafkaConnect null status should be consistent")
		case "ReadReplicaClusterIds":
			require.Equal(t, planModel.ReadReplicaClusterIds.IsNull(), applyModel.ReadReplicaClusterIds.IsNull(), "ReadReplicaClusterIds null status should be consistent")
		case "CustomerManagedResources":
			require.Equal(t, planModel.CustomerManagedResources.IsNull(), applyModel.CustomerManagedResources.IsNull(), "CustomerManagedResources null status should be consistent")
		case "Prometheus":
			require.Equal(t, planModel.Prometheus.IsNull(), applyModel.Prometheus.IsNull(), "Prometheus null status should be consistent")
		case "RedpandaConsole":
			require.Equal(t, planModel.RedpandaConsole.IsNull(), applyModel.RedpandaConsole.IsNull(), "RedpandaConsole null status should be consistent")
		case "MaintenanceWindowConfig":
			require.Equal(t, planModel.MaintenanceWindowConfig.IsNull(), applyModel.MaintenanceWindowConfig.IsNull(), "MaintenanceWindowConfig null status should be consistent")
		case "GCPGlobalAccessEnabled":
			require.Equal(t, planModel.GCPGlobalAccessEnabled.IsNull(), applyModel.GCPGlobalAccessEnabled.IsNull(), "GCPGlobalAccessEnabled null status should be consistent")
			if !planModel.GCPGlobalAccessEnabled.IsNull() && !applyModel.GCPGlobalAccessEnabled.IsNull() {
				require.Equal(t, planModel.GCPGlobalAccessEnabled.ValueBool(), applyModel.GCPGlobalAccessEnabled.ValueBool(), "GCPGlobalAccessEnabled value should be consistent")
			}
		case "ClusterConfiguration":
			require.Equal(t, planModel.ClusterConfiguration.IsNull(), applyModel.ClusterConfiguration.IsNull(), "ClusterConfiguration null status should be consistent")
		}
	}

	testCases := []struct {
		name            string
		planCluster     *controlplanev1.Cluster
		applyCluster    *controlplanev1.Cluster
		contingentPlan  ContingentFields
		contingentApply ContingentFields
	}{
		{
			name: "minimal_cluster_with_progressive_addition",
			planCluster: &controlplanev1.Cluster{
				Id:             "rp-test123",
				Name:           "test-cluster",
				ConnectionType: controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:           controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier: "tier-1",
				Region:         "us-east-1",
				State:          controlplanev1.Cluster_STATE_CREATING,
			},
			applyCluster: &controlplanev1.Cluster{
				Id:             "rp-test123",
				Name:           "test-cluster",
				ConnectionType: controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:           controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier: "tier-1",
				Region:         "us-east-1",
				State:          controlplanev1.Cluster_STATE_READY,
				CreatedAt:      timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
					Url: "https://test.example.com",
				},
				ClusterConfiguration: &controlplanev1.Cluster_ClusterConfiguration{},
			},
			contingentPlan: ContingentFields{
				RedpandaVersion: types.StringValue("v24.1.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags:            types.MapNull(types.StringType),
			},
			contingentApply: ContingentFields{
				RedpandaVersion: types.StringValue("v24.1.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags:            types.MapNull(types.StringType),
			},
		},
		{
			name: "cluster_with_optional_features_empty",
			planCluster: &controlplanev1.Cluster{
				Id:             "rp-test456",
				Name:           "test-cluster-2",
				ConnectionType: controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:           controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier: "tier-2",
				Region:         "us-central1",
				State:          controlplanev1.Cluster_STATE_CREATING,
			},
			applyCluster: &controlplanev1.Cluster{
				Id:             "rp-test456",
				Name:           "test-cluster-2",
				ConnectionType: controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE,
				CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:           controlplanev1.Cluster_TYPE_BYOC,
				ThroughputTier: "tier-2",
				Region:         "us-central1",
				State:          controlplanev1.Cluster_STATE_READY,
				CreatedAt:      timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				KafkaConnect: &controlplanev1.KafkaConnect{
					Enabled: false,
				},
				AwsPrivateLink: &controlplanev1.Cluster_AWSPrivateLink{
					Enabled: false,
				},
			},
			contingentPlan: ContingentFields{
				RedpandaVersion: types.StringValue("v24.1.1"),
				AllowDeletion:   types.BoolValue(false),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"env": types.StringValue("test"),
				}),
			},
			contingentApply: ContingentFields{
				RedpandaVersion: types.StringValue("v24.1.1"),
				AllowDeletion:   types.BoolValue(false),
				Tags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"env": types.StringValue("test"),
				}),
			},
		},
		{
			name: "cluster_with_enabled_features",
			planCluster: &controlplanev1.Cluster{
				Id:             "rp-test789",
				Name:           "test-cluster-3",
				ConnectionType: controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:           controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier: "tier-1",
				Region:         "us-west-2",
				State:          controlplanev1.Cluster_STATE_CREATING,
				KafkaApi: &controlplanev1.Cluster_KafkaAPI{
					SeedBrokers: []string{"broker1:9092"},
				},
				KafkaConnect: &controlplanev1.KafkaConnect{
					Enabled: true,
				},
				Prometheus: &controlplanev1.Cluster_Prometheus{
					Url: "https://prometheus.example.com",
				},
			},
			applyCluster: &controlplanev1.Cluster{
				Id:             "rp-test789",
				Name:           "test-cluster-3",
				ConnectionType: controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:  controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				Type:           controlplanev1.Cluster_TYPE_DEDICATED,
				ThroughputTier: "tier-1",
				Region:         "us-west-2",
				State:          controlplanev1.Cluster_STATE_READY,
				CreatedAt:      timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				KafkaApi: &controlplanev1.Cluster_KafkaAPI{
					SeedBrokers: []string{"broker1:9092"},
				},
				KafkaConnect: &controlplanev1.KafkaConnect{
					Enabled: true,
				},
				Prometheus: &controlplanev1.Cluster_Prometheus{
					Url: "https://prometheus.example.com",
				},
			},
			contingentPlan: ContingentFields{
				RedpandaVersion: types.StringValue("v24.1.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags:            types.MapNull(types.StringType),
			},
			contingentApply: ContingentFields{
				RedpandaVersion: types.StringValue("v24.1.1"),
				AllowDeletion:   types.BoolValue(true),
				Tags:            types.MapNull(types.StringType),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			planModel := &ResourceModel{}
			planResult, planDiags := planModel.GetUpdatedModel(ctx, tc.planCluster, tc.contingentPlan)
			require.False(t, planDiags.HasError(), "Plan phase should not have errors")
			require.NotNil(t, planResult, "Plan result should not be nil")

			applyModel := &ResourceModel{}
			applyResult, applyDiags := applyModel.GetUpdatedModel(ctx, tc.applyCluster, tc.contingentApply)
			require.False(t, applyDiags.HasError(), "Apply phase should not have errors")
			require.NotNil(t, applyResult, "Apply result should not be nil")
			fields := []string{
				"Name", "ID", "ConnectionType", "CloudProvider", "ClusterType",
				"RedpandaVersion", "ThroughputTier", "Region", "Zones", "AllowDeletion",
				"CreatedAt", "State", "StateDescription", "Tags", "ResourceGroupID",
				"NetworkID", "ClusterAPIURL", "AwsPrivateLink", "GcpPrivateServiceConnect",
				"AzurePrivateLink", "KafkaAPI", "HTTPProxy", "SchemaRegistry",
				"KafkaConnect", "ReadReplicaClusterIDs", "CustomerManagedResources",
				"Prometheus", "RedpandaConsole", "MaintenanceWindowConfig",
				"GCPGlobalAccessEnabled", "ClusterConfiguration",
			}

			for _, field := range fields {
				t.Run(field, func(t *testing.T) {
					compareModels(t, planResult, applyResult, field)
				})
			}
		})
	}
}
