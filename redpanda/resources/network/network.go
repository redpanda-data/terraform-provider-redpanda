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

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func generateModel(cloudProvider string, nw *controlplanev1.Network, diags diag.Diagnostics) (*models.Network, diag.Diagnostics) {
	output := &models.Network{
		CloudProvider:   types.StringValue(utils.CloudProviderToString(nw.GetCloudProvider())),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(nw.GetClusterType())),
		ID:              types.StringValue(nw.GetId()),
		Name:            types.StringValue(nw.GetName()),
		Region:          types.StringValue(nw.GetRegion()),
		ResourceGroupID: types.StringValue(nw.GetResourceGroupId()),
	}

	if nw.GetCidrBlock() != "" && nw.CidrBlock != "0.0.0.0/0" {
		output.CidrBlock = types.StringValue(nw.GetCidrBlock())
	}
	if nw.HasCustomerManagedResources() {
		output.CustomerManagedResources = types.ObjectNull(cmrType)
		return generateModelCMR(cloudProvider, nw, output, diags)
	}
	output.CustomerManagedResources = types.ObjectNull(cmrType)
	return output, diags
}

func generateModelCMR(cloudProvider string, nw *controlplanev1.Network, output *models.Network, diags diag.Diagnostics) (*models.Network, diag.Diagnostics) {
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
		awsObj, d := generateModelAWSCMR(awsData, diags)
		if d.HasError() {
			diags.AddError("failed to generate AWS CustomerManagedResources object", "could not create AWS CustomerManagedResources object")
			diags.Append(d...)
			return nil, diags
		}
		// Create final CMR object and assign to output
		cmrObj, d := types.ObjectValue(cmrType, map[string]attr.Value{
			"aws": awsObj,
			"gcp": types.ObjectNull(gcpType),
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return nil, diags
		}
		output.CustomerManagedResources = cmrObj
	case "gcp":
		if !nw.GetCustomerManagedResources().HasGcp() {
			diags.AddError("Network GCP CustomerManagedResources not found",
				"CloudProvider is GCP and CustomerManagedResources is set but does not contain the GCP object. This is a config error.")
			return nil, diags
		}

		// Get GCP data
		gcpData := nw.GetCustomerManagedResources().GetGcp()
		gcpObj, d := generateModelGCPCMR(gcpData, diags)
		if d.HasError() {
			diags.AddError("failed to generate GCP CustomerManagedResources object", "could not create GCP CustomerManagedResources object")
			diags.Append(d...)
			return nil, diags
		}
		// Create final CMR object and assign to output
		cmrObj, d := types.ObjectValue(cmrType, map[string]attr.Value{
			"gcp": gcpObj,
			"aws": types.ObjectNull(awsType),
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return nil, diags
		}
		output.CustomerManagedResources = cmrObj
	default:
		output.CustomerManagedResources = types.ObjectNull(cmrType)
	}
	return output, diags
}

func generateModelGCPCMR(gcpData *controlplanev1.Network_CustomerManagedResources_GCP, diags diag.Diagnostics) (basetypes.ObjectValue, diag.Diagnostics) {
	// Initialize GCP values map with default null values
	gcpVal := make(map[string]attr.Value)
	for k, v := range gcpValueDefaults {
		gcpVal[k] = v
	}

	// Set values for fields that exist in the input
	if gcpData.GetNetworkName() != "" {
		gcpVal["network_name"] = types.StringValue(gcpData.GetNetworkName())
	}

	if gcpData.GetNetworkProjectId() != "" {
		gcpVal["network_project_id"] = types.StringValue(gcpData.GetNetworkProjectId())
	}

	if gcpData.HasManagementBucket() {
		obj, d := types.ObjectValue(gcpBucketType, map[string]attr.Value{
			"name": types.StringValue(gcpData.GetManagementBucket().GetName()),
		})
		if d.HasError() {
			diags.AddError("failed to generate management bucket object", "could not create management bucket object")
			diags.Append(d...)
		} else {
			gcpVal["management_bucket"] = obj
		}
	}

	// Create GCP object
	gcpObj, d := types.ObjectValue(gcpType, gcpVal)
	if d.HasError() {
		diags.AddError("failed to generate GCP object", "could not create GCP object")
		diags.Append(d...)
		return types.ObjectNull(gcpType), diags
	}

	return gcpObj, diags
}

func generateModelAWSCMR(awsData *controlplanev1.Network_CustomerManagedResources_AWS, diags diag.Diagnostics) (basetypes.ObjectValue, diag.Diagnostics) {
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
		return types.ObjectNull(awsType), diags
	}

	return awsObj, diags
}

