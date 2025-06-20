// Copyright 2023 Redpanda Data, Inc.
//
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

package network

import (
	"context"
	"strings"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAWSCMR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		cmrObj      types.Object
		expectError bool
		validate    func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS, err error)
	}{
		{
			name: "valid AWS CMR with all fields",
			cmrObj: types.ObjectValueMust(
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
									"arns": types.ListType{ElemType: types.StringType},
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
									"arns": types.ListType{ElemType: types.StringType},
								},
							},
						},
						map[string]attr.Value{
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:s3:::test-bucket"),
								},
							),
							"dynamodb_table": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
								},
							),
							"vpc": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345"),
								},
							),
							"private_subnets": types.ObjectValueMust(
								map[string]attr.Type{
									"arns": types.ListType{ElemType: types.StringType},
								},
								map[string]attr.Value{
									"arns": types.ListValueMust(
										types.StringType,
										[]attr.Value{
											types.StringValue("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-12345"),
											types.StringValue("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-67890"),
										},
									),
								},
							),
						},
					),
				},
			),
			expectError: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "arn:aws:s3:::test-bucket", result.ManagementBucket.Arn)
				assert.Equal(t, "arn:aws:dynamodb:us-east-1:123456789012:table/test-table", result.DynamodbTable.Arn)
				assert.Equal(t, "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345", result.Vpc.Arn)
				assert.ElementsMatch(t, []string{
					"arn:aws:ec2:us-east-1:123456789012:subnet/subnet-12345",
					"arn:aws:ec2:us-east-1:123456789012:subnet/subnet-67890",
				}, result.PrivateSubnets.Arns)
			},
		},
		{
			name: "missing AWS object",
			cmrObj: types.ObjectValueMust(
				map[string]attr.Type{},
				map[string]attr.Value{},
			),
			expectError: true,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "could not get AWS object from CustomerManagedResources")
				assert.Nil(t, result)
			},
		},
		{
			name: "missing management bucket",
			cmrObj: types.ObjectValueMust(
				map[string]attr.Type{
					"aws": types.ObjectType{
						AttrTypes: map[string]attr.Type{
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
									"arns": types.ListType{ElemType: types.StringType},
								},
							},
						},
					},
				},
				map[string]attr.Value{
					"aws": types.ObjectValueMust(
						map[string]attr.Type{
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
									"arns": types.ListType{ElemType: types.StringType},
								},
							},
						},
						map[string]attr.Value{
							"dynamodb_table": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
								},
							),
							"vpc": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345"),
								},
							),
							"private_subnets": types.ObjectValueMust(
								map[string]attr.Type{
									"arns": types.ListType{ElemType: types.StringType},
								},
								map[string]attr.Value{
									"arns": types.ListValueMust(
										types.StringType,
										[]attr.Value{
											types.StringValue("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-12345"),
										},
									),
								},
							),
						},
					),
				},
			),
			expectError: true,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "could not get management bucket from AWS object")
				assert.Nil(t, result)
			},
		},
		{
			name: "empty private subnets list",
			cmrObj: types.ObjectValueMust(
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
									"arns": types.ListType{ElemType: types.StringType},
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
									"arns": types.ListType{ElemType: types.StringType},
								},
							},
						},
						map[string]attr.Value{
							"management_bucket": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:s3:::test-bucket"),
								},
							),
							"dynamodb_table": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
								},
							),
							"vpc": types.ObjectValueMust(
								map[string]attr.Type{
									"arn": types.StringType,
								},
								map[string]attr.Value{
									"arn": types.StringValue("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345"),
								},
							),
							"private_subnets": types.ObjectValueMust(
								map[string]attr.Type{
									"arns": types.ListType{ElemType: types.StringType},
								},
								map[string]attr.Value{
									"arns": types.ListValueMust(
										types.StringType,
										[]attr.Value{},
									),
								},
							),
						},
					),
				},
			),
			expectError: false,
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources_AWS, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "arn:aws:s3:::test-bucket", result.ManagementBucket.Arn)
				assert.Equal(t, "arn:aws:dynamodb:us-east-1:123456789012:table/test-table", result.DynamodbTable.Arn)
				assert.Equal(t, "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345", result.Vpc.Arn)
				assert.Empty(t, result.PrivateSubnets.Arns)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateAWSCMR(ctx, tt.cmrObj)
			tt.validate(t, result, err)
		})
	}
}

