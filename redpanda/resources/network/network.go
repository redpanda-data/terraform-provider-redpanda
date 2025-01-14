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
	"context"

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

	awsValue := network.CustomerManagedResourcesValue{AWS: &network.AWSResources{}}
	if nw.CustomerManagedResources != nil {
		if nw.CustomerManagedResources.CloudProvider != nil {
			switch cloudProvider {
			case "aws":
				if nw.CustomerManagedResources.CloudProvider != nil {
					awsCMR := nw.CustomerManagedResources.CloudProvider.(*controlplanev1beta2.Network_CustomerManagedResources_Aws).Aws
					if awsCMR.ManagementBucket != nil {
						awsValue.AWS.ManagementBucket = &network.AWSBucket{
							ARN: types.StringValue(awsCMR.ManagementBucket.Arn),
						}
					}
				}
			}
		}
	case "gcp":
		// TODO placeholder so that the linter will stop complaining
	}
	output.CustomerManagedResources = awsValue.Type(context.Background())
	return output
}

func generateNetworkCMR(cloudProvider string, model network.Network) *controlplanev1beta2.Network_CustomerManagedResources {
	switch cloudProvider {
	case "aws":
		crmFromModel := model.CustomerManagedResources.ValueType(context.Background()).(network.CustomerManagedResourcesValue).AWS
		output := &controlplanev1beta2.Network_CustomerManagedResources_Aws{}
		if crmFromModel.ManagementBucket != nil {
			output.Aws.ManagementBucket = &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{
				Arn: crmFromModel.ManagementBucket.ARN,
			}
		}
		if crmFromModel.DynamoDBTable != nil {
			output.Aws.DynamodbTable = &controlplanev1beta2.CustomerManagedDynamoDBTable{
				Arn: crmFromModel.DynamoDBTable.ARN,
			}
		}
		if crmFromModel.VPC != nil {
			output.Aws.Vpc = &controlplanev1beta2.CustomerManagedAWSVPC{
				Arn: crmFromModel.VPC.ARN,
			}
		}
		if crmFromModel.PrivateSubnets != nil {
			output.Aws.PrivateSubnets = &controlplanev1beta2.CustomerManagedAWSSubnets{
				Arns: utils.TypeListToStringSlice(crmFromModel.PrivateSubnets.ARNs),
			}
		}

		if crmFromModel.PublicSubnets != nil {
			output.Aws.PublicSubnets = &controlplanev1beta2.CustomerManagedAWSSubnets{
				Arns: crmFromModel.PublicSubnets.ARNs,
			}
		}
		return &controlplanev1beta2.Network_CustomerManagedResources{
			CloudProvider: output,
		}
	default:
		return nil
	}
}
