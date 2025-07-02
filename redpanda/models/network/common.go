package network

import (
	"context"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pkg/errors"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func generateNetworkGCPCMR(ctx context.Context, cmrObj types.Object) (*controlplanev1.Network_CustomerManagedResources_GCP, error) {
	gcpRet := &controlplanev1.Network_CustomerManagedResources_GCP{
		NetworkName:      "",
		NetworkProjectId: "",
		ManagementBucket: &controlplanev1.CustomerManagedGoogleCloudStorageBucket{
			Name: "",
		},
	}

	gcp, err := utils.GetObjectFromAttributes(ctx, "gcp", cmrObj.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get GCP object from CustomerManagedResources")
	}

	networkName, err := utils.GetStringFromAttributes("network_name", gcp.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get network name from GCP object")
	}
	gcpRet.NetworkName = networkName

	networkProjectID, err := utils.GetStringFromAttributes("network_project_id", gcp.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get network project id from GCP object")
	}
	gcpRet.NetworkProjectId = networkProjectID

	managementBucket, err := utils.GetObjectFromAttributes(ctx, "management_bucket", gcp.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get management bucket object from GCP object")
	}
	managementBucketName, err := utils.GetStringFromAttributes("name", managementBucket.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get management bucket name from management bucket object")
	}
	gcpRet.ManagementBucket.Name = managementBucketName

	return gcpRet, nil
}

func generateNetworkAWSCMR(ctx context.Context, cmrObj types.Object) (*controlplanev1.Network_CustomerManagedResources_AWS, error) {
	awsRet := &controlplanev1.Network_CustomerManagedResources_AWS{
		ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{},
		DynamodbTable:    &controlplanev1.CustomerManagedDynamoDBTable{},
		Vpc:              &controlplanev1.CustomerManagedAWSVPC{},
		PrivateSubnets:   &controlplanev1.CustomerManagedAWSSubnets{},
	}

	aws, err := utils.GetObjectFromAttributes(ctx, "aws", cmrObj.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get AWS object from CustomerManagedResources")
	}
	mgmtBktArn, err := getArnFromAttributes(ctx, "management_bucket", aws.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get management bucket from AWS object")
	}
	awsRet.ManagementBucket.Arn = mgmtBktArn

	dynamoArn, err := getArnFromAttributes(ctx, "dynamodb_table", aws.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get dynamodb table from AWS object")
	}
	awsRet.DynamodbTable.Arn = dynamoArn

	vpcArn, err := getArnFromAttributes(ctx, "vpc", aws.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get vpc from AWS object")
	}
	awsRet.Vpc.Arn = vpcArn

	privateSubnetsArns, err := utils.GetARNListFromAttributes("private_subnets", aws.Attributes())
	if err != nil {
		return nil, errors.Wrap(err, "could not get private subnets from AWS object")
	}
	awsRet.PrivateSubnets.Arns = privateSubnetsArns

	return awsRet, nil
}

func getArnFromAttributes(ctx context.Context, key string, att map[string]attr.Value) (string, error) {
	a, err := utils.GetObjectFromAttributes(ctx, key, att)
	if err != nil {
		return "", err
	}
	return utils.GetStringFromAttributes("arn", a.Attributes())
}

func generateModelGCPCMR(gcpData *controlplanev1.Network_CustomerManagedResources_GCP) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	gcpVal := getGcpNullAttributes()
	if gcpData.GetNetworkName() != "" {
		gcpVal["network_name"] = types.StringValue(gcpData.GetNetworkName())
	}

	if gcpData.GetNetworkProjectId() != "" {
		gcpVal["network_project_id"] = types.StringValue(gcpData.GetNetworkProjectId())
	}

	if gcpData.HasManagementBucket() {
		obj, d := types.ObjectValue(map[string]attr.Type{
			"name": types.StringType,
		}, map[string]attr.Value{
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
	gcpObj, d := types.ObjectValue(getGcpType(), gcpVal)
	if d.HasError() {
		diags.AddError("failed to generate GCP object", "could not create GCP object")
		diags.Append(d...)
		return getGcpNull(), diags
	}
	return gcpObj, diags
}

func generateModelAWSCMR(_ context.Context, awsData *controlplanev1.Network_CustomerManagedResources_AWS) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	awsVal := getAwsNullAttributes()

	if awsData.HasManagementBucket() {
		obj, d := types.ObjectValue(map[string]attr.Type{
			"arn": types.StringType,
		}, map[string]attr.Value{
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
		obj, d := types.ObjectValue(map[string]attr.Type{
			"arn": types.StringType,
		}, map[string]attr.Value{
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
		obj, d := types.ObjectValue(map[string]attr.Type{
			"arn": types.StringType,
		}, map[string]attr.Value{
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
		obj, d := types.ObjectValue(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: types.StringType,
			},
		}, map[string]attr.Value{
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
	awsObj, d := types.ObjectValue(getAwsType(), awsVal)
	if d.HasError() {
		diags.AddError("failed to generate AWS object", "could not create AWS object")
		diags.Append(d...)
		return getAwsNull(), diags
	}

	return awsObj, diags
}
