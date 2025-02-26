package network

import (
	"context"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/stretchr/testify/assert"
)

func TestGenerateNetworkCMR(t *testing.T) {
	tests := []struct {
		name    string
		model   models.Network
		want    *controlplanev1beta2.Network_CustomerManagedResources
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
			want: &controlplanev1beta2.Network_CustomerManagedResources{
				CloudProvider: &controlplanev1beta2.Network_CustomerManagedResources_Aws{
					Aws: &controlplanev1beta2.Network_CustomerManagedResources_AWS{
						ManagementBucket: &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{
							Arn: "arn:aws:s3:::rp-879326078624-us-east-2-mgmt-20250116215415273800000009",
						},
						DynamodbTable: &controlplanev1beta2.CustomerManagedDynamoDBTable{
							Arn: "arn:aws:dynamodb:us-east-2:879326078624:table/rp-879326078624-us-east-2-mgmt-tflock-mgkr8cfwlj",
						},
						Vpc: &controlplanev1beta2.CustomerManagedAWSVPC{
							Arn: "arn:aws:ec2:us-east-2:879326078624:vpc/vpc-00398e37a5081c45d",
						},
						PrivateSubnets: &controlplanev1beta2.CustomerManagedAWSSubnets{
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
			name: "gcp provider",
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
