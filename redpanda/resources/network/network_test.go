package network

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/stretchr/testify/assert"
)

func TestGenerateNetworkCMR(t *testing.T) {
	tests := []struct {
		name    string
		model   models.Network
		want    *controlplanev1.Network_CustomerManagedResources
		wantErr bool
	}{
		{
			name: "successful aws config",
			model: models.Network{
				CloudProvider: types.StringValue("aws"),
				CustomerManagedResources: types.ObjectValueMust(
					map[string]attr.Type{
						"aws": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"management_bucket": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arn": types.StringType,
									},
								},
								"dynamodb_table": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arn": types.StringType,
									},
								},
								"vpc": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arn": types.StringType,
									},
								},
								"private_subnets": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arns": types.ListType{
											ElemType: types.StringType,
										},
									},
								},
								"public_subnets": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arns": types.ListType{
											ElemType: types.StringType,
										},
									},
								},
							},
						},
					},
					map[string]attr.Value{
						"aws": types.ObjectValueMust(
							map[string]attr.Type{
								"management_bucket": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arn": types.StringType,
									},
								},
								"dynamodb_table": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arn": types.StringType,
									},
								},
								"vpc": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arn": types.StringType,
									},
								},
								"private_subnets": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arns": types.ListType{
											ElemType: types.StringType,
										},
									},
								},
								"public_subnets": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"arns": types.ListType{
											ElemType: types.StringType,
										},
									},
								},
							},
							map[string]attr.Value{
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{
										"arn": types.StringType,
									},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:s3:::rp-879326078624-us-east-2-mgmt-20250116215415273800000009"),
									},
								),
								"dynamodb_table": types.ObjectValueMust(
									map[string]attr.Type{
										"arn": types.StringType,
									},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:dynamodb:us-east-2:879326078624:table/rp-879326078624-us-east-2-mgmt-tflock-mgkr8cfwlj"),
									},
								),
								"vpc": types.ObjectValueMust(
									map[string]attr.Type{
										"arn": types.StringType,
									},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:ec2:us-east-2:879326078624:vpc/vpc-00398e37a5081c45d"),
									},
								),
								"private_subnets": types.ObjectValueMust(
									map[string]attr.Type{
										"arns": types.ListType{
											ElemType: types.StringType,
										},
									},
									map[string]attr.Value{
										"arns": types.ListValueMust(
											types.StringType,
											[]attr.Value{
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-00e09f8421e896955"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-07cae67987779c5ea"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0abe52a23ef66841b"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-01e11c77fd3fbe32c"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0605966e91c95a2da"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-02cf3488fc8ecf66c"),
											},
										),
									},
								),
								"public_subnets": types.ObjectValueMust(
									map[string]attr.Type{
										"arns": types.ListType{
											ElemType: types.StringType,
										},
									},
									map[string]attr.Value{
										"arns": types.ListValueMust(
											types.StringType,
											[]attr.Value{
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-00e09f8421e896955"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-07cae67987779c5ea"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0abe52a23ef66841b"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-01e11c77fd3fbe32c"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0605966e91c95a2da"),
												types.StringValue("arn:aws:ec2:us-east-2:879326078624:subnet/subnet-02cf3488fc8ecf66c"),
											},
										),
									},
								),
							},
						),
					},
				),
			},
			want: &controlplanev1.Network_CustomerManagedResources{
				CloudProvider: &controlplanev1.Network_CustomerManagedResources_Aws{
					Aws: &controlplanev1.Network_CustomerManagedResources_AWS{
						ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
							Arn: "arn:aws:s3:::rp-879326078624-us-east-2-mgmt-20250116215415273800000009",
						},
						DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
							Arn: "arn:aws:dynamodb:us-east-2:879326078624:table/rp-879326078624-us-east-2-mgmt-tflock-mgkr8cfwlj",
						},
						Vpc: &controlplanev1.CustomerManagedAWSVPC{
							Arn: "arn:aws:ec2:us-east-2:879326078624:vpc/vpc-00398e37a5081c45d",
						},
						PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
							Arns: []string{
								"arn:aws:ec2:us-east-2:879326078624:subnet/subnet-00e09f8421e896955",
								"arn:aws:ec2:us-east-2:879326078624:subnet/subnet-07cae67987779c5ea",
								"arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0abe52a23ef66841b",
								"arn:aws:ec2:us-east-2:879326078624:subnet/subnet-01e11c77fd3fbe32c",
								"arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0605966e91c95a2da",
								"arn:aws:ec2:us-east-2:879326078624:subnet/subnet-02cf3488fc8ecf66c",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "successful gcp config",
			model: models.Network{
				CloudProvider: types.StringValue("gcp"),
				CustomerManagedResources: types.ObjectValueMust(
					map[string]attr.Type{
						"gcp": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"network_name":       types.StringType,
								"network_project_id": types.StringType,
								"management_bucket": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"name": types.StringType,
									},
								},
							},
						},
					},
					map[string]attr.Value{
						"gcp": types.ObjectValueMust(
							map[string]attr.Type{
								"network_name":       types.StringType,
								"network_project_id": types.StringType,
								"management_bucket": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"name": types.StringType,
									},
								},
							},
							map[string]attr.Value{
								"network_name":       types.StringValue("test-network"),
								"network_project_id": types.StringValue("test-project"),
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{
										"name": types.StringType,
									},
									map[string]attr.Value{
										"name": types.StringValue("test-bucket"),
									},
								),
							},
						),
					},
				),
			},
			want: &controlplanev1.Network_CustomerManagedResources{
				CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{
					Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{
						NetworkName:      "test-network",
						NetworkProjectId: "test-project",
						ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
							Name: "test-bucket",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "null customer managed resources",
			model: models.Network{
				CloudProvider:            types.StringValue("aws"),
				CustomerManagedResources: types.ObjectNull(cmrType),
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "missing aws object",
			model: models.Network{
				CloudProvider: types.StringValue("aws"),
				CustomerManagedResources: types.ObjectValueMust(
					map[string]attr.Type{
						"aws": types.ObjectType{
							AttrTypes: map[string]attr.Type{},
						},
					},
					map[string]attr.Value{
						"aws": types.ObjectNull(map[string]attr.Type{}),
					},
				),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing gcp object",
			model: models.Network{
				CloudProvider: types.StringValue("gcp"),
				CustomerManagedResources: types.ObjectValueMust(
					map[string]attr.Type{
						"gcp": types.ObjectType{
							AttrTypes: map[string]attr.Type{},
						},
					},
					map[string]attr.Value{
						"gcp": types.ObjectNull(map[string]attr.Type{}),
					},
				),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "gcp provider with null cmr",
			model: models.Network{
				CloudProvider:            types.StringValue("gcp"),
				CustomerManagedResources: types.ObjectNull(cmrType),
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := diag.Diagnostics{}
			got, gotDiags := generateNetworkCMR(context.Background(), tt.model, diags)

			if tt.wantErr {
				if !gotDiags.HasError() {
					t.Error("generateNetworkCMR() expected error but got none")
				}
				return
			}

			if gotDiags.HasError() {
				t.Errorf("generateNetworkCMR() unexpected error: %v", gotDiags)
				return
			}

			assert.Equal(t, tt.want, got, "unexpected result from generateNetworkCMR()")
		})
	}
}

func TestGenerateModelGCPCMR(t *testing.T) {
	tests := []struct {
		name    string
		gcpData *controlplanev1.Network_CustomerManagedResources_GCP
		want    basetypes.ObjectValue
		wantErr bool
	}{
		{
			name: "with all fields populated",
			gcpData: &controlplanev1.Network_CustomerManagedResources_GCP{
				NetworkName:      "test-network",
				NetworkProjectId: "test-project",
				ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
					Name: "test-bucket",
				},
			},
			want: types.ObjectValueMust(
				gcpType,
				map[string]attr.Value{
					"network_name":       types.StringValue("test-network"),
					"network_project_id": types.StringValue("test-project"),
					"management_bucket": types.ObjectValueMust(
						gcpBucketType,
						map[string]attr.Value{
							"name": types.StringValue("test-bucket"),
						},
					),
				},
			),
			wantErr: false,
		},
		{
			name: "with missing management bucket",
			gcpData: &controlplanev1.Network_CustomerManagedResources_GCP{
				NetworkName:      "test-network",
				NetworkProjectId: "test-project",
			},
			want: types.ObjectValueMust(
				gcpType,
				map[string]attr.Value{
					"network_name":       types.StringValue("test-network"),
					"network_project_id": types.StringValue("test-project"),
					"management_bucket":  types.ObjectNull(gcpBucketType),
				},
			),
			wantErr: false,
		},
		{
			name: "with only network name",
			gcpData: &controlplanev1.Network_CustomerManagedResources_GCP{
				NetworkName: "test-network",
			},
			want: types.ObjectValueMust(
				gcpType,
				map[string]attr.Value{
					"network_name":       types.StringValue("test-network"),
					"network_project_id": types.StringNull(),
					"management_bucket":  types.ObjectNull(gcpBucketType),
				},
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := diag.Diagnostics{}
			got, gotDiags := generateModelGCPCMR(tt.gcpData, diags)

			if tt.wantErr {
				if !gotDiags.HasError() {
					t.Error("generateModelGCPCMR() expected error but got none")
				}
				return
			}

			if gotDiags.HasError() {
				t.Errorf("generateModelGCPCMR() unexpected error: %v", gotDiags)
				return
			}

			assert.Equal(t, tt.want, got, "unexpected result from generateModelGCPCMR()")
		})
	}
}

func TestGenerateModel(t *testing.T) {
	tests := []struct {
		name          string
		cloudProvider string
		network       *controlplanev1.Network
		want          *models.Network
		wantErr       bool
	}{
		{
			name:          "network with AWS CMR",
			cloudProvider: "aws",
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-name",
				Region:          "us-east-2",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				ResourceGroupId: "resource-group-id",
				CidrBlock:       "10.0.0.0/16",
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Aws{
						Aws: &controlplanev1.Network_CustomerManagedResources_AWS{
							ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
								Arn: "test-bucket-arn",
							},
							DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
								Arn: "test-dynamo-arn",
							},
							Vpc: &controlplanev1.CustomerManagedAWSVPC{
								Arn: "test-vpc-arn",
							},
							PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
								Arns: []string{"subnet-1", "subnet-2"},
							},
						},
					},
				},
			},
			want: &models.Network{
				ID:              types.StringValue("test-id"),
				Name:            types.StringValue("test-name"),
				Region:          types.StringValue("us-east-2"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("byoc"),
				ResourceGroupID: types.StringValue("resource-group-id"),
				CidrBlock:       types.StringValue("10.0.0.0/16"),
				CustomerManagedResources: types.ObjectValueMust(
					cmrType,
					map[string]attr.Value{
						"aws": types.ObjectValueMust(
							awsType,
							map[string]attr.Value{
								"management_bucket": types.ObjectValueMust(
									singleElementContainer,
									map[string]attr.Value{
										"arn": types.StringValue("test-bucket-arn"),
									},
								),
								"dynamodb_table": types.ObjectValueMust(
									singleElementContainer,
									map[string]attr.Value{
										"arn": types.StringValue("test-dynamo-arn"),
									},
								),
								"vpc": types.ObjectValueMust(
									singleElementContainer,
									map[string]attr.Value{
										"arn": types.StringValue("test-vpc-arn"),
									},
								),
								"private_subnets": types.ObjectValueMust(
									multiElementContainer,
									map[string]attr.Value{
										"arns": types.ListValueMust(
											types.StringType,
											[]attr.Value{
												types.StringValue("subnet-1"),
												types.StringValue("subnet-2"),
											},
										),
									},
								),
							},
						),
						"gcp": types.ObjectNull(gcpType),
					},
				),
			},
			wantErr: false,
		},
		{
			name:          "network with GCP CMR",
			cloudProvider: "gcp",
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-name",
				Region:          "us-central1",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				ResourceGroupId: "resource-group-id",
				CidrBlock:       "10.0.0.0/16",
				CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
					CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{
						Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{
							NetworkName:      "test-network",
							NetworkProjectId: "test-project",
							ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
								Name: "test-bucket",
							},
						},
					},
				},
			},
			want: &models.Network{
				ID:              types.StringValue("test-id"),
				Name:            types.StringValue("test-name"),
				Region:          types.StringValue("us-central1"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("byoc"),
				ResourceGroupID: types.StringValue("resource-group-id"),
				CidrBlock:       types.StringValue("10.0.0.0/16"),
				CustomerManagedResources: types.ObjectValueMust(
					cmrType,
					map[string]attr.Value{
						"gcp": types.ObjectValueMust(
							gcpType,
							map[string]attr.Value{
								"network_name":       types.StringValue("test-network"),
								"network_project_id": types.StringValue("test-project"),
								"management_bucket": types.ObjectValueMust(
									gcpBucketType,
									map[string]attr.Value{
										"name": types.StringValue("test-bucket"),
									},
								),
							},
						),
						"aws": types.ObjectNull(awsType),
					},
				),
			},
			wantErr: false,
		},
		{
			name:          "network without CMR",
			cloudProvider: "aws",
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-name",
				Region:          "us-east-2",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				ResourceGroupId: "resource-group-id",
				CidrBlock:       "10.0.0.0/16",
			},
			want: &models.Network{
				ID:                       types.StringValue("test-id"),
				Name:                     types.StringValue("test-name"),
				Region:                   types.StringValue("us-east-2"),
				CloudProvider:            types.StringValue("aws"),
				ClusterType:              types.StringValue("dedicated"),
				ResourceGroupID:          types.StringValue("resource-group-id"),
				CidrBlock:                types.StringValue("10.0.0.0/16"),
				CustomerManagedResources: types.ObjectNull(cmrType),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := diag.Diagnostics{}
			got, gotDiags := generateModel(tt.cloudProvider, tt.network, diags)

			if tt.wantErr {
				if !gotDiags.HasError() {
					t.Error("generateModel() expected error but got none")
				}
				return
			}

			if gotDiags.HasError() {
				t.Errorf("generateModel() unexpected error: %v", gotDiags)
				return
			}

			assert.Equal(t, tt.want, got, "unexpected result from generateModel()")
		})
	}
}
