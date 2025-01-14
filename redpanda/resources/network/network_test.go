package network

import (
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
)

func Test_generateModel(t *testing.T) {
	type args struct {
		cloudProvider string
		nw            *controlplanev1beta2.Network
	}
	tests := []struct {
		name string
		args args
		want *network.Network
	}{
		{
			name: "aws dedicated",
			args: args{
				cloudProvider: "aws",
				nw: &controlplanev1beta2.Network{
					Id:              "1",
					Name:            "test",
					ResourceGroupId: "1",
					CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS,
					Region:          "us-west-2",
					CidrBlock:       "1.2.3.4/24",
					ClusterType:     controlplanev1beta2.Cluster_TYPE_DEDICATED,
				},
			},
			want: &network.Network{
				ID:              types.StringValue("1"),
				Name:            types.StringValue("test"),
				ResourceGroupID: types.StringValue("1"),
				CloudProvider:   types.StringValue("aws"),
				Region:          types.StringValue("us-west-2"),
				CidrBlock:       types.StringValue("1.2.3.4/24"),
				ClusterType:     types.StringValue("dedicated"),
			},
		},
		{
			name: "gcp dedicated",
			args: args{
				cloudProvider: "gcp",
				nw: &controlplanev1beta2.Network{
					Id:              "1",
					Name:            "test",
					ResourceGroupId: "1",
					CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
					Region:          "us-west-2",
					CidrBlock:       "1.2.3.4/24",
					ClusterType:     controlplanev1beta2.Cluster_TYPE_DEDICATED,
				},
			},
			want: &network.Network{
				ID:              types.StringValue("1"),
				Name:            types.StringValue("test"),
				ResourceGroupID: types.StringValue("1"),
				CloudProvider:   types.StringValue("gcp"),
				Region:          types.StringValue("us-west-2"),
				CidrBlock:       types.StringValue("1.2.3.4/24"),
				ClusterType:     types.StringValue("dedicated"),
			},
		},
		{
			name: "aws byoc",
			args: args{
				cloudProvider: "aws",
				nw: &controlplanev1beta2.Network{
					Id:              "1",
					Name:            "test",
					ResourceGroupId: "1",
					CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS,
					Region:          "us-west-2",
					CidrBlock:       "1.2.3.4/24",
					ClusterType:     controlplanev1beta2.Cluster_TYPE_BYOC,
					CustomerManagedResources: &controlplanev1beta2.Network_CustomerManagedResources{
						CloudProvider: &controlplanev1beta2.Network_CustomerManagedResources_Aws{
							Aws: &controlplanev1beta2.Network_CustomerManagedResources_AWS{
								ManagementBucket: &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{
									Arn: "arn:aws:s3:::bucket",
								},
								DynamodbTable: &controlplanev1beta2.CustomerManagedDynamoDBTable{
									Arn: "arn:aws:dynamodb:us-west-2:123456789012:table/TableName",
								},
								Vpc: &controlplanev1beta2.CustomerManagedAWSVPC{
									Arn: "arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345678",
								},
								PrivateSubnets: &controlplanev1beta2.CustomerManagedAWSSubnets{
									Arns: []string{"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345678"},
								},
								PublicSubnets: &controlplanev1beta2.CustomerManagedAWSSubnets{
									Arns: []string{"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345678"},
								},
							},
						},
					},
				},
			},
			want: &network.Network{
				ID:              types.StringValue("1"),
				Name:            types.StringValue("test"),
				ResourceGroupID: types.StringValue("1"),
				CloudProvider:   types.StringValue("aws"),
				Region:          types.StringValue("us-west-2"),
				CidrBlock:       types.StringValue("1.2.3.4/24"),
				ClusterType:     types.StringValue("byoc"),
				CustomerManagedResources: network.CustomerManagedResourcesValue{
					AWS: &network.AWSResources{
						ManagementBucket: &network.AWSBucket{
							ARN: types.StringValue("arn:aws:s3:::bucket"),
						},
						DynamoDBTable: &network.AWSDynamoDBTable{
							ARN: types.StringValue("arn:aws:dynamodb:us-west-2:123456789012:table/TableName"),
						},
						VPC: &network.AWSVPC{
							ARN: types.StringValue("arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345678"),
						},
						PrivateSubnets: &network.AWSSubnets{
							ARNs: utils.StringSliceToTypeList([]string{"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345678"}),
						},
						PublicSubnets: &network.AWSSubnets{
							ARNs: utils.StringSliceToTypeList([]string{"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345678"}),
						},
					},
				},
			},
		},
		{
			name: "gcp byoc",
			args: args{
				cloudProvider: "gcp",
				nw: &controlplanev1beta2.Network{
					Id:              "1",
					Name:            "test",
					ResourceGroupId: "1",
					CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
					Region:          "us-west-2",
					CidrBlock:       "1.2.3.4/24",
					ClusterType:     controlplanev1beta2.Cluster_TYPE_BYOC,
				},
			},
			want: &network.Network{
				ID:              types.StringValue("1"),
				Name:            types.StringValue("test"),
				ResourceGroupID: types.StringValue("1"),
				CloudProvider:   types.StringValue("gcp"),
				Region:          types.StringValue("us-west-2"),
				CidrBlock:       types.StringValue("1.2.3.4/24"),
				ClusterType:     types.StringValue("byoc"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, generateModel(tt.args.cloudProvider, tt.args.nw))
		})
	}
}
