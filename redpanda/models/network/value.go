package network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// CustomerManagedResourcesValue represents the concrete value with data
type CustomerManagedResourcesValue struct {
	AWS       AWSResources
	isNull    bool
	isUnknown bool
}

func (v CustomerManagedResourcesValue) ToObjectValue(_ context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	if v.IsNull() {
		return basetypes.NewObjectNull(CustomerManagedResourcesType{}.AttrTypes()), diags
	}

	if v.IsUnknown() {
		return basetypes.NewObjectUnknown(CustomerManagedResourcesType{}.AttrTypes()), diags
	}

	// Build AWS attributes - initialize with empty map instead of nil
	awsAttrs := map[string]attr.Value{
		"management_bucket": types.ObjectNull(map[string]attr.Type{
			"arn": types.StringType,
		}),
		"dynamodb_table": types.ObjectNull(map[string]attr.Type{
			"arn": types.StringType,
		}),
		"vpc": types.ObjectNull(map[string]attr.Type{
			"arn": types.StringType,
		}),
		"private_subnets": types.ObjectNull(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: types.StringType,
			},
		}),
		"public_subnets": types.ObjectNull(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: types.StringType,
			},
		}),
	}

	// Only override with non-empty values
	if !utils.IsStructEmpty(v.AWS.ManagementBucket) {
		mbAttrs := map[string]attr.Value{
			"arn": v.AWS.ManagementBucket.ARN,
		}
		mbObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": types.StringType,
		}, mbAttrs)
		diags.Append(d...)
		awsAttrs["management_bucket"] = mbObj
	}

	if !utils.IsStructEmpty(v.AWS.DynamoDBTable) {
		dtAttrs := map[string]attr.Value{
			"arn": v.AWS.DynamoDBTable.ARN,
		}
		dtObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": types.StringType,
		}, dtAttrs)
		diags.Append(d...)
		awsAttrs["dynamodb_table"] = dtObj
	}

	if !utils.IsStructEmpty(v.AWS.VPC) {
		vpcAttrs := map[string]attr.Value{
			"arn": v.AWS.VPC.ARN,
		}
		vpcObj, d := types.ObjectValue(map[string]attr.Type{
			"arn": types.StringType,
		}, vpcAttrs)
		diags.Append(d...)
		awsAttrs["vpc"] = vpcObj
	}

	if !utils.IsStructEmpty(v.AWS.PrivateSubnets) {
		psAttrs := map[string]attr.Value{
			"arns": v.AWS.PrivateSubnets.ARNs,
		}
		psObj, d := types.ObjectValue(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: types.StringType,
			},
		}, psAttrs)
		diags.Append(d...)
		awsAttrs["private_subnets"] = psObj
	}

	if !utils.IsStructEmpty(v.AWS.PublicSubnets) {
		psAttrs := map[string]attr.Value{
			"arns": v.AWS.PublicSubnets.ARNs,
		}
		psObj, d := types.ObjectValue(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: types.StringType,
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
	if utils.IsStructEmpty(v.AWS) {
		return "CustomerManagedResources{}"
	}

	return fmt.Sprintf("CustomerManagedResources{AWS: %+v}", v.AWS)
}

// Type returns the underlying type value
func (CustomerManagedResourcesValue) Type(_ context.Context) attr.Type {
	return CustomerManagedResourcesType{}
}

func (v CustomerManagedResourcesValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	if v.isUnknown {
		return tftypes.NewValue(CustomerManagedResourcesType{}.TerraformType(ctx), tftypes.UnknownValue), nil
	}
	if v.isNull {
		return tftypes.NewValue(CustomerManagedResourcesType{}.TerraformType(ctx), nil), nil
	}

	// Initialize all required fields with null values
	awsFields := map[string]tftypes.Value{
		"management_bucket": tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, nil),
			},
		),
		"dynamodb_table": tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, nil),
			},
		),
		"vpc": tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, nil),
			},
		),
		"private_subnets": tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arns": tftypes.List{ElementType: tftypes.String},
				},
			},
			map[string]tftypes.Value{
				"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
		),
		"public_subnets": tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arns": tftypes.List{ElementType: tftypes.String},
				},
			},
			map[string]tftypes.Value{
				"arns": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
		),
	}

	if !utils.IsStructEmpty(v.AWS.ManagementBucket) {
		mbValue := map[string]tftypes.Value{
			"arn": tftypes.NewValue(tftypes.String, nil),
		}
		if v.AWS.ManagementBucket.ARN.IsUnknown() {
			mbValue["arn"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		} else if !v.AWS.ManagementBucket.ARN.IsNull() {
			mbValue["arn"] = tftypes.NewValue(tftypes.String, v.AWS.ManagementBucket.ARN.ValueString())
		}
		awsFields["management_bucket"] = tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"arn": tftypes.String,
				},
			},
			mbValue,
		)
	}

	// Similar blocks for other fields...
	// Add the rest of your existing field handling code here

	// When returning the value, all required fields will be present
	return tftypes.NewValue(
		CustomerManagedResourcesType{}.TerraformType(ctx),
		map[string]tftypes.Value{
			"aws": tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"management_bucket": tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"arn": tftypes.String,
							},
						},
						"dynamodb_table": tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"arn": tftypes.String,
							},
						},
						"vpc": tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"arn": tftypes.String,
							},
						},
						"private_subnets": tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"arns": tftypes.List{ElementType: tftypes.String},
							},
						},
						"public_subnets": tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"arns": tftypes.List{ElementType: tftypes.String},
							},
						},
					},
				},
				awsFields,
			),
		},
	), nil
}

