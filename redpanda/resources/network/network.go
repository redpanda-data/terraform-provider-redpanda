// Copyright 2024 Redpanda Data, Inc.
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
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/network"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func generateModel(cloudProvider string, nw *controlplanev1beta2.Network) *network.Network {
	output := &network.Network{
		CidrBlock:       types.StringValue(nw.CidrBlock),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(nw.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(nw.ClusterType)),
		ID:              types.StringValue(nw.Id),
		Name:            types.StringValue(nw.Name),
		Region:          types.StringValue(nw.Region),
		ResourceGroupID: types.StringValue(nw.ResourceGroupId),
	}

	if nw.CustomerManagedResources == nil || nw.CustomerManagedResources.CloudProvider == nil {
		return output
	}
	var cloudVal network.CustomerManagedResourcesValue
	switch cloudProvider {
	case "aws":
		if nw.CustomerManagedResources.CloudProvider != nil {
			cloudVal.AWS = network.AWSResources{}
			awsCMR := nw.CustomerManagedResources.CloudProvider.(*controlplanev1beta2.Network_CustomerManagedResources_Aws).Aws
			if awsCMR.ManagementBucket != nil {
				cloudVal.AWS.ManagementBucket = network.AWSBucket{
					ARN: types.StringValue(awsCMR.ManagementBucket.Arn),
				}
			}
			if awsCMR.DynamodbTable != nil {
				cloudVal.AWS.DynamoDBTable = network.AWSDynamoDBTable{
					ARN: types.StringValue(awsCMR.DynamodbTable.Arn),
				}
			}
			if awsCMR.Vpc != nil {
				cloudVal.AWS.VPC = network.AWSVPC{
					ARN: types.StringValue(awsCMR.Vpc.Arn),
				}
			}
			if awsCMR.PrivateSubnets != nil {
				cloudVal.AWS.PrivateSubnets = network.AWSSubnets{
					ARNs: utils.StringSliceToTypeList(awsCMR.PrivateSubnets.Arns),
				}
			}
			if awsCMR.PublicSubnets != nil {
				cloudVal.AWS.PublicSubnets = network.AWSSubnets{
					ARNs: utils.StringSliceToTypeList(awsCMR.PublicSubnets.Arns),
				}
			}
		}
	case "gcp":
		// TODO placeholder so that the linter will stop complaining
	}
	output.CustomerManagedResources = cloudVal
	return output
}

func generateNetworkCMR(cloudProvider string, model network.Network) *controlplanev1beta2.Network_CustomerManagedResources {
	switch cloudProvider {
	case "aws":
		crmFromModel := model.CustomerManagedResources.AWS
		output := &controlplanev1beta2.Network_CustomerManagedResources_Aws{
			Aws: &controlplanev1beta2.Network_CustomerManagedResources_AWS{},
		}

		if !utils.IsStructEmpty(crmFromModel.ManagementBucket) {
			output.Aws.ManagementBucket = &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{
				Arn: crmFromModel.ManagementBucket.ARN.ValueString(),
			}
		}

		if !utils.IsStructEmpty(crmFromModel.DynamoDBTable) {
			output.Aws.DynamodbTable = &controlplanev1beta2.CustomerManagedDynamoDBTable{
				Arn: crmFromModel.DynamoDBTable.ARN.ValueString(),
			}
		}

		if !utils.IsStructEmpty(crmFromModel.VPC) {
			output.Aws.Vpc = &controlplanev1beta2.CustomerManagedAWSVPC{
				Arn: crmFromModel.VPC.ARN.ValueString(),
			}
		}

		if !utils.IsStructEmpty(crmFromModel.PrivateSubnets) {
			output.Aws.PrivateSubnets = &controlplanev1beta2.CustomerManagedAWSSubnets{
				Arns: utils.TypeListToStringSlice(crmFromModel.PrivateSubnets.ARNs),
			}
		}

		if !utils.IsStructEmpty(crmFromModel.PublicSubnets) {
			output.Aws.PublicSubnets = &controlplanev1beta2.CustomerManagedAWSSubnets{
				Arns: utils.TypeListToStringSlice(crmFromModel.PublicSubnets.ARNs),
			}
		}

		return &controlplanev1beta2.Network_CustomerManagedResources{
			CloudProvider: output,
		}
	default:
		return nil
	}
}