func TestGetArnFromAttributes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		attributes  map[string]attr.Value
		expected    string
		expectError bool
	}{
		{
			name: "valid arn attribute",
			key:  "test_resource",
			attributes: map[string]attr.Value{
				"test_resource": types.ObjectValueMust(
					map[string]attr.Type{
						"arn": types.StringType,
					},
					map[string]attr.Value{
						"arn": types.StringValue("arn:aws:s3:::test-bucket"),
					},
				),
			},
			expected:    "arn:aws:s3:::test-bucket",
			expectError: false,
		},
		{
			name: "missing resource key",
			key:  "missing_resource",
			attributes: map[string]attr.Value{
				"other_resource": types.ObjectValueMust(
					map[string]attr.Type{
						"arn": types.StringType,
					},
					map[string]attr.Value{
						"arn": types.StringValue("arn:aws:s3:::test-bucket"),
					},
				),
			},
			expected:    "",
			expectError: true,
		},
		{
			name: "missing arn in resource",
			key:  "test_resource",
			attributes: map[string]attr.Value{
				"test_resource": types.ObjectValueMust(
					map[string]attr.Type{
						"name": types.StringType,
					},
					map[string]attr.Value{
						"name": types.StringValue("test-name"),
					},
				),
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getArnFromAttributes(ctx, tt.key, tt.attributes)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGenerateNetworkCMR(t *testing.T) {
	ctx := context.Background()

	// Helper function to create CMR types
	createCMRAttrTypes := func() map[string]attr.Type {
		return map[string]attr.Type{
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
							"arns": types.ListType{ElemType: types.StringType},
						},
					},
				},
			},
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
		}
	}

	tests := []struct {
		name     string
		model    ResourceModel
		validate func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics)
	}{
		{
			name: "null customer managed resources",
			model: ResourceModel{
				CloudProvider:            types.StringValue("aws"),
				CustomerManagedResources: types.ObjectNull(createCMRAttrTypes()),
			},
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics) {
				require.False(t, diags.HasError())
				assert.Nil(t, result)
			},
		},
		{
			name: "valid AWS customer managed resources",
			model: ResourceModel{
				CloudProvider: types.StringValue("aws"),
				CustomerManagedResources: types.ObjectValueMust(
					createCMRAttrTypes(),
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
										"arns": types.ListType{ElemType: types.StringType},
									},
								},
							},
							map[string]attr.Value{
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{
										"arn": types.StringType,
									},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:s3:::test-bucket"),
									},
								),
								"dynamodb_table": types.ObjectValueMust(
									map[string]attr.Type{
										"arn": types.StringType,
									},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
									},
								),
								"vpc": types.ObjectValueMust(
									map[string]attr.Type{
										"arn": types.StringType,
									},
									map[string]attr.Value{
										"arn": types.StringValue("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345"),
									},
								),
								"private_subnets": types.ObjectValueMust(
									map[string]attr.Type{
										"arns": types.ListType{ElemType: types.StringType},
									},
									map[string]attr.Value{
										"arns": types.ListValueMust(
											types.StringType,
											[]attr.Value{
												types.StringValue("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-12345"),
											},
										),
									},
								),
							},
						),
						"gcp": types.ObjectNull(map[string]attr.Type{
							"network_name":       types.StringType,
							"network_project_id": types.StringType,
							"management_bucket": types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"name": types.StringType,
								},
							},
						}),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics) {
				require.False(t, diags.HasError())
				require.NotNil(t, result)
				require.NotNil(t, result.GetAws())
				assert.Equal(t, "arn:aws:s3:::test-bucket", result.GetAws().ManagementBucket.Arn)
				assert.Equal(t, "arn:aws:dynamodb:us-east-1:123456789012:table/test-table", result.GetAws().DynamodbTable.Arn)
				assert.Equal(t, "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345", result.GetAws().Vpc.Arn)
				assert.Equal(t, []string{"arn:aws:ec2:us-east-1:123456789012:subnet/subnet-12345"}, result.GetAws().PrivateSubnets.Arns)
			},
		},
		{
			name: "valid GCP customer managed resources",
			model: ResourceModel{
				CloudProvider: types.StringValue("gcp"),
				CustomerManagedResources: types.ObjectValueMust(
					createCMRAttrTypes(),
					map[string]attr.Value{
						"aws": types.ObjectNull(map[string]attr.Type{
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
									"arns": types.ListType{ElemType: types.StringType},
								},
							},
						}),
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
								"network_project_id": types.StringValue("test-project-123"),
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{
										"name": types.StringType,
									},
									map[string]attr.Value{
										"name": types.StringValue("test-gcp-bucket"),
									},
								),
							},
						),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics) {
				require.False(t, diags.HasError())
				require.NotNil(t, result)
				require.NotNil(t, result.GetGcp())
				assert.Equal(t, "test-network", result.GetGcp().NetworkName)
				assert.Equal(t, "test-project-123", result.GetGcp().NetworkProjectId)
				assert.Equal(t, "test-gcp-bucket", result.GetGcp().ManagementBucket.Name)
			},
		},
		{
			name: "unsupported cloud provider",
			model: ResourceModel{
				CloudProvider: types.StringValue("azure"),
				CustomerManagedResources: types.ObjectValueMust(
					createCMRAttrTypes(),
					map[string]attr.Value{
						"aws": types.ObjectNull(map[string]attr.Type{
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
									"arns": types.ListType{ElemType: types.StringType},
								},
							},
						}),
						"gcp": types.ObjectNull(map[string]attr.Type{
							"network_name":       types.StringType,
							"network_project_id": types.StringType,
							"management_bucket": types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"name": types.StringType,
								},
							},
						}),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics) {
				require.False(t, diags.HasError())
				assert.Nil(t, result)
			},
		},
		{
			name: "AWS with invalid CMR structure",
			model: ResourceModel{
				CloudProvider: types.StringValue("aws"),
				CustomerManagedResources: types.ObjectValueMust(
					map[string]attr.Type{
						"invalid": types.StringType,
					},
					map[string]attr.Value{
						"invalid": types.StringValue("invalid-structure"),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics) {
				require.True(t, diags.HasError())
				assert.Nil(t, result)
				assert.Contains(t, diags[0].Summary(), "failed to generate AWS CustomerManagedResources")
			},
		},
		{
			name: "GCP with invalid CMR structure",
			model: ResourceModel{
				CloudProvider: types.StringValue("gcp"),
				CustomerManagedResources: types.ObjectValueMust(
					map[string]attr.Type{
						"invalid": types.StringType,
					},
					map[string]attr.Value{
						"invalid": types.StringValue("invalid-structure"),
					},
				),
			},
			validate: func(t *testing.T, result *controlplanev1.Network_CustomerManagedResources, diags diag.Diagnostics) {
				require.True(t, diags.HasError())
				assert.Nil(t, result)
				assert.Contains(t, diags[0].Summary(), "failed to generate GCP CustomerManagedResources")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := tt.model.generateNetworkCMR(ctx)
			tt.validate(t, result, diags)
		})
	}
}

