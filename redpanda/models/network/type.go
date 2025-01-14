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

// CustomerManagedResourcesType represents the Type definition
type CustomerManagedResourcesType struct{}

var (
	_ basetypes.ObjectValuable = CustomerManagedResourcesValue{}
	_ basetypes.ObjectTypable  = CustomerManagedResourcesType{}
)

// ValueFromObject returns the custom struct populated with available data
func (CustomerManagedResourcesType) ValueFromObject(_ context.Context, obj basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	// Handle null/unknown cases
	if obj.IsNull() {
		return CustomerManagedResourcesValue{
			isNull: true,
		}, diags
	}
	if obj.IsUnknown() {
		return CustomerManagedResourcesValue{
			isUnknown: true,
		}, diags
	}

	// Get the attributes map from the object
	attributes := obj.Attributes()

	// Extract AWS data if it exists
	awsObj, ok := attributes["aws"]
	if !ok {
		return CustomerManagedResourcesValue{}, diags
	}

	awsValue, ok := awsObj.(types.Object)
	if !ok {
		diags.AddError(
			"Invalid AWS Value Type",
			fmt.Sprintf("Expected types.Object for AWS configuration, got %T", awsObj),
		)
		return CustomerManagedResourcesValue{}, diags
	}

	if awsValue.IsNull() || awsValue.IsUnknown() {
		return CustomerManagedResourcesValue{
			AWS: &AWSResources{},
		}, diags
	}

	awsAttrs := awsValue.Attributes()
	awsResources := &AWSResources{}

	// Extract Management Bucket
	if mb, ok := awsAttrs["management_bucket"].(types.Object); ok && !mb.IsNull() {
		mbAttrs := mb.Attributes()
		if arn, ok := mbAttrs["arn"].(types.String); ok {
			awsResources.ManagementBucket = &AWSBucket{
				ARN: arn,
			}
		}
	}

	// Extract DynamoDB Table
	if dt, ok := awsAttrs["dynamodb_table"].(types.Object); ok && !dt.IsNull() {
		dtAttrs := dt.Attributes()
		if arn, ok := dtAttrs["arn"].(types.String); ok {
			awsResources.DynamoDBTable = &AWSDynamoDBTable{
				ARN: arn,
			}
		}
	}

	// Extract VPC
	if vpc, ok := awsAttrs["vpc"].(types.Object); ok && !vpc.IsNull() {
		vpcAttrs := vpc.Attributes()
		if arn, ok := vpcAttrs["arn"].(types.String); ok {
			awsResources.VPC = &AWSVPC{
				ARN: arn,
			}
		}
	}

	// Extract Private Subnets
	if ps, ok := awsAttrs["private_subnets"].(types.Object); ok && !ps.IsNull() {
		psAttrs := ps.Attributes()
		if arns, ok := psAttrs["arns"].(types.List); ok {
			awsResources.PrivateSubnets = &AWSSubnets{
				ARNs: arns,
			}
		}
	}

	// Extract Public Subnets
	if ps, ok := awsAttrs["public_subnets"].(types.Object); ok && !ps.IsNull() {
		psAttrs := ps.Attributes()
		if arns, ok := psAttrs["arns"].(types.List); ok {
			awsResources.PublicSubnets = &AWSSubnets{
				ARNs: arns,
			}
		}
	}

	return CustomerManagedResourcesValue{
		AWS: awsResources,
	}, diags
}

// TerraformType returns the Terraform type equivalent to that in the schema
func (CustomerManagedResourcesType) TerraformType(_ context.Context) tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"aws": tftypes.Object{
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
							"arns": tftypes.List{
								ElementType: tftypes.String,
							},
						},
					},
					"public_subnets": tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"arns": tftypes.List{
								ElementType: tftypes.String,
							},
						},
					},
				},
			},
		},
	}
}

// ValueType returns a new empty value of this type
func (CustomerManagedResourcesType) ValueType(_ context.Context) attr.Value {
	return CustomerManagedResourcesValue{}
}

// Equal returns true if the candidate is also a CustomerManagedResourcesType
func (CustomerManagedResourcesType) Equal(other attr.Type) bool {
	_, ok := other.(CustomerManagedResourcesType)
	return ok
}

// String returns a human-friendly version of the type
func (CustomerManagedResourcesType) String() string {
	return "CustomerManagedResourcesType"
}