func generateNetworkCMR(ctx context.Context, model models.Network, diags diag.Diagnostics) (*controlplanev1.Network_CustomerManagedResources, diag.Diagnostics) {
	cmr := &controlplanev1.Network_CustomerManagedResources{}

	if model.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	// If CustomerManagedResources is not null, process it
	switch model.CloudProvider.ValueString() {
	case "aws":
		awsCMR, d := generateAWSCMR(ctx, model, diags)
		if d.HasError() {
			d.AddError("failed to generate AWS CustomerManagedResources", "could not create AWS CustomerManagedResources")
			return nil, d
		}
		cmr.SetAws(awsCMR)
		return cmr, nil
	case "gcp":
		gcpCMR, d := generateGCPCMR(ctx, model, diags)
		if d.HasError() {
			d.AddError("failed to generate GCP CustomerManagedResources", "could not create GCP CustomerManagedResources")
			return nil, d
		}
		cmr.SetGcp(gcpCMR)
	default:
		return nil, nil
	}
	return cmr, diags
}

func generateGCPCMR(ctx context.Context, model models.Network, diags diag.Diagnostics) (*controlplanev1.Network_CustomerManagedResources_GCP, diag.Diagnostics) {
	gcpRet := &controlplanev1.Network_CustomerManagedResources_GCP{
		NetworkName:      "",
		NetworkProjectId: "",
		ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
			Name: "",
		},
	}

	// Get the GCP object from CustomerManagedResources
	var cmrObj types.Object
	if d := model.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); d.HasError() {
		d.AddError("failed to get CustomerManagedResources from network model", "could not get CustomerManagedResources from network model")
		return nil, d
	}

	gcp, d := getObjectFromAttributes(ctx, "gcp", cmrObj.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get GCP object from CustomerManagedResources", "could not get GCP object from CustomerManagedResources")
		return nil, d
	}

	// network name
	networkName, d := getStringValue("network_name", gcp.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get network name from GCP object", "could not get network name from GCP object")
		return nil, d
	}
	gcpRet.NetworkName = networkName

	// network project id
	networkProjectID, d := getStringValue("network_project_id", gcp.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get network project id from GCP object", "could not get network project id from GCP object")
		return nil, d
	}
	gcpRet.NetworkProjectId = networkProjectID

	// management bucket
	managementBucket, d := getObjectFromAttributes(ctx, "management_bucket", gcp.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get management bucket object from GCP object", "could not get management bucket object from GCP object")
		return nil, d
	}
	managementBucketName, d := getStringValue("name", managementBucket.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get management bucket name from management bucket object", "could not get management bucket name from management bucket object")
		return nil, d
	}
	gcpRet.ManagementBucket.Name = managementBucketName

	return gcpRet, diags
}

func generateAWSCMR(ctx context.Context, model models.Network, diags diag.Diagnostics) (*controlplanev1.Network_CustomerManagedResources_AWS, diag.Diagnostics) {
	awsRet := &controlplanev1.Network_CustomerManagedResources_AWS{
		ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{},
		DynamodbTable:    &controlplanev1.CustomerManagedDynamoDBTable{},
		Vpc:              &controlplanev1.CustomerManagedAWSVPC{},
		PrivateSubnets:   &controlplanev1.CustomerManagedAWSSubnets{},
	}
	// Get the AWS object from CustomerManagedResources
	var cmrObj types.Object
	if d := model.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); d.HasError() {
		d.AddError("failed to get CustomerManagedResources from network model", "could not get CustomerManagedResources from network model")
		return nil, d
	}

	aws, d := getObjectFromAttributes(ctx, "aws", cmrObj.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get AWS object from CustomerManagedResources", "could not get AWS object from CustomerManagedResources")
		return nil, d
	}
	// bucket
	mgmtBktArn, d := getArnFromAttributes("management_bucket", aws.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get management bucket from AWS object", "could not get management bucket from AWS object")
		return nil, d
	}
	awsRet.ManagementBucket.Arn = mgmtBktArn

	// dynamo
	dynamoArn, d := getArnFromAttributes("dynamodb_table", aws.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get dynamodb table from AWS object", "could not get dynamodb table from AWS object")
		return nil, d
	}
	awsRet.DynamodbTable.Arn = dynamoArn

	// vpc
	vpcArn, d := getArnFromAttributes("vpc", aws.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get vpc from AWS object", "could not get vpc from AWS object")
		return nil, d
	}
	awsRet.Vpc.Arn = vpcArn

	// private subnets
	privateSubnetsArns, d := getListFromAttributes("private_subnets", aws.Attributes(), diags)
	if d.HasError() {
		d.AddError("failed to get private subnets from AWS object", "could not get private subnets from AWS object")
		return nil, d
	}
	awsRet.PrivateSubnets.Arns = privateSubnetsArns

	return awsRet, diags
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

func getArnFromAttributes(key string, att map[string]attr.Value, diags diag.Diagnostics) (string, diag.Diagnostics) {
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

// Helper to get string value directly
func getStringValue(key string, attributes map[string]attr.Value, diags diag.Diagnostics) (string, diag.Diagnostics) {
	if val, ok := attributes[key].(types.String); ok {
		return val.ValueString(), diags
	}

	diags.AddError(fmt.Sprintf("%s not found", key), "string value is missing or malformed")
	return "", diags
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