func TestGetUpdatedModel(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		initialModel  ResourceModel
		network       *controlplanev1.Network
		expectedModel *ResourceModel
		expectError   bool
		errorContains string
	}{
		{
			name: "basic network without CMR",
			initialModel: ResourceModel{},
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-network",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				Region:          "us-east-1",
				ResourceGroupId: "rg-123",
				CidrBlock:       "10.0.0.0/16",
			},
			expectedModel: &ResourceModel{
				ID:                       types.StringValue("test-id"),
				Name:                     types.StringValue("test-network"),
				CloudProvider:            types.StringValue("aws"),
				ClusterType:              types.StringValue("dedicated"),
				Region:                   types.StringValue("us-east-1"),
				ResourceGroupID:          types.StringValue("rg-123"),
				CidrBlock:                types.StringValue("10.0.0.0/16"),
				CustomerManagedResources: getCMRNull(),
			},
			expectError: false,
		},
		{
			name: "network with empty CIDR block",
			initialModel: ResourceModel{},
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-network",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				ClusterType:     controlplanev1.Cluster_TYPE_BYOC,
				Region:          "us-central1",
				ResourceGroupId: "rg-456",
				CidrBlock:       "",
			},
			expectedModel: &ResourceModel{
				ID:                       types.StringValue("test-id"),
				Name:                     types.StringValue("test-network"),
				CloudProvider:            types.StringValue("gcp"),
				ClusterType:              types.StringValue("byoc"),
				Region:                   types.StringValue("us-central1"),
				ResourceGroupID:          types.StringValue("rg-456"),
				CidrBlock:                types.StringNull(),
				CustomerManagedResources: getCMRNull(),
			},
			expectError: false,
		},
		{
			name: "network with 0.0.0.0/0 CIDR block",
			initialModel: ResourceModel{},
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-network",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				Region:          "eu-west-1",
				ResourceGroupId: "rg-789",
				CidrBlock:       "0.0.0.0/0",
			},
			expectedModel: &ResourceModel{
				ID:                       types.StringValue("test-id"),
				Name:                     types.StringValue("test-network"),
				CloudProvider:            types.StringValue("aws"),
				ClusterType:              types.StringValue("dedicated"),
				Region:                   types.StringValue("eu-west-1"),
				ResourceGroupID:          types.StringValue("rg-789"),
				CidrBlock:                types.StringNull(),
				CustomerManagedResources: getCMRNull(),
			},
			expectError: false,
		},
		{
			name: "network with AWS CMR",
			initialModel: ResourceModel{},
			network: func() *controlplanev1.Network {
				cmr := &controlplanev1.Network_CustomerManagedResources{}
				cmr.SetAws(&controlplanev1.Network_CustomerManagedResources_AWS{
					ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
						Arn: "arn:aws:s3:::test-bucket",
					},
					DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
						Arn: "arn:aws:dynamodb:us-west-2:123456789012:table/test-table",
					},
					Vpc: &controlplanev1.CustomerManagedAWSVPC{
						Arn: "arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345",
					},
					PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
						Arns: []string{
							"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345",
							"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-67890",
						},
					},
				})
				return &controlplanev1.Network{
					Id:                       "test-id",
					Name:                     "test-network",
					CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
					ClusterType:              controlplanev1.Cluster_TYPE_BYOC,
					Region:                   "us-west-2",
					ResourceGroupId:          "rg-111",
					CustomerManagedResources: cmr,
				}
			}(),
			expectedModel: &ResourceModel{
				ID:              types.StringValue("test-id"),
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("us-west-2"),
				ResourceGroupID: types.StringValue("rg-111"),
				CidrBlock:       types.StringNull(),
				// CustomerManagedResources will be populated by generateModelCMR
			},
			expectError: false,
		},
		{
			name: "network with GCP CMR",
			initialModel: ResourceModel{},
			network: func() *controlplanev1.Network {
				cmr := &controlplanev1.Network_CustomerManagedResources{}
				cmr.SetGcp(&controlplanev1.Network_CustomerManagedResources_GCP{
					NetworkName:      "test-network",
					NetworkProjectId: "test-project-123",
					ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
						Name: "test-gcp-bucket",
					},
				})
				return &controlplanev1.Network{
					Id:                       "test-id",
					Name:                     "test-network",
					CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
					ClusterType:              controlplanev1.Cluster_TYPE_BYOC,
					Region:                   "us-central1",
					ResourceGroupId:          "rg-222",
					CustomerManagedResources: cmr,
				}
			}(),
			expectedModel: &ResourceModel{
				ID:              types.StringValue("test-id"),
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("us-central1"),
				ResourceGroupID: types.StringValue("rg-222"),
				CidrBlock:       types.StringNull(),
				// CustomerManagedResources will be populated by generateModelCMR
			},
			expectError: false,
		},
		{
			name: "network with unspecified cloud provider",
			initialModel: ResourceModel{},
			network: &controlplanev1.Network{
				Id:              "test-id",
				Name:            "test-network",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				Region:          "unknown",
				ResourceGroupId: "rg-333",
			},
			expectedModel: &ResourceModel{
				ID:                       types.StringValue("test-id"),
				Name:                     types.StringValue("test-network"),
				CloudProvider:            types.StringValue("unspecified"),
				ClusterType:              types.StringValue("dedicated"),
				Region:                   types.StringValue("unknown"),
				ResourceGroupID:          types.StringValue("rg-333"),
				CidrBlock:                types.StringNull(),
				CustomerManagedResources: getCMRNull(),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.initialModel
			result, diags := model.GetUpdatedModel(ctx, tt.network)

			if tt.expectError {
				require.True(t, diags.HasError())
				if tt.errorContains != "" {
					assert.Contains(t, diags[0].Summary(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.False(t, diags.HasError())
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedModel.ID, result.ID)
				assert.Equal(t, tt.expectedModel.Name, result.Name)
				assert.Equal(t, tt.expectedModel.CloudProvider, result.CloudProvider)
				assert.Equal(t, tt.expectedModel.ClusterType, result.ClusterType)
				assert.Equal(t, tt.expectedModel.Region, result.Region)
				assert.Equal(t, tt.expectedModel.ResourceGroupID, result.ResourceGroupID)
				assert.Equal(t, tt.expectedModel.CidrBlock, result.CidrBlock)
				
				// For CMR, check if it's null or has values
				if tt.network.CustomerManagedResources == nil {
					assert.True(t, result.CustomerManagedResources.IsNull())
				} else {
					assert.False(t, result.CustomerManagedResources.IsNull())
				}
			}
		})
	}
}

