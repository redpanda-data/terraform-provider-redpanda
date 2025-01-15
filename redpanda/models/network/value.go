package network

import (
	"context"
	"fmt"

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

// ToObjectValue converts our custom resource value to an objectvalue
func (v CustomerManagedResourcesValue) ToObjectValue(_ context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
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
			"arn": v.AWS.ManagementBucket.ARN,
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
			"arn": v.AWS.DynamoDBTable.ARN,
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
			"arn": v.AWS.VPC.ARN,
		}
		vpcObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": basetypes.StringType{},
		}, vpcAttrs)
		diags.Append(d...)
		awsAttrs["vpc"] = vpcObj
	}

	// Private Subnets
	if v.AWS.PrivateSubnets != nil {
		psAttrs := map[string]attr.Value{
			"arns": v.AWS.PrivateSubnets.ARNs,
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
		psAttrs := map[string]attr.Value{
			"arns": v.AWS.PublicSubnets.ARNs,
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

// Type returns the underlying type value
func (CustomerManagedResourcesValue) Type(_ context.Context) attr.Type {
	return CustomerManagedResourcesType{}
}

// ToTerraformValue converts our custom value to a terraform value
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
		awsMap["private_subnets"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arns": tftypes.List{
						ElementType: tftypes.String,
					},
				},
			},
			map[string]tftypes.Value{
				"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, v.AWS.PrivateSubnets.ARNs),
			},
		)
	}

	if v.AWS.PublicSubnets != nil {
		awsMap["public_subnets"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arns": tftypes.List{
						ElementType: tftypes.String,
					},
				},
			},
			map[string]tftypes.Value{
				"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, v.AWS.PublicSubnets.ARNs),
			},
		)
	}

	return tftypes.NewValue(CustomerManagedResourcesType{}.TerraformType(ctx), awsMap), nil
}

// Equal returns true if the two values are equal
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

	// Add nil checks for each field
	if (v.AWS.ManagementBucket == nil) != (o.AWS.ManagementBucket == nil) {
		return false
	}
	if (v.AWS.DynamoDBTable == nil) != (o.AWS.DynamoDBTable == nil) {
		return false
	}
	if (v.AWS.VPC == nil) != (o.AWS.VPC == nil) {
		return false
	}
	if (v.AWS.PrivateSubnets == nil) != (o.AWS.PrivateSubnets == nil) {
		return false
	}
	if (v.AWS.PublicSubnets == nil) != (o.AWS.PublicSubnets == nil) {
		return false
	}

	// Only compare non-nil fields
	if v.AWS.ManagementBucket != nil && !v.AWS.ManagementBucket.ARN.Equal(o.AWS.ManagementBucket.ARN) {
		return false
	}
	if v.AWS.DynamoDBTable != nil && !v.AWS.DynamoDBTable.ARN.Equal(o.AWS.DynamoDBTable.ARN) {
		return false
	}
	if v.AWS.VPC != nil && !v.AWS.VPC.ARN.Equal(o.AWS.VPC.ARN) {
		return false
	}
	if v.AWS.PrivateSubnets != nil && !v.AWS.PrivateSubnets.ARNs.Equal(o.AWS.PrivateSubnets.ARNs) {
		return false
	}
	if v.AWS.PublicSubnets != nil && !v.AWS.PublicSubnets.ARNs.Equal(o.AWS.PublicSubnets.ARNs) {
		return false
	}

	return true
}
