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
	"fmt"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func generateModel(cloudProvider string, nw *controlplanev1beta2.Network) *models.Network {
	output := &models.Network{
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
	switch cloudProvider {
	case "aws":
		awsContainer, ok := nw.CustomerManagedResources.CloudProvider.(*controlplanev1beta2.Network_CustomerManagedResources_Aws)
		if !ok {
			break
		}
		awsData := awsContainer.Aws
		retVal := awsValue
		if awsData.ManagementBucket != nil {
			retVal["management_bucket"] = types.ObjectValueMust(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.ManagementBucket.Arn),
			})
		}
		if awsData.DynamodbTable != nil {
			retVal["dynamodb_table"] = types.ObjectValueMust(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.DynamodbTable.Arn),
			})
		}
		if awsData.Vpc != nil {
			retVal["vpc"] = types.ObjectValueMust(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.Vpc.Arn),
			})
		}
		if awsData.PrivateSubnets != nil {
			retVal["private_subnets"] = types.ObjectValueMust(multiElementContainer, map[string]attr.Value{
				"arns": utils.StringSliceToTypeList(awsData.PrivateSubnets.Arns),
			})
		}
		if awsData.PublicSubnets != nil {
			retVal["public_subnets"] = types.ObjectValueMust(multiElementContainer, map[string]attr.Value{
				"arns": utils.StringSliceToTypeList(awsData.PublicSubnets.Arns),
			})
		}
		crmVal := crmVal
		crmVal["aws"] = basetypes.NewObjectValueMust(awsType, retVal)
		output.CustomerManagedResources = types.ObjectValueMust(cmrType, crmVal)

	case "gcp":
		// TODO placeholder so that the linter will stop complaining
	}
	return output
}

func generateNetworkCMR(ctx context.Context, model models.Network, diags diag.Diagnostics) (*controlplanev1beta2.Network_CustomerManagedResources, diag.Diagnostics) {
	cmr := &controlplanev1beta2.Network_CustomerManagedResources{}

	if model.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	// If CustomerManagedResources is not null, process it
	switch model.CloudProvider.ValueString() {
	case "aws":
		awsRet := &controlplanev1beta2.Network_CustomerManagedResources_AWS{
			ManagementBucket: &controlplanev1beta2.CustomerManagedAWSCloudStorageBucket{},
			DynamodbTable:    &controlplanev1beta2.CustomerManagedDynamoDBTable{},
			Vpc:              &controlplanev1beta2.CustomerManagedAWSVPC{},
			PrivateSubnets:   &controlplanev1beta2.CustomerManagedAWSSubnets{},
			PublicSubnets:    &controlplanev1beta2.CustomerManagedAWSSubnets{},
		}
		// Get the AWS object from CustomerManagedResources
		var cmrObj types.Object
		if d := model.CustomerManagedResources.As(context.Background(), &cmrObj, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		}); d.HasError() {
			return nil, d
		}

		aws, d := getObjectFromAttributes(ctx, "aws", cmrObj.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		// bucket
		mgmtBktArn, d := getStringFromAttributes("management_bucket", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.ManagementBucket.Arn = mgmtBktArn

		// dynamo
		dynamoArn, d := getStringFromAttributes("dynamodb_table", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.DynamodbTable.Arn = dynamoArn

		// vpc
		vpcArn, d := getStringFromAttributes("vpc", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.Vpc.Arn = vpcArn

		// private subnets
		privateSubnetsArns, d := getListFromAttributes("private_subnets", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.PrivateSubnets.Arns = privateSubnetsArns

		// public subnets
		publicSubnetsArns, d := getListFromAttributes("public_subnets", aws.Attributes(), diags)
		if d.HasError() {
			return nil, d
		}
		awsRet.PublicSubnets.Arns = publicSubnetsArns

		cmr.CloudProvider = &controlplanev1beta2.Network_CustomerManagedResources_Aws{
			Aws: awsRet,
		}
		return cmr, nil
	case "gcp":
		// TODO placeholder so that the linter will stop complaining
		return nil, nil
	default:
		return nil, nil
	}
}

func getObjectFromAttributes(ctx context.Context, key string, att map[string]attr.Value, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		return types.ObjectNull(map[string]attr.Type{}), append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "object is missing or malformed for network resource"))
	}
	var keyVal types.Object
	if err := attVal.As(ctx, &keyVal, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); err != nil {
		return types.ObjectNull(map[string]attr.Type{}), append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "value is missing or malformed for network resource"))
	}
	return keyVal, nil
}

func getStringFromAttributes(key string, att map[string]attr.Value, diags diag.Diagnostics) (string, diag.Diagnostics) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		return "", append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "object is missing or malformed for network resource"))
	}
	rt, ok := attVal.Attributes()["arn"].(types.String)
	if !ok {
		return "", append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "string is missing or malformed for network resource"))
	}
	return rt.ValueString(), nil
}
func getListFromAttributes(key string, att map[string]attr.Value, diags diag.Diagnostics) ([]string, diag.Diagnostics) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		return nil, append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "object is missing or malformed for network resource"))
	}
	rt, ok := attVal.Attributes()["arns"].(types.List)
	if !ok {
		return nil, append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "list is missing or malformed for network resource"))
	}
	return utils.TypeListToStringSlice(rt), nil
}
