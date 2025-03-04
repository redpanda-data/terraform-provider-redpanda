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

func generateModel(cloudProvider string, nw *controlplanev1beta2.Network, diags diag.Diagnostics) (*models.Network, diag.Diagnostics) {
	output := &models.Network{
		CloudProvider:   types.StringValue(utils.CloudProviderToString(nw.GetCloudProvider())),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(nw.GetClusterType())),
		ID:              types.StringValue(nw.GetId()),
		Name:            types.StringValue(nw.GetName()),
		Region:          types.StringValue(nw.GetRegion()),
		ResourceGroupID: types.StringValue(nw.GetResourceGroupId()),
	}

	if nw.GetCidrBlock() != "" {
		output.CidrBlock = types.StringValue(nw.GetCidrBlock())
	}
	if nw.HasCustomerManagedResources() {
		output.CustomerManagedResources = types.ObjectNull(cmrType)
		return generateModelCMR(cloudProvider, nw, output, diags)
	}
	output.CustomerManagedResources = types.ObjectNull(cmrType)
	return output, diags
}

func generateModelCMR(cloudProvider string, nw *controlplanev1beta2.Network, output *models.Network, diags diag.Diagnostics) (*models.Network, diag.Diagnostics) {
	if nw == nil || !nw.HasCustomerManagedResources() {
		output.CustomerManagedResources = types.ObjectNull(cmrType)
		return output, diags
	}

	switch cloudProvider {
	case "aws":
		if !nw.GetCustomerManagedResources().HasAws() {
			diags.AddError("Network AWS CustomerManagedResources not found",
				"CloudProvider is AWS and CustomerManagedResources is set but does not contain the AWS object. This is a config error.")
			return nil, diags
		}

		// Get AWS data
		awsData := nw.GetCustomerManagedResources().GetAws()

		// Initialize AWS values map with default null values
		awsVal := make(map[string]attr.Value)
		for k, v := range awsValueDefaults {
			awsVal[k] = v
		}

		// Set values for fields that exist in the input
		if awsData.HasManagementBucket() {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.GetManagementBucket().GetArn()),
			})
			if d.HasError() {
				diags.AddError("failed to generate management bucket object", "could not create management bucket object")
				diags.Append(d...)
			} else {
				awsVal["management_bucket"] = obj
			}
		}

		if awsData.HasDynamodbTable() {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.GetDynamodbTable().GetArn()),
			})
			if d.HasError() {
				diags.AddError("failed to generate dynamodb table object", "could not create dynamodb table object")
				diags.Append(d...)
			} else {
				awsVal["dynamodb_table"] = obj
			}
		}

		if awsData.HasVpc() {
			obj, d := types.ObjectValue(singleElementContainer, map[string]attr.Value{
				"arn": types.StringValue(awsData.GetVpc().GetArn()),
			})
			if d.HasError() {
				diags.AddError("failed to generate vpc object", "could not create vpc object")
				diags.Append(d...)
			} else {
				awsVal["vpc"] = obj
			}
		}

		if awsData.HasPrivateSubnets() {
			obj, d := types.ObjectValue(multiElementContainer, map[string]attr.Value{
				"arns": utils.StringSliceToTypeList(awsData.GetPrivateSubnets().GetArns()),
			})
			if d.HasError() {
				diags.AddError("failed to generate private subnets object", "could not create private subnets object")
				diags.Append(d...)
			} else {
				awsVal["private_subnets"] = obj
			}
		}

		// Create AWS object
		awsObj, d := types.ObjectValue(awsType, awsVal)
		if d.HasError() {
			diags.AddError("failed to generate AWS object", "could not create AWS object")
			diags.Append(d...)
			return nil, diags
		}

		// Create final CMR object and assign to output
		cmrObj, d := types.ObjectValue(cmrType, map[string]attr.Value{
			"aws": awsObj,
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return nil, diags
		}

		output.CustomerManagedResources = cmrObj
		return output, diags

	case "gcp":
		// Placeholder for GCP implementation
		output.CustomerManagedResources = types.ObjectNull(cmrType)
		return output, diags
	default:
		output.CustomerManagedResources = types.ObjectNull(cmrType)
		return output, diags
	}
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
		}
		// Get the AWS object from CustomerManagedResources
		var cmrObj types.Object
		if d := model.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
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