func (v CustomerManagedResourcesValue) Equal(other attr.Value) bool {
	o, ok := other.(CustomerManagedResourcesValue)
	if !ok {
		return false
	}

	if utils.IsStructEmpty(v.AWS) && utils.IsStructEmpty(o.AWS) {
		return true
	}
	if utils.IsStructEmpty(v.AWS) || utils.IsStructEmpty(o.AWS) {
		return false
	}

	// Compare each field - no need for nil checks since utils.IsStructEmpty will handle those
	if !utils.IsStructEmpty(v.AWS.ManagementBucket) || !utils.IsStructEmpty(o.AWS.ManagementBucket) {
		if utils.IsStructEmpty(v.AWS.ManagementBucket) || utils.IsStructEmpty(o.AWS.ManagementBucket) {
			return false
		}
		if !v.AWS.ManagementBucket.ARN.Equal(o.AWS.ManagementBucket.ARN) {
			return false
		}
	}

	if !utils.IsStructEmpty(v.AWS.DynamoDBTable) || !utils.IsStructEmpty(o.AWS.DynamoDBTable) {
		if utils.IsStructEmpty(v.AWS.DynamoDBTable) || utils.IsStructEmpty(o.AWS.DynamoDBTable) {
			return false
		}
		if !v.AWS.DynamoDBTable.ARN.Equal(o.AWS.DynamoDBTable.ARN) {
			return false
		}
	}

	if !utils.IsStructEmpty(v.AWS.VPC) || !utils.IsStructEmpty(o.AWS.VPC) {
		if utils.IsStructEmpty(v.AWS.VPC) || utils.IsStructEmpty(o.AWS.VPC) {
			return false
		}
		if !v.AWS.VPC.ARN.Equal(o.AWS.VPC.ARN) {
			return false
		}
	}

	if !utils.IsStructEmpty(v.AWS.PrivateSubnets) || !utils.IsStructEmpty(o.AWS.PrivateSubnets) {
		if utils.IsStructEmpty(v.AWS.PrivateSubnets) || utils.IsStructEmpty(o.AWS.PrivateSubnets) {
			return false
		}
		if !v.AWS.PrivateSubnets.ARNs.Equal(o.AWS.PrivateSubnets.ARNs) {
			return false
		}
	}

	if !utils.IsStructEmpty(v.AWS.PublicSubnets) || !utils.IsStructEmpty(o.AWS.PublicSubnets) {
		if utils.IsStructEmpty(v.AWS.PublicSubnets) || utils.IsStructEmpty(o.AWS.PublicSubnets) {
			return false
		}
		if !v.AWS.PublicSubnets.ARNs.Equal(o.AWS.PublicSubnets.ARNs) {
			return false
		}
	}

	return true
}
