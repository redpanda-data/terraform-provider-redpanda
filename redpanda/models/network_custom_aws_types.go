package models

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// CustomerManagedResourcesType represents the Type definition
type CustomerManagedResourcesType struct{}

func (t CustomerManagedResourcesType) ValueFromObject(ctx context.Context, obj basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

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

	awsAttrs := awsObj.(types.Object).Attributes()
	awsResources := &AWSResources{}

	// Extract Management Bucket
	if mb, ok := awsAttrs["management_bucket"].(types.Object); ok && !mb.IsNull() {
		mbAttrs := mb.Attributes()
		if arn, ok := mbAttrs["arn"].(types.String); ok {
			awsResources.ManagementBucket = &AWSBucket{
				ARN: arn.ValueString(),
			}
		}
	}

	// Extract DynamoDB Table
	if dt, ok := awsAttrs["dynamodb_table"].(types.Object); ok && !dt.IsNull() {
		dtAttrs := dt.Attributes()
		if arn, ok := dtAttrs["arn"].(types.String); ok {
			awsResources.DynamoDBTable = &AWSDynamoDBTable{
				ARN: arn.ValueString(),
			}
		}
	}

	// Extract VPC
	if vpc, ok := awsAttrs["vpc"].(types.Object); ok && !vpc.IsNull() {
		vpcAttrs := vpc.Attributes()
		if arn, ok := vpcAttrs["arn"].(types.String); ok {
			awsResources.VPC = &AWSVPC{
				ARN: arn.ValueString(),
			}
		}
	}

	// Extract Private Subnets
	if ps, ok := awsAttrs["private_subnets"].(types.Object); ok && !ps.IsNull() {
		psAttrs := ps.Attributes()
		if arns, ok := psAttrs["arns"].(types.List); ok {
			var arnsList []string
			for _, elem := range arns.Elements() {
				if arnStr, ok := elem.(types.String); ok {
					arnsList = append(arnsList, arnStr.ValueString())
				}
			}
			awsResources.PrivateSubnets = &AWSSubnets{
				ARNs: arnsList,
			}
		}
	}

	// Extract Public Subnets
	if ps, ok := awsAttrs["public_subnets"].(types.Object); ok && !ps.IsNull() {
		psAttrs := ps.Attributes()
		if arns, ok := psAttrs["arns"].(types.List); ok {
			var arnsList []string
			for _, elem := range arns.Elements() {
				if arnStr, ok := elem.(types.String); ok {
					arnsList = append(arnsList, arnStr.ValueString())
				}
			}
			awsResources.PublicSubnets = &AWSSubnets{
				ARNs: arnsList,
			}
		}
	}

	return CustomerManagedResourcesValue{
		AWS: awsResources,
	}, diags
}

func (t CustomerManagedResourcesType) TerraformType(ctx context.Context) tftypes.Type {
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
func (t CustomerManagedResourcesType) ValueType(ctx context.Context) attr.Value {
	return CustomerManagedResourcesValue{}
}

// Equal returns true if the other Type is also a CustomerManagedResourcesType
func (t CustomerManagedResourcesType) Equal(other attr.Type) bool {
	_, ok := other.(CustomerManagedResourcesType)
	return ok
}

// String returns a human-friendly version of the type
func (t CustomerManagedResourcesType) String() string {
	return "CustomerManagedResourcesType"
}

// ApplyTerraform5AttributePathStep applies path step navigation to the type
func (t CustomerManagedResourcesType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	attrName, ok := step.(tftypes.AttributeName)
	if !ok {
		return nil, fmt.Errorf("cannot apply step %T to CustomerManagedResourcesType", step)
	}

	// TODO support gcp and azure
	if attrName == "aws" {
		return &AWSResources{}, nil
	}

	return nil, fmt.Errorf("attribute %s not found in CustomerManagedResourcesType", attrName)
}

func (t CustomerManagedResourcesType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
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
	err := in.As(&attributes)
	if err != nil {
		return nil, err
	}

	awsData, ok := attributes["aws"]
	if !ok {
		return CustomerManagedResourcesValue{}, nil
	}

	var awsMap map[string]tftypes.Value
	err = awsData.As(&awsMap)
	if err != nil {
		return nil, err
	}

	aws := &AWSResources{}

	// Parse each AWS component
	if mbVal, ok := awsMap["management_bucket"]; ok {
		var mbMap map[string]tftypes.Value
		if err := mbVal.As(&mbMap); err == nil {
			var arn string
			if arnVal := mbMap["arn"]; !arnVal.IsNull() {
				arnVal.As(&arn)
				aws.ManagementBucket = &AWSBucket{ARN: arn}
			}
		}
	}

	if dtVal, ok := awsMap["dynamodb_table"]; ok {
		var dtMap map[string]tftypes.Value
		if err := dtVal.As(&dtMap); err == nil {
			var arn string
			if arnVal := dtMap["arn"]; !arnVal.IsNull() {
				arnVal.As(&arn)
				aws.DynamoDBTable = &AWSDynamoDBTable{ARN: arn}
			}
		}
	}

	if vpcVal, ok := awsMap["vpc"]; ok {
		var vpcMap map[string]tftypes.Value
		if err := vpcVal.As(&vpcMap); err == nil {
			var arn string
			if arnVal := vpcMap["arn"]; !arnVal.IsNull() {
				arnVal.As(&arn)
				aws.VPC = &AWSVPC{ARN: arn}
			}
		}
	}

	if psVal, ok := awsMap["private_subnets"]; ok {
		var psMap map[string]tftypes.Value
		if err := psVal.As(&psMap); err == nil {
			var arns []string
			if arnsVal := psMap["arns"]; !arnsVal.IsNull() {
				arnsVal.As(&arns)
				aws.PrivateSubnets = &AWSSubnets{ARNs: arns}
			}
		}
	}

	if psVal, ok := awsMap["public_subnets"]; ok {
		var psMap map[string]tftypes.Value
		if err := psVal.As(&psMap); err == nil {
			var arns []string
			if arnsVal := psMap["arns"]; !arnsVal.IsNull() {
				arnsVal.As(&arns)
				aws.PublicSubnets = &AWSSubnets{ARNs: arns}
			}
		}
	}

	return CustomerManagedResourcesValue{
		AWS: aws,
	}, nil
}

// AttrTypes returns the attribute types for CustomerManagedResources
func (t CustomerManagedResourcesType) AttrTypes() map[string]attr.Type {
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