// ApplyTerraform5AttributePathStep implements the walk through nested attributes functionality
// for customer managed resources. This enables proper type traversal of our nested type structure.
func (CustomerManagedResourcesType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (any, error) {
	attrName, ok := step.(tftypes.AttributeName)
	if !ok {
		return nil, fmt.Errorf("cannot apply step %T to CustomerManagedResourcesType", step)
	}

	switch attrName {
	case "aws":
		return CustomerManagedResourcesType{}.AttrTypes()["aws"], nil
	default:
		return nil, fmt.Errorf("attribute %s not found in CustomerManagedResourcesType", attrName)
	}
}

// AttrTypes returns the attribute types for CustomerManagedResources
func (CustomerManagedResourcesType) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"aws": basetypes.ObjectType{
			AttrTypes: map[string]attr.Type{
				"management_bucket": basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"arn": basetypes.StringType{},
					},
				},
				"dynamodb_table": basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"arn": basetypes.StringType{},
					},
				},
				"vpc": basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"arn": basetypes.StringType{},
					},
				},
				"private_subnets": basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"arns": basetypes.ListType{
							ElemType: basetypes.StringType{},
						},
					},
				},
				"public_subnets": basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"arns": basetypes.ListType{
							ElemType: basetypes.StringType{},
						},
					},
				},
			},
		},
	}
}

// ValueFromTerraform handles conversion from Terraform values
func (CustomerManagedResourcesType) ValueFromTerraform(_ context.Context, in tftypes.Value) (attr.Value, error) {
	if !in.IsKnown() {
		return CustomerManagedResourcesValue{
			isUnknown: true,
		}, nil
	}
	if in.IsNull() {
		return CustomerManagedResourcesValue{
			isNull: true,
		}, nil
	}

	var attributes map[string]tftypes.Value
	if err := in.As(&attributes); err != nil {
		return nil, err
	}

	awsData, ok := attributes["aws"]
	if !ok {
		return CustomerManagedResourcesValue{}, nil
	}

	var awsMap map[string]tftypes.Value
	if err := awsData.As(&awsMap); err != nil {
		return nil, err
	}

	aws := &AWSResources{}

	// Parse Management Bucket
	if mbVal, ok := awsMap["management_bucket"]; ok && !mbVal.IsNull() {
		var mbMap map[string]tftypes.Value
		if err := mbVal.As(&mbMap); err == nil {
			var arnString string
			if arnVal := mbMap["arn"]; !arnVal.IsNull() {
				if err := arnVal.As(&arnString); err == nil {
					aws.ManagementBucket = &AWSBucket{
						ARN: types.StringValue(arnString),
					}
				}
			}
		}
	}

	// Parse DynamoDB Table
	if dtVal, ok := awsMap["dynamodb_table"]; ok && !dtVal.IsNull() {
		var dtMap map[string]tftypes.Value
		if err := dtVal.As(&dtMap); err == nil {
			var arnString string
			if arnVal := dtMap["arn"]; !arnVal.IsNull() {
				if err := arnVal.As(&arnString); err == nil {
					aws.DynamoDBTable = &AWSDynamoDBTable{
						ARN: types.StringValue(arnString),
					}
				}
			}
		}
	}

	// Parse VPC
	if vpcVal, ok := awsMap["vpc"]; ok && !vpcVal.IsNull() {
		var vpcMap map[string]tftypes.Value
		if err := vpcVal.As(&vpcMap); err == nil {
			var arnString string
			if arnVal := vpcMap["arn"]; !arnVal.IsNull() {
				if err := arnVal.As(&arnString); err == nil {
					aws.VPC = &AWSVPC{
						ARN: types.StringValue(arnString),
					}
				}
			}
		}
	}

	// Parse Private Subnets
	if psVal, ok := awsMap["private_subnets"]; ok && !psVal.IsNull() {
		var psMap map[string]tftypes.Value
		if err := psVal.As(&psMap); err == nil {
			if arnsVal := psMap["arns"]; !arnsVal.IsNull() {
				var arnStrings []string
				if err := arnsVal.As(&arnStrings); err == nil {
					aws.PrivateSubnets = &AWSSubnets{
						ARNs: utils.StringSliceToTypeList(arnStrings),
					}
				}
			}
		}
	}

	// Parse Public Subnets
	if psVal, ok := awsMap["public_subnets"]; ok && !psVal.IsNull() {
		var psMap map[string]tftypes.Value
		if err := psVal.As(&psMap); err == nil {
			if arnsVal := psMap["arns"]; !arnsVal.IsNull() {
				var arnStrings []string
				if err := arnsVal.As(&arnStrings); err == nil {
					aws.PublicSubnets = &AWSSubnets{
						ARNs: utils.StringSliceToTypeList(arnStrings),
					}
				}
			}
		}
	}

	return CustomerManagedResourcesValue{
		AWS: aws,
	}, nil
}
