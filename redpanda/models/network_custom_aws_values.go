package models

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// CustomerManagedResourcesValue represents the concrete value with data
type CustomerManagedResourcesValue struct {
	AWS       *AWSResources
	isNull    bool
	isUnknown bool
}

func (v CustomerManagedResourcesValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	if v.IsNull() {
		return basetypes.NewObjectNull(CustomerManagedResourcesType{}.AttrTypes()), diags
	}

	if v.IsUnknown() {
		return basetypes.NewObjectUnknown(CustomerManagedResourcesType{}.AttrTypes()), diags
	}

	if v.AWS == nil {
		return basetypes.NewObjectNull(CustomerManagedResourcesType{}.AttrTypes()), diags
	}

	// Build AWS attributes
	awsAttrs := make(map[string]attr.Value)

	// Management Bucket
	if v.AWS.ManagementBucket != nil {
		mbAttrs := map[string]attr.Value{
			"arn": basetypes.NewStringValue(v.AWS.ManagementBucket.ARN),
		}
		mbObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": basetypes.StringType{},
		}, mbAttrs)
		diags.Append(d...)
		awsAttrs["management_bucket"] = mbObj
	}

	// DynamoDB Table
	if v.AWS.DynamoDBTable != nil {
		dtAttrs := map[string]attr.Value{
			"arn": basetypes.NewStringValue(v.AWS.DynamoDBTable.ARN),
		}
		dtObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": basetypes.StringType{},
		}, dtAttrs)
		diags.Append(d...)
		awsAttrs["dynamodb_table"] = dtObj
	}

	// VPC
	if v.AWS.VPC != nil {
		vpcAttrs := map[string]attr.Value{
			"arn": basetypes.NewStringValue(v.AWS.VPC.ARN),
		}
		vpcObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": basetypes.StringType{},
		}, vpcAttrs)
		diags.Append(d...)
		awsAttrs["vpc"] = vpcObj
	}

	// Private Subnets
	if v.AWS.PrivateSubnets != nil {
		arnValues := make([]attr.Value, 0, len(v.AWS.PrivateSubnets.ARNs))
		for _, arn := range v.AWS.PrivateSubnets.ARNs {
			arnValues = append(arnValues, basetypes.NewStringValue(arn))
		}
		arnsList, d := types.ListValue(basetypes.StringType{}, arnValues)
		diags.Append(d...)

		psAttrs := map[string]attr.Value{
			"arns": arnsList,
		}
		psObj, d := types.ObjectValue(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: basetypes.StringType{},
			},
		}, psAttrs)
		diags.Append(d...)
		awsAttrs["private_subnets"] = psObj
	}

	// Public Subnets
	if v.AWS.PublicSubnets != nil {
		arnValues := make([]attr.Value, 0, len(v.AWS.PublicSubnets.ARNs))
		for _, arn := range v.AWS.PublicSubnets.ARNs {
			arnValues = append(arnValues, basetypes.NewStringValue(arn))
		}
		arnsList, d := types.ListValue(basetypes.StringType{}, arnValues)
		diags.Append(d...)

		psAttrs := map[string]attr.Value{
			"arns": arnsList,
		}
		psObj, d := types.ObjectValue(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: basetypes.StringType{},
			},
		}, psAttrs)
		diags.Append(d...)
		awsAttrs["public_subnets"] = psObj
	}

	// Create AWS object
	awsObj, d := types.ObjectValue(CustomerManagedResourcesType{}.AttrTypes()["aws"].(basetypes.ObjectType).AttrTypes, awsAttrs)
	diags.Append(d...)

	// Create final object
	obj, d := types.ObjectValue(CustomerManagedResourcesType{}.AttrTypes(), map[string]attr.Value{
		"aws": awsObj,
	})
	diags.Append(d...)

	return obj, diags
}

// IsNull returns true if the Value is null
func (v CustomerManagedResourcesValue) IsNull() bool {
	return v.isNull
}

// IsUnknown returns true if the Value is unknown
func (v CustomerManagedResourcesValue) IsUnknown() bool {
	return v.isUnknown
}

// String returns a summary representation
func (v CustomerManagedResourcesValue) String() string {
	if v.IsUnknown() {
		return "<unknown>"
	}
	if v.IsNull() {
		return "<null>"
	}
	if v.AWS == nil {
		return "CustomerManagedResources{}"
	}

	return fmt.Sprintf("CustomerManagedResources{AWS: %+v}", v.AWS)
}

func (v CustomerManagedResourcesValue) Type(ctx context.Context) attr.Type {
	return CustomerManagedResourcesType{}
}

func (v CustomerManagedResourcesValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	if v.AWS == nil {
		return tftypes.NewValue(CustomerManagedResourcesType{}.TerraformType(ctx), nil), nil
	}

	awsMap := make(map[string]tftypes.Value)

	// Convert each component back to Terraform values
	if v.AWS.ManagementBucket != nil {
		awsMap["management_bucket"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, v.AWS.ManagementBucket.ARN),
			},
		)
	}

	if v.AWS.DynamoDBTable != nil {
		awsMap["dynamodb_table"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, v.AWS.DynamoDBTable.ARN),
			},
		)
	}

	if v.AWS.VPC != nil {
		awsMap["vpc"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, v.AWS.VPC.ARN),
			},
		)
	}

	if v.AWS.PrivateSubnets != nil {
		arns := make([]tftypes.Value, 0, len(v.AWS.PrivateSubnets.ARNs))
		for _, arn := range v.AWS.PrivateSubnets.ARNs {
			arns = append(arns, tftypes.NewValue(tftypes.String, arn))
		}
		awsMap["private_subnets"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arns": tftypes.List{
						ElementType: tftypes.String,
					},
				},
			},
			map[string]tftypes.Value{
				"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, arns),
			},
		)
	}

	if v.AWS.PublicSubnets != nil {
		arns := make([]tftypes.Value, 0, len(v.AWS.PublicSubnets.ARNs))
		for _, arn := range v.AWS.PublicSubnets.ARNs {
			arns = append(arns, tftypes.NewValue(tftypes.String, arn))
		}
		awsMap["public_subnets"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arns": tftypes.List{
						ElementType: tftypes.String,
					},
				},
			},
			map[string]tftypes.Value{
				"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, arns),
			},
		)
	}

	return tftypes.NewValue(CustomerManagedResourcesType{}.TerraformType(ctx), awsMap), nil
}

func (v CustomerManagedResourcesValue) Equal(other attr.Value) bool {
	o, ok := other.(CustomerManagedResourcesValue)
	if !ok {
		return false
	}

	if v.AWS == nil && o.AWS == nil {
		return true
	}
	if v.AWS == nil || o.AWS == nil {
		return false
	}

	// Compare all AWS components
	return v.AWS.ManagementBucket.ARN == o.AWS.ManagementBucket.ARN &&
		v.AWS.DynamoDBTable.ARN == o.AWS.DynamoDBTable.ARN &&
		v.AWS.VPC.ARN == o.AWS.VPC.ARN &&
		reflect.DeepEqual(v.AWS.PrivateSubnets.ARNs, o.AWS.PrivateSubnets.ARNs) &&
		reflect.DeepEqual(v.AWS.PublicSubnets.ARNs, o.AWS.PublicSubnets.ARNs)
}
