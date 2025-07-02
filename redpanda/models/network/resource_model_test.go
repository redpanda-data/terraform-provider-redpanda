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

func TestResourceModel_GetUpdatedModel(t *testing.T) {
	ctx := context.Background()
	createdTime := time.Now()

	tests := []struct {
		name     string
		network  *controlplanev1.Network
		validate func(t *testing.T, result *ResourceModel)
	}{
		{
			name: "aws_dedicated_network_example",
			network: &controlplanev1.Network{
				Id:              "net-abc123def",
				Name:            "testname",
				ResourceGroupId: "rg-123456",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				Region:          "us-east-2",
				CidrBlock:       "10.0.0.0/20",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-abc123def", result.ID.ValueString())
				require.Equal(t, "testname", result.Name.ValueString())
				require.Equal(t, "rg-123456", result.ResourceGroupID.ValueString())
				require.Equal(t, "aws", result.CloudProvider.ValueString())
				require.Equal(t, "dedicated", result.ClusterType.ValueString())
				require.Equal(t, "us-east-2", result.Region.ValueString())
				require.Equal(t, "10.0.0.0/20", result.CidrBlock.ValueString())
			},
		},
		{
			name: "gcp_byovpc_network_with_cmr",
			network: &controlplanev1.Network{
				Id:              "net-gcp789xyz",
				Name:            "testname",
				ResourceGroupId: "rg-456789",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				Region:          "us-central1",
				State:           controlplanev1.Network_STATE_READY,
				CreatedAt:       timestamppb.New(createdTime),
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{
							NetworkName:      "redpanda-network-vpc",
							NetworkProjectId: "hallowed-ray-376320",
							ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "redpanda-management-bucket-testname",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *ResourceModel) {
				require.Equal(t, "net-gcp789xyz", result.ID.ValueString())
				require.Equal(t, "testname", result.Name.ValueString())
				require.Equal(t, "gcp", result.CloudProvider.ValueString())
				require.Equal(t, "byoc", result.ClusterType.ValueString())
				require.Equal(t, "us-central1", result.Region.ValueString())
				require.False(t, result.CustomerManagedResources.IsNull())
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