func TestGetNetworkCreate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		model            ResourceModel
		expectedCreate   *controlplanev1.NetworkCreate
		expectError      bool
		errorContains    string
	}{
		{
			name: "valid network create without CMR",
			model: ResourceModel{
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("dedicated"),
				Region:          types.StringValue("us-east-1"),
				ResourceGroupID: types.StringValue("rg-123"),
				CidrBlock:       types.StringValue("10.0.0.0/16"),
				CustomerManagedResources: types.ObjectNull(getCMRType()),
			},
			expectedCreate: &controlplanev1.NetworkCreate{
				Name:                     "test-network",
				CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:              controlplanev1.Cluster_TYPE_DEDICATED,
				Region:                   "us-east-1",
				ResourceGroupId:          "rg-123",
				CidrBlock:                "10.0.0.0/16",
				CustomerManagedResources: nil,
			},
			expectError: false,
		},
		{
			name: "valid network create with GCP cloud provider",
			model: ResourceModel{
				Name:            types.StringValue("gcp-network"),
				CloudProvider:   types.StringValue("gcp"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("us-central1"),
				ResourceGroupID: types.StringValue("rg-456"),
				CidrBlock:       types.StringValue("10.1.0.0/16"),
				CustomerManagedResources: types.ObjectNull(getCMRType()),
			},
			expectedCreate: &controlplanev1.NetworkCreate{
				Name:                     "gcp-network",
				CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
				ClusterType:              controlplanev1.Cluster_TYPE_BYOC,
				Region:                   "us-central1",
				ResourceGroupId:          "rg-456",
				CidrBlock:                "10.1.0.0/16",
				CustomerManagedResources: nil,
			},
			expectError: false,
		},
		{
			name: "valid network create with Azure cloud provider",
			model: ResourceModel{
				Name:            types.StringValue("azure-network"),
				CloudProvider:   types.StringValue("azure"),
				ClusterType:     types.StringValue("dedicated"),
				Region:          types.StringValue("eastus"),
				ResourceGroupID: types.StringValue("rg-789"),
				CidrBlock:       types.StringValue("10.2.0.0/16"),
				CustomerManagedResources: types.ObjectNull(getCMRType()),
			},
			expectedCreate: &controlplanev1.NetworkCreate{
				Name:                     "azure-network",
				CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
				ClusterType:              controlplanev1.Cluster_TYPE_DEDICATED,
				Region:                   "eastus",
				ResourceGroupId:          "rg-789",
				CidrBlock:                "10.2.0.0/16",
				CustomerManagedResources: nil,
			},
			expectError: false,
		},
		{
			name: "network create with AWS CMR",
			model: ResourceModel{
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("byoc"),
				Region:          types.StringValue("us-west-2"),
				ResourceGroupID: types.StringValue("rg-111"),
				CidrBlock:       types.StringValue("10.3.0.0/16"),
				CustomerManagedResources: types.ObjectValueMust(
					getCMRType(),
					map[string]attr.Value{
						"aws": types.ObjectValueMust(
							getAwsType(),
							map[string]attr.Value{
								"management_bucket": types.ObjectValueMust(
									map[string]attr.Type{"arn": types.StringType},
									map[string]attr.Value{"arn": types.StringValue("arn:aws:s3:::test-bucket")},
								),
								"dynamodb_table": types.ObjectValueMust(
									map[string]attr.Type{"arn": types.StringType},
									map[string]attr.Value{"arn": types.StringValue("arn:aws:dynamodb:us-west-2:123456789012:table/test-table")},
								),
								"vpc": types.ObjectValueMust(
									map[string]attr.Type{"arn": types.StringType},
									map[string]attr.Value{"arn": types.StringValue("arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345")},
								),
								"private_subnets": types.ObjectValueMust(
									map[string]attr.Type{"arns": types.ListType{ElemType: types.StringType}},
									map[string]attr.Value{
										"arns": types.ListValueMust(
											types.StringType,
											[]attr.Value{
												types.StringValue("arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345"),
											},
										),
									},
								),
							},
						),
						"gcp": types.ObjectNull(getGcpType()),
					},
				),
			},
			expectedCreate: func() *controlplanev1.NetworkCreate {
				cmr := &controlplanev1.Network_CustomerManagedResources{}
				cmr.SetAws(&controlplanev1.Network_CustomerManagedResources_AWS{
					ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{
						Arn: "arn:aws:s3:::test-bucket",
					},
					DynamodbTable: &controlplanev1.CustomerManagedDynamoDBTable{
						Arn: "arn:aws:dynamodb:us-west-2:123456789012:table/test-table",
					},
					Vpc: &controlplanev1.CustomerManagedAWSVPC{
						Arn: "arn:aws:ec2:us-west-2:123456789012:vpc/vpc-12345",
					},
					PrivateSubnets: &controlplanev1.CustomerManagedAWSSubnets{
						Arns: []string{"arn:aws:ec2:us-west-2:123456789012:subnet/subnet-12345"},
					},
				})
				return &controlplanev1.NetworkCreate{
					Name:                     "test-network",
					CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
					ClusterType:              controlplanev1.Cluster_TYPE_BYOC,
					Region:                   "us-west-2",
					ResourceGroupId:          "rg-111",
					CidrBlock:                "10.3.0.0/16",
					CustomerManagedResources: cmr,
				}
			}(),
			expectError: false,
		},
		{
			name: "invalid cloud provider",
			model: ResourceModel{
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("invalid-provider"),
				ClusterType:     types.StringValue("dedicated"),
				Region:          types.StringValue("us-east-1"),
				ResourceGroupID: types.StringValue("rg-123"),
				CidrBlock:       types.StringValue("10.0.0.0/16"),
			},
			expectedCreate: nil,
			expectError:    true,
			errorContains:  "error getting cloud provider",
		},
		{
			name: "invalid cluster type",
			model: ResourceModel{
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("invalid-type"),
				Region:          types.StringValue("us-east-1"),
				ResourceGroupID: types.StringValue("rg-123"),
				CidrBlock:       types.StringValue("10.0.0.0/16"),
			},
			expectedCreate: nil,
			expectError:    true,
			errorContains:  "error getting cluster type",
		},
		{
			name: "network with empty CIDR block",
			model: ResourceModel{
				Name:            types.StringValue("test-network"),
				CloudProvider:   types.StringValue("aws"),
				ClusterType:     types.StringValue("dedicated"),
				Region:          types.StringValue("us-east-1"),
				ResourceGroupID: types.StringValue("rg-123"),
				CidrBlock:       types.StringNull(),
			},
			expectedCreate: &controlplanev1.NetworkCreate{
				Name:            "test-network",
				CloudProvider:   controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ClusterType:     controlplanev1.Cluster_TYPE_DEDICATED,
				Region:          "us-east-1",
				ResourceGroupId: "rg-123",
				CidrBlock:       "",
			},
			expectError: false,
		},
		{
			name: "network with all null/empty values",
			model: ResourceModel{
				Name:            types.StringNull(),
				CloudProvider:   types.StringNull(),
				ClusterType:     types.StringNull(),
				Region:          types.StringNull(),
				ResourceGroupID: types.StringNull(),
				CidrBlock:       types.StringNull(),
			},
			expectedCreate: nil,
			expectError:    true,
			errorContains:  "error getting cloud provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := tt.model.GetNetworkCreate(ctx)

			if tt.expectError {
				require.True(t, diags.HasError())
				if tt.errorContains != "" {
					found := false
					for _, d := range diags {
						if strings.Contains(d.Summary(), tt.errorContains) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected error containing '%s' but got %v", tt.errorContains, diags)
				}
				assert.Nil(t, result)
			} else {
				require.False(t, diags.HasError())
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedCreate.Name, result.Name)
				assert.Equal(t, tt.expectedCreate.CloudProvider, result.CloudProvider)
				assert.Equal(t, tt.expectedCreate.ClusterType, result.ClusterType)
				assert.Equal(t, tt.expectedCreate.Region, result.Region)
				assert.Equal(t, tt.expectedCreate.ResourceGroupId, result.ResourceGroupId)
				assert.Equal(t, tt.expectedCreate.CidrBlock, result.CidrBlock)
				
				// Compare CMR
				if tt.expectedCreate.CustomerManagedResources == nil {
					assert.Nil(t, result.CustomerManagedResources)
				} else {
					require.NotNil(t, result.CustomerManagedResources)
					// Deep comparison of CMR would go here based on cloud provider
				}
			}
		})
	}
}
